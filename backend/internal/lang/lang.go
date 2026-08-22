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

// Detect reports whether a language pack is present in the data directory and,
// if several are, returns the highest version found.
func Detect(dataDir string) (present bool, version string) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return false, ""
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
	if len(versions) == 0 {
		return false, ""
	}
	sort.Strings(versions)
	return true, versions[len(versions)-1]
}

// Path returns the expected pack location for a version inside a data directory.
func Path(dataDir, version string) string {
	return filepath.Join(dataDir, "langpack-hu-v"+version+".sqlite")
}
