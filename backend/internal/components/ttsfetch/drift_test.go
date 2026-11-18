package ttsfetch

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The embedded manifests are copies (make sync-components). They must equal
// their sources, and the voices manifest must describe the real files of this
// checkout, or an install would "download" files that can never verify.

func repoRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "services", "tts", "vendor", "MANIFEST.sha256")); err != nil {
		t.Skip("not inside the repository checkout")
	}
	return root
}

func TestEmbeddedManifestsEqualTheirSources(t *testing.T) {
	root := repoRoot(t)
	m := Embedded()
	for _, c := range []struct {
		name string
		got  []byte
		src  string
	}{
		{"MANIFEST.sha256", m.Vendor, filepath.Join("services", "tts", "vendor", "MANIFEST.sha256")},
		{"requirements.lock", m.Lock, filepath.Join("services", "tts", "vendor", "requirements.lock")},
		{"VOICES.sha256", m.Voices, filepath.Join("langpack", "voices", "MANIFEST.sha256")},
	} {
		want, err := os.ReadFile(filepath.Join(root, c.src))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(c.got, want) {
			t.Errorf("%s drifted from %s: run `make sync-components`", c.name, c.src)
		}
	}
}

func TestVoicesManifestVerifiesAgainstTheRealFiles(t *testing.T) {
	root := repoRoot(t)
	es, err := ParseManifest(Embedded().Voices)
	if err != nil || len(es) != 6 {
		t.Fatalf("%v, %d entries", err, len(es))
	}
	for _, e := range es {
		if !Valid(filepath.Join(root, "langpack", "voices", e.Path), e.Size, e.Sum) {
			t.Errorf("%s does not match the voices manifest: run `make tts-vendor`'s voices step (scripts/tts_vendor.py voices) and `make sync-components`", e.Path)
		}
	}
}

func TestVendorManifestPlansEveryPlatformAndMatchesFileSizes(t *testing.T) {
	root := repoRoot(t)
	for _, p := range []string{"darwin-arm64", "linux-amd64", "linux-arm64"} {
		files, err := Plan(Embedded(), p, true, true)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if len(files) != 8+6 { // tarball + 7 wheels, 3 voices x 2 files
			t.Errorf("%s: %d files", p, len(files))
		}
		for _, f := range files {
			if f.Repo[:8] != "services" {
				continue
			}
			st, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.Repo)))
			if err != nil || st.Size() != f.Size {
				t.Errorf("%s: size on disk differs from the manifest (%v)", f.Repo, err)
			}
		}
	}
}
