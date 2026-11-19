package writing

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Parse statuses.
const (
	StatusOK      = "ok"
	StatusPartial = "partial"
	StatusFailed  = "failed"
)

// Parser limits.
const (
	maxErrors       = 60
	maxFieldRunes   = 2000
	maxStrengths    = 5
	maxVocab        = 20
	maxCorrectedRun = 12500 // 2 x the 6000-rune body limit + 500
)

// Error is one reported mistake. N is its 1-based position in Result.Errors.
type Error struct {
	N       int    `json:"n"`
	Quote   string `json:"quote"`
	Nth     int    `json:"nth"`
	Type    string `json:"type"`
	Fix     string `json:"fix"`
	Hint    string `json:"hint"`
	Why     string `json:"why"`
	Rule    string `json:"rule"`
	Example string `json:"example"`
}

// Result is what Parse understood of an answer. Hungarian in Quote, Fix,
// Example and Corrected is model-written and display-only.
type Result struct {
	Status    string // StatusOK, StatusPartial or StatusFailed
	Truncated bool   // the answer ended before [END]
	Dropped   int    // error blocks discarded (incomplete or over the cap)
	NoHeader  bool   // no TANULOM-CHECK line was found
	Version   int    // from the header; 0 when absent. >PromptVersion is parsed anyway
	Level     string // CEFR level the model assigned, or ""
	Summary   string
	Strengths []string
	Errors    []Error
	Vocab     []string
	Corrected string
}

var (
	reThink   = regexp.MustCompile(`(?is)<think>.*?</think>`)
	reHeader  = regexp.MustCompile(`(?i)TANULOM-CHECK\s*(\d+)`)
	reSection = regexp.MustCompile(`(?i)^\W*\[\s*(ERROR|VOCAB|CORRECTED|END)\s*\d*\s*\]`)
	reKey     = regexp.MustCompile("(?i)^\\s*[-*•>]*\\s*[*_`]*(LEVEL|SUMMARY|STRENGTH|QUOTE|NTH|TYPE|FIX|HINT|WHY|RULE|EXAMPLE)[*_`]*\\s*[:：]\\s*\\**\\s*(.*)$")
	reBullet  = regexp.MustCompile(`^\s*(?:[-*\x{2022}]|\d+[.)])\s+`)
	reLevel   = regexp.MustCompile(`(?i)\b([ABC][12])\b`)
)

const quoteTrim = "«»“”„‟\"'`‘’ \t"

// normalize implements parser rule 1 up to the choice of the fenced block.
func normalize(raw string) string {
	s := strings.TrimPrefix(raw, "\xef\xbb\xbf")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = norm.NFC.String(s)
	return reThink.ReplaceAllString(s, "")
}

// pickBlock returns the content of the first fenced block that looks like an
// answer (a missing closing fence is tolerated), else the whole text.
func pickBlock(s string) string {
	lines := strings.Split(s, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			continue
		}
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[j]), "```") {
			j++
		}
		block := strings.Join(lines[i+1:j], "\n")
		if looksLikeAnswer(block) {
			return block
		}
		i = j // skip past the closing fence
	}
	return s
}

func looksLikeAnswer(block string) bool {
	u := strings.ToUpper(block)
	return strings.Contains(u, "TANULOM-CHECK") || strings.Contains(u, "[ERROR")
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxFieldRunes {
		s = string([]rune(s)[:maxFieldRunes])
	}
	return s
}

// builder accumulates the error block being read.
type builder struct {
	e     Error
	has   bool
	field *string // field a continuation line extends
}

