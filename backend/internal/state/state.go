// Package state persists a tiny bootstrap file recording which data directory is
// active. It lives outside the data directory (in the user config dir) so that a
// custom --data-dir survives a restart that omits the flag.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State is the bootstrap record.
type State struct {
	DataDir string `json:"dataDir"`
}

// path returns the location of state.json, or "" if no suitable directory exists.
func path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "tanulom", "state.json")
}

// Load reads the bootstrap state. A missing or unreadable file yields a zero State
// rather than an error: this is a hint, never a requirement.
func Load() State {
	p := path()
	if p == "" {
		return State{}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return State{}
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}
	}
	return s
}

// Save records the active data directory. Failures are returned but callers
// generally ignore them — losing this file degrades convenience, not correctness.
func Save(s State) error {
	p := path()
	if p == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}
