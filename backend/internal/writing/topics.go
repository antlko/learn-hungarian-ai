// Package writing is the pure core of the «Письмо» (Writing) feature: the topic
// catalogue, the prompt handed to a teacher (the local AI or an external chat),
// the forgiving parser of the answer and the anchoring of every reported error
// in the learner's text. Nothing here touches the database, the HTTP layer or
// the language pack, and nothing here mints a card or a review: the check marks
// a text, it creates nothing.
package writing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed topics.json
var topicsJSON []byte

// TopicsFormat is the only topics.json format this code reads.
const TopicsFormat = 1

// Genres is the closed list of topic genres.
var Genres = []string{"email", "description", "narrative", "opinion"}

// Levels is the closed list of levels the catalogue covers.
var Levels = []string{"A1", "A2", "B1", "B2"}

// Topic is one authored writing task. Titles and briefs are in the learner's
// languages only (no Hungarian). The word range is advice, never a lock.
type Topic struct {
	Code     string `json:"code"`
	Level    string `json:"level"`
	Genre    string `json:"genre"`
	TitleRU  string `json:"titleRu"`
	TitleEN  string `json:"titleEn"`
	BriefRU  string `json:"briefRu"`
	BriefEN  string `json:"briefEn"`
	MinWords int    `json:"minWords"`
	MaxWords int    `json:"maxWords"`
}

type topicsFile struct {
	Format int     `json:"format"`
	Topics []Topic `json:"topics"`
}

// ParseTopics decodes and validates a topics file. A malformed file is a build
// defect, so every problem is an error naming the offending topic.
func ParseTopics(data []byte) ([]Topic, error) {
	var f topicsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("writing topics: %w", err)
	}
	if f.Format != TopicsFormat {
		return nil, fmt.Errorf("writing topics: format %d, want %d", f.Format, TopicsFormat)
	}
	if len(f.Topics) == 0 {
		return nil, fmt.Errorf("writing topics: no topics")
	}
	seen := map[string]bool{}
	for i, t := range f.Topics {
		where := fmt.Sprintf("writing topics: topic #%d (%q)", i+1, t.Code)
		switch {
		case t.Code == "":
			return nil, fmt.Errorf("%s: empty code", where)
		case seen[t.Code]:
			return nil, fmt.Errorf("%s: duplicate code", where)
		case !contains(Levels, t.Level):
			return nil, fmt.Errorf("%s: unknown level %q", where, t.Level)
		case !contains(Genres, t.Genre):
			return nil, fmt.Errorf("%s: unknown genre %q", where, t.Genre)
		case t.TitleRU == "" || t.TitleEN == "" || t.BriefRU == "" || t.BriefEN == "":
			return nil, fmt.Errorf("%s: empty title or brief", where)
		case t.MinWords <= 0 || t.MaxWords < t.MinWords:
			return nil, fmt.Errorf("%s: bad word range %d-%d", where, t.MinWords, t.MaxWords)
		}
		seen[t.Code] = true
	}
	return f.Topics, nil
}

var (
	topicsOnce sync.Once
	topicsVal  []Topic
	topicsErr  error
)

// Topics returns the embedded catalogue in file order (a fresh slice each call).
// The file is parsed once; the server should call it at boot and refuse to start
// on an error.
func Topics() ([]Topic, error) {
	topicsOnce.Do(func() { topicsVal, topicsErr = ParseTopics(topicsJSON) })
	if topicsErr != nil {
		return nil, topicsErr
	}
	return append([]Topic(nil), topicsVal...), nil
}

// FindTopic returns the topic with the given code.
func FindTopic(code string) (Topic, bool) {
	all, err := Topics()
	if err != nil {
		return Topic{}, false
	}
	for _, t := range all {
		if t.Code == code {
			return t, true
		}
	}
	return Topic{}, false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
