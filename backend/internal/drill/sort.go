package drill

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"tanulom/internal/curriculum"
)

// The sort drill: put six words under their suffixes.
//
//	ház  híd  kert  szék  lány  tó     ->   -ban | -ben
//
// A multi-part CHOICE item (MagyarOK-style "sort by harmony class"). Each tile
// is a lemma, and its column is the allomorph of the ATTESTED inflected form of
// that lemma, found through the same path as the allomorph drill — so an
// exception such as híd → hídon is right by construction and nothing here ever
// decides a column from a harmony rule. A lemma the pack attests with BOTH
// allomorphs is not offered (the answer would be arguable).
//
// Grading is the one subtle part. The item has several tiles but the scheduler
// hears ONE review per required feature: Again when any tile is wrong, Good at
// most when all are right (a tap is recognition). Per-tile reviews would put six
// reviews of one feature in the same second into the feature's FSRS state,
// which is noise, not evidence. See ADR-086.

const (
	sortTiles      = 6
	sortMaxColumns = 3
	// sortDraws bounds the words examined for one item.
	sortDraws = 36
)

// Tile is one word to sort.
type Tile struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	GlossRU string `json:"glossRu,omitempty"`
	GlossEN string `json:"glossEn,omitempty"`

	lemmaID int64
}

// PartResult is the verdict on one tile, sent only after grading.
type PartResult struct {
	ID       string `json:"id"`
	Correct  bool   `json:"correct"`
	Expected string `json:"expected"`
}

// ErrBadParts reports a sort answer that does not fit its item.
var ErrBadParts = errors.New("sort answer does not match the item")

// sortFeatures are the topic's features a sort item can be built for: those
// with two or three distinct allomorphs. The harmony topic sorts by whichever
// early suffix it draws, as the allomorph drill does.
func (g *Generator) sortFeatures(t curriculum.Topic) []Feature {
	pool := g.topicAllomorphFeatures(t)
	if t.MotorSkill() {
		pool = g.harmonyDrillFeatures()
	}
	var out []Feature
	for _, f := range pool {
		if n := len(Distinct(f.Allomorphs)); n >= 2 && n <= sortMaxColumns {
			out = append(out, f)
		}
	}
	return out
}

type sortCand struct {
	w     Word
	col   string
	mixed bool
}

// mixedVowels reports a stem with both a back and a front vowel (`hotel`,
// `papír`): the words that make harmony hard.
func mixedVowels(word string) bool {
	back, front := false, false
	for _, v := range Vowels(word) {
		if backVowels[v] {
			back = true
		} else {
			front = true
		}
	}
	return back && front
}

func (g *Generator) sortItem(req Request) (Item, bool) {
	pool := g.sortFeatures(req.Topic)
	harmonyOnly := req.Topic.MotorSkill()
	if len(pool) == 0 {
		return Item{}, false
	}
	// Each try draws up to sortDraws words, so a few features are plenty.
	for try := range 4 {
		// The first two tries also skip tiles with a long or parenthetical
		// gloss; if that leaves too few, the last two drop just that rule so a
		// session never comes back short. Slang, vulgar words, inflected forms,
		// letters and abbreviations are never accepted.
		strict := try < 2
		f := g.weightedFeature(pool, req)
		cols := Distinct(f.Allomorphs)
		words, spec := g.poolFor(f)

		byCol := map[string][]sortCand{}
		seen := map[int64]bool{}
		total := 0
		for range sortDraws {
			w, ok := g.pickWord(words, req)
			if !ok || seen[w.LemmaID] {
				continue
			}
			seen[w.LemmaID] = true
			if !g.tileWorthy(w, strict) {
				continue
			}
			_, used, _, ok := g.attestedAllomorph(w, f, spec)
			if !ok {
				continue
			}
			byCol[used] = append(byCol[used], sortCand{w: w, col: used, mixed: mixedVowels(w.Lemma)})
			total++
			if total >= sortTiles+4 && g.sortCovered(byCol, cols) {
				break
			}
		}
		picked, ok := pickTiles(byCol, cols, req)
		if !ok {
			continue
		}

		req.Rand.Shuffle(len(picked), func(i, j int) { picked[i], picked[j] = picked[j], picked[i] })
		var tiles []Tile
		var lines []string
		for i, c := range picked {
			id := "t" + strconv.Itoa(i)
			ru, en := g.gloss(c.w.LemmaID)
			ru, en = tileGloss(ru), tileGloss(en)
			tiles = append(tiles, Tile{ID: id, Text: c.w.Lemma, GlossRU: ru, GlossEN: en, lemmaID: c.w.LemmaID})
			lines = append(lines, id+"="+"-"+c.col)
		}
		columns := make([]string, len(cols))
		for i, c := range cols {
			columns[i] = "-" + c
		}

		feats := []string{curriculum.FeatureVowelHarmony}
		headline := curriculum.FeatureVowelHarmony
		if !harmonyOnly {
			feats = []string{f.ID, curriculum.FeatureVowelHarmony}
			headline = f.ID
		}
		return Item{
			TopicCode: req.Topic.Code, Kind: KindSort, Phase: req.Phase,
			PromptKey:  "drill.prompt.sort",
			PromptArgs: map[string]string{"feature": f.ID},
			Columns:    columns, Tiles: tiles,
			FeatureIDs: feats, Headline: headline,
			Attested: true, expected: strings.Join(lines, "\n"),
		}, true
	}
	return Item{}, false
}

