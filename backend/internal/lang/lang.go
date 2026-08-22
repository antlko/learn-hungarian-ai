// Package lang locates, verifies and opens the Hungarian language pack — a
// versioned, read-only SQLite file holding lemmas, wordforms, morphological
// features, sentences, minimal pairs and the curriculum.
//
// The pack is kept separate from the learner database on purpose: upgrading it is
// a file swap that cannot put learner data at risk. See ADR-002 and
// docs/LANGPACK.md.
//
// Phase 1 implements detection only. Opening and ATTACHing arrive in Phase 2.
package lang

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// SchemaVersion is the pack schema this binary understands. A pack declaring a
// different major version is refused at startup with an actionable message
// rather than failing confusingly later.
const SchemaVersion = 1

// FilePattern matches pack filenames: langpack-hu-v1.sqlite, langpack-hu-v2.sqlite, ...
var FilePattern = regexp.MustCompile(`^langpack-hu-v(\d+(?:\.\d+)?)\.sqlite$`)

// Detect reports the highest USABLE pack version in the data directory.
//
// "Usable" means it opens, not merely that a matching filename exists. Those are
// different things and the difference is not academic: a truncated download or
// an interrupted copy leaves a zero-byte langpack-hu-v1.sqlite, which the old
// name-only check reported as present. The server then started with morphology,
// reading, drills, the ear trainer and the curriculum all dead behind a message
// telling the user to download a pack that was, as far as they could see,
// already there. Verifying costs one Open per candidate at startup.
func Detect(dataDir string) (version string, ok bool) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return "", false
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if m := FilePattern.FindStringSubmatch(e.Name()); m != nil {
			versions = append(versions, m[1])
		}
	}
	sort.Strings(versions)
	// Newest first: an older pack is an acceptable fallback when the newest
	// one is broken, and saying nothing at all would be worse than either.
	for i := len(versions) - 1; i >= 0; i-- {
		p, err := openQuiet(Path(dataDir, versions[i]))
		if err != nil {
			slog.Warn("ignoring an unusable language pack",
				"version", versions[i], "error", err)
			continue
		}
		_ = p.Close()
		return versions[i], true
	}
	return "", false
}

// Path returns the expected pack location for a version inside a data directory.
func Path(dataDir, version string) string {
	return filepath.Join(dataDir, "langpack-hu-v"+version+".sqlite")
}
