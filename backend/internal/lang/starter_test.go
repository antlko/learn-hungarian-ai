package lang

import (
	"os"
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
