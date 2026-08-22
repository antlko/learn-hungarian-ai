package lang

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// The whole point of embedding a pack is that an empty data directory is not a
// broken install. This is that promise, tested.
func TestEnsurePackInstallsIntoAnEmptyDir(t *testing.T) {
	dir := t.TempDir()

	version, installed, err := EnsurePack(dir)
	if err != nil {
		t.Fatalf("EnsurePack: %v", err)
	}
	if !installed {
		t.Fatal("nothing was installed into an empty directory")
	}
	if version != StarterVersion {
		t.Errorf("version = %q, want %q", version, StarterVersion)
	}

	// The file has to be a pack, not merely a file — the failure this whole
	// design replaced was a zero-byte download that Detect happily reported.
	if v, ok := Detect(dir); !ok || v != version {
		t.Fatalf("Detect after install = (%q, %v), want (%q, true)", v, ok, version)
	}
	p, err := Open(Path(dir, version))
	if err != nil {
		t.Fatalf("the installed pack does not open: %v", err)
	}
	defer p.Close()
	if p.Meta.Edition != EditionStarter {
		t.Errorf("edition = %q, want %q", p.Meta.Edition, EditionStarter)
	}

	// The ear trainer needs its clips beside the pack, or the first session
	// refuses with a talker count of zero.
	clips, _ := filepath.Glob(filepath.Join(dir, "media", "hvpt", "*.ogg"))
	if len(clips) < 300 {
		t.Errorf("installed %d clips into media/hvpt, want the inventory", len(clips))
	}
}

// Second boot must be a no-op. If it were not, every restart would overwrite a
// pack the user may have replaced with the full edition.
func TestEnsurePackLeavesAnInstalledPackAlone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := EnsurePack(dir); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(Path(dir, StarterVersion))
	if err != nil {
		t.Fatal(err)
	}

	version, installed, err := EnsurePack(dir)
	if err != nil {
		t.Fatalf("second EnsurePack: %v", err)
	}
	if installed {
		t.Error("the second boot reinstalled a pack that was already there")
	}
	if version != StarterVersion {
		t.Errorf("version = %q, want %q", version, StarterVersion)
	}
	after, err := os.Stat(Path(dir, StarterVersion))
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("the pack file was rewritten on a boot that had nothing to do")
	}
}

// What the binary carries is what every install gets, so the tarball's contents
// are a shipped artefact and deserve a test.
//
// This exists because of a real failure: `make starter` on macOS packed an
// AppleDouble `._x` sidecar beside all 384 clips, and bsdtar HIDES those from
// its own `tar tzf` listing — so the tarball looked clean, and every fresh
// install silently grew 385 junk files. Go's tar reader shows them, so this
// test can see what the build machine could not.
func TestEmbeddedMediaCarriesOnlyMedia(t *testing.T) {
	zr, err := gzip.NewReader(bytes.NewReader(starterMedia))
	if err != nil {
		t.Fatalf("the embedded media tarball does not decompress: %v", err)
	}
	defer zr.Close()

	clips := 0
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("reading the tarball: %v", err)
		}
		base := filepath.Base(hdr.Name)
		if strings.HasPrefix(base, "._") || base == ".DS_Store" {
			t.Errorf("%s is a Finder artefact, not media — rebuild with `make starter`", hdr.Name)
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Ext(base) != ".ogg" {
			t.Errorf("%s is not an Opus clip; the ear trainer plays .ogg only", hdr.Name)
			continue
		}
		if !strings.HasPrefix(hdr.Name, "hvpt/") {
			t.Errorf("%s is outside hvpt/, which is the only tree the pack references", hdr.Name)
		}
		clips++
	}

	// A floor, not the exact count: adding contrasts must not fail this test,
	// but shipping a tarball that lost its clips has to.
	if clips < 300 {
		t.Errorf("the embedded tarball has %d clips; the ear trainer needs its inventory", clips)
	}
}
