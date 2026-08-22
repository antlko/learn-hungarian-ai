package srs

import (
	"path/filepath"
	"testing"
	"time"

	"tanulom/internal/db"
)

func newEngine(t *testing.T) (*Engine, int64) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	uid, err := d.CreateUser("a@example.com", "hash", db.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(NewStore(d.DB), 0.90, 20), uid
}

// clozeNote builds a note whose blanked span carries three features, the way a
// real `házaimban` cloze would.
func clozeNote(uid int64) Note {
	return Note{
		UserID: uid, Kind: "sentence",
		HU:          "A házaimban sok könyv van.",
		TargetForm:  "házaimban",
		TargetStart: 2, TargetEnd: 11,
		TargetFeats: []string{"Case=Ine", "Number[psed]=Plur", "Person[psor]=1"},
		GlossEN:     "There are many books in my houses.",
	}
}

func TestCreateNoteSpawnsOneCardPerDirection(t *testing.T) {
	e, uid := newEngine(t)

	dirs := []Direction{Recognize, Produce, Cloze}
	id, cards, err := e.Store().CreateNote(clozeNote(uid), dirs, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 || len(cards) != 3 {
		t.Fatalf("got note %d with %d cards", id, len(cards))
	}
	for _, c := range cards {
		if !c.State.IsNew() {
			t.Errorf("%s should start new", c.Direction)
		}
	}
}

func TestNoteRoundTripsItsTargetFeatures(t *testing.T) {
	e, uid := newEngine(t)
	id, _, err := e.Store().CreateNote(clozeNote(uid), []Direction{Cloze}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.Store().Note(uid, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.TargetFeats) != 3 || got.TargetFeats[0] != "Case=Ine" {
		t.Errorf("target features lost in round trip: %v", got.TargetFeats)
	}
}

func TestClozeAnswerAdvancesBothStateSpaces(t *testing.T) {
	// This is the product thesis as an assertion: one answer moves the card AND
	// each morphological feature the item required.
	e, uid := newEngine(t)
	now := time.Now().UTC()

	_, cards, err := e.Store().CreateNote(clozeNote(uid), []Direction{Cloze}, now)
	if err != nil {
		t.Fatal(err)
	}
	note, _ := e.Store().Note(uid, cards[0].NoteID)

	res, err := e.Answer(uid, cards[0], note,
		Outcome{Correct: true, Duration: 3 * time.Second}, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GradedFeatures) != 3 {
		t.Fatalf("want 3 features graded, got %d", len(res.GradedFeatures))
	}
	if !res.NextDue.After(now) {
		t.Errorf("card should be rescheduled forward, got %v", res.NextDue)
	}

	for _, fid := range note.TargetFeats {
		fs, err := e.Store().FeatureState(nil, uid, fid, now)
		if err != nil {
			t.Fatal(err)
		}
		if fs.State.Reps != 1 {
			t.Errorf("%s: reps = %d, want 1", fid, fs.State.Reps)
		}
		if fs.Total != 1 || fs.Accuracy < 0.99 {
			t.Errorf("%s: accuracy %.2f over %d observations", fid, fs.Accuracy, fs.Total)
		}
	}
}

func TestNonClozeDirectionsDoNotGradeFeatures(t *testing.T) {
	e, uid := newEngine(t)
	now := time.Now().UTC()

	_, cards, _ := e.Store().CreateNote(clozeNote(uid), []Direction{Produce}, now)
	note, _ := e.Store().Note(uid, cards[0].NoteID)

	res, err := e.Answer(uid, cards[0], note, Outcome{Correct: true}, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GradedFeatures) != 0 {
		t.Errorf("produce graded %d features; only cloze should", len(res.GradedFeatures))
	}
	fs, _ := e.Store().FeatureState(nil, uid, "Case=Ine", now)
	if fs.State.Reps != 0 {
		t.Error("a free-production answer must not credit a specific suffix")
	}
}

func TestAttributedErrorBlamesOnlyTheGuiltyFeature(t *testing.T) {
	e, uid := newEngine(t)
	now := time.Now().UTC()

	_, cards, _ := e.Store().CreateNote(clozeNote(uid), []Direction{Cloze}, now)
	note, _ := e.Store().Note(uid, cards[0].NoteID)

	res, err := e.Answer(uid, cards[0], note,
		Outcome{Correct: false}, "Case=Ine", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GradedFeatures) != 1 || res.GradedFeatures[0].FeatureID != "Case=Ine" {
		t.Fatalf("want only Case=Ine blamed, got %+v", res.GradedFeatures)
	}
	// The possessive features must be untouched, not merely un-penalised.
	fs, _ := e.Store().FeatureState(nil, uid, "Person[psor]=1", now)
	if fs.Total != 0 {
		t.Error("an innocent feature was graded")
	}
}

func TestAnswerIsAtomic(t *testing.T) {
	// A card whose schedule advanced but whose features did not would leave the
	// two state spaces disagreeing with nothing able to detect it.
	e, uid := newEngine(t)
	now := time.Now().UTC()

	_, cards, _ := e.Store().CreateNote(clozeNote(uid), []Direction{Cloze}, now)
	note, _ := e.Store().Note(uid, cards[0].NoteID)

	if _, err := e.Answer(uid, cards[0], note, Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}

	var cardReps, featureRows int
	_ = e.Store().db.QueryRow(`SELECT reps FROM card WHERE id=?`, cards[0].ID).Scan(&cardReps)
	_ = e.Store().db.QueryRow(`SELECT COUNT(*) FROM feature_review WHERE user_id=?`, uid).Scan(&featureRows)

	if cardReps != 1 || featureRows != 3 {
		t.Errorf("card reps=%d, feature reviews=%d — the two spaces are out of step",
			cardReps, featureRows)
	}
}

func TestMasteryReportsAutomationSeparatelyFromAccuracy(t *testing.T) {
	// Vowel harmony is a motor skill: correct-but-slow is not mastered.
	e, uid := newEngine(t)
	now := time.Now().UTC()

	_, cards, _ := e.Store().CreateNote(clozeNote(uid), []Direction{Cloze}, now)
	note, _ := e.Store().Note(uid, cards[0].NoteID)

	// 25 correct but laboured answers.
	card := cards[0]
	for i := range 25 {
		res, err := e.Answer(uid, card, note,
			Outcome{Correct: true, Duration: 9 * time.Second, MedianDuration: 3 * time.Second},
			"", now.Add(time.Duration(i)*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		card = res.Card
	}

	got, err := e.Mastery(uid, 2500, now.Add(30*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 features in the heatmap, got %d", len(got))
	}
	for _, m := range got {
		if m.Accuracy < 0.9 {
			t.Errorf("%s: accuracy %.2f, expected high", m.FeatureID, m.Accuracy)
		}
		if m.Automated {
			t.Errorf("%s: marked automated despite ~9s answers", m.FeatureID)
		}
	}
}

func TestLemmaStatusDeclarationDoesNotCreateReviewLoad(t *testing.T) {
	// Marking words known while reading must not manufacture cards, or the
	// reader becomes a trap.
	e, uid := newEngine(t)
	if err := e.Store().SetLemmaStatus(uid, 42, "known"); err != nil {
		t.Fatal(err)
	}
	known, err := e.Store().KnownLemmaIDs(uid)
	if err != nil {
		t.Fatal(err)
	}
	if known[42] != "known" {
		t.Errorf("got %v", known)
	}
	var cards int
	_ = e.Store().db.QueryRow(`SELECT COUNT(*) FROM card`).Scan(&cards)
	if cards != 0 {
		t.Errorf("declaring a word known created %d cards", cards)
	}
}

func TestUnknownLemmaStatusIsRejected(t *testing.T) {
	e, uid := newEngine(t)
	if err := e.Store().SetLemmaStatus(uid, 1, "sort-of"); err == nil {
		t.Error("want an error for an invalid status")
	}
}

func TestMasteryReportsNonZeroRetrievabilityAfterAReview(t *testing.T) {
	// Regression: Mastery did not load the `state` column, so every feature
	// arrived at FSRS looking New — and FSRS reports zero retrievability for a
	// New card. The heatmap read 0.00 across the board however much stability
	// had actually accumulated.
	e, uid := newEngine(t)
	now := time.Now().UTC()

	_, cards, _ := e.Store().CreateNote(clozeNote(uid), []Direction{Cloze}, now)
	note, _ := e.Store().Note(uid, cards[0].NoteID)

	if _, err := e.Answer(uid, cards[0], note,
		Outcome{Correct: true, Duration: 3 * time.Second}, "", now); err != nil {
		t.Fatal(err)
	}

	got, err := e.Mastery(uid, 2500, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("no features in the heatmap")
	}
	for _, m := range got {
		if m.Retrievability <= 0 {
			t.Errorf("%s: retrievability %.3f right after a correct review",
				m.FeatureID, m.Retrievability)
		}
		if m.Retrievability > 1 {
			t.Errorf("%s: retrievability %.3f out of range", m.FeatureID, m.Retrievability)
		}
	}
}

func TestFindDuplicateStopsDoubleAdding(t *testing.T) {
	// A learner rereading a paragraph will press "add card" on a word they
	// already added. Creating a second identical note doubles their review load
	// for no gain while appearing to have worked.
	e, uid := newEngine(t)
	now := time.Now().UTC()
	n := clozeNote(uid)

	if id, err := e.Store().FindDuplicate(uid, n.HU, n.TargetForm); err != nil || id != 0 {
		t.Fatalf("fresh store: got (%d, %v)", id, err)
	}

	created, _, err := e.Store().CreateNote(n, []Direction{Cloze}, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.Store().FindDuplicate(uid, n.HU, n.TargetForm)
	if err != nil {
		t.Fatal(err)
	}
	if got != created {
		t.Errorf("duplicate lookup: got %d, want %d", got, created)
	}

	// A different target form in the same sentence is a genuinely different card.
	other := n
	other.TargetForm = "könyv"
	if id, _ := e.Store().FindDuplicate(uid, other.HU, other.TargetForm); id != 0 {
		t.Error("a different target form was wrongly treated as a duplicate")
	}
}
