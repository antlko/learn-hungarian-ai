package components

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The offline `tts` runtime (ADR-080). A bundle is a directory tree shaped like
// services/tts/vendor — MANIFEST.sha256, requirements.lock and one directory per
// platform holding a python-build-standalone tarball plus a wheelhouse — that
// comes from the operator's --tts-runtime-dir or from the binary itself
// (package ttsbundle, build tag `ttsbundle`). Nothing here opens a socket.

const (
	manifestName = "MANIFEST.sha256"
	lockName     = "requirements.lock"
	stampName    = "runtime.stamp"
)

// Hard caps on what an archive may unpack to (the real interpreter is about
// 120 MB over ~10k entries). Variables only so a test can lower them.
var (
	maxUnpackBytes   int64 = 2 << 30
	maxUnpackEntries       = 200_000
)

// Keys and steps of the bundled runtime.
const (
	StepVerify = "component.step.verify"
	StepUnpack = "component.step.unpack"
	StepVoices = "component.step.voices"

	// KeyNoTtsRuntime: this platform has no bundled runtime, or this build and
	// the operator's flag provide none. Device speech remains the floor.
	KeyNoTtsRuntime = "errors.componentNoTtsRuntime"
	// KeyBundleCorrupt: a bundled file does not match MANIFEST.sha256 (or the
	// archive is malformed). Nothing is installed.
	KeyBundleCorrupt = "errors.componentBundleCorrupt"
)

// platformDir is the vendor sub-directory of a GOOS/GOARCH, "" when unsupported.
func platformDir(goos, goarch string) string {
	switch goos + "-" + goarch {
	case "darwin-arm64", "linux-amd64", "linux-arm64":
		return goos + "-" + goarch
	}
	return ""
}

// bundle is a located runtime: its tree and the platform directory in it.
type bundle struct {
	fsys     fs.FS
	platform string
}

// locateBundle finds the runtime: the operator's directory first, then the one
// embedded in the binary. The second result is "" or KeyNoTtsRuntime.
func (m *Manager) locateBundle() (bundle, string) {
	platform := platformDir(m.opt.GOOS, m.opt.GOARCH)
	if platform == "" {
		return bundle{}, KeyNoTtsRuntime
	}
	var candidates []fs.FS
	if d := m.opt.TTSRuntimeDir; d != "" {
		candidates = append(candidates, os.DirFS(d))
	}
	if m.opt.EmbeddedBundle != nil {
		if f, ok := m.opt.EmbeddedBundle(); ok {
			candidates = append(candidates, f)
		}
	}
	for _, f := range candidates {
		if st, err := fs.Stat(f, platform); err != nil || !st.IsDir() {
			continue
		}
		if st, err := fs.Stat(f, manifestName); err != nil || !st.Mode().IsRegular() {
			continue
		}
		return bundle{fsys: f, platform: platform}, ""
	}
	return bundle{}, KeyNoTtsRuntime
}

func corrupt(format string, a ...any) *Error {
	return &Error{Key: KeyBundleCorrupt, Detail: fmt.Sprintf(format, a...)}
}

func hexSum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// stamp is the content of runtime.stamp: it names the exact MANIFEST, lock and
// platform the unpacked environment was built from.
func (b bundle) stamp() (string, error) {
	man, lock, err := b.readMeta()
	if err != nil {
		return "", err
	}
	return b.stampOf(man, lock), nil
}

// readMeta reads MANIFEST and lock once; verifyCopy and the stamp both use
// these bytes, so the stamp names exactly what was verified.
func (b bundle) readMeta() (man, lock []byte, err error) {
	if man, err = fs.ReadFile(b.fsys, manifestName); err != nil {
		return nil, nil, err
	}
	if lock, err = fs.ReadFile(b.fsys, lockName); err != nil {
		return nil, nil, err
	}
	return man, lock, nil
}

func (b bundle) stampOf(man, lock []byte) string {
	return "manifest=" + hexSum(man) + "\nlock=" + hexSum(lock) + "\nplatform=" + b.platform + "\n"
}

