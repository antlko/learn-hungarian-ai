// Package pictures is the picture catalogue for picture-based grammar exercises:
// concrete A1 nouns drawn as Noto Emoji SVGs (Apache-2.0, see
// THIRD_PARTY_LICENSES) and a few original one-colour line drawings of places
// (box, table, ...) that carry the case relations a picture can show.
//
// The catalogue ships inside the binary. Lemma-to-lemma-id resolution lives in
// the drill package; here lemmas stay plain strings and Resolve drops unknown
// ones with a warning, like readsets.
package pictures

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

//go:embed catalog.json svg/*.svg
var files embed.FS

// Format is the only catalogue format this code reads.
const Format = 1

// Roles of an entry.
const (
	RoleObject = "object"
	RolePlace  = "place"
)

// Entry is one picture of a noun.
type Entry struct {
	ID    string `json:"id"`
	Lemma string `json:"lemma"`
	UPOS  string `json:"upos"`
	// Pic is the file name under svg/.
	Pic   string `json:"pic"`
	AltRu string `json:"altRu"`
	AltEn string `json:"altEn"`
	Role  string `json:"role"`
	// Relations maps a spatial relation ("in", "on") to the pack case that
	// expresses it ("Ine", "Sup"); places only.
	Relations map[string]string `json:"relations,omitempty"`
	// Size is how big an object is: "tiny" (fruit-sized things, a key, a ring),
	// "small" (a mouse, a book, a bottle), "medium" (bag-size: a cat, a laptop) or
	// "large" (a person, a vehicle, a building, a tree). Objects only.
	Size string `json:"size,omitempty"`
	// Animal marks a creature. An animal goes in or on a place only when the
	// place's capacity is "large" (a box, a shelf, a house), never a cup, bag or
	// basket. Objects only.
	Animal bool `json:"animal,omitempty"`
	// Capacity is the largest object size a place holds. Places only. A scene
	// pairs an object with a place only when Size <= Capacity.
	Capacity string `json:"capacity,omitempty"`
}

// Sizes in increasing order.
const (
	SizeTiny   = "tiny"
	SizeSmall  = "small"
	SizeMedium = "medium"
	SizeLarge  = "large"
)

// SizeRank orders sizes; unknown sizes rank -1.
func SizeRank(s string) int {
	switch s {
	case SizeTiny:
		return 0
	case SizeSmall:
		return 1
	case SizeMedium:
		return 2
	case SizeLarge:
		return 3
	}
	return -1
}

// Fits reports whether an object of the given size goes in or on a place of the
// given capacity. A large object fits nowhere: a scene never draws one.
func Fits(objectSize, placeCapacity string) bool {
	o, c := SizeRank(objectSize), SizeRank(placeCapacity)
	return o >= 0 && c >= 0 && o <= c && o < SizeRank(SizeLarge)
}

// FitsEntry is Fits for entries: the size rule, and an animal only in a place of
// capacity large.
func FitsEntry(object, place Entry) bool {
	if object.Animal && SizeRank(place.Capacity) < SizeRank(SizeLarge) {
		return false
	}
	return Fits(object.Size, place.Capacity)
}

// Catalog is the immutable parsed catalogue.
type Catalog struct {
	entries []Entry
	byID    map[string]int
	byLemma map[string][]int
	svg     map[string][]byte
}

type fileFormat struct {
	Format  int     `json:"format"`
	Entries []Entry `json:"entries"`
}

var (
	defOnce sync.Once
	defCat  *Catalog
	defErr  error
)

// Default returns the embedded catalogue, parsed once.
func Default() (*Catalog, error) {
	defOnce.Do(func() { defCat, defErr = load() })
	return defCat, defErr
}

