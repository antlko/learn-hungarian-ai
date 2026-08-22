package lang

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// The starter pack, baked into the binary.
//
// An app that boots into "no language pack is installed — download one from
// Settings" is not a product, it is a build step wearing a UI. Every screen
// except Settings is dead, and the learner is asked to solve a distribution
// problem before they can read a word of Hungarian.
//
// So the binary carries a compact pack: the whole curriculum, the top ~4600
// lemmas with their forms and glosses, twelve thousand sentences the starter can
// fully explain, every minimal pair, and the ear trainer's audio. It is enough
// to work through the first months for real. The 252 MB pack is an UPGRADE that
// widens the vocabulary — offered, never required.
//
// See langpack/pipeline/starter.py for exactly what is kept and why.

//go:embed starter/langpack-hu-starter.sqlite.gz
var starterPack []byte

// starterMedia holds the pre-rendered ear-trainer audio. Optional: a build made
// without ffmpeg ships an empty archive and the Ear screen says its audio is
// missing, which is the honest degradation.
//
//go:embed starter/media.tar.gz
var starterMedia []byte

// StarterVersion is the pack version the embedded starter installs as. It must
// match the version the pipeline stamps, or Open will refuse its own artefact.
const StarterVersion = "1"

// Edition tells a starter pack from the full one, so the UI can offer the
// upgrade without guessing from the file size.
const (
	EditionStarter = "starter"
	EditionFull    = "full"
)

// EnsurePack guarantees a usable pack in the data directory.
//
// It returns the version present. When nothing usable is there it extracts the
// embedded starter — including a CORRUPT or truncated file, which is the case
// that used to strand the app: Detect saw the filename, Open failed, and the
// server started with every language feature dead and a message telling the user
// to go and download what was already sitting on disk.
func EnsurePack(dataDir string) (version string, installed bool, err error) {
	if v, ok := Detect(dataDir); ok {
		return v, false, nil
	}
	if len(starterPack) == 0 {
		return "", false, errors.New("this build embeds no starter language pack")
	}

	dest := Path(dataDir, StarterVersion)
	// Write beside the target and rename: a half-extracted pack that Detect
	// then accepts would reproduce exactly the failure this function exists to
	// remove.
	tmp := dest + ".installing"
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", false, err
	}
	defer os.Remove(tmp)

	if err := gunzipTo(starterPack, tmp); err != nil {
		return "", false, fmt.Errorf("extract starter pack: %w", err)
	}
	// Prove it opens before it becomes the installed pack. Quietly: this is a
	// probe, and the real load is logged by the caller a moment later.
	if p, err := openQuiet(tmp); err != nil {
		return "", false, fmt.Errorf("embedded starter pack is unusable: %w", err)
	} else {
		p.Close()
	}
	// A stale zero-byte or unreadable file may be sitting at dest.
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return "", false, fmt.Errorf("install starter pack: %w", err)
	}

	if err := extractStarterMedia(dataDir); err != nil {
		// Audio is a nice-to-have; the pack is not. Log and carry on — the Ear
		// screen already reports missing audio honestly.
		slog.Warn("starter media not installed", "error", err)
	}
	return StarterVersion, true, nil
}

func gunzipTo(gz []byte, dest string) error {
	zr, err := gzip.NewReader(strings.NewReader(string(gz)))
	if err != nil {
		return err
	}
	defer zr.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, zr); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// extractStarterMedia unpacks the embedded audio into <dataDir>/media, skipping
// files that already exist so a re-install never clobbers a learner's own media.
func extractStarterMedia(dataDir string) error {
	if len(starterMedia) == 0 {
		return nil
	}
	zr, err := gzip.NewReader(strings.NewReader(string(starterMedia)))
	if err != nil {
		return err
	}
	defer zr.Close()

	root := filepath.Join(dataDir, "media")
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		// AppleDouble sidecars and Finder droppings are not media. The Makefile
		// already builds the tarball without them, but the tarball is built on
		// whatever machine ran `make starter`, and macOS tar adds these while
		// hiding them from its own listing — so a recipe alone is not a
		// guarantee. Refusing them here is.
		if base := filepath.Base(hdr.Name); strings.HasPrefix(base, "._") || base == ".DS_Store" {
			continue
		}
		// Tar paths come from our own build, but a path that escapes the media
		// directory is worth refusing on principle rather than on trust.
		dest := filepath.Join(root, filepath.Clean("/"+hdr.Name))
		if !strings.HasPrefix(dest, root+string(os.PathSeparator)) {
			continue
		}
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
}

// EmbeddedStarterBytes reports the size of the embedded pack, for the About
// screen and for a build-time sanity check that the artefact was not forgotten.
func EmbeddedStarterBytes() int { return len(starterPack) }

var _ embed.FS // keep the embed import meaningful if the directives move
