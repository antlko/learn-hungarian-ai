package api

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"tanulom/internal/auth"
	"tanulom/internal/lang"
)

// The starter pack is a 24 MB gzip that unpacks to a large SQLite file.
// api.New installs it into any data dir that has no pack, so every test that
// used a bare t.TempDir() paid that decompression again -- hundreds of times,
// which is what pushed this package past Go's 10-minute timeout under -race.
// The pack is read-only (ATTACHed, never written), so one extraction is shared
// by hard link; see seedStarterPack.
var (
	starterOnce sync.Once
	starterDir  string
	starterErr  error
)

func TestMain(m *testing.M) {
	// Cheap password hashing: the tests are not testing bcrypt.
	auth.BcryptCost = bcrypt.MinCost
	code := m.Run()
	if starterDir != "" {
		os.RemoveAll(starterDir)
	}
	os.Exit(code)
}

// seedStarterPack makes dir look like a first boot that has already installed
// the starter pack, without decompressing it again.
func seedStarterPack(t testing.TB, dir string) {
	t.Helper()
	starterOnce.Do(func() {
		starterDir, starterErr = os.MkdirTemp("", "tanulom-starter-")
		if starterErr == nil {
			_, _, starterErr = lang.EnsurePack(starterDir)
		}
	})
	if starterErr != nil {
		t.Fatalf("prepare shared starter pack: %v", starterErr)
	}
	// Only the pack file: the 340-odd media files cost more to link than the
	// pack itself and no test in this package reads them.
	ents, err := os.ReadDir(starterDir)
	if err != nil {
		t.Fatalf("seed starter pack: %v", err)
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if err := linkFile(filepath.Join(starterDir, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			t.Fatalf("seed starter pack: %v", err)
		}
	}
}

// linkFile hard-links src to dst, copying when the filesystems differ.
func linkFile(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		b, rerr := os.ReadFile(src)
		if rerr != nil {
			return fmt.Errorf("link %s: %v; read: %w", src, err, rerr)
		}
		return os.WriteFile(dst, b, 0o644)
	}
	return nil
}
