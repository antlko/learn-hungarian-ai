// Package morph turns Hungarian text into lemmas and morphological features.
//
// It never calls out to a Python analyser at runtime: the language pack ships a
// precomputed form -> (lemma, features) index, so lookup is an indexed query.
// See ADR-002 and docs/LANGPACK.md.
package morph

import (
	"strings"
	"unicode"
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
func SplitSentences(tokens []Token) []Sentence {
	var out []Sentence
	start := 0

	for i, t := range tokens {
		if t.IsWord || !strings.ContainsAny(t.Text, sentenceEnders) {
			continue
		}
		if t.Text == "." && i > 0 && isNumeric(tokens[i-1].Text) {
			continue // ordinal or date, e.g. "1848." or "22."
		}
		// Absorb any run of following punctuation (`?!`, `..."`).
		end := i + 1
		for end < len(tokens) && !tokens[end].IsWord &&
			strings.ContainsAny(tokens[end].Text, sentenceEnders+`"»'`) {
			end++
		}
		out = append(out, Sentence{Start: start, End: end})
		start = end
	}
	if start < len(tokens) {
		out = append(out, Sentence{Start: start, End: len(tokens)})
	}
	return out
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
