// Package ttsfetch downloads the Piper runtime and the three voices ONCE from
// the project's own public repository (ADR-081) and proves every byte against a
// hash manifest compiled into the binary. The binary and the images carry only
// those small manifests; nothing here is ever reachable from a request.
//
// What is fixed in code, never taken from a route, a config field or a file
// that arrived over the network:
//
//   - the host (raw.githubusercontent.com), https only, no redirect off-host;
//   - the URL: repo constant + ref + path built from the embedded manifest;
//   - the exact size and sha256 of every file.
//
// An operator may point the fetcher at a mirror of the same layout through the
// flag/env --tts-base-url (https, or http on loopback); that value is validated
// by New and goes nowhere else.
package ttsfetch

import (
	"embed"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Repo is the public repository the files come from.
const Repo = "antlko/learn-hungarian-ai"

// Host is the only host the default base URL uses.
const Host = "raw.githubusercontent.com"

// Paths of the two trees in the repository.
const (
	vendorPath = "services/tts/vendor"
	voicesPath = "langpack/voices"
)

//go:embed data/MANIFEST.sha256 data/requirements.lock data/VOICES.sha256
var embedded embed.FS

// Manifests are the three small, committed inputs. `make sync-components`
// copies them from services/tts/vendor and langpack/voices; a Go test fails on drift.
type Manifests struct {
	Vendor []byte // MANIFEST.sha256 of services/tts/vendor (sha256  size  <platform>/<file>)
	Lock   []byte // requirements.lock
	Voices []byte // VOICES.sha256 (sha256  size  hu_HU-<persona>-medium.onnx[.json])
}

// Embedded returns the manifests compiled into this binary.
func Embedded() Manifests {
	read := func(n string) []byte { b, _ := embedded.ReadFile("data/" + n); return b }
	return Manifests{Vendor: read("MANIFEST.sha256"), Lock: read("requirements.lock"), Voices: read("VOICES.sha256")}
}

// Kind classifies a failure for the UI: a flaky network is retryable as is, a
// file that does not match its pin is not (and was deleted).
type Kind int

const (
	KindDownload Kind = iota // network, HTTP status, timeout
	KindCorrupt              // size or sha256 differs from the manifest
)

var (
	// ErrDownload and ErrCorrupt match with errors.Is.
	ErrDownload = errors.New("download failed")
	ErrCorrupt  = errors.New("file does not match its pin")
)

// Error is a classified failure. File is the manifest-relative name.
type Error struct {
	Kind Kind
	File string
	Err  error
}

func (e *Error) Error() string {
	what := "download failed"
	if e.Kind == KindCorrupt {
		what = "does not match its pin"
	}
	if e.File == "" {
		return fmt.Sprintf("%s: %v", what, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.File, what, e.Err)
}
func (e *Error) Unwrap() error { return e.Err }
func (e *Error) Is(t error) bool {
	return (t == ErrDownload && e.Kind == KindDownload) || (t == ErrCorrupt && e.Kind == KindCorrupt)
}

func corrupt(file, format string, a ...any) *Error {
	return &Error{Kind: KindCorrupt, File: file, Err: fmt.Errorf(format, a...)}
}

// File is one downloadable file.
type File struct {
	Rel  string // path under the cache dir: <platform>/<name> or voices/<name>
	Repo string // path under the repository root
	Sum  string // sha256, lowercase hex
	Size int64
}

// Entry is one manifest line.
type Entry struct {
	Sum  string
	Size int64
	Path string
}

var (
	reSum      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reName     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
	rePlatform = regexp.MustCompile(`^(darwin|linux)-(arm64|amd64)$`)
)

// ParseManifest reads `sha256  size  path` lines (blank lines and # comments
// skipped). Every path component must be a plain file name: no traversal, no
// separators beyond the one directory level.
func ParseManifest(b []byte) ([]Entry, error) {
	var out []Entry
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 || !reSum.MatchString(f[0]) {
			return nil, corrupt("", "manifest line %d is malformed", i+1)
		}
		size, err := strconv.ParseInt(f[1], 10, 64)
		if err != nil || size <= 0 {
			return nil, corrupt("", "manifest line %d has a bad size", i+1)
		}
		if p := f[2]; p != path.Clean(p) || path.IsAbs(p) {
			return nil, corrupt("", "manifest line %d has an unsafe path", i+1)
		}
		out = append(out, Entry{Sum: f[0], Size: size, Path: f[2]})
	}
	return out, nil
}

// Plan lists the files to fetch for a platform: its runtime files from the
// vendor manifest (the `source/` tree is never fetched) and/or the voices.
// Names are validated here, so nothing that reaches a URL can traverse.
func Plan(m Manifests, platform string, runtime, voices bool) ([]File, error) {
	if !rePlatform.MatchString(platform) {
		return nil, corrupt("", "unsupported platform %q", platform)
	}
	var out []File
	if runtime {
		es, err := ParseManifest(m.Vendor)
		if err != nil {
			return nil, err
		}
		prefix := platform + "/"
		for _, e := range es {
			if !strings.HasPrefix(e.Path, prefix) {
				continue
			}
			name := strings.TrimPrefix(e.Path, prefix)
			if !reName.MatchString(name) || strings.Contains(name, "..") {
				return nil, corrupt(e.Path, "unsafe file name")
			}
			out = append(out, File{Rel: e.Path, Repo: vendorPath + "/" + e.Path, Sum: e.Sum, Size: e.Size})
		}
		if len(out) == 0 {
			return nil, corrupt("", "the manifest has no files for %s", platform)
		}
	}
	if voices {
		es, err := ParseManifest(m.Voices)
		if err != nil {
			return nil, err
		}
		if len(es) == 0 {
			return nil, corrupt("", "the voices manifest is empty")
		}
		for _, e := range es {
			if !reName.MatchString(e.Path) || strings.Contains(e.Path, "..") || strings.Contains(e.Path, "/") {
				return nil, corrupt(e.Path, "unsafe file name")
			}
			out = append(out, File{Rel: "voices/" + e.Path, Repo: voicesPath + "/" + e.Path, Sum: e.Sum, Size: e.Size})
		}
	}
	return out, nil
}

var reRef = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// RefFor is the git ref the files are read from: the build's version when it is
// a release tag (the files at that tag are what its manifests pin), else main.
func RefFor(version string) string {
	if reRef.MatchString(version) {
		return version
	}
	return "main"
}
