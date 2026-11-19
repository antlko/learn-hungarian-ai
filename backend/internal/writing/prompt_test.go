package writing

import (
	"regexp"
	"strings"
	"testing"

	"tanulom/internal/lang"
)

const essay = "Tegnap a boltban mentem. Vettem egy kenyér és tejet."

func TestBuildRussianGolden(t *testing.T) {
	p := Build(Params{Lang: "ru", Level: "A1", TopicTitle: "Вчера", TopicBrief: "Расскажите.", MinWords: 40, MaxWords: 65, Text: essay + "\r\n"})
	for _, want := range []string{
		"- The learner works at CEFR level A1.\n",
		"- Task: Вчера. Расскажите.\n",
		"- Suggested length: 40–65 words.\n",
		"- The learner's own language is Russian.\n",
		"5. Write HINT, WHY, RULE, SUMMARY, STRENGTH and VOCAB notes in Russian.",
		"EXAMPLE: <one more correct Hungarian sentence that uses the same rule> — <its translation into Russian>",
		"starts with ```text and ends with ```.",
		"\nTHE LEARNER'S TEXT\n<<<\n" + essay + "\n>>>\n",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "{{") || strings.Contains(p, "@@@") {
		t.Error("unsubstituted placeholder")
	}
	if !strings.HasPrefix(p, "You are an experienced teacher of Hungarian") {
		t.Error("prompt does not open with the role line")
	}
}

func TestBuildEnglishAndVariants(t *testing.T) {
	p := Build(Params{Lang: "en", Text: essay})
	for _, want := range []string{
		"CEFR level A2.", "- Task: free topic chosen by the learner.\n",
		"- The learner's own language is English.\n", "translation into English>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "Suggested length") {
		t.Error("length line present without a range")
	}
	// Text containing a placeholder is not substituted into.
	if q := Build(Params{Text: "a {{LEVEL}} b"}); !strings.Contains(q, "<<<\na {{LEVEL}} b\n>>>") {
		t.Error("essay was altered")
	}
}

func TestPromptNeverContainsONLY(t *testing.T) {
	for _, l := range []string{"ru", "en"} {
		if strings.Contains(Build(Params{Lang: l, TopicTitle: "x", Text: essay}), "ONLY") {
			t.Errorf("prompt (%s) contains the word ONLY", l)
		}
	}
}

func exampleBlock(t *testing.T) string {
	t.Helper()
	p := Build(Params{Lang: "en", Text: essay})
	_, after, ok := strings.Cut(p, "AN EXAMPLE OF THE FORMAT")
	if !ok {
		t.Fatal("no example in prompt")
	}
	_, after, _ = strings.Cut(after, "```text\n")
	block, _, ok := strings.Cut(after, "\n```")
	if !ok {
		t.Fatal("example block not closed")
	}
	return block
}

// The worked example must be parseable by our own parser and its quotes must
// land in a text that contains them, or the example teaches a broken format.
func TestPromptExampleParsesAndAnchors(t *testing.T) {
	res := Parse(exampleBlock(t))
	if res.Status != StatusOK || len(res.Errors) != 2 || res.Truncated || res.NoHeader || res.Version != 1 {
		t.Fatalf("example parse = %+v", res)
	}
	segs, anchored := Anchor(essay, res.Errors)
	if !anchored[0] || !anchored[1] {
		t.Fatalf("anchored = %v", anchored)
	}
	var hit []string
	for _, s := range segs {
		if len(s.Errors) > 0 {
			hit = append(hit, s.Text)
		}
	}
	if strings.Join(hit, "|") != "boltban mentem|egy kenyér" {
		t.Errorf("highlighted = %q", hit)
	}
}

// The example's Hungarian is model-written; check it against the starter pack.
func TestPromptExampleFormsAreInThePack(t *testing.T) {
	dir := t.TempDir()
	version, _, err := lang.EnsurePack(dir)
	if err != nil {
		t.Skipf("starter pack unavailable: %v", err)
	}
	pack, err := lang.Open(lang.Path(dir, version))
	if err != nil {
		t.Skipf("starter pack does not open: %v", err)
	}
	defer pack.Close()
	block := exampleBlock(t)
	for _, form := range []string{"boltba", "kenyeret", "tejet", "parkba", "levelet"} {
		if !regexp.MustCompile(`(?i)\b` + form + `\b`).MatchString(block) {
			t.Errorf("%s is not used in the example", form)
		}
		var n int
		if err := pack.DB().QueryRow(`SELECT COUNT(*) FROM wordform WHERE form = ?`, form).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("form %q is not in the starter pack", form)
		}
	}
}
