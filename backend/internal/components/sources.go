package components

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

// The service sources the binary carries. services/<id>/ is the source of
// truth (the Dockerfiles build from it); src/ is a generated copy because
// go:embed cannot reach outside its package. `make sync-components` refreshes
// it and TestEmbeddedSourcesMatchServices fails on drift.
//
//go:embed src
var sourcesFS embed.FS

// Source returns one embedded file.
func Source(id, name string) ([]byte, error) {
	return sourcesFS.ReadFile("src/" + id + "/" + name)
}

// writeSources makes dir hold exactly the embedded files, rewriting any whose
// bytes differ. Run before every install AND every start: what runs is only
// ever what this binary carries, even if the tree was edited on disk.
func writeSources(id, dir string) error {
	sp := specs[id]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range sp.files {
		want, err := Source(id, name)
		if err != nil {
			return fmt.Errorf("embedded source %s/%s: %w", id, name, err)
		}
		path := filepath.Join(dir, name)
		if have, err := os.ReadFile(path); err == nil && sha256.Sum256(have) == sha256.Sum256(want) && bytes.Equal(have, want) {
			continue
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			return err
		}
	}
	return nil
}
