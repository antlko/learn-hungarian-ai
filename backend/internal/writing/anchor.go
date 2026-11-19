package writing

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Segment is a piece of the checked text and the indices (into the errors slice
// given to Anchor) of every error covering it. Overlapping errors are allowed.
type Segment struct {
	Text   string `json:"text"`
	Errors []int  `json:"errors"`
}

// foldLevel selects how aggressively text is normalised for matching.
type foldLevel int

const (
	foldLoose  foldLevel = iota // case, whitespace runs, quote and dash variants
	foldAccent                  // loose plus diacritics removed
)

// folded is a rune sequence with, for every rune, the byte range it came from.
type folded struct {
	r          []rune
	start, end []int
}

func foldRune(r rune, lvl foldLevel) rune {
	switch r {
	case '‘', '’', '‚', 'ʼ', '´', '`':
		return '\''
	case '“', '”', '„', '‟', '«', '»':
		return '"'
	case '‐', '‑', '‒', '–', '—', '−':
		return '-'
	}
	r = unicode.ToLower(r)
	if lvl == foldAccent {
		for _, d := range norm.NFD.String(string(r)) {
			return d // the base letter comes first
		}
	}
	return r
}

func fold(s string, lvl foldLevel) folded {
	var f folded
	inSpace := false
	for i, r := range s {
		w := len(string(r))
		if unicode.IsSpace(r) {
			if inSpace {
				f.end[len(f.end)-1] = i + w
				continue
			}
			inSpace = true
			f.r = append(f.r, ' ')
			f.start = append(f.start, i)
			f.end = append(f.end, i+w)
			continue
		}
		inSpace = false
		f.r = append(f.r, foldRune(r, lvl))
		f.start = append(f.start, i)
		f.end = append(f.end, i+w)
	}
	return f
}

// trimmedRunes returns the folded quote with outer spaces removed.
func (f folded) trimmedRunes() []rune {
	r := f.r
	for len(r) > 0 && r[0] == ' ' {
		r = r[1:]
	}
	for len(r) > 0 && r[len(r)-1] == ' ' {
		r = r[:len(r)-1]
	}
	return r
}

// occurrences lists the start indices of non-overlapping matches of n in h.
func occurrences(h, n []rune) []int {
	var out []int
	if len(n) == 0 {
		return out
	}
	for i := 0; i+len(n) <= len(h); {
		match := true
		for j := range n {
			if h[i+j] != n[j] {
				match = false
				break
			}
		}
		if match {
			out = append(out, i)
			i += len(n)
		} else {
			i++
		}
	}
	return out
}

// pick chooses the nth (1-based) match, falling back to the first.
func pick(occ []int, nth int) int {
	if nth < 1 || nth > len(occ) {
		nth = 1
	}
	return occ[nth-1]
}

// maxFoldedQuote bounds the folded matching stages (runes of QUOTE).
const maxFoldedQuote = 120

type span struct{ start, end int }

// locate finds one error's quote in text; ok is false when it cannot be placed.
func locate(text string, quote string, nth int, loose, accent folded) (span, bool) {
	quote = strings.TrimSpace(quote)
	if quote == "" {
		return span{}, false
	}
	// (a) exact substring at NTH, first occurrence if NTH is too large.
	var occ []int
	for i := 0; i+len(quote) <= len(text); {
		k := strings.Index(text[i:], quote)
		if k < 0 {
			break
		}
		occ = append(occ, i+k)
		i += k + len(quote)
	}
	if len(occ) > 0 {
		s := pick(occ, nth)
		return span{s, s + len(quote)}, true
	}
	// A quote is one to six words. A longer one that did not match exactly is
	// not worth the quadratic folded scan; the error is listed, not placed.
	if utf8.RuneCountInString(quote) > maxFoldedQuote {
		return span{}, false
	}
	// (b) case, whitespace, quote and dash folding.
	if n := fold(quote, foldLoose).trimmedRunes(); len(n) > 0 {
		if occ := occurrences(loose.r, n); len(occ) > 0 {
			s := pick(occ, nth)
			return span{loose.start[s], loose.end[s+len(n)-1]}, true
		}
	}
	// (c) diacritics folded as well: only if unambiguous.
	if n := fold(quote, foldAccent).trimmedRunes(); len(n) > 0 {
		if occ := occurrences(accent.r, n); len(occ) == 1 {
			s := occ[0]
			return span{accent.start[s], accent.end[s+len(n)-1]}, true
		}
	}
	return span{}, false
}

// Anchor places every error in the text that was checked. It returns the text
// cut into Segments (their texts concatenate to the NFC form of text, the form
// all offsets refer to; the browser never sees offsets) and, per error, whether
// its quote was found. Unanchored errors are simply not highlighted.
func Anchor(text string, errs []Error) ([]Segment, []bool) {
	text = norm.NFC.String(text)
	anchored := make([]bool, len(errs))
	if text == "" {
		return nil, anchored
	}
	loose, accent := fold(text, foldLoose), fold(text, foldAccent)
	spans := make([]span, len(errs))
	cuts := []int{0, len(text)}
	for i, e := range errs {
		s, ok := locate(text, e.Quote, e.Nth, loose, accent)
		if !ok {
			continue
		}
		spans[i], anchored[i] = s, true
		cuts = append(cuts, s.start, s.end)
	}
	sort.Ints(cuts)
	var segs []Segment
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		if a == b {
			continue
		}
		seg := Segment{Text: text[a:b], Errors: []int{}}
		for j, s := range spans {
			if anchored[j] && s.start <= a && b <= s.end {
				seg.Errors = append(seg.Errors, j)
			}
		}
		segs = append(segs, seg)
	}
	return segs, anchored
}
