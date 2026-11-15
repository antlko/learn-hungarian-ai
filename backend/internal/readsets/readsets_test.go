package readsets

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func load(t *testing.T) *Catalog {
	t.Helper()
	data, err := os.ReadFile("testdata/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestOrderAndGet(t *testing.T) {
	c := load(t)
	var codes []string
	for _, s := range c.Sets() {
		codes = append(codes, s.Code)
	}
	if got := strings.Join(codes, ","); got != "m1-u1-dialogue,m1-u1,m1-u1-long,m1-u1-talk,t-x" {
		t.Errorf("order: %s", got)
	}
	if s, ok := c.Get("m1-u1"); !ok || s.Unit != "m1-u1" || s.Register != "standard" {
		t.Errorf("get: %+v %v", s, ok)
	}
	if _, ok := c.Get("nope"); ok {
		t.Error("unknown code found")
	}
	if len(c.Hash()) != 16 {
		t.Errorf("hash %q", c.Hash())
	}
}

func TestBodyJoins(t *testing.T) {
	c := load(t)
	want := map[string]string{
		"m1-u1":          "Látom a házat. Az utcán van.\n\nKöszönöm, jól.",
		"m1-u1-long":     "Látom a házat.\n\nKöszönöm, jól.",
		"m1-u1-dialogue": "– Szia! Hogy vagy?\n– Köszönöm, jól.",
	}
	for code, w := range want {
		s, _ := c.Get(code)
		if got := s.Body(); got != w {
			t.Errorf("%s: %q, want %q", code, got, w)
		}
	}
	// A mini tier is dialogue-form whatever its genre.
	if got := (Set{Tier: "mini", Genre: "x", Lines: []Line{{HU: "Hm."}, {HU: "Ja."}}}).Body(); got != "– Hm.\n– Ja." {
		t.Errorf("mini: %q", got)
	}
}

func TestDialogueFormByGenreAndTier(t *testing.T) {
	for genre, want := range map[string]bool{"dialogue": true, "phone": true, "chat": true, "voicemsg": false, "story": false} {
		if got := (Set{Tier: "talk", Genre: genre}).DialogueForm(); got != want {
			t.Errorf("talk %s: %v, want %v", genre, got, want)
		}
	}
	var f file
	if err := json.Unmarshal([]byte(`{"sets":[{"code":"a","sceneRu":"Р","sceneEn":"E"}]}`), &f); err != nil || f.Sets[0].SceneRU != "Р" || f.Sets[0].SceneEN != "E" {
		t.Errorf("scene: %+v %v", f, err)
	}
}

func TestParseFailsLoudly(t *testing.T) {
	line := `{"hu":"A.","para":0}`
	for name, data := range map[string]string{
		"not json":   `{`,
		"bad format": `{"format":2,"sets":[]}`,
		"no code":    `{"format":1,"sets":[{"code":"","lines":[` + line + `]}]}`,
		"duplicate":  `{"format":1,"sets":[{"code":"a","lines":[` + line + `]},{"code":"a","lines":[` + line + `]}]}`,
		"no lines":   `{"format":1,"sets":[{"code":"a","lines":[]}]}`,
		"empty line": `{"format":1,"sets":[{"code":"a","lines":[{"hu":" "}]}]}`,
	} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
}

// The catalogue the binary really embeds. Skipped until it is generated.
func TestEmbeddedCatalogueIsSound(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatalf("embedded catalogue does not parse: %v", err)
	}
	if len(c.Sets()) == 0 {
		t.Skip("catalogue not generated yet")
	}
	ords := map[int]string{}
	tiers := map[string]map[string]bool{} // unit -> standard tiers
	for _, s := range c.Sets() {
		if len(s.Lines) == 0 {
			t.Errorf("%s: no lines", s.Code)
		}
		if prev, dup := ords[s.Ord]; dup {
			t.Errorf("ord %d shared by %s and %s", s.Ord, prev, s.Code)
		}
		ords[s.Ord] = s.Code
		if s.Unit == "" {
			t.Errorf("%s: no unit", s.Code)
		}
		if s.Register == "standard" {
			if tiers[s.Unit] == nil {
				tiers[s.Unit] = map[string]bool{}
			}
			tiers[s.Unit][s.Tier] = true
		}
	}
	for unit, ts := range tiers {
		for _, tier := range []string{"short", "medium", "long"} {
			if !ts[tier] {
				t.Errorf("unit %s has no standard %s text", unit, tier)
			}
		}
	}
}