// Parse reads a teacher's answer: a raw block, a whole chat message with
// chatter around it, with or without a fence, bold keys, bullets, CRLF. It never
// fails outright; Status says how much was understood. See the package rules in
// the writing design (nine rules) — each is marked below.
func Parse(raw string) Result {
	var res Result
	body := pickBlock(normalize(raw)) // rule 1
	lines := strings.Split(body, "\n")

	// Rule 2: where to start.
	start := -1
	for i, l := range lines {
		if m := reHeader.FindStringSubmatch(l); m != nil {
			res.Version, _ = strconv.Atoi(m[1])
			start = i + 1
			break
		}
	}
	if start < 0 {
		res.NoHeader = true
		for i, l := range lines {
			if reSection.MatchString(l) || reKey.MatchString(l) {
				start = i
				break
			}
		}
	}
	if start < 0 {
		res.Status = StatusFailed
		return res
	}

	section := "head"
	recognised := 0
	ended := false
	var cur builder
	var headField *string
	var corrected []string

	flush := func() {
		if !cur.has {
			return
		}
		e := cur.e
		cur = builder{}
		e.Quote = strings.Trim(clip(e.Quote), quoteTrim)
		e.Fix = strings.Trim(clip(e.Fix), quoteTrim)
		e.Type = normalizeType(e.Type)
		e.Hint, e.Why, e.Rule, e.Example = clip(e.Hint), clip(e.Why), clip(e.Rule), clip(e.Example)
		if e.Nth < 1 {
			e.Nth = 1
		}
		if e.Quote == "" || (e.Fix == "" && e.Type != TypeExtraWord) { // rule 7
			res.Dropped++
			return
		}
		if len(res.Errors) >= maxErrors { // rule 8
			res.Dropped++
			return
		}
		e.N = len(res.Errors) + 1
		res.Errors = append(res.Errors, e)
	}
	// appendTo extends a field with a continuation line (rule 5).
	appendTo := func(dst *string, line string) {
		if *dst == "" {
			*dst = line
		} else {
			*dst += "\n" + line
		}
	}

	for _, l := range lines[start:] {
		if m := reSection.FindStringSubmatch(l); m != nil { // rule 3
			recognised++
			flush()
			headField = nil
			switch strings.ToUpper(m[1]) {
			case "ERROR":
				section = "error"
				cur = builder{has: true}
			case "VOCAB":
				section = "vocab"
			case "CORRECTED":
				section = "corrected"
			case "END":
				ended = true
			}
			if ended {
				break
			}
			continue
		}
		switch section {
		case "corrected": // rule 5: verbatim
			corrected = append(corrected, l)
			continue
		case "vocab":
			if t := strings.TrimSpace(reBullet.ReplaceAllString(l, "")); t != "" && len(res.Vocab) < maxVocab {
				res.Vocab = append(res.Vocab, clip(t))
			}
			continue
		}
		if m := reKey.FindStringSubmatch(l); m != nil { // rule 4
			recognised++
			key, val := strings.ToUpper(m[1]), strings.TrimSpace(m[2])
			switch key {
			case "LEVEL":
				if lv := reLevel.FindString(val); lv != "" {
					res.Level = strings.ToUpper(lv)
				}
				headField = nil
			case "SUMMARY":
				res.Summary = val
				headField = &res.Summary
			case "STRENGTH":
				if len(res.Strengths) < maxStrengths {
					res.Strengths = append(res.Strengths, val)
					headField = &res.Strengths[len(res.Strengths)-1]
				} else {
					headField = nil
				}
			default:
				// An error field. A repeated QUOTE, or one outside any
				// [ERROR] header, opens a new block.
				if key == "QUOTE" && (!cur.has || cur.e.Quote != "") {
					flush()
					cur = builder{has: true}
				}
				if !cur.has {
					cur = builder{has: true}
				}
				section = "error"
				headField = nil
				switch key {
				case "QUOTE":
					cur.e.Quote, cur.field = val, &cur.e.Quote
				case "NTH":
					n, err := strconv.Atoi(strings.Trim(val, " .#*`"))
					if err != nil || n < 1 {
						n = 1
					}
					cur.e.Nth, cur.field = n, nil
				case "TYPE":
					cur.e.Type, cur.field = val, &cur.e.Type
				case "FIX":
					cur.e.Fix, cur.field = val, &cur.e.Fix
				case "HINT":
					cur.e.Hint, cur.field = val, &cur.e.Hint
				case "WHY":
					cur.e.Why, cur.field = val, &cur.e.Why
				case "RULE":
					cur.e.Rule, cur.field = val, &cur.e.Rule
				case "EXAMPLE":
					cur.e.Example, cur.field = val, &cur.e.Example
				}
			}
			continue
		}
		// Rule 5: a non-key line continues the previous field.
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if cur.has && cur.field != nil {
			appendTo(cur.field, t)
		} else if headField != nil {
			appendTo(headField, t)
		}
	}
	flush()

	if !ended {
		res.Truncated = true
	}
	res.Summary = clip(res.Summary)
	for i := range res.Strengths {
		res.Strengths[i] = clip(res.Strengths[i])
	}
	res.Strengths = nonEmpty(res.Strengths)
	c := strings.TrimSpace(strings.Join(corrected, "\n"))
	if r := []rune(c); len(r) > maxCorrectedRun {
		c = string(r[:maxCorrectedRun])
	}
	res.Corrected = c

	switch { // rule 9
	case recognised == 0:
		res.Status = StatusFailed
	case res.Truncated || res.Dropped > 0 || res.NoHeader:
		res.Status = StatusPartial
	default:
		res.Status = StatusOK
	}
	return res
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
