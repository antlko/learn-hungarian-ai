package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"tanulom/internal/drill"
)

// Two batches in a row share no item, whether or not the first was answered,
// the served JSON carries no answer, and the log keeps what the next request
// needs to remember.
func TestDrillNextVariesAndLogsItemKeys(t *testing.T) {
	t.Parallel()
	s, c := newPackedServer(t)

	resp, body := c.do(http.MethodGet, "/api/drill/next?topic=locative-grid&count=3", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("drill/next: %d %s", resp.StatusCode, body)
	}
	// The expected answer never travels: no field for it, in any item.
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Items) == 0 {
		t.Fatalf("no items: %s", body)
	}
	for _, it := range raw.Items {
		for k := range it {
			if strings.EqualFold(k, "expected") || strings.EqualFold(k, "answer") {
				t.Errorf("served item has a %q field: %v", k, it)
			}
		}
	}
	var first DrillNextResponse
	if err := json.Unmarshal(body, &first); err != nil {
		t.Fatal(err)
	}

	// Fetched again BEFORE answering: served-but-unanswered items count as seen.
	var second DrillNextResponse
	c.json(http.MethodGet, "/api/drill/next?topic=locative-grid&count=3", nil, &second)
	keys := map[string]bool{}
	for _, it := range first.Items {
		keys[it.Key()] = true
	}
	for _, it := range second.Items {
		if keys[it.Key()] {
			t.Errorf("item %s was served again before it was answered", it.Key())
		}
	}

	// Answer the first batch; the log must carry the key and the sentence.
	for _, it := range first.Items {
		id, expected := peekAnswer(t, s, it.ID)
		var res DrillAnswerResponse
		if r := c.json(http.MethodPost, "/api/drill/answer",
			DrillAnswerRequest{ItemID: id, Answer: expected, DurationMs: 800}, &res); r.StatusCode != 200 {
			t.Fatalf("answer: %d", r.StatusCode)
		}
	}
	rows, err := s.db.DB.Query(`SELECT kind, item_key, corpus_sentence FROM drill_attempt ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var kind, key string
		var sent int64
		if err := rows.Scan(&kind, &key, &sent); err != nil {
			t.Fatal(err)
		}
		n++
		if !strings.HasPrefix(key, kind+"|") {
			t.Errorf("item_key %q does not start with the kind %q", key, kind)
		}
	}
	if n != len(first.Items) {
		t.Errorf("%d attempts logged, want %d", n, len(first.Items))
	}
}

// A question-word item survives the whole round trip: served without its
// answer, graded by string, classified when wrong, logged under its kind.
func TestQwordRoundTrip(t *testing.T) {
	t.Parallel()
	s, c := newPackedServer(t)

	mk := func() drill.Item {
		it := drill.Item{
			TopicCode: "locative-grid", Kind: drill.KindQword, Phase: drill.Blocked,
			PromptKey: "drill.prompt.qword", PromptArgs: map[string]string{"lemma": "ház"},
			Parts: []drill.FramePart{
				{Text: "A "}, {Text: "házban", Focus: true}, {Text: " van egy nagy könyv."}},
			Lemma: "ház", LemmaID: 1, SentenceID: 104,
			Options: []drill.Option{{Text: "hol", Attested: true}, {Text: "hova", Attested: true},
				{Text: "honnan", Attested: true}},
			FeatureIDs: []string{"Case=Ine"}, Headline: "Case=Ine", Attested: true,
		}
		return it.WithExpected("hol")
	}

	for _, tc := range []struct {
		answer  string
		correct bool
		class   string
	}{{"hol", true, ""}, {"hova", false, "direction"}, {"honnan", false, "source"}} {
		served, err := s.pending.Put(1, []drill.Item{mk()})
		if err != nil {
			t.Fatal(err)
		}
		wire, _ := json.Marshal(served[0])
		if strings.Contains(string(wire), `"hol"`) && !strings.Contains(string(wire), `"options"`) {
			t.Errorf("the answer leaked outside the options: %s", wire)
		}
		if !strings.Contains(string(wire), `"parts":[{"text":"A "},{"text":"házban","focus":true},`) {
			t.Errorf("parts do not serialise as documented: %s", wire)
		}
		var res DrillAnswerResponse
		if r := c.json(http.MethodPost, "/api/drill/answer",
			DrillAnswerRequest{ItemID: served[0].ID, Answer: tc.answer, DurationMs: 700}, &res); r.StatusCode != 200 {
			t.Fatalf("answer %q: %d", tc.answer, r.StatusCode)
		}
		if res.Correct != tc.correct || res.Expected != "hol" {
			t.Errorf("%q: correct=%v expected=%q", tc.answer, res.Correct, res.Expected)
		}
		if res.Rating > 3 {
			t.Errorf("a tap counts at most Good, got rating %d", res.Rating)
		}
		var class, kind string
		if err := s.db.DB.QueryRow(`SELECT error_class, kind FROM drill_attempt ORDER BY id DESC LIMIT 1`).
			Scan(&class, &kind); err != nil {
			t.Fatal(err)
		}
		if class != tc.class || kind != "qword" {
			t.Errorf("%q: logged class=%q kind=%q, want %q/qword", tc.answer, class, kind, tc.class)
		}
	}
}

// A malformed match answer is a 400 BEFORE the item is taken, so the learner
// can fix it; the served JSON carries no answer; a valid answer is graded and
// returns per-tile verdicts like sort.
func TestMatchRoundTrip(t *testing.T) {
	t.Parallel()
	s, c := newPackedServer(t)

	mk := func() drill.Item {
		it := drill.Item{
			TopicCode: "locative-grid", Kind: drill.KindMatch, Phase: drill.Blocked,
			PromptKey: "drill.prompt.match", PromptArgs: map[string]string{"lemma": "ház", "cue": "qword"},
			Lemma: "ház", LemmaID: 1,
			Columns:    []string{"hol", "hova", "honnan"},
			Tiles:      []drill.Tile{{ID: "t0", Text: "házból"}, {ID: "t1", Text: "házban"}, {ID: "t2", Text: "házba"}},
			FeatureIDs: []string{"Case=Ela", "Case=Ill", "Case=Ine"}, Headline: "Case=Ela+Case=Ill+Case=Ine",
			Attested: true,
		}
		return it.WithExpected("t0=honnan\nt1=hol\nt2=hova")
	}
	served, err := s.pending.Put(1, []drill.Item{mk()})
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(served[0])
	if strings.Contains(string(wire), "t0=honnan") || strings.Contains(string(wire), `"expected"`) {
		t.Errorf("the answer leaked: %s", wire)
	}
	id := served[0].ID

	for name, parts := range map[string]map[string]string{
		"missing":   {"t0": "honnan", "t1": "hol"},
		"two under": {"t0": "hol", "t1": "hol", "t2": "hova"},
		"bad cue":   {"t0": "honnan", "t1": "hol", "t2": "nobody"},
	} {
		resp, _ := c.do(http.MethodPost, "/api/drill/answer",
			DrillAnswerRequest{ItemID: id, Parts: parts, DurationMs: 900})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}
	// Still answerable: none of those took the item.
	var res DrillAnswerResponse
	if r := c.json(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{
		ItemID: id, Parts: map[string]string{"t0": "honnan", "t1": "hol", "t2": "hova"}, DurationMs: 900}, &res); r.StatusCode != 200 {
		t.Fatalf("a valid answer: %d", r.StatusCode)
	}
	if !res.Correct || len(res.PartResults) != 3 {
		t.Errorf("want a correct answer with 3 part results, got %+v", res)
	}
	var kind, key string
	if err := s.db.DB.QueryRow(`SELECT kind, item_key FROM drill_attempt ORDER BY id DESC LIMIT 1`).Scan(&kind, &key); err != nil ||
		kind != "match" || !strings.HasPrefix(key, "match|1|") {
		t.Errorf("logged %q %q (%v)", kind, key, err)
	}
}

// A scene item: picture on the wire, no answer; a wrong form is classified.
func TestSceneRoundTrip(t *testing.T) {
	t.Parallel()
	s, c := newPackedServer(t)
	it := drill.Item{
		TopicCode: "locative-grid", Kind: drill.KindScene, Phase: drill.Blocked,
		PromptKey: "drill.prompt.scene", PromptArgs: map[string]string{"question": "hova", "place": "ház"},
		Lemma: "ház", LemmaID: 1,
		Options:    []drill.Option{{Text: "házban", Attested: true}, {Text: "házba", Attested: true}, {Text: "házból", Attested: true}},
		FeatureIDs: []string{"Case=Ill"}, Headline: "Case=Ill", Attested: true,
		Picture: &drill.Picture{Place: "house", Object: "book", Relation: "in", Motion: "into",
			AltEn: "book going into the house", AltRu: "книга: движется внутрь (дом)"},
	}
	served, err := s.pending.Put(1, []drill.Item{it.WithExpected("házba")})
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(served[0])
	if !strings.Contains(string(wire), `"picture":{"place":"house","object":"book","relation":"in","motion":"into"`) ||
		strings.Contains(string(wire), `"expected"`) {
		t.Errorf("wire: %s", wire)
	}
	var res DrillAnswerResponse
	if r := c.json(http.MethodPost, "/api/drill/answer",
		DrillAnswerRequest{ItemID: served[0].ID, Answer: "házba", DurationMs: 700}, &res); r.StatusCode != 200 || !res.Correct || res.Rating > 3 {
		t.Fatalf("right answer: %d %+v", r.StatusCode, res)
	}
}
