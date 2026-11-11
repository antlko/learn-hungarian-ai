package lang

import (
	"testing"

	_ "modernc.org/sqlite"
)

// The starter pack is compiled into an executable we publish, so what is inside
// it is a redistribution question and not only a size question.
//
// Tatoeba is CC BY 2.0 FR and travels freely. OPUS OpenSubtitles is free for
// research and is the source of the pack's conversational sentences, but OPUS
// asks for a citation rather than granting a content licence — which does not
// cover embedding film dialogue in a binary. `pipeline/starter.py` therefore
// selects `source = 'tatoeba'` with an ALLOW-list, and this is that boundary
// checked on the committed artefact rather than trusted to a comment.
//
// A new corpus added to the full pack will fail here until somebody decides,
// deliberately, that it may ship. That is the intended failure.
//
// The one exception is `authored`: the written dialogues (ADR-066), which are
// not a corpus but text written for tanulom under the pack's own licence,
// CC BY-SA 4.0. They are checked by their licence column, not waved through.
func TestStarterPackShipsOnlyRedistributableSentences(t *testing.T) {
	dir := t.TempDir()
	version, _, err := EnsurePack(dir)
	if err != nil {
		t.Fatalf("EnsurePack: %v", err)
	}
	p, err := Open(Path(dir, version))
	if err != nil {
		t.Fatalf("open starter pack: %v", err)
	}
	defer p.Close()

	rows, err := p.DB().Query(`SELECT source, COUNT(*) FROM sentence GROUP BY source`)
	if err != nil {
		t.Fatalf("query sources: %v", err)
	}
	defer rows.Close()

	const allowed = "tatoeba"
	var total int
	for rows.Next() {
		var source string
		var n int
		if err := rows.Scan(&source, &n); err != nil {
			t.Fatal(err)
		}
		total += n
		if source != allowed && source != "authored" {
			t.Errorf("the embedded pack carries %d sentences from %q; only %q (and the "+
				"authored dialogues) may be redistributed — see langpack/LICENSES.md and "+
				"pipeline/starter.py", n, source, allowed)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	var foreign int
	if err := p.DB().QueryRow(`SELECT COUNT(*) FROM sentence
		WHERE source = 'authored' AND license NOT LIKE 'CC BY-SA 4.0%'`).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	if foreign != 0 {
		t.Errorf("%d authored sentences carry a licence other than the pack's CC BY-SA 4.0", foreign)
	}
	if total == 0 {
		t.Error("the starter pack has no sentences at all")
	}
}