// runtimeReady is the cheap "already unpacked" check: the stamp matches the
// current bundle and the interpreter is there. Two small reads and a stat.
func (m *Manager) runtimeReady(id string) bool {
	b, key := m.locateBundle()
	if key != "" {
		return false
	}
	want, err := b.stamp()
	if err != nil {
		return false
	}
	have, err := os.ReadFile(filepath.Join(m.final(id), stampName))
	if err != nil || string(have) != want {
		return false
	}
	st, err := os.Stat(m.python3(id))
	return err == nil && st.Mode().IsRegular()
}

// python3 is the bundled interpreter of a component.
func (m *Manager) python3(id string) string {
	return filepath.Join(m.final(id), "python", "bin", "python3")
}

type manifestEntry struct {
	sum  string
	size int64
	path string
}

var reHexSum = regexp.MustCompile(`^[0-9a-f]{64}$`)

// parseManifest reads `sha256  size  path` lines (blank lines and # comments
// skipped). A path must be a clean relative slash path.
func parseManifest(b []byte) ([]manifestEntry, error) {
	var out []manifestEntry
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 || !reHexSum.MatchString(f[0]) {
			return nil, corrupt("MANIFEST line %d is malformed", i+1)
		}
		size, err := strconv.ParseInt(f[1], 10, 64)
		if err != nil || size < 0 {
			return nil, corrupt("MANIFEST line %d has a bad size", i+1)
		}
		if p := f[2]; p != path.Clean(p) || path.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../") {
			return nil, corrupt("MANIFEST line %d has an unsafe path", i+1)
		}
		out = append(out, manifestEntry{sum: f[0], size: size, path: f[2]})
	}
	return out, nil
}

var reLockHash = regexp.MustCompile(`--hash=sha256:([0-9a-f]{64})`)

// verifyCopy checks every file of THIS platform's directory against the
// MANIFEST, that the lock lists exactly the wheels' hashes, and copies the
// verified bytes into dst (hash computed while copying, so what pip and tar
// read later is what was verified). Any mismatch, missing or unexpected file is
// KeyBundleCorrupt. Returns the tarball's path in dst.
func verifyCopy(ctx context.Context, b bundle, man, lock []byte, dst string, progress func(float64)) (string, error) {
	entries, err := parseManifest(man)
	if err != nil {
		return "", err
	}
	prefix := b.platform + "/"
	var mine []manifestEntry
	for _, e := range entries {
		if strings.HasPrefix(e.path, prefix) {
			mine = append(mine, e)
		}
	}
	listed := map[string]bool{}
	for _, e := range mine {
		if strings.Contains(strings.TrimPrefix(e.path, prefix), "/") {
			return "", corrupt("nested path %s", e.path)
		}
		listed[e.path] = true
	}
	dirEntries, err := fs.ReadDir(b.fsys, b.platform)
	if err != nil {
		return "", corrupt("%s unreadable", b.platform)
	}
	for _, de := range dirEntries {
		if !listed[prefix+de.Name()] {
			return "", corrupt("%s is not in %s", prefix+de.Name(), manifestName)
		}
	}
	lockHashes := map[string]bool{}
	for _, mm := range reLockHash.FindAllStringSubmatch(string(lock), -1) {
		lockHashes[mm[1]] = true
	}
	tarName := ""
	wheels := 0
	var total int64
	for _, e := range mine {
		total += e.size
		base := path.Base(e.path)
		switch {
		case strings.HasSuffix(base, ".whl"):
			wheels++
			if !lockHashes[e.sum] {
				return "", corrupt("%s is not pinned by %s", base, lockName)
			}
		case strings.HasPrefix(base, "cpython-3.12.") && strings.HasSuffix(base, "-install_only_stripped.tar.gz"):
			if tarName != "" {
				return "", corrupt("two interpreter archives in %s", b.platform)
			}
			tarName = base
		default:
			return "", corrupt("unexpected file %s", e.path)
		}
	}
	if tarName == "" || wheels == 0 {
		return "", corrupt("%s lacks the interpreter or the wheels", b.platform)
	}
	var done int64
	for _, e := range mine {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		src, err := b.fsys.Open(e.path)
		if err != nil {
			return "", corrupt("%s is missing", e.path)
		}
		out, err := os.OpenFile(filepath.Join(dst, path.Base(e.path)), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
		if err != nil {
			_ = src.Close()
			return "", err
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(out, h), src)
		_ = src.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return "", err
		}
		if n != e.size || hex.EncodeToString(h.Sum(nil)) != e.sum {
			return "", corrupt("%s does not match %s", e.path, manifestName)
		}
		done += e.size
		if progress != nil && total > 0 {
			progress(float64(done) / float64(total))
		}
	}
	if err := os.WriteFile(filepath.Join(dst, lockName), lock, 0o644); err != nil {
		return "", err
	}
	return filepath.Join(dst, tarName), nil
}

