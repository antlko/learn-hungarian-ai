package pictures

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestCatalogLoads(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(c.All()); n < 60 || n > 150 {
		t.Fatalf("entries = %d, want 60..150", n)
	}
	if len(c.Places()) < 8 || len(c.Objects()) < 50 {
		t.Fatalf("places=%d objects=%d", len(c.Places()), len(c.Objects()))
	}
	if got := c.ByLemma("macska"); len(got) != 1 || got[0].Role != RoleObject {
		t.Fatalf("macska: %+v", got)
	}
	for _, p := range c.Places() {
		for rel, cs := range p.Relations {
			if rel == "" || (cs != "Ine" && cs != "Sup") {
				t.Errorf("%s: bad relation %q=%q", p.ID, rel, cs)
			}
		}
	}
	// Every embedded svg is referenced by an entry.
	used := map[string]bool{}
	for _, e := range c.All() {
		used[e.Pic] = true
	}
	ents, _ := fs.ReadDir(files, "svg")
	for _, de := range ents {
		if !used[de.Name()] {
			t.Errorf("svg/%s is not referenced by the catalogue", de.Name())
		}
	}
}

func TestResolveDropsUnknown(t *testing.T) {
	c, _ := Default()
	got := c.Resolve(func(l, _ string) (int64, bool) { return 7, l == "macska" })
	if len(got) != 1 || got[0].LemmaID != 7 {
		t.Fatalf("got %+v", got)
	}
}

var bad = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<script`),
	regexp.MustCompile(`(?i)\son[a-z]+\s*=`),
	regexp.MustCompile(`(?i)javascript:`),
	regexp.MustCompile(`(?i)<foreignObject`),
	regexp.MustCompile(`(?i)<image`),
	regexp.MustCompile(`(?i)url\(\s*['"]?\s*(https?:|//|data:)`),
	regexp.MustCompile(`(?i)<!ENTITY`),
}
var href = regexp.MustCompile(`(?i)(?:xlink:)?href\s*=\s*["']([^"']*)["']`)

func TestSVGSanitised(t *testing.T) {
	total := 0
	ents, _ := fs.ReadDir(files, "svg")
	for _, de := range ents {
		b, _ := files.ReadFile("svg/" + de.Name())
		total += len(b)
		s := string(b)
		for _, re := range bad {
			if re.MatchString(s) {
				t.Errorf("%s matches %s", de.Name(), re)
			}
		}
		for _, m := range href.FindAllStringSubmatch(s, -1) {
			if !strings.HasPrefix(m[1], "#") {
				t.Errorf("%s: external href %q", de.Name(), m[1])
			}
		}
		if !strings.Contains(s, "<svg") {
			t.Errorf("%s is not an svg", de.Name())
		}
	}
	if total > 1500*1024 {
		t.Fatalf("embedded svg total %d bytes > 1.5 MB", total)
	}
}

func TestSizesAndCapacities(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, o := range c.Objects() {
		if SizeRank(o.Size) < 0 {
			t.Errorf("object %s has size %q", o.ID, o.Size)
		}
		if o.Animal && o.Size == SizeTiny {
			t.Errorf("animal %s cannot be tiny", o.ID)
		}
		counts[o.Size]++
	}
	for _, p := range c.Places() {
		if SizeRank(p.Capacity) < 0 || p.Animal {
			t.Errorf("place %s: capacity %q animal %v", p.ID, p.Capacity, p.Animal)
		}
	}
	if counts[SizeTiny] < 8 || counts[SizeSmall] < 15 || counts[SizeMedium] < 15 || counts[SizeLarge] < 20 {
		t.Errorf("implausible size classes: %v", counts)
	}
	e := func(id string) Entry {
		x, ok := c.ByID(id)
		if !ok {
			t.Fatalf("no %s", id)
		}
		return x
	}
	table := map[string]map[string]bool{
		// The cup holds only tiny, non-animal things.
		"cup": {"key": true, "ring": true, "cherry": true, "pen": true, "banana": false, "phone": false, "scissors": false, "orange": false, "glasses": false, "grapes": false, "lemon": false, "apple": false, "cucumber": false, "turtle": false, "frog": false, "mouse": false, "bee": false, "dog": false, "taxi": false},
		// A bag, basket or drawer holds up to medium, but no animal.
		"bag": {"key": true, "book": true, "dog": false, "cat": false, "mouse": false, "taxi": false, "monkey": false},
		// A chair takes small things only: no watermelon.
		"chair": {"key": true, "book": true, "watermelon": false, "dog": false},
		// Large capacity admits animals up to medium; large things and monkeys never.
		"box":   {"dog": true, "cat": true, "pineapple": true, "monkey": false, "taxi": false, "horse": false},
		"house": {"dog": true, "hospital": false, "computer": false},
	}
	for place, objs := range table {
		for obj, want := range objs {
			if got := FitsEntry(e(obj), e(place)); got != want {
				t.Errorf("%s in/on %s: %v, want %v", obj, place, got, want)
			}
		}
	}
	if Fits("", "large") || Fits("small", "") {
		t.Error("unknown sizes fit nothing")
	}
}
