// Package curriculum owns the grammar topic DAG and a learner's position in it.
//
// The DAG itself lives in the language pack (`grammar_topic`), because an
// ordering of Hungarian grammar is language knowledge, not learner state. This
// package reads it, resolves prerequisites, and answers the only question the
// rest of the app asks: which topics may this learner work on right now, and why
// not the others.
//
// The ordering is not the conventional "here are the eighteen cases". It follows
// the corrected difficulty map in [R15] — harmony first as a motor skill,
// politeness in month one because it is socially costly, the definite
// conjugation in the middle where it starts blocking communication, and word
// order last behind listening volume. See
// .claude/research/15-hungarian-difficulty.md and langpack/pipeline/curriculum.py.
package curriculum

import (
	"fmt"
	"sort"
	"strings"

	"tanulom/internal/lang"
)

// Topic is one teachable grammar topic, straight from the pack.
type Topic struct {
	ID          int64    `json:"id"`
	Code        string   `json:"code"`
	Level       string   `json:"level"`
	TitleRU     string   `json:"titleRu"`
	TitleEN     string   `json:"titleEn"`
	ExplainRU   string   `json:"explainRu"`
	ExplainEN   string   `json:"explainEn"`
	PrereqCodes []string `json:"prereqCodes"`
	FeatureIDs  []string `json:"featureIds"`
	TeachOrder  int      `json:"teachOrder"`
}

// Feature ids that change how a topic is gated or graded. Keyed by feature
// rather than by topic code so that renaming or splitting a topic in the pack
// cannot silently drop the special handling.
const (
	// FeatureVowelHarmony marks a motor-skill topic: mastery additionally
	// requires latency, not accuracy alone. [R15]
	FeatureVowelHarmony = "Skill=VowelHarmony"
	// FeaturePreverbDetach marks the focus/word-order topic, which is locked
	// behind listening volume regardless of prerequisites. [R15]
	FeaturePreverbDetach = "Skill=PreverbDetach"
)

// MotorSkill reports whether mastery of this topic must be measured in latency
// as well as accuracy. See internal/drill/harmony.go.
func (t Topic) MotorSkill() bool { return t.hasFeature(FeatureVowelHarmony) }

// NeedsListeningVolume reports whether this topic carries the extra listening
// gate. Word order encodes focus, and the brief is explicit that it cannot be
// acquired from rules — it needs volume of heard Hungarian first. [R15]
func (t Topic) NeedsListeningVolume() bool { return t.hasFeature(FeaturePreverbDetach) }

func (t Topic) hasFeature(id string) bool {
	for _, f := range t.FeatureIDs {
		if f == id {
			return true
		}
	}
	return false
}

// DAG is the loaded, validated topic graph.
type DAG struct {
	topics []Topic
	byCode map[string]Topic
	// unlocks maps a topic to the topics that name it as a prerequisite, so the
	// UI can say "finishing this opens two more" without rescanning.
	unlocks map[string][]string
}

