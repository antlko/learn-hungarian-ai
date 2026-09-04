// Package morph turns Hungarian text into lemmas and morphological features.
//
// It never calls out to a Python analyser at runtime: the language pack ships a
// precomputed form -> (lemma, features) index, so lookup is an indexed query.
// See ADR-002 and docs/LANGPACK.md.
package morph

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token is one orthographic unit of a text.
type Token struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
	// Start and End are byte offsets into the original string, so a caller can
	// highlight the token without re-finding it.
	Start int `json:"start"`
	End   int `json:"end"`
	// IsWord distinguishes words from punctuation and whitespace runs.
	IsWord bool `json:"isWord"`
}

// Sentence is a half-open token range.
type Sentence struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// isHungarianLetter reports whether r can occur inside a Hungarian word.
//
// Hungarian's long vowels (á é í ó ő ú ű) and the umlauted pair (ö ü) are all
// covered by unicode.IsLetter; the interesting cases are the connectors.
func isHungarianLetter(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Tokenize splits text into tokens, keeping byte offsets.
//
// Two Hungarian-specific decisions:
//
//   - Digraphs (cs, gy, ly, ny, sz, ty, zs) and the trigraph dzs are single
//     letters in Hungarian, but they are written with ordinary Latin characters,
//     so no special handling is needed at the token level. They matter to
//     segmentation, not to tokenisation.
//   - The hyphen is word-internal in compounds and in forms like `van-e`, so it
//     is kept when it sits between letters and dropped when it is dangling.
func Tokenize(text string) []Token {
	var out []Token
	runes := []rune(text)
	offsets := make([]int, len(runes)+1)
	{
		b := 0
		for i, r := range runes {
			offsets[i] = b
			b += len(string(r))
		}
		offsets[len(runes)] = b
	}

	i := 0
	idx := 0
	for i < len(runes) {
		r := runes[i]

		if unicode.IsSpace(r) {
			i++
			continue
		}

		if isHungarianLetter(r) {
			start := i
			for i < len(runes) {
				if isHungarianLetter(runes[i]) {
					i++
					continue
				}
				// A hyphen or apostrophe continues the word only when a letter
				// follows: `van-e` is one token, `ház-` is not.
				if (runes[i] == '-' || runes[i] == '\'') &&
					i+1 < len(runes) && isHungarianLetter(runes[i+1]) {
					i++
					continue
				}
				break
			}
			out = append(out, Token{
				Index: idx, Text: string(runes[start:i]),
				Start: offsets[start], End: offsets[i], IsWord: true,
			})
			idx++
			continue
		}

		// Punctuation: one token per rune, so offsets stay exact.
		out = append(out, Token{
			Index: idx, Text: string(r),
			Start: offsets[i], End: offsets[i+1], IsWord: false,
		})
		idx++
		i++
	}
	return out
}

// sentenceEnders are the characters that can close a sentence.
const sentenceEnders = ".!?…"

// SplitSentences groups tokens into sentences.
//
// The naive "split on ." rule is wrong for Hungarian in a specific and very
// common way: ordinals are written with a trailing dot (`3.` = "third", `1848.`
// = "of 1848"), and dates are written `2026. augusztus 22.`. A dot straight
// after digits is therefore treated as an ordinal marker, not a boundary,
// unless the next word starts a new sentence by beginning with a capital and
// following a gap.
//
// **A LINE BREAK also ends a sentence**, and needs the source text to see:
// whitespace is not tokenised, so the only trace of a newline is the gap
// between one token's End and the next one's Start. Without this a heading —
// which carries no terminal punctuation — swallowed the sentence after it, and
// a card mined from the first line of a text asked about
// `Egy átlagos nap Budapesten Reggel hét órakor ____.`: a title welded to a
// sentence, which is not something anybody wrote.
func SplitSentences(text string, tokens []Token) []Sentence {
	var out []Sentence
	start := 0

	for i, t := range tokens {
		var end int
		switch {
		case !t.IsWord && strings.ContainsAny(t.Text, sentenceEnders):
			if t.Text == "." && i > 0 && isNumeric(tokens[i-1].Text) {
				continue // ordinal or date, e.g. "1848." or "22."
			}
			// Absorb any run of following punctuation (`?!`, `..."`).
			end = i + 1
			for end < len(tokens) && !tokens[end].IsWord &&
				strings.ContainsAny(tokens[end].Text, sentenceEnders+`"»'`) {
				end++
			}
		case endsLine(text, tokens, i):
			end = i + 1
		default:
			continue
		}
		// The punctuation run may already have carried us past this boundary.
		if end <= start {
			continue
		}
		out = append(out, Sentence{Start: start, End: end})
		start = end
	}
	if start < len(tokens) {
		out = append(out, Sentence{Start: start, End: len(tokens)})
	}
	return out
}

// endsLine reports whether the gap after token i breaks the line.
//
// A BLANK line is always a boundary: nothing spans a paragraph break. A single
// newline is a boundary only when the next token starts with a capital, because
// the other thing a single newline means is a hard-wrapped paragraph — and the
// reader invites the learner to paste subtitles, which wrap mid-sentence:
//
//	Nem tudom, hogy mit
//	csináljak most.
//
// Splitting there would cut a sentence in half. Hungarian capitalises sentence
// starts and proper nouns but not common nouns, so a wrapped line beginning
// with a capital is rare, and a genuine new line beginning with one is not.
func endsLine(text string, tokens []Token, i int) bool {
	if i+1 >= len(tokens) {
		return false
	}
	lo, hi := tokens[i].End, tokens[i+1].Start
	if lo < 0 || hi > len(text) || lo >= hi {
		return false
	}
	gap := text[lo:hi]
	n := strings.Count(gap, "\n")
	if n == 0 {
		return false
	}
	if n > 1 {
		return true // a blank line: a paragraph break
	}
	r, _ := utf8.DecodeRuneInString(tokens[i+1].Text)
	return unicode.IsUpper(r)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// Words returns only the word tokens.
func Words(tokens []Token) []Token {
	out := make([]Token, 0, len(tokens))
	for _, t := range tokens {
		if t.IsWord {
			out = append(out, t)
		}
	}
	return out
}