func load() (*Catalog, error) {
	raw, err := files.ReadFile("catalog.json")
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("pictures: catalog.json: %w", err)
	}
	if f.Format != Format {
		return nil, fmt.Errorf("pictures: unsupported format %d", f.Format)
	}
	c := &Catalog{entries: f.Entries, byID: map[string]int{}, byLemma: map[string][]int{}, svg: map[string][]byte{}}
	for i, e := range f.Entries {
		if e.ID == "" || e.Lemma == "" || e.UPOS == "" || e.AltRu == "" || e.AltEn == "" {
			return nil, fmt.Errorf("pictures: entry %d (%q) has an empty field", i, e.ID)
		}
		if _, dup := c.byID[e.ID]; dup {
			return nil, fmt.Errorf("pictures: duplicate id %q", e.ID)
		}
		if e.Role != RoleObject && e.Role != RolePlace {
			return nil, fmt.Errorf("pictures: %q has invalid role %q", e.ID, e.Role)
		}
		if e.Role == RoleObject && len(e.Relations) > 0 {
			return nil, fmt.Errorf("pictures: %q: only places carry relations", e.ID)
		}
		if e.Role == RolePlace && len(e.Relations) == 0 {
			return nil, fmt.Errorf("pictures: place %q has no relations", e.ID)
		}
		if e.Role == RoleObject && (SizeRank(e.Size) < 0 || e.Capacity != "") {
			return nil, fmt.Errorf("pictures: object %q needs a size tiny|small|medium|large and no capacity", e.ID)
		}
		if e.Role == RolePlace && (SizeRank(e.Capacity) < 0 || e.Size != "") {
			return nil, fmt.Errorf("pictures: place %q needs a capacity tiny|small|medium|large and no size", e.ID)
		}
		b, err := files.ReadFile("svg/" + e.Pic)
		if err != nil {
			return nil, fmt.Errorf("pictures: %q: missing svg %q", e.ID, e.Pic)
		}
		c.byID[e.ID] = i
		c.byLemma[e.Lemma] = append(c.byLemma[e.Lemma], i)
		c.svg[e.ID] = b
	}
	return c, nil
}

// All returns every entry in catalogue order.
func (c *Catalog) All() []Entry { return append([]Entry(nil), c.entries...) }

// ByID returns the entry with the given id.
func (c *Catalog) ByID(id string) (Entry, bool) {
	i, ok := c.byID[id]
	if !ok {
		return Entry{}, false
	}
	return c.entries[i], true
}

// ByLemma returns the entries drawn for a lemma (usually one).
func (c *Catalog) ByLemma(lemma string) []Entry {
	var out []Entry
	for _, i := range c.byLemma[lemma] {
		out = append(out, c.entries[i])
	}
	return out
}

func (c *Catalog) withRole(role string) []Entry {
	var out []Entry
	for _, e := range c.entries {
		if e.Role == role {
			out = append(out, e)
		}
	}
	return out
}

// Places returns the entries a thing can be put in or on.
func (c *Catalog) Places() []Entry { return c.withRole(RolePlace) }

// Objects returns the entries that are things.
func (c *Catalog) Objects() []Entry { return c.withRole(RoleObject) }

// SVG returns the picture bytes for an entry id.
func (c *Catalog) SVG(id string) ([]byte, bool) {
	b, ok := c.svg[id]
	return b, ok
}

// Resolved is an entry bound to its dictionary lemma id.
type Resolved struct {
	Entry
	LemmaID int64
}

// Resolve binds entries to lemma ids; entries whose lemma the pack does not
// know are dropped with a warning (a pack and a catalogue may disagree).
func (c *Catalog) Resolve(lookup func(lemma, upos string) (int64, bool)) []Resolved {
	var out []Resolved
	var dropped []string
	for _, e := range c.entries {
		id, ok := lookup(e.Lemma, e.UPOS)
		if !ok {
			dropped = append(dropped, e.Lemma)
			continue
		}
		out = append(out, Resolved{Entry: e, LemmaID: id})
	}
	if len(dropped) > 0 {
		sort.Strings(dropped)
		slog.Warn("pictures: lemmas not in the language pack, entries dropped", "n", len(dropped), "lemmas", strings.Join(dropped, ","))
	}
	return out
}
