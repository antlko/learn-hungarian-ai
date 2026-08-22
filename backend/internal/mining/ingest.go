package mining

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Text ingestion.
//
// The reader accepts whatever a learner actually wants to read — a pasted news
// article, a subtitle file, a chat log — rather than a curated graded corpus.
// Difficulty is controlled by SELECTION (the n+1 scorer), not by infantilising
// the content, because adults benefit most from extensive reading and are least
// tolerant of childish material.

// Kind is where a text came from.
type Kind string

const (
	KindPaste     Kind = "paste"
	KindURL       Kind = "url"
	KindSRT       Kind = "srt"
	KindEPUB      Kind = "epub"
	KindGenerated Kind = "generated"
)

// MaxTextBytes bounds a single ingest. Large enough for a long article, small
// enough that tokenising it stays interactive.
const MaxTextBytes = 512 * 1024

// Ingested is a cleaned text ready to be analysed.
type Ingested struct {
	Title string
	Body  string
	Kind  Kind
}

var (
	// SRT: an index line, a timing line, then the caption.
	srtBlockRe  = regexp.MustCompile(`(?m)^\d+\s*$`)
	srtTimingRe = regexp.MustCompile(`\d{2}:\d{2}:\d{2}[,.]\d{3}\s*-->\s*\d{2}:\d{2}:\d{2}[,.]\d{3}`)
	// Formatting tags that subtitle files carry.
	tagRe = regexp.MustCompile(`</?[a-zA-Z][^>]*>|\{[^}]*\}`)
	// Runs of blank lines collapse to a paragraph break.
	blankRunRe = regexp.MustCompile(`\n{3,}`)
)

// ErrEmpty is returned when a source yields no readable text.
var ErrEmpty = errors.New("no readable text found")

// FromPaste cleans pasted text.
func FromPaste(title, body string) (Ingested, error) {
	body = normaliseWhitespace(body)
	if body == "" {
		return Ingested{}, ErrEmpty
	}
	if len(body) > MaxTextBytes {
		return Ingested{}, fmt.Errorf("text is too long (%d bytes, limit %d)", len(body), MaxTextBytes)
	}
	if !utf8.ValidString(body) {
		return Ingested{}, errors.New("text is not valid UTF-8")
	}
	return Ingested{Title: titleOr(title, body), Body: body, Kind: KindPaste}, nil
}

// FromSRT extracts the spoken text from a subtitle file.
//
// Subtitles are the single best source of conversational Hungarian a learner can
// legally obtain, but the file is mostly not text: indices, timecodes and
// formatting tags all have to go, and a caption split across two lines has to be
// rejoined or every sentence ends up truncated mid-clause.
func FromSRT(title, raw string) (Ingested, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || srtTimingRe.MatchString(line) || srtBlockRe.MatchString(line) {
			continue
		}
		lines = append(lines, tagRe.ReplaceAllString(line, ""))
	}

	// Rejoin captions: a line that does not end a sentence continues into the
	// next one.
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(line)
		if i == len(lines)-1 {
			break
		}
		if endsSentence(line) {
			b.WriteString("\n")
		} else {
			b.WriteString(" ")
		}
	}

	body := normaliseWhitespace(b.String())
	if body == "" {
		return Ingested{}, ErrEmpty
	}
	return Ingested{Title: titleOr(title, body), Body: body, Kind: KindSRT}, nil
}

// FromHTML strips markup, keeping the readable body.
//
// Deliberately simple: enough for a pasted article, and it never executes or
// fetches anything. Scripts and styles are removed wholesale rather than merely
// untagged, or their contents would end up in the reading text.
func FromHTML(title, raw string) (Ingested, error) {
	for _, tag := range []string{"script", "style", "noscript", "svg"} {
		raw = stripElement(raw, tag)
	}
	// Block-level tags become newlines so sentences do not run together.
	raw = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|br|tr)>`).ReplaceAllString(raw, "\n")
	raw = tagRe.ReplaceAllString(raw, "")
	raw = unescapeEntities(raw)

	body := normaliseWhitespace(raw)
	if body == "" {
		return Ingested{}, ErrEmpty
	}
	return Ingested{Title: titleOr(title, body), Body: body, Kind: KindURL}, nil
}

func stripElement(s, tag string) string {
	re := regexp.MustCompile(`(?is)<` + tag + `[^>]*>.*?</` + tag + `>`)
	return re.ReplaceAllString(s, " ")
}

var entities = strings.NewReplacer(
	"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">",
	"&quot;", `"`, "&#39;", "'", "&apos;", "'", "&hellip;", "…",
	"&mdash;", "—", "&ndash;", "–",
)

func unescapeEntities(s string) string { return entities.Replace(s) }

func normaliseWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, " ", " ")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		out = append(out, strings.Join(strings.Fields(line), " "))
	}
	return strings.TrimSpace(blankRunRe.ReplaceAllString(strings.Join(out, "\n"), "\n\n"))
}

func endsSentence(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	return strings.ContainsAny(line[len(line)-1:], ".!?…\"»")
}

// titleOr derives a title from the opening words when none was supplied.
func titleOr(title, body string) string {
	if t := strings.TrimSpace(title); t != "" {
		return truncate(t, 120)
	}
	first := body
	if i := strings.IndexAny(body, ".\n"); i > 0 {
		first = body[:i]
	}
	return truncate(strings.TrimSpace(first), 80)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
