package lang

import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
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
// fully explain. It is enough
// to work through the first months for real. The 252 MB pack is an UPGRADE that
// widens the vocabulary — offered, never required.
//
// See langpack/pipeline/starter.py for exactly what is kept and why.

//go:embed starter/langpack-hu-starter.sqlite.gz
var starterPack []byte

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

// EmbeddedStarterBytes reports the size of the embedded pack, for the About
// screen and for a build-time sanity check that the artefact was not forgotten.
func EmbeddedStarterBytes() int { return len(starterPack) }

var _ embed.FS // keep the embed import meaningful if the directives move
