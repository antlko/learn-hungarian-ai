package hvpt

import (
	"database/sql"
	"errors"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"tanulom/internal/lang"
	"tanulom/internal/srs"
)

// A miniature pack with two contrasts and controllable talker counts.
func testInventory(t *testing.T, talkers []string) *Inventory {
	t.Helper()
	path := filepath.Join(t.TempDir(), "langpack-hu-v1.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE minimal_pair (id INTEGER PRIMARY KEY, a TEXT, b TEXT, contrast TEXT,
			priority INTEGER, gloss_a_ru TEXT, gloss_b_ru TEXT, gloss_a_en TEXT,
			gloss_b_en TEXT, context_a TEXT, context_b TEXT);
		CREATE TABLE pair_audio (pair_id INTEGER, member TEXT, talker_id TEXT,
			talker_kind TEXT, rate REAL, path TEXT, license TEXT);
		INSERT INTO pack_meta VALUES ('version','1'),('schema_version','1'),('gloss_langs','["en"]');
		INSERT INTO minimal_pair VALUES
			(1,'kor','kór','o/ó',1,'возраст','болезнь','age','disease','',''),
			(2,'öt','őt','ö/ő',1,'пять','его','five','him','',''),
			(3,'hal','hall','l/ll',3,'рыба','слышит','fish','hears','','');`); err != nil {
		t.Fatal(err)
	}
	for _, pairID := range []int{1, 2} { // pair 3 gets NO audio on purpose
		for _, member := range []string{"a", "b"} {
			for _, talker := range talkers {
				if _, err := db.Exec(`INSERT INTO pair_audio VALUES (?,?,?,?,1.0,?,?)`,
					pairID, member, talker, "piper",
					"hvpt/x.wav", "CC0"); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	db.Close()
	p, err := lang.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	inv, err := LoadInventory(p)
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

// [R7]: a session below the talker floor is REFUSED with the numbers, never
// quietly degraded. This is the honesty invariant of the whole trainer.
func TestSessionRefusedBelowTalkerFloor(t *testing.T) {
	inv := testInventory(t, []string{"anna", "berta", "imre"})
	sched := srs.NewScheduler(0.9)

	_, err := inv.SessionPlan(nil, 10, 5, rand.New(rand.NewSource(1)), time.Now(), sched)
	var floor *TalkerFloorError
	if !errors.As(err, &floor) {
		t.Fatalf("want TalkerFloorError, got %v", err)
	}
	if floor.Have != 3 || floor.Need != 5 {
		t.Errorf("the refusal must carry the numbers: %+v", floor)
	}

	// At an admin-lowered floor of 3, the same inventory trains.
	trials, err := inv.SessionPlan(nil, 10, 3, rand.New(rand.NewSource(1)), time.Now(), sched)
	if err != nil || len(trials) != 10 {
		t.Fatalf("plan at floor 3: %v (%d trials)", err, len(trials))
	}
}

// Talker variety is engineered, not hoped for: consecutive trials rotate voice.
func TestSessionRotatesTalkers(t *testing.T) {
	inv := testInventory(t, []string{"anna", "berta", "imre"})
	trials, err := inv.SessionPlan(nil, 12, 3, rand.New(rand.NewSource(2)), time.Now(), srs.NewScheduler(0.9))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for i, tr := range trials {
		seen[tr.TalkerID]++
		if i > 0 && trials[i-1].TalkerID == tr.TalkerID {
			t.Fatalf("trials %d and %d share talker %s — variety is the mechanism", i-1, i, tr.TalkerID)
		}
	}
	if len(seen) != 3 {
		t.Errorf("12 trials across 3 voices must use all 3, got %v", seen)
	}
}

// A pair with no audio for both members must never be served.
func TestHalfRecordedPairIsNotServed(t *testing.T) {
	inv := testInventory(t, []string{"anna", "berta", "imre"})
	if inv.Audible(3) {
		t.Fatal("pair 3 has no audio in the fixture")
	}
	trials, err := inv.SessionPlan(nil, 40, 3, rand.New(rand.NewSource(3)), time.Now(), srs.NewScheduler(0.9))
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range trials {
		if tr.Pair.ID == 3 {
			t.Fatal("served the audio-less pair")
		}
	}
	// And the contrast list is honest about it.
	for _, c := range inv.AudibleContrasts() {
		if c == "l/ll" {
			t.Error("l/ll has no audible pair and must not be listed")
		}
	}
}

// The played member is server-held: a trial as serialised carries no answer.
func TestTrialAnswerStaysUnexported(t *testing.T) {
	inv := testInventory(t, []string{"anna", "berta", "imre"})
	trials, _ := inv.SessionPlan(nil, 1, 3, rand.New(rand.NewSource(4)), time.Now(), srs.NewScheduler(0.9))
	tr := trials[0]
	if tr.Played() != "a" && tr.Played() != "b" {
		t.Fatal("the trial must know its own answer")
	}
	// The JSON tag on the field is "-": encoding it must not leak. Checked by
	// construction here — the field is unexported — and by the API test.
	if tr.AudioURL == "" {
		t.Error("a trial without audio is unplayable")
	}
}

// Adaptive weights: due + weak beats fresh; nothing ever reaches zero, because
// maintenance never stops. [R7]
func TestAdaptiveWeights(t *testing.T) {
	sched := srs.NewScheduler(0.9)
	now := time.Now()
	weak := ContrastState{Contrast: "o/ó", Accuracy: 0.4, Total: 20}
	weak.State.Due = now.Add(-time.Hour) // due
	strong := ContrastState{Contrast: "ö/ő", Accuracy: 0.98, Total: 40}
	strong.State.Due = now.Add(240 * time.Hour) // far future

	w := contrastWeights([]string{"o/ó", "ö/ő"}, map[string]ContrastState{
		"o/ó": weak, "ö/ő": strong,
	}, now, sched)
	if w["o/ó"] <= w["ö/ő"] {
		t.Errorf("a due, weak contrast must outweigh a strong future one: %v", w)
	}
	if w["ö/ő"] <= 0 {
		t.Error("no contrast is ever complete — the weight floor must be positive")
	}
}
