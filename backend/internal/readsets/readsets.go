// Package readsets is the reading-text catalogue (ADR-076): the ready-made texts
// of the library (a short, a medium and a long text per course unit, and later
// talks and mini-dialogues) shipped INSIDE the binary as one committed JSON
// file. Nothing here reads or writes the language pack: shipping a new binary is
// enough to update the texts, for the full and the starter edition alike.
//
// The catalogue is parsed once into an immutable in-memory value. A malformed
// file is a build defect, so Default returns an error and the server refuses to
// boot rather than run with an empty library.
package readsets

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

//go:embed catalog.json
var embedded []byte

// Format is the only catalogue format this code reads.
const Format = 1

// DialogueTurn is the Hungarian turn marker opening each line of a dialogue-form
// text; the catalogue stores the line without it.
const DialogueTurn = "– "

// Line is one sentence (or one turn of a dialogue) with its translations.
type Line struct {
	HU   string `json:"hu"`
	RU   string `json:"ru"`
	EN   string `json:"en"`
	Para int    `json:"para"`
	Who  string `json:"who,omitempty"`
}

// Set is one reading text.
type Set struct {
	Code     string `json:"code"`
	Unit     string `json:"unit"`
	UnitOrd  int    `json:"unitOrd"`
	Level    string `json:"level"`
	Track    string `json:"track"`
	Register string `json:"register"`
	Tier     string `json:"tier"`
	Genre    string `json:"genre"`
	Ord      int    `json:"ord"`
	TitleRU  string `json:"titleRu"`
	TitleEN  string `json:"titleEn"`
	ThemeRU  string `json:"themeRu"`
	ThemeEN  string `json:"themeEn"`
	// SceneRU/SceneEN is the one-line situation of a mini-dialogue (optional).
	SceneRU string `json:"sceneRu,omitempty"`
	SceneEN string `json:"sceneEn,omitempty"`
	Checked string `json:"checked"`
	Words   int    `json:"words"`
	Lines   []Line `json:"lines"`
}

// DialogueForm reports whether the text is a script (one turn per line) rather
// than prose.
// A voice message is prose; a phone call and a chat are turns.
func (s Set) DialogueForm() bool {
	return s.Tier == "mini" || s.Genre == "dialogue" || s.Genre == "phone" || s.Genre == "chat"
}

// Body is the text the set imports as. A dialogue-form set: one turn per line,
// each opened by «– », so the reader shows a script. Any other set: sentences of
// one paragraph joined by a space, paragraphs by a blank line (which always ends a
// sentence in the splitter, so a sentence without closing punctuation cannot
// swallow the next one).
func (s Set) Body() string {
	if s.DialogueForm() {
		out := make([]string, len(s.Lines))
		for i, l := range s.Lines {
			out[i] = DialogueTurn + strings.TrimSpace(l.HU)
		}
		return strings.Join(out, "\n")
	}
	var groups []string
	for i, l := range s.Lines {
		hu := strings.TrimSpace(l.HU)
		if i > 0 && l.Para == s.Lines[i-1].Para {
			groups[len(groups)-1] += " " + hu
		} else {
			groups = append(groups, hu)
		}
	}
	return strings.Join(groups, "\n\n")
}

// Catalog is an immutable set of texts in list order.
type Catalog struct {
	sets   []Set
	byCode map[string]int
	hash   string
}

type file struct {
	Format int   `json:"format"`
	Sets   []Set `json:"sets"`
}

// Parse validates and indexes a catalogue file. Sets are ordered by (ord, code).
func Parse(data []byte) (*Catalog, error) {
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("reading-set catalogue: %w", err)
	}
	if f.Format != Format {
		return nil, fmt.Errorf("reading-set catalogue: format %d, this build reads %d", f.Format, Format)
	}
	c := &Catalog{sets: f.Sets, byCode: make(map[string]int, len(f.Sets))}
	sort.SliceStable(c.sets, func(i, j int) bool {
		if c.sets[i].Ord != c.sets[j].Ord {
			return c.sets[i].Ord < c.sets[j].Ord
		}
		return c.sets[i].Code < c.sets[j].Code
	})
	for i, s := range c.sets {
		if s.Code == "" {
			return nil, fmt.Errorf("reading-set catalogue: set %d has no code", i)
		}
		if _, dup := c.byCode[s.Code]; dup {
			return nil, fmt.Errorf("reading-set catalogue: duplicate code %q", s.Code)
		}
		if len(s.Lines) == 0 {
			return nil, fmt.Errorf("reading-set catalogue: set %q has no lines", s.Code)
		}
		for j, l := range s.Lines {
			if strings.TrimSpace(l.HU) == "" {
				return nil, fmt.Errorf("reading-set catalogue: set %q line %d is empty", s.Code, j)
			}
		}
		c.byCode[s.Code] = i
	}
	sum := sha256.Sum256(data)
	c.hash = hex.EncodeToString(sum[:8])
	return c, nil
}

// Sets returns the texts in list order. The slice is shared: do not modify it.
func (c *Catalog) Sets() []Set { return c.sets }

// Get returns one set by code.
func (c *Catalog) Get(code string) (Set, bool) {
	i, ok := c.byCode[code]
	if !ok {
		return Set{}, false
	}
	return c.sets[i], true
}

// Hash identifies the catalogue's content (it changes whenever a text does).
func (c *Catalog) Hash() string { return c.hash }

var (
	once   sync.Once
	cat    *Catalog
	catErr error
)

// Default parses the embedded file once.
func Default() (*Catalog, error) {
	once.Do(func() {
		cat, catErr = Parse(embedded)
		if catErr == nil {
			slog.Info("reading-set catalogue loaded", "sets", len(cat.sets), "hash", cat.hash)
		}
	})
	return cat, catErr
}