// Load reads the topic DAG out of a language pack.
//
// A pack with a broken DAG is a build error we would rather catch here than
// discover as a learner stuck behind a cycle, so Load validates and refuses.
func Load(p *lang.Pack) (*DAG, error) {
	if p == nil {
		return nil, fmt.Errorf("no language pack")
	}
	rows, err := p.DB().Query(`
		SELECT id, code, level, title_ru, title_en, explain_md_ru, explain_md_en,
		       prereq_codes, feature_ids, teach_order
		  FROM grammar_topic ORDER BY teach_order`)
	if err != nil {
		return nil, fmt.Errorf("load curriculum: %w", err)
	}
	defer rows.Close()

	d := &DAG{byCode: map[string]Topic{}, unlocks: map[string][]string{}}
	for rows.Next() {
		var t Topic
		var prereqs, feats string
		if err := rows.Scan(&t.ID, &t.Code, &t.Level, &t.TitleRU, &t.TitleEN,
			&t.ExplainRU, &t.ExplainEN, &prereqs, &feats, &t.TeachOrder); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		t.PrereqCodes = splitList(prereqs)
		t.FeatureIDs = splitList(feats)
		d.topics = append(d.topics, t)
		d.byCode[t.Code] = t
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(d.topics) == 0 {
		return nil, fmt.Errorf("language pack contains no grammar topics")
	}
	for _, t := range d.topics {
		for _, p := range t.PrereqCodes {
			d.unlocks[p] = append(d.unlocks[p], t.Code)
		}
	}
	if problems := d.Validate(); len(problems) > 0 {
		return nil, fmt.Errorf("language pack curriculum is invalid: %s",
			strings.Join(problems, "; "))
	}
	return d, nil
}

// splitList parses the pack's comma-separated list columns.
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Validate reports structural problems: unknown prerequisites, self-references
// and cycles. The pipeline checks the same things at build time; checking again
// here is cheap and means a hand-edited pack cannot wedge a learner.
func (d *DAG) Validate() []string {
	var problems []string
	for _, t := range d.topics {
		for _, p := range t.PrereqCodes {
			if p == t.Code {
				problems = append(problems, t.Code+" is its own prerequisite")
				continue
			}
			if _, ok := d.byCode[p]; !ok {
				problems = append(problems, t.Code+" requires unknown topic "+p)
			}
		}
	}

	// Kahn's algorithm: anything left with unresolved prerequisites is in a cycle.
	indegree := map[string]int{}
	for _, t := range d.topics {
		for _, p := range t.PrereqCodes {
			if _, ok := d.byCode[p]; ok && p != t.Code {
				indegree[t.Code]++
			}
		}
	}
	queue := make([]string, 0, len(d.topics))
	for _, t := range d.topics {
		if indegree[t.Code] == 0 {
			queue = append(queue, t.Code)
		}
	}
	settled := 0
	for len(queue) > 0 {
		code := queue[0]
		queue = queue[1:]
		settled++
		for _, next := range d.unlocks[code] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if settled != len(d.topics) {
		var stuck []string
		for _, t := range d.topics {
			if indegree[t.Code] > 0 {
				stuck = append(stuck, t.Code)
			}
		}
		sort.Strings(stuck)
		problems = append(problems, "prerequisite cycle among: "+strings.Join(stuck, ", "))
	}
	return problems
}

// Topics returns every topic in teach order.
func (d *DAG) Topics() []Topic { return d.topics }

// Topic looks one up by code.
func (d *DAG) Topic(code string) (Topic, bool) {
	t, ok := d.byCode[code]
	return t, ok
}

// Unlocks returns the topics that directly depend on this one.
func (d *DAG) Unlocks(code string) []string {
	out := append([]string(nil), d.unlocks[code]...)
	sort.Strings(out)
	return out
}

// AncestorsOf returns every transitive prerequisite of a topic, nearest first.
// Used by the "why is this locked" explanation, which names the specific topic
// standing in the way rather than saying "prerequisites unmet".
func (d *DAG) AncestorsOf(code string) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(string)
	walk = func(c string) {
		t, ok := d.byCode[c]
		if !ok {
			return
		}
		for _, p := range t.PrereqCodes {
			if seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
			walk(p)
		}
	}
	walk(code)
	return out
}

// FeatureTopic maps a morphological feature back to the topic that teaches it.
//
// This is the join that makes error-driven follow-up possible: a wrong inessive
// in a reading review is an error in `Case=Ine`, which is taught by
// `locative-grid`, which is the topic whose drills should come next. When two
// topics teach the same feature the earlier one wins, because that is where the
// rule was actually introduced.
func (d *DAG) FeatureTopic(featureID string) (Topic, bool) {
	best := Topic{}
	found := false
	for _, t := range d.topics {
		if !t.hasFeature(featureID) {
			continue
		}
		if !found || t.TeachOrder < best.TeachOrder {
			best, found = t, true
		}
	}
	return best, found
}

// FeatureTopics indexes every feature to its teaching topic, for callers that
// resolve many at once.
func (d *DAG) FeatureTopics() map[string]string {
	out := map[string]string{}
	for i := len(d.topics) - 1; i >= 0; i-- {
		t := d.topics[i]
		for _, f := range t.FeatureIDs {
			out[f] = t.Code // earlier topics overwrite later ones
		}
	}
	return out
}
