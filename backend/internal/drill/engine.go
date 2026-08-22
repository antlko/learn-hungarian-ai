package drill

import (
	"fmt"
	"strings"
	"time"

	"tanulom/internal/curriculum"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// Answering a drill item.
//
// One drill answer emits the same two things a review answer does, minus the
// card: N feature reviews into the morphological state space, and one movement
// of the topic's gate. Both in ONE transaction — a topic whose accuracy advanced
// while its features did not would leave the gate and the heatmap disagreeing
// about what the learner has practised, and nothing downstream could detect it.

// Engine grades drill answers and moves the gate.
type Engine struct {
	store    *Store
	srs      *srs.Engine
	dag      *curriculum.DAG
	analyzer *morph.Analyzer
	cfg      GateConfig
}

// NewEngine wires the pieces. `srsEngine` supplies the shared scheduler so that
// a feature's FSRS state advances identically whether the evidence came from a
// review or from a drill.
func NewEngine(store *Store, srsEngine *srs.Engine, dag *curriculum.DAG,
	analyzer *morph.Analyzer, cfg GateConfig) *Engine {
	return &Engine{store: store, srs: srsEngine, dag: dag, analyzer: analyzer, cfg: cfg.normalized()}
}

// AnswerInput is one answered item.
type AnswerInput struct {
	UserID int64
	Item   Item
	Answer string
	// UsedHint caps the grade at Hard: retrieval that needed help was not
	// retrieval. Mirrors the review loop.
	UsedHint   bool
	DurationMs int64
	Origin     string
	Gate       Gate
}

// AnswerResult is everything the UI needs to render the outcome and the gate.
type AnswerResult struct {
	Correct        bool               `json:"correct"`
	Expected       string             `json:"expected"`
	Rating         int                `json:"rating"`
	Diff           *morph.Diff        `json:"diff,omitempty"`
	GradedFeatures []srs.FeatureGrade `json:"gradedFeatures"`
	Progress       Progress           `json:"progress"`
	Decision       Decision           `json:"decision"`
	// FollowUp is what this specific mistake says to practise next, including
	// the honest "a drill cannot fix this" cases. See from_error.go.
	FollowUp []Target `json:"followUp,omitempty"`
}

// Answer grades one item and applies its consequences.
func (e *Engine) Answer(in AnswerInput, now time.Time) (AnswerResult, error) {
	item := in.Item
	res := AnswerResult{Expected: item.Expected()}

	outcome := srs.Outcome{
		UsedHint: in.UsedHint,
		Duration: time.Duration(in.DurationMs) * time.Millisecond,
	}

	// Progress must be read before it is folded: the median latency the rating
	// compares against is the learner's pace BEFORE this answer, otherwise the
	// answer would be racing against itself.
	before, err := e.store.Progress(nil, in.UserID, item.TopicCode)
	if err != nil {
		return res, err
	}
	outcome.MedianDuration = time.Duration(before.MedianMs) * time.Millisecond

	var wrongFeature, errorClass string
	if item.Kind.Typed() {
		d := morph.DiffForms(in.Answer, item.Expected(), e.analyzer)
		res.Diff = &d
		outcome.Correct = d.Match
		outcome.PartiallyCorrect = d.NearMatch
		wrongFeature = d.WrongFeature
		errorClass = string(d.Class)
	} else {
		// A choice item is graded by string equality, case-insensitively: the
		// options came from the server, and a sentence-initial capital is not a
		// mistake.
		outcome.Correct = strings.EqualFold(strings.TrimSpace(in.Answer), item.Expected())
		if !outcome.Correct {
			errorClass = string(classifyChoice(item, in.Answer, e.analyzer))
			if errorClass == string(morph.ErrHarmony) {
				wrongFeature = item.Headline
			}
		}
	}
	res.Correct = outcome.Correct

	rating := srs.DeriveRating(outcome)
	res.Rating = int(rating)

	// Only the features the item required, and — when the diff localised the
	// failure — only the one that was actually wrong. Charging a learner for the
	// case when they got the case right and the harmony wrong is exactly the
	// noise this project refuses to put in the heatmap.
	grades := srs.RequiredFeatures(item.FeatureIDs, outcome, wrongFeature)
	res.GradedFeatures = grades

	after := Fold(before, item.Phase, outcome.Correct, in.DurationMs, e.cfg)
	decision := Evaluate(after, in.Gate, e.cfg)
	after = Apply(after, decision, now)
	res.Progress, res.Decision = after, decision

	tx, err := e.store.DB().Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	// The origin comes off the SERVED item, not off the request: the client
	// never sees it, so a placement probe cannot be laundered into ordinary
	// practice or the reverse.
	origin := item.Origin
	if in.Origin != "" {
		origin = in.Origin
	}
	if origin == "" {
		origin = "drill"
	}
	if err := e.srs.GradeFeatures(tx, in.UserID, grades, 0, outcome, origin, now); err != nil {
		return res, err
	}
	if err := e.store.AppendAttempt(tx, in.UserID, Attempt{
		TopicCode: item.TopicCode, Kind: item.Kind, Phase: item.Phase,
		Prompt: promptOf(item), Expected: item.Expected(), Answer: in.Answer,
		Correct: outcome.Correct, DurationMs: in.DurationMs,
		FeatureIDs: item.FeatureIDs, ErrorClass: errorClass, Origin: origin,
	}); err != nil {
		return res, err
	}
	if err := e.store.SaveProgress(tx, in.UserID, after); err != nil {
		return res, err
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit drill answer: %w", err)
	}

	if !outcome.Correct {
		res.FollowUp = FromError(morph.ErrorClass(errorClass), wrongFeature, item.FeatureIDs, e.dag)
	}
	return res, nil
}

// classifyChoice works out what a wrong multiple-choice pick means.
//
// A choice item has no typed string to diff, but the option the learner picked
// still says something: choosing `házben` over `házban` is a harmony error, and
// choosing `házba` over `házban` is a case error. Reusing the same classifier as
// the review diff keeps one vocabulary of mistakes across the whole app.
func classifyChoice(item Item, chosen string, a *morph.Analyzer) morph.ErrorClass {
	if strings.TrimSpace(chosen) == "" {
		return morph.ErrUnknown
	}
	switch item.Kind {
	case KindConjugation:
		return morph.ErrDefinite
	case KindCase:
		return morph.ErrCase
	case KindAllomorph:
		return morph.ErrHarmony
	}
	d := morph.DiffForms(chosen, item.Expected(), a)
	if d.Class == morph.NoError {
		return morph.ErrUnknown
	}
	return d.Class
}

// promptOf renders a stable, language-neutral description of the item for the
// attempt log. Not shown to the learner — the UI renders PromptKey — but a log
// row that cannot be read back is not a log.
func promptOf(item Item) string {
	parts := []string{item.PromptKey}
	if item.Lemma != "" {
		parts = append(parts, item.Lemma)
	}
	if item.Headline != "" {
		parts = append(parts, item.Headline)
	}
	if item.Frame != "" {
		parts = append(parts, item.Frame)
	}
	return strings.Join(parts, " · ")
}
