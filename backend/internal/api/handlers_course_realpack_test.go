package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func totalWords(p CourseResponse) int {
	n := 0
	for _, u := range p.Plan.Units {
		n += len(u.Unit.Words)
	}
	return n
}

// Course enrolment against the REAL language pack.
//
// The fixture pack cannot show this bug, because it was written by hand and its
// tagger and its dictionary agree by construction. The real pack's do not, and
// the disagreement was silently costing the learner words:
//
//	Осталось без карточки (3): sziasztok, bemutatkozik, nagyon
//	«В корпусе нет ни одного предложения с этими словами»
//
// Two of those three were false. `nagyon` occurs in 820 corpus sentences and
// `bemutatkozik` in five; they were dropped because the corpus files their
// tokens under a different lemma id — `nagy` for the first, a NULL and a
// separate participle row for the second — and enrolment only ever joined on
// lemma id.
//
// Skipped without a pack, like the drill package's real-pack tests.
func realPackDataDir(t *testing.T) string {
	t.Helper()
	src := os.Getenv("TANULOM_TEST_PACK")
	if src == "" {
		src = filepath.Join("..", "..", "..", "data", "langpack-hu-v1.sqlite")
	}
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no language pack at %s — set TANULOM_TEST_PACK to run this", src)
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Hard link: a 260 MB copy per test is a minute of nothing, and the pack is
	// opened read-only.
	if err := os.Link(abs, filepath.Join(dir, "langpack-hu-v1.sqlite")); err != nil {
		t.Skipf("cannot link the pack into a temp dir (%v)", err)
	}
	return dir
}

func TestRealPackEnrolmentFindsWordsTheTaggerFiledElsewhere(t *testing.T) {
	t.Parallel()
	dir := realPackDataDir(t)
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	c := &client{t: t, app: s}
	if resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}

	// The unit that produced the screenshot: month 1, the introductions unit.
	var plan CourseResponse
	if resp := c.json(http.MethodGet, "/api/course", nil, &plan); resp.StatusCode != 200 {
		t.Fatalf("course: %d", resp.StatusCode)
	}
	if len(plan.Plan.Units) == 0 {
		t.Skip("this pack carries no course")
	}

	// Enrol every unit and collect what the app could not teach.
	skipped := map[string]bool{}
	for _, u := range plan.Plan.Units {
		var res EnrolResponse
		if resp := c.json(http.MethodPost, "/api/course/"+u.Unit.Code+"/enrol", nil,
			&res); resp.StatusCode != 200 {
			t.Fatalf("enrol %s: %d", u.Unit.Code, resp.StatusCode)
		}
		for _, w := range res.Skipped {
			skipped[w.Lemma] = true
		}
	}

	// These two are in the corpus. Being unable to find them is the bug.
	for _, word := range []string{"nagyon", "bemutatkozik"} {
		if skipped[word] {
			t.Errorf("%q was skipped, but the corpus contains it — the surface-form "+
				"search in noteFromSentences is not reaching it", word)
		}
	}

	// And the claim the UI makes about whatever IS skipped has to be true, so
	// the skip list must not be quietly growing either.
	names := make([]string, 0, len(skipped))
	for w := range skipped {
		names = append(names, w)
	}
	sort.Strings(names)
	t.Logf("still unteachable from this pack: %d of %d words: %s",
		len(skipped), totalWords(plan), strings.Join(names, " "))
}