// sortCovered reports whether every column has at least two candidates.
func (g *Generator) sortCovered(byCol map[string][]sortCand, cols []string) bool {
	for _, c := range cols {
		if len(byCol[c]) < 2 {
			return false
		}
	}
	return true
}

// pickTiles chooses six candidates, at least one per column, as evenly as the
// material allows, with one mixed-vowel word when any was found.
func pickTiles(byCol map[string][]sortCand, cols []string, req Request) ([]sortCand, bool) {
	for _, c := range cols {
		if len(byCol[c]) == 0 {
			return nil, false
		}
	}
	for _, c := range cols {
		l := byCol[c]
		req.Rand.Shuffle(len(l), func(i, j int) { l[i], l[j] = l[j], l[i] })
		// Mixed-vowel words first inside a column, so they win the quota.
		sort.SliceStable(l, func(i, j int) bool { return l[i].mixed && !l[j].mixed })
	}
	quota := map[string]int{}
	taken := 0
	for round := 0; taken < sortTiles; round++ {
		progressed := false
		for _, c := range cols {
			if taken >= sortTiles {
				break
			}
			if quota[c] < len(byCol[c]) {
				quota[c]++
				taken++
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}
	if taken < sortTiles {
		return nil, false
	}
	var picked []sortCand
	for _, c := range cols {
		picked = append(picked, byCol[c][:quota[c]]...)
	}
	return picked, true
}

// sortKey identifies a sort item: the feature and the SET of lemmas, however
// the tiles were shuffled.
func sortKey(i Item) string {
	ids := make([]string, 0, len(i.Tiles))
	for _, t := range i.Tiles {
		ids = append(ids, strconv.FormatInt(t.lemmaID, 10))
	}
	sort.Strings(ids)
	return string(KindSort) + "|" + i.PromptArgs["feature"] + "|" + strings.Join(ids, ",")
}

// sortLemmasOfKey reads the lemma ids back out of a sort item key.
func sortLemmasOfKey(key string) []int64 {
	parts := strings.SplitN(key, "|", 3)
	if len(parts) < 3 {
		return nil
	}
	var out []int64
	for _, s := range strings.Split(parts[2], ",") {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id != 0 {
			out = append(out, id)
		}
	}
	return out
}

// expectedParts decodes the canonical `tileID=column` lines.
func expectedParts(expected string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(expected, "\n") {
		if id, col, ok := strings.Cut(line, "="); ok {
			out[id] = col
		}
	}
	return out
}

// CheckParts validates a sort answer against the item: every tile answered,
// nothing extra, every column one of the item's.
func (i Item) CheckParts(parts map[string]string) error {
	if len(parts) != len(i.Tiles) {
		return fmt.Errorf("%w: expected %d tiles, got %d", ErrBadParts, len(i.Tiles), len(parts))
	}
	for _, t := range i.Tiles {
		col, ok := parts[t.ID]
		if !ok {
			return fmt.Errorf("%w: tile %q is missing", ErrBadParts, t.ID)
		}
		valid := false
		for _, c := range i.Columns {
			if strings.TrimSpace(col) == c {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("%w: tile %q has no such column", ErrBadParts, t.ID)
		}
	}
	if i.Kind == KindMatch {
		// A match is a bijection: every cue gets exactly one form.
		used := map[string]bool{}
		for _, t := range i.Tiles {
			col := strings.TrimSpace(parts[t.ID])
			if used[col] {
				return fmt.Errorf("%w: column %q has more than one tile", ErrBadParts, col)
			}
			used[col] = true
		}
	}
	return nil
}

// gradeParts compares an already-validated answer with the expected columns.
func (i Item) gradeParts(parts map[string]string) ([]PartResult, bool) {
	want := expectedParts(i.expected)
	all := true
	out := make([]PartResult, 0, len(i.Tiles))
	for _, t := range i.Tiles {
		ok := strings.TrimSpace(parts[t.ID]) == want[t.ID]
		all = all && ok
		out = append(out, PartResult{ID: t.ID, Correct: ok, Expected: want[t.ID]})
	}
	return out, all
}

// encodeParts renders a sort answer canonically, in tile order, for the log.
func encodeParts(i Item, parts map[string]string) string {
	lines := make([]string, 0, len(i.Tiles))
	for _, t := range i.Tiles {
		lines = append(lines, t.ID+"="+strings.TrimSpace(parts[t.ID]))
	}
	return strings.Join(lines, "\n")
}

// poorTileMarkers in a gloss mark a word that makes a bad warm-up tile: units,
// letter names, abbreviations, and slang, vulgar or insulting words (`henry` "SI
// unit", `enn` "the name of the Latin script letter N", `pasi` "chap, dude",
// `tesó` "(colloquial) brother").
var poorTileMarkers = []string{
	"unit", "name of the", "abbreviation", "abbr.", "slang", "colloquial", "colloq.",
	"coll.", "informal", "familiar", "dude", "chap", "archaic", "dated", "dialect",
	"vulgar", "derogatory", "pejorative", "pej.", "insult", "insulting", "offensive",
	"son of a bitch", "obscene",
}

// poorTileRe matches the markers as whole words: "unit" must not hit
// "community", nor "chap" hit "chapel".
var poorTileRe = regexp.MustCompile(`(^|\W)(` + strings.Join(quoteAll(poorTileMarkers), "|") + `)(\W|$)`)

// poorTileRU are the Russian dictionary labels, as substrings: Go's \b is ASCII.
var poorTileRU = []string{"разг", "прост.", "сленг", "вульг", "груб", "бран.", "оскорб", "уничиж", "жарг", "сокр."}

// sortTileBlocklist are lemmas never used as a sort tile: slang and insults the
// gloss markers miss, and words that are fine vocabulary but not warm-up tiles.
var sortTileBlocklist = map[string]bool{
	"szemétláda": true, "idióta": true, "kretén": true, "barom": true, "szex": true,
	"drog": true, "dolcsi": true, "doki": true, "tesó": true, "meló": true, "cucc": true,
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = regexp.QuoteMeta(s)
	}
	return out
}

// maxTileGloss is the longest gloss (after tileGloss) a tile may carry.
const maxTileGloss = 32

// minTileLemma is the shortest lemma (in letters) a tile may show: `b`, `én`.
const minTileLemma = 3

// allGlosses returns every gloss of a lemma, lower case, one per entry.
func (g *Generator) allGlosses(lemmaID int64) []string {
	rows, err := g.pack.DB().Query(`SELECT gloss FROM lemma_gloss WHERE lemma_id = ?`, lemmaID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			out = append(out, strings.ToLower(s))
		}
	}
	return out
}

// tileWorthy reports whether a lemma makes a sensible warm-up tile: a plain
// common noun or adjective that is a dictionary headword (not an inflected form
// of another word), at least three letters long, with a short gloss and no
// slang, vulgar or abbreviation label on ANY of its senses.
func (g *Generator) tileWorthy(w Word, strict bool) bool {
	if w.UPOS != "NOUN" && w.UPOS != "ADJ" {
		return false
	}
	if len([]rune(w.Lemma)) < minTileLemma || sortTileBlocklist[w.Lemma] {
		return false
	}
	for _, gl := range g.allGlosses(w.LemmaID) {
		if poorTileRe.MatchString(gl) || strings.Contains(gl, "his/her") || strings.Contains(gl, "inflection") ||
			strings.Contains(gl, "form of") {
			return false
		}
		if strict && strings.HasPrefix(gl, "(") {
			return false
		}
		for _, m := range poorTileRU {
			if strings.Contains(gl, m) {
				return false
			}
		}
	}
	// An inflected form of another word (`tudta`, `jötte`, `légy`, `amin`): the
	// string is also a tagged reading of a different lemma.
	if all, err := g.analyzer.Lookup(w.Lemma); err == nil {
		for _, a := range all {
			if a.Source != "generated" && a.LemmaID != w.LemmaID && len(a.Feats) > 0 {
				return false
			}
		}
	}
	if !strict {
		return true // the long-gloss limit is the one rule the last tries drop
	}
	ru, en := g.gloss(w.LemmaID)
	return len([]rune(tileGloss(en))) <= maxTileGloss && len([]rune(tileGloss(ru))) <= maxTileGloss
}

// tileGloss shortens a dictionary gloss for a tile: the parenthetical
// definitions some entries carry do not fit a button.
func tileGloss(g string) string {
	if i := strings.Index(g, " ("); i > 0 {
		g = g[:i]
	}
	return strings.TrimSpace(g)
}
