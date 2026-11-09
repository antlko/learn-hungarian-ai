package api

import (
	"net/http"
	"testing"
	"time"

	"tanulom/internal/course"
	"tanulom/internal/srs"
)

// seedCard writes a note + one Review-state card for a lemma the way a
// graduated word would look: `days` is the stored interval. The settled rule
// (ADR-058) reads the interval through srs.StageOf, so the fixture sets only
// that and the flags it must respect.
func seedCard(t *testing.T, s *Server, lemmaID int64, days int, suspended, trashed bool) {
	t.Helper()
	var uid int64
	if err := s.db.QueryRow(`SELECT id FROM user LIMIT 1`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	trash := any(nil)
	if trashed {
		trash = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}
	res, err := s.db.Exec(`INSERT INTO note (user_id, kind, lemma_id, hu, target_form, trashed_at)
		VALUES (?, 'vocab', ?, 'szó', 'szó', ?)`, uid, lemmaID, trash)
	if err != nil {
		t.Fatal(err)
	}
	nid, _ := res.LastInsertId()
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	sus := 0
	if suspended {
		sus = 1
	}
	if _, err := s.db.Exec(`INSERT INTO card (note_id, direction, due, stability, difficulty,
		scheduled_days, reps, state, last_review, suspended) VALUES (?,'recognize',?,?,5,?,3,2,?,?)`,
		nid, now, float64(days), days, now, sus); err != nil {
		t.Fatal(err)
	}
	s.invalidateCounted(uid)
}

func settledOf(p CourseResponse, code string) int {
	for _, st := range p.Plan.Units {
		if st.Unit.Code == code {
			return st.Settled
		}
	}
	return -1
}

func TestCourseSettledCountsKnownAndMatureOnly(t *testing.T) {
	s, c := newCourseServer(t, true)
	var fresh CourseResponse
	c.json(http.MethodGet, "/api/course", nil, &fresh)
	if settledOf(fresh, "b1") != 0 || fresh.Totals.Settled != 0 {
		t.Fatalf("fresh deck settled = %+v", fresh.Totals)
	}

	// b1: ház (1) mature; könyv (2) young, plus a suspended-mature and a
	// trashed-mature card, which must not make it settled.
	declare(t, s, 1, "learning")
	declare(t, s, 2, "learning")
	seedCard(t, s, 1, srs.MatureDays, false, false)
	seedCard(t, s, 2, srs.MatureDays-1, false, false)
	seedCard(t, s, 2, 90, true, false)
	seedCard(t, s, 2, 90, false, true)
	// i1: lát (3) is known. a1: egy (6) learning on its first step.
	declare(t, s, 3, "known")
	declare(t, s, 6, "learning")

	var got CourseResponse
	c.json(http.MethodGet, "/api/course", nil, &got)
	if n := settledOf(got, "b1"); n != 1 {
		t.Errorf("b1 settled = %d, want 1 (mature counts; young, suspended, trashed do not)", n)
	}
	if n := settledOf(got, "i1"); n != 1 {
		t.Errorf("i1 settled = %d, want 1 (known counts)", n)
	}
	if n := settledOf(got, "a1"); n != 0 {
		t.Errorf("a1 settled = %d, want 0", n)
	}
	if n := settledOf(got, "t1"); n != 0 {
		t.Errorf("theme with nothing in play settled = %d", n)
	}
	tot := got.Totals
	if tot.Settled != 2 || tot.PerTier["basic"].Settled != 1 ||
		tot.PerTier["intermediate"].Settled != 1 || tot.PerTier["advanced"].Settled != 0 {
		t.Errorf("totals = %+v", tot)
	}
	// Settled is a subset of the deck and never exceeds learned's universe.
	for _, st := range got.Plan.Units {
		if st.Settled > st.WordsInPlay {
			t.Errorf("%s: settled %d > inPlay %d", st.Unit.Code, st.Settled, st.WordsInPlay)
		}
	}
}

func TestCourseResponseCarriesUnlockRule(t *testing.T) {
	_, c := newCourseServer(t, true)
	var got CourseResponse
	c.json(http.MethodGet, "/api/course", nil, &got)
	if got.Unlock.Share != course.UnlockShare || got.Unlock.SettledDays != srs.MatureDays {
		t.Errorf("unlock = %+v, want the course/srs constants", got.Unlock)
	}
	if want := (course.Unlock{Share: 0.7, SettledDays: 21}); got.Unlock != want {
		t.Errorf("unlock = %+v, want %+v", got.Unlock, want)
	}
}

// A pack without the tier column resolves, and settled is simply 0 for a fresh
// learner: the older pack is unaffected.
func TestCourseSettledOnOlderPack(t *testing.T) {
	s, c := newCourseServer(t, false)
	declare(t, s, 3, "known")
	var got CourseResponse
	c.json(http.MethodGet, "/api/course", nil, &got)
	if settledOf(got, "i1") != 1 || got.Unlock.Share != 0.7 {
		t.Errorf("older pack: %+v", got.Totals)
	}
}
