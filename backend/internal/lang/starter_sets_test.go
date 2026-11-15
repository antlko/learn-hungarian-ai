package lang

import "testing"

// Reading texts left the pack (ADR-076): they ship in the binary's own
// catalogue (internal/readsets). The starter must still install and open, and
// nothing may depend on it carrying reading sets or authored sentences.
func TestStarterOpensWithoutReadingSets(t *testing.T) {
	dir := t.TempDir()
	version, _, err := EnsurePack(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(Path(dir, version))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var n int
	if err := p.DB().QueryRow(`SELECT COUNT(*) FROM sentence`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("starter has no sentences: %d, %v", n, err)
	}
}
