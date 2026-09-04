package tutor

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Retrieval: which sections of the corpus to put in front of the model.
//
// Keyword-and-topic scoring over titled sections, on purpose. An embedding
// index would be a second model to deploy inside a static binary (ADR-001)
// for a corpus of a few hundred sections, and it would make "why did the tutor
// cite this" unanswerable. This is answerable: the topic matched, or these
// words did.

// Query is what the learner asked, and where they asked it.
type Query struct {
	// Topic is the open curriculum topic's code. Sections tagged with it are
	// boosted, and the best of them is included even when nothing else scores —
	// the topic is the one thing the question is certainly about.
	Topic string
	// Text is the question itself, in whatever language it came.
	Text string
	// Expand carries the topic's own vocabulary — its English title and its
	// features' English labels — so a Russian question about «инессив» still
	// reaches the English section on the inessive.
	Expand []string
}

// Hit is a retrieved section with the score that put it there.
type Hit struct {
	Section
	Score float64
}

const (
	topicBoost  = 6.0
	titleWeight = 3.0
	bodyWeight  = 1.5
	// formHit is what a whole-word match on a token with Hungarian letters is
	// worth. A form the learner quoted is the most specific thing a question
	// can carry, and the section that contains it outranks the open topic's
	// own section — which is kept in play separately.
	formHit      = 8.0
	expandWeight = 0.6 // expansion terms are hints, not the learner's words
)

// Retrieve scores every section against the query and returns the best k.
//
// Sections that score nothing are never returned, except that the open
// topic's own sections fill in when the question matched nothing at all — a
// question with no grammar term and no Hungarian in it ("why is this so hard")
// is still about the topic on screen.
func (kb *KB) Retrieve(q Query, k int) []Hit {
	if k <= 0 {
		k = 3
	}
	terms := tokens(q.Text)
	var expand []string
	for _, e := range q.Expand {
		expand = append(expand, tokens(e)...)
	}

	hits := make([]Hit, 0, len(kb.sections))
	for i := range kb.sections {
		sec := &kb.sections[i]
		score := 0.0
		if q.Topic != "" && hasTopic(sec.Topics, q.Topic) {
			score += topicBoost
		}
		score += termScore(sec, terms, 1.0)
		score += termScore(sec, expand, expandWeight)
		if score > 0 {
			hits = append(hits, Hit{Section: *sec, Score: score})
		}
	}
	// Deterministic: score, then corpus order. A tie must not depend on map
	// iteration or the learner would see the citations flicker between asks.
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })

	// Keep the topic's own material in play when the words matched elsewhere:
	// at least one hit for the open topic, if any section carries it.
	out := make([]Hit, 0, k)
	topicSeen := false
	for _, h := range hits {
		if len(out) == k {
			break
		}
		out = append(out, h)
		if q.Topic != "" && hasTopic(h.Topics, q.Topic) {
			topicSeen = true
		}
	}
	if q.Topic != "" && !topicSeen {
		for _, h := range hits {
			if hasTopic(h.Topics, q.Topic) {
				if len(out) == k {
					out = out[:k-1]
				}
				out = append(out, h)
				break
			}
		}
	}
	return out
}

func termScore(sec *Section, terms []string, weight float64) float64 {
	score := 0.0
	seen := map[string]bool{}
	for _, t := range terms {
		if seen[t] {
			continue
		}
		seen[t] = true
		if hasHungarianLetter(t) {
			if countWord(sec.lower, t) > 0 {
				score += formHit * weight
			}
			continue
		}
		if strings.Contains(sec.titleLower, t) {
			score += titleWeight * weight
		}
		if n := strings.Count(sec.lower, t); n > 0 {
			// Saturating term frequency: the tenth mention is worth little
			// more than the third, and a long section must not win by bulk.
			tf := float64(n)
			score += bodyWeight * weight * tf / (tf + 1.5)
		}
	}
	return score
}

// countWord counts whole-word occurrences: `ház` must not match `házban`.
func countWord(text, word string) int {
	n, from := 0, 0
	for {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return n
		}
		start := from + i
		end := start + len(word)
		before := start == 0 || !isLetterAt(text, start-1)
		after := end == len(text) || !isLetterAt(text, end)
		if before && after {
			n++
		}
		from = end
	}
}

func isLetterAt(s string, i int) bool {
	r, _ := utf8.DecodeRuneInString(s[i:])
	if i > 0 && !utf8.RuneStart(s[i]) {
		// Inside a multi-byte rune: step back to its start.
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
		r, _ = utf8.DecodeRuneInString(s[i:])
	}
	return unicode.IsLetter(r)
}

func hasTopic(topics []string, want string) bool {
	for _, t := range topics {
		if t == want {
			return true
		}
	}
	return false
}

// stopwords are the words that carry no retrieval signal in the three
// languages a question arrives in. Short and deliberately incomplete: a
// missed stopword costs a little noise, a wrongly listed content word costs a
// hit.
var stopwords = map[string]bool{
	// en
	"the": true, "and": true, "for": true, "with": true, "what": true, "why": true, "how": true,
	"does": true, "did": true, "this": true, "that": true, "here": true, "there": true, "not": true,
	"are": true, "was": true, "you": true, "can": true, "please": true, "explain": true, "mean": true,
	"use": true, "used": true, "when": true, "which": true, "from": true, "into": true, "have": true,
	"has": true, "instead": true, "difference": true, "between": true, "give": true, "more": true,
	"example": true, "examples": true, "word": true, "words": true, "sentence": true, "hungarian": true,
	// ru
	"что": true, "как": true, "почему": true, "это": true, "здесь": true, "или": true, "для": true,
	"при": true, "чем": true, "чему": true, "ещё": true, "еще": true, "мне": true, "можно": true,
	"объясни": true, "объясните": true, "пример": true, "примеры": true, "примеров": true, "слово": true,
	"слова": true, "разница": true, "между": true, "когда": true, "если": true, "надо": true, "нужно": true,
	"почему-то": true, "венгерском": true, "венгерский": true, "предложении": true, "предложение": true,
	// hu function words that appear in every example
	"egy": true, "van": true, "nem": true, "hogy": true,
}

// tokens lowercases and splits on anything that is not a letter, keeping a
// leading hyphen so `-ban` matches the suffix as written in the corpus.
func tokens(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		t := cur.String()
		cur.Reset()
		if t == "" || t == "-" {
			return
		}
		bare := strings.TrimPrefix(t, "-")
		if len([]rune(bare)) < 3 || stopwords[bare] {
			return
		}
		out = append(out, t)
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r):
			cur.WriteRune(r)
		case r == '-' && cur.Len() == 0:
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

func hasHungarianLetter(s string) bool {
	return strings.ContainsAny(s, "áéíóöőúüű")
}
