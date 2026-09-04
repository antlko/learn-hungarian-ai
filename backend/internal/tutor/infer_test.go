package tutor

import (
	"strings"
	"testing"
)

func testIndex() []TopicIndex {
	return []TopicIndex{
		NewTopicIndex("vowel-harmony",
			[]string{"Гармония гласных", "Vowel harmony"},
			[]string{"гармония гласных", "vowel harmony"},
			nil,
			"Суффиксы выбирают гласную по гласным основы. Задние и передние гласные. Примеры: ház, kert.",
			[]string{"Skill=VowelHarmony"}),
		NewTopicIndex("locative-grid",
			[]string{"Локативная система", "The locative grid"},
			[]string{"инессив — в", "inessive", "иллатив", "illative"},
			[]string{"ban", "ben", "ba", "be"},
			"Девять клеток: где, куда, откуда. Гласные основы решают вариант суффикса.",
			[]string{"Case=Ine", "Case=Ill"}),
		NewTopicIndex("plural",
			[]string{"Множественное число", "The plural"},
			[]string{"множественное число", "plural"},
			[]string{"k", "ok", "ek", "ök", "ak"},
			"Суффикс -k с соединительной гласной.",
			[]string{"Number=Plur"}),
	}
}

// The analyser's reading of a quoted form is the strongest signal: a learner
// who writes `házban` is asking about the case it carries, whatever else the
// sentence says.
func TestInferTopicFollowsTheFormsFeatures(t *testing.T) {
	got := InferTopic("Почему házban, а не házben?", []string{"Case=Ine"}, "", testIndex())
	if got.Topic != "locative-grid" || got.Sticky {
		t.Errorf("got %+v, want locative-grid from Case=Ine", got)
	}
}

func TestInferTopicReadsTitlesLabelsAndSuffixShapes(t *testing.T) {
	cases := map[string]string{
		"что такое гармония гласных?":        "vowel-harmony",
		"How does the plural work?":          "plural",
		"чем отличается -ek от -ok?":         "plural",
		"когда -ban, а когда -ben?":          "locative-grid",
		"explain the inessive with examples": "locative-grid",
		"Девять клеток: где, куда и откуда?": "locative-grid",
		"как дела":                           "",
		"":                                   "",
	}
	for q, want := range cases {
		if got := InferTopic(q, nil, "", testIndex()); got.Topic != want {
			t.Errorf("%q: got %+v, want %q", q, got, want)
		}
	}
}

// Lesson prose alone is capped: a lesson that mentions everything must not win
// every question, but a question in the lesson's own words still reaches it.
func TestInferTopicCapsLessonVocabulary(t *testing.T) {
	got := InferTopic("гласные основы решают вариант суффикса", nil, "", testIndex())
	if got.Topic != "locative-grid" || got.Score > lessonCap+0.01 {
		t.Errorf("lesson words: got %+v, want locative-grid at the cap", got)
	}
}

// A follow-up with no signal stays on the lesson the last answer used; with no
// previous grounding, nothing is chosen and the prompt says so.
func TestInferTopicIsStickyOnFollowUps(t *testing.T) {
	got := InferTopic("а ещё примеры?", nil, "vowel-harmony", testIndex())
	if got.Topic != "vowel-harmony" || !got.Sticky {
		t.Errorf("follow-up: got %+v, want the sticky topic", got)
	}
	if got := InferTopic("а ещё примеры?", nil, "", testIndex()); got.Topic != "" {
		t.Errorf("no grounding at all: got %+v, want none", got)
	}
	// A question with its own signal overrides the sticky topic.
	if got := InferTopic("how does the plural work?", nil, "vowel-harmony", testIndex()); got.Topic != "plural" || got.Sticky {
		t.Errorf("new subject: got %+v, want plural", got)
	}
}

func TestInferTokensKeepTwoLetterSuffixShapes(t *testing.T) {
	got := strings.Join(inferTokens("Чем -ek отличается от -em и от ок?"), " ")
	if want := "-ek отличается -em"; got != want {
		t.Errorf("inferTokens = %q, want %q", got, want)
	}
}