// untarGz unpacks a .tar.gz under dest. It refuses absolute names, any `..`
// component, entries that would be written through a symlink, symlinks and
// hard links that resolve outside dest, special files, and archives over the
// size or entry cap. File modes are preserved.
func untarGz(ctx context.Context, src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return corrupt("interpreter archive: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var total int64
	entries := 0
	var links []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			for _, l := range links {
				if err := resolveInside(dest, l); err != nil {
					return err
				}
			}
			return nil
		}
		if err != nil {
			return corrupt("interpreter archive: %v", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entries++; entries > maxUnpackEntries {
			return corrupt("archive has too many entries")
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		rel, err := safeRel(h.Name)
		if err != nil {
			return err
		}
		if rel == "." {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err := noSymlinkAncestor(dest, rel); err != nil {
			return err
		}
		perm := h.FileInfo().Mode().Perm()
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, perm|0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if total += h.Size; total > maxUnpackBytes || h.Size < 0 {
				return corrupt("archive unpacks to more than %d bytes", maxUnpackBytes)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target) // never write through an existing link
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm|0o200)
			if err != nil {
				return err
			}
			n, err := io.Copy(out, io.LimitReader(tr, h.Size))
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			if n != h.Size {
				return corrupt("%s is truncated", rel)
			}
			if err := os.Chmod(target, perm); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// The target is relative to the link's own directory and must stay inside.
			// A lexical check cannot see through symlinked directories made
			// elsewhere in the archive, so every link is also resolved ON DISK
			// after extraction (resolveInside); here only the obvious refusals.
			if h.Linkname == "" || path.IsAbs(h.Linkname) || filepath.IsAbs(h.Linkname) || strings.Contains(h.Linkname, "\\") {
				return corrupt("symlink %s has an absolute or empty target", rel)
			}
			links = append(links, rel)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(h.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			// A hard link's name is relative to the archive root.
			ln, err := safeRel(h.Linkname)
			if err != nil {
				return corrupt("hard link %s escapes the archive root", rel)
			}
			if err := noSymlinkAncestor(dest, ln); err != nil {
				return err
			}
			from := filepath.Join(dest, filepath.FromSlash(ln))
			if fi, err := os.Lstat(from); err != nil || !fi.Mode().IsRegular() {
				return corrupt("hard link %s points at nothing regular", rel)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Link(from, target); err != nil {
				return err
			}
		default:
			return corrupt("%s is a special file", rel)
		}
	}
}

// resolveInside follows the symlink at rel (relative to dest) the way the
// kernel would, one component at a time through the links that exist on disk
// now, and refuses if any step leaves dest. Dangling targets are fine as long as
// the remaining path stays inside lexically.
func resolveInside(dest, rel string) error {
	root := filepath.Clean(dest)
	inside := func(p string) bool { return p == root || strings.HasPrefix(p, root+string(filepath.Separator)) }
	cur := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
	target, err := os.Readlink(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil // overwritten by a later entry (a case-insensitive name clash): no link remains
	}
	parts := strings.Split(filepath.ToSlash(target), "/")
	for hops := 0; len(parts) > 0; {
		p := parts[0]
		parts = parts[1:]
		switch p {
		case "", ".":
			continue
		case "..":
			cur = filepath.Dir(cur)
		default:
			cur = filepath.Join(cur, p)
		}
		if !inside(cur) {
			return corrupt("symlink %s resolves outside the archive root", rel)
		}
		fi, err := os.Lstat(cur)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if hops++; hops > 255 {
			return corrupt("symlink %s loops", rel)
		}
		next, err := os.Readlink(cur)
		if err != nil || filepath.IsAbs(next) {
			return corrupt("symlink %s resolves through an absolute link", rel)
		}
		cur = filepath.Dir(cur)
		parts = append(strings.Split(filepath.ToSlash(next), "/"), parts...)
	}
	return nil
}

// safeRel cleans an archive name into a relative slash path inside the root, or
// refuses it: absolute, or any `..` component (even one that would cancel out).
func safeRel(name string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(name) || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return "", corrupt("unsafe archive path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", corrupt("unsafe archive path %q", name)
		}
	}
	return path.Clean(name), nil
}

// noSymlinkAncestor refuses a path whose existing parents (under dest) include a
// symlink: a link made earlier in the same archive must never redirect a later write.
func noSymlinkAncestor(dest, rel string) error {
	parts := strings.Split(rel, "/")
	cur := dest
	for _, p := range parts[:len(parts)-1] {
		cur = filepath.Join(cur, p)
		fi, err := os.Lstat(cur)
		if err != nil {
			return nil // nothing there yet: MkdirAll will create a real directory
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return corrupt("%s is written through a symlink", rel)
		}
	}
	return nil
}

// ---- voices ----

// embeddedVoices is the bundle's voices/ directory when it holds a voice.
func (m *Manager) embeddedVoices() (fs.FS, bool) {
	if m.opt.EmbeddedBundle == nil {
		return nil, false
	}
	root, ok := m.opt.EmbeddedBundle()
	if !ok {
		return nil, false
	}
	sub, err := fs.Sub(root, "voices")
	if err != nil || len(findVoicesFS(sub)) == 0 {
		return nil, false
	}
	return sub, true
}

func findVoicesFS(fsys fs.FS) []string {
	var out []string
	for _, p := range Personas {
		f := VoiceFile(p)
		a, e1 := fs.Stat(fsys, f)
		b, e2 := fs.Stat(fsys, f+".json")
		if e1 == nil && e2 == nil && a.Mode().IsRegular() && b.Mode().IsRegular() {
			out = append(out, p)
		}
	}
	return out
}

// extractedVoices is where embedded voices are unpacked.
func (m *Manager) extractedVoices() string { return filepath.Join(m.final(TTS), "voices") }

// voiceSource picks where voices come from, in order: the operator's
// --voices-dir, the voices embedded in the binary (extracted under
// <data>/components/tts/voices), then <data>/voices. The first with at least
// one voice wins; with none, the operator's dir (else <data>/voices).
func (m *Manager) voiceSource() (dir string, embedded fs.FS, voices []string) {
	if d := m.opt.VoicesDir; d != "" {
		if v := FindVoices(d); len(v) > 0 {
			return d, nil, v
		}
	}
	if e, ok := m.embeddedVoices(); ok {
		return m.extractedVoices(), e, findVoicesFS(e)
	}
	def := filepath.Join(m.opt.DataDir, "voices")
	if v := FindVoices(def); len(v) > 0 {
		return def, nil, v
	}
	if m.opt.VoicesDir != "" {
		return m.opt.VoicesDir, nil, nil
	}
	return def, nil, nil
}

// extractVoices copies the embedded voice files to the extracted directory,
// skipping a file whose bytes already hash the same. A no-op for any other source.
func (m *Manager) extractVoices(ctx context.Context) error {
	dir, emb, _ := m.voiceSource()
	if emb == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ents, err := fs.ReadDir(emb, ".")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "hu_HU-") && !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := copyIfDifferent(emb, name, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func fileSum(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

func copyIfDifferent(src fs.FS, name, dst string) error {
	in, err := src.Open(name)
	if err != nil {
		return err
	}
	defer in.Close()
	want, wantN, err := fileSum(in)
	if err != nil {
		return err
	}
	if f, err := os.Open(dst); err == nil {
		have, haveN, _ := fileSum(f)
		_ = f.Close()
		if haveN == wantN && have == want {
			return nil
		}
	}
	in2, err := src.Open(name)
	if err != nil {
		return err
	}
	defer in2.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), in2)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != want {
		err = corrupt("%s changed while it was read", name)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// IsBundled reports whether the component ships offline (no install from the
// network; Enable/Disable instead of Install).
func IsBundled(id string) bool { return specs[id].bundled }

// asError returns err as an *Error when it is one.
func asError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
