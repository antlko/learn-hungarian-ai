package writing

import (
	"regexp"
	"strings"
	"testing"
)

func TestTopicsCatalogueLoads(t *testing.T) {
	all, err := Topics()
	if err != nil {
		t.Fatal(err)
	}
	perLevel := map[string]int{}
	perGenre := map[string]map[string]int{}
	hu := regexp.MustCompile(`[őűŐŰáéíóöúüÁÉÍÓÖÚÜ]`) // Cyrillic/English briefs must not carry Hungarian
	for _, tp := range all {
		perLevel[tp.Level]++
		if perGenre[tp.Level] == nil {
			perGenre[tp.Level] = map[string]int{}
		}
		perGenre[tp.Level][tp.Genre]++
		for _, s := range []string{tp.TitleEN, tp.BriefEN, tp.TitleRU, tp.BriefRU} {
			if hu.MatchString(s) {
				t.Errorf("%s: Hungarian text in %q", tp.Code, s)
			}
		}
	}
	for _, lv := range Levels {
		if perLevel[lv] != 8 {
			t.Errorf("level %s has %d topics, want 8", lv, perLevel[lv])
		}
		for _, g := range Genres {
			if perGenre[lv][g] < 1 {
				t.Errorf("level %s has no %s topic", lv, g)
			}
		}
	}
	if tp, ok := FindTopic(all[0].Code); !ok || tp.Code != all[0].Code {
		t.Error("FindTopic failed")
	}
	if _, ok := FindTopic("nope"); ok {
		t.Error("FindTopic found a missing code")
	}
}

func TestParseTopicsRejectsMalformed(t *testing.T) {
	good := `{"code":"a","level":"A1","genre":"email","titleRu":"т","titleEn":"t","briefRu":"б","briefEn":"b","minWords":40,"maxWords":60}`
	cases := map[string]struct{ json, want string }{
		"not json":  {`{`, "writing topics"},
		"format":    {`{"format":2,"topics":[` + good + `]}`, "format 2"},
		"empty":     {`{"format":1,"topics":[]}`, "no topics"},
		"duplicate": {`{"format":1,"topics":[` + good + `,` + good + `]}`, "duplicate code"},
		"level":     {`{"format":1,"topics":[` + strings.Replace(good, `"A1"`, `"C9"`, 1) + `]}`, "unknown level"},
		"genre":     {`{"format":1,"topics":[` + strings.Replace(good, `"email"`, `"poem"`, 1) + `]}`, "unknown genre"},
		"range":     {`{"format":1,"topics":[` + strings.Replace(good, `"maxWords":60`, `"maxWords":10`, 1) + `]}`, "bad word range"},
		"brief":     {`{"format":1,"topics":[` + strings.Replace(good, `"briefEn":"b"`, `"briefEn":""`, 1) + `]}`, "empty title or brief"},
		"code":      {`{"format":1,"topics":[` + strings.Replace(good, `"code":"a"`, `"code":""`, 1) + `]}`, "empty code"},
	}
	for name, c := range cases {
		_, err := ParseTopics([]byte(c.json))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want containing %q", name, err, c.want)
		}
	}
	if _, err := ParseTopics([]byte(`{"format":1,"topics":[` + good + `]}`)); err != nil {
		t.Errorf("valid file rejected: %v", err)
	}
}

func TestTypesAndHints(t *testing.T) {
	if len(Types) != 17 || Types[len(Types)-1] != TypeOther {
		t.Errorf("Types = %v", Types)
	}
	for ty, code := range grammarTopicHint {
		if !contains(Types, ty) || code == "" {
			t.Errorf("bad hint %q -> %q", ty, code)
		}
	}
	if GrammarTopicHint(TypeHarmony) != "vowel-harmony" || GrammarTopicHint(TypeArticle) != "" {
		t.Error("hint lookup wrong")
	}
	for in, want := range map[string]string{"Vowel length": "vowel_length", "case (illative)": "case", "WORD-ORDER": "word_order", "banana": "other", "": "other"} {
		if got := normalizeType(in); got != want {
			t.Errorf("normalizeType(%q) = %q, want %q", in, got, want)
		}
	}
}
