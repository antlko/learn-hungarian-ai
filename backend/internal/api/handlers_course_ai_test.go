package api

import (
	"net/http"
	"testing"
)

func newAICourseServer(t *testing.T, answer string) (*Server, *client) {
	t.Helper()
	s, c := newGreetingServer(t, [][2]string{{"szia", "INTJ"}, {"sziasztok", "INTJ"}}, func(exec func(string, ...any)) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (20,'szia','INTJ',900)`)
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (21,'sziasztok','INTJ',901)`)
		exec(`INSERT INTO wordform VALUES ('szia',20,'INTJ','','wiktionary','high')`)
		exec(`INSERT INTO sentence VALUES (201,'Szia!','test','CC0',1,1.0)`)
		exec(`INSERT INTO sentence_token VALUES (201,0,'',20,'')`)
	})
	fake, _ := fakeLLM(t, answer)
	enableLLM(t, s, fake.URL)
	return s, c
}

const aiGood = `{"sentences":[
 {"hu":"Sziasztok, jó reggelt!","tr":"Привет всем, доброе утро!"},
 {"hu":"Jó reggelt mindenkinek!","tr":"x"},
 {"hu":"Sziasztok egy kettő három négy öt hat hét nyolc kilenc tíz tizenegy tizenkettő.","tr":"too long"},
 {"hu":"Sziasztok!","tr":"Привет!"}]}`

func TestCourseAIProposeValidatesHeadword(t *testing.T) {
	t.Parallel()
	s, c := newAICourseServer(t, aiGood)
	c.json(http.MethodPost, "/api/course/g1/enrol", nil, nil)

	var out CourseProposeResponse
	if resp := c.json(http.MethodPost, "/api/course/units/g1/words/sziasztok/propose", nil, &out); resp.StatusCode != 200 {
		t.Fatalf("propose: %d", resp.StatusCode)
	}
	if out.Model != "test-model" || out.Lemma != "sziasztok" {
		t.Errorf("response = %+v", out)
	}
	// The sentence without the headword and the 13-token one are rejected.
	if len(out.Proposals) != 2 || out.Proposals[0].HU != "Sziasztok, jó reggelt!" || out.Proposals[1].HU != "Sziasztok!" {
		t.Errorf("proposals = %+v, want the two valid ones", out.Proposals)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM note`).Scan(&n)
	if n != 1 { // only «szia» from enrolment
		t.Errorf("notes = %d: proposing must store nothing", n)
	}
}

func TestCourseAIAcceptMakesALabelledCourseNote(t *testing.T) {
	t.Parallel()
	s, c := newAICourseServer(t, aiGood)
	c.json(http.MethodPost, "/api/course/g1/enrol", nil, nil)

	noExample := func() []string {
		var plan CourseResponse
		c.json(http.MethodGet, "/api/course", nil, &plan)
		for _, st := range plan.Plan.Units {
			if st.Unit.Code == "g1" {
				return st.NoExample
			}
		}
		return nil
	}
	if got := noExample(); len(got) != 1 || got[0] != "sziasztok" {
		t.Fatalf("noExample before = %v", got)
	}

	// A sentence without the headword is refused, as is a stranger's word.
	bad := CourseAcceptRequest{HU: "Jó reggelt!", TR: "x"}
	if resp := c.json(http.MethodPost, "/api/course/units/g1/words/sziasztok/accept", bad, nil); resp.StatusCode != 400 {
		t.Errorf("sentence without headword = %d, want 400", resp.StatusCode)
	}
	good := CourseAcceptRequest{HU: "Sziasztok, jó reggelt!", TR: "Привет всем"}
	var out CourseAcceptResponse
	if resp := c.json(http.MethodPost, "/api/course/units/g1/words/sziasztok/accept", good, &out); resp.StatusCode != 200 || out.Duplicate {
		t.Fatalf("accept = %d %+v", resp.StatusCode, out)
	}

	var source, feats string
	var sentenceID *int64
	var start, end, cards int
	if err := s.db.QueryRow(`SELECT source, target_feats, sentence_id, target_start, target_end
		FROM note WHERE hu = ?`, good.HU).Scan(&source, &feats, &sentenceID, &start, &end); err != nil {
		t.Fatal(err)
	}
	if source != "course:g1" || feats != "" || sentenceID != nil || start != 0 || end != len("Sziasztok") {
		t.Errorf("note = %q feats %q sid %v span %d..%d", source, feats, sentenceID, start, end)
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM card c JOIN note n ON n.id=c.note_id WHERE n.hu = ?`, good.HU).Scan(&cards)
	if cards == 0 {
		t.Error("accepting must create cards")
	}
	if got := noExample(); len(got) != 0 {
		t.Errorf("noExample after = %v, want none", got)
	}

	// Idempotent: the same sentence again makes no second note, and the word is
	// now in the deck, so it is no longer eligible at all.
	if resp := c.json(http.MethodPost, "/api/course/units/g1/words/sziasztok/accept", good, nil); resp.StatusCode != 409 {
		t.Errorf("second accept = %d, want 409", resp.StatusCode)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM note WHERE hu = ?`, good.HU).Scan(&n)
	if n != 1 {
		t.Errorf("notes for the sentence = %d, want 1", n)
	}
}

func TestCourseAIRefusesIneligibleWords(t *testing.T) {
	t.Parallel()
	s, c := newAICourseServer(t, aiGood)
	post := func(word string) int {
		return c.json(http.MethodPost, "/api/course/units/g1/words/"+word+"/propose", nil, nil).StatusCode
	}
	if got := post("sziasztok"); got != 409 {
		t.Errorf("unit not taken up = %d, want 409", got)
	}
	c.json(http.MethodPost, "/api/course/g1/enrol", nil, nil)
	if got := post("szia"); got != 409 {
		t.Errorf("word with a pack example = %d, want 409", got)
	}
	if got := post("nincs"); got != 404 {
		t.Errorf("word not in the unit = %d, want 404", got)
	}
	if got := c.json(http.MethodPost, "/api/course/units/zz/words/szia/propose", nil, nil).StatusCode; got != 404 {
		t.Errorf("unknown unit = %d, want 404", got)
	}
	// «Знаю»: the word is in play as known, so it is not offered.
	if _, err := s.db.Exec(`INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (1, 21, 'known')`); err != nil {
		t.Fatal(err)
	}
	if got := post("sziasztok"); got != 409 {
		t.Errorf("known word = %d, want 409", got)
	}
}

func TestCourseAIWithoutAModelIsRefusedClearly(t *testing.T) {
	t.Parallel()
	_, c := newGreetingServer(t, [][2]string{{"sziasztok", "INTJ"}}, func(exec func(string, ...any)) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (21,'sziasztok','INTJ',901)`)
	})
	c.json(http.MethodPost, "/api/course/g1/enrol", nil, nil)
	var body map[string]string
	if resp := c.json(http.MethodPost, "/api/course/units/g1/words/sziasztok/propose", nil, &body); resp.StatusCode != 503 || body["error"] != "errors.llmOff" {
		t.Errorf("no model = %d %v, want 503 errors.llmOff", resp.StatusCode, body)
	}
}
