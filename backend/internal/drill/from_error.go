package drill

import (
	"sort"
	"time"

	"tanulom/internal/curriculum"
	"tanulom/internal/morph"
)

// Error -> underlying feature -> targeted follow-up.
//
// "Wrong" is not feedback. What decides the remedy is the CLASS of the mistake,
// because the remedies are genuinely different skills: a wrong allomorph needs
// repetition volume, a wrong case needs the rule restated, and a dropped vowel
// length needs the EAR — no amount of typing practice fixes a contrast the
// learner cannot hear. Corrective feedback that names the error and routes to
// the right remedy outperforms simply supplying the answer. [R9], [R15]
//
// There is deliberately no follow-up QUEUE table. A queue drifts: items are
// enqueued from an error, the learner then fixes the weakness elsewhere, and the
// queue keeps serving stale work. Instead the targets are derived on demand from
// the error record we already keep — `feature_review` and `drill_attempt` — so
// the follow-up is always a statement about the learner's current state.

// FeatureError is one feature the learner has been getting wrong.
type FeatureError struct {
	FeatureID string    `json:"featureId"`
	Misses    int       `json:"misses"`
	LastSeen  time.Time `json:"lastSeen"`
}

// Target is what to drill next, and why.
type Target struct {
	FeatureID string `json:"featureId"`
	TopicCode string `json:"topicCode"`
	// Kinds are the item kinds that actually address this error class. Serving a
	// recognition MCQ for a harmony error would let the learner pass by
	// elimination without ever producing the form.
	Kinds []Kind `json:"kinds"`
	// Remedy names what will fix it. Not always a drill — see RemedyEar.
	Remedy Remedy `json:"remedy"`
	Misses int    `json:"misses"`
}

// Remedy is where a class of error has to be fixed. An i18n key, like Reason.
type Remedy string

const (
	// RemedyVolume: understood but not automatic. Repetition, measured in speed.
	RemedyVolume Remedy = "drill.remedy.volume"
	// RemedyRule: the rule itself is not held. Rule card, then recognition, then
	// production. [R4]
	RemedyRule Remedy = "drill.remedy.rule"
	// RemedyEar: a perception problem wearing a spelling costume. Vowel length
	// and rounding are phonemic in Hungarian and absent from both our learners'
	// L1s, so the fix is HVPT, not more typing. Saying so out loud matters: the
	// app must not pretend a typing drill treats a hearing problem.
	RemedyEar Remedy = "drill.remedy.ear"
	// RemedyVocabulary: a different word entirely. This is the review loop's
	// job, not the grammar drill's.
	RemedyVocabulary Remedy = "drill.remedy.vocabulary"
	// RemedyNone: a typo. Not a knowledge failure and not worth acting on.
	RemedyNone Remedy = "drill.remedy.none"
)

// Actionable reports whether a grammar drill can do anything about this remedy.
func (r Remedy) Actionable() bool { return r == RemedyVolume || r == RemedyRule }

// remedyFor maps an error class to its treatment. The table is the whole point
// of classifying errors in the first place — see internal/morph/diff.go.
func remedyFor(class morph.ErrorClass) (Remedy, []Kind) {
	switch class {
	case morph.ErrHarmony:
		return RemedyVolume, []Kind{KindAllomorph, KindTransform}
	case morph.ErrCase:
		return RemedyRule, []Kind{KindCase, KindMCQ, KindCloze}
	case morph.ErrDefinite:
		return RemedyRule, []Kind{KindConjugation}
	case morph.ErrPossessor:
		return RemedyRule, []Kind{KindMCQ, KindTransform}
	case morph.ErrVowelLength, morph.ErrVowelQuality:
		return RemedyEar, nil
	case morph.ErrStem:
		return RemedyVocabulary, nil
	case morph.ErrSpelling:
		return RemedyNone, nil
	}
	return RemedyRule, []Kind{KindCloze}
}

// FromError turns one graded mistake into the follow-up it deserves.
//
// `wrongFeature` is the diff's precise attribution and is preferred when
// present; without it we fall back to the features the item required, because
// blaming a feature the item never tested is exactly the noise this project
// refuses to put in the heatmap.
func FromError(class morph.ErrorClass, wrongFeature string, required []string, dag *curriculum.DAG) []Target {
	remedy, kinds := remedyFor(class)
	if remedy == RemedyNone {
		return nil
	}

	ids := required
	if wrongFeature != "" {
		ids = []string{wrongFeature}
	}
	if class == morph.ErrHarmony {
		// A harmony error is a failure of the harmony skill, whichever suffix it
		// surfaced on. Both are recorded: the skill is what needs volume, the
		// suffix is where to get it.
		ids = append([]string{curriculum.FeatureVowelHarmony}, ids...)
	}

	seen := map[string]bool{}
	out := make([]Target, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		t := Target{FeatureID: id, Kinds: kinds, Remedy: remedy, Misses: 1}
		if dag != nil {
			if topic, ok := dag.FeatureTopic(id); ok {
				t.TopicCode = topic.Code
			}
		}
		out = append(out, t)
	}
	return out
}

// FollowUp turns the learner's recent error record into a prioritised set of
// targets, newest and most-missed first.
//
// `unlocked` filters to topics the learner may actually work on: proposing
// drills for a locked topic would be advice they cannot take.
func FollowUp(errs []FeatureError, dag *curriculum.DAG, unlocked map[string]bool, limit int) []Target {
	if limit <= 0 {
		limit = 6
	}
	out := make([]Target, 0, len(errs))
	for _, e := range errs {
		t := Target{FeatureID: e.FeatureID, Misses: e.Misses, Remedy: RemedyRule}
		if dag != nil {
			topic, ok := dag.FeatureTopic(e.FeatureID)
			if !ok {
				// A feature no topic teaches is a curriculum hole, not a learner
				// problem. Skip it rather than inventing a topic to blame.
				continue
			}
			t.TopicCode = topic.Code
			if unlocked != nil && !unlocked[topic.Code] {
				continue
			}
			if topic.MotorSkill() || e.FeatureID == curriculum.FeatureVowelHarmony {
				t.Remedy = RemedyVolume
				t.Kinds = []Kind{KindAllomorph, KindTransform}
			} else {
				t.Kinds = []Kind{KindCase, KindMCQ, KindCloze}
			}
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Misses > out[j].Misses })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// WeakFeatures flattens targets into the id list the generator over-samples.
func WeakFeatures(targets []Target) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		if t.Remedy.Actionable() {
			out = append(out, t.FeatureID)
		}
	}
	return out
}
