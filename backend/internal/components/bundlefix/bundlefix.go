// Package bundlefix builds a tiny stand-in for services/tts/vendor so tests
// never unpack the real 25 MB interpreter: a "python" tarball whose python3 is
// a shell script that records its argv and environment and creates the files a
// pip install would. Mirror lays the same files out like the repository (what a
// download mirror serves) with matching manifests. Test support only; nothing in
// the server imports it.
package bundlefix

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tanulom/internal/components/ttsfetch"
)

// Options tweak the stub.
type Options struct {
	// PipFail makes the stub's `pip install` exit 1 with a pip-like message.
	PipFail bool
}

// Tarball is the interpreter archive's file name inside the platform directory.
const Tarball = "cpython-3.12.99-fake-install_only_stripped.tar.gz"

const stub = `#!/bin/sh
d="$(cd "$(dirname "$0")/.." && pwd)"
printf '%s\n' "$*" >> "$d/argv.log"
env | sort > "$d/env.log"
case "$*" in
*"pip install"*)
  @PIP@
  mkdir -p "$d/lib/site-packages/piper"
  : > "$d/lib/site-packages/piper/__init__.py"
  ;;
esac
exit 0
`

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Write creates dir as a bundle for platform (e.g. "linux-amd64"): MANIFEST.sha256,
// requirements.lock and <platform>/{tarball, two wheels}.
func Write(dir, platform string, o Options) error {
	pipFail := ":"
	if o.PipFail {
		pipFail = `echo "ERROR: Could not install packages due to an error" >&2; exit 1`
	}
	files := map[string][]byte{
		platform + "/" + Tarball:                              tarball(strings.Replace(stub, "@PIP@", pipFail, 1)),
		platform + "/piper_tts-1.8.0-cp39-abi3-fake.whl":      []byte("piper wheel bytes"),
		platform + "/onnxruntime-1.32.0-cp312-cp312-fake.whl": []byte("onnx wheel bytes"),
	}
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var manifest, lock strings.Builder
	lock.WriteString("# fixture\n")
	for _, n := range names {
		b := files[n]
		fmt.Fprintf(&manifest, "%s  %d  %s\n", sum(b), len(b), n)
		if strings.HasSuffix(n, ".whl") {
			base := filepath.Base(n)
			pkg := strings.ReplaceAll(strings.SplitN(base, "-", 3)[0], "_", "-") + "==" + strings.SplitN(base, "-", 3)[1]
			fmt.Fprintf(&lock, "%s --hash=sha256:%s\n", pkg, sum(b))
		}
		p := filepath.Join(dir, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "MANIFEST.sha256"), []byte(manifest.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "requirements.lock"), []byte(lock.String()), 0o644)
}

func tarball(python3 string) []byte {
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	add := func(h tar.Header, body string) {
		h.Size = int64(len(body))
		_ = tw.WriteHeader(&h)
		_, _ = tw.Write([]byte(body))
	}
	_ = tw.WriteHeader(&tar.Header{Name: "python/", Typeflag: tar.TypeDir, Mode: 0o755})
	_ = tw.WriteHeader(&tar.Header{Name: "python/bin/", Typeflag: tar.TypeDir, Mode: 0o755})
	add(tar.Header{Name: "python/bin/python3.12", Typeflag: tar.TypeReg, Mode: 0o755}, python3)
	_ = tw.WriteHeader(&tar.Header{Name: "python/bin/python3", Typeflag: tar.TypeSymlink, Linkname: "python3.12", Mode: 0o777})
	add(tar.Header{Name: "python/lib/README", Typeflag: tar.TypeReg, Mode: 0o644}, "fixture")
	_ = tw.Close()
	_ = gz.Close()
	return raw.Bytes()
}

// Voices are the three personas' stand-in files: name -> bytes.
func Voices() map[string][]byte {
	out := map[string][]byte{}
	for _, p := range []string{"anna", "berta", "imre"} {
		out["hu_HU-"+p+"-medium.onnx"] = []byte("onnx-" + p + strings.Repeat("-model", 200))
		out["hu_HU-"+p+"-medium.onnx.json"] = []byte(`{"voice":"` + p + `"}`)
	}
	return out
}

// Mirror writes a repository-shaped tree under root —
// services/tts/vendor/<platform>/... and langpack/voices/... — and returns the
// manifests that pin exactly those bytes (what the binary embeds in production).
func Mirror(root, platform string, o Options) (ttsfetch.Manifests, error) {
	vendor := filepath.Join(root, "services", "tts", "vendor")
	if err := Write(vendor, platform, o); err != nil {
		return ttsfetch.Manifests{}, err
	}
	vdir := filepath.Join(root, "langpack", "voices")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		return ttsfetch.Manifests{}, err
	}
	vs := Voices()
	var names []string
	for n := range vs {
		names = append(names, n)
	}
	sort.Strings(names)
	var vm strings.Builder
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(vdir, n), vs[n], 0o644); err != nil {
			return ttsfetch.Manifests{}, err
		}
		fmt.Fprintf(&vm, "%s  %d  %s\n", sum(vs[n]), len(vs[n]), n)
	}
	man, err := os.ReadFile(filepath.Join(vendor, "MANIFEST.sha256"))
	if err != nil {
		return ttsfetch.Manifests{}, err
	}
	lock, err := os.ReadFile(filepath.Join(vendor, "requirements.lock"))
	if err != nil {
		return ttsfetch.Manifests{}, err
	}
	return ttsfetch.Manifests{Vendor: man, Lock: lock, Voices: []byte(vm.String())}, nil
}
