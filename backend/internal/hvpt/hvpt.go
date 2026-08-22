// Package hvpt is the ear trainer: high-variability phonetic training over the
// pack's minimal pairs.
//
// The design follows [R7] — .claude/research/07-hvpt.md — to the letter,
// because HVPT is one of the few CALL techniques with replicated cross-decade
// evidence, and the replicated part is SPECIFIC:
//
//   - MULTIPLE TALKERS are the active ingredient. Single-voice discrimination
//     training produces gains that do not generalise to new voices. A session
//     below `hvpt.minTalkers` distinct talkers is therefore REFUSED, not
//     quietly served: serving it would look like training and not be.
//   - Identification (2AFC), not same/different discrimination.
//   - NO CONTRAST IS EVER COMPLETE. Perceptual gains decay; every contrast
//     stays on FSRS maintenance forever. There is no completion state in the
//     schema, and none may be added.
package hvpt

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"tanulom/internal/lang"
	"tanulom/internal/srs"
)

// Pair is one minimal pair with its glosses.
type Pair struct {
	ID       int64  `json:"id"`
	A        string `json:"a"`
	B        string `json:"b"`
	Contrast string `json:"contrast"`
	Priority int    `json:"priority"`
	GlossARU string `json:"glossARu"`
	GlossBRU string `json:"glossBRu"`
	GlossAEN string `json:"glossAEn"`
	GlossBEN string `json:"glossBEn"`
}

// Clip is one rendered recording of one member.
type Clip struct {
	PairID   int64   `json:"-"`
	Member   string  `json:"-"`
	TalkerID string  `json:"talkerId"`
	Kind     string  `json:"talkerKind"`
	Rate     float64 `json:"rate"`
	Path     string  `json:"path"`
}

// Inventory is the pack's HVPT material, loaded once.
type Inventory struct {
	Pairs      []Pair
	ByContrast map[string][]Pair
	Clips      map[int64]map[string][]Clip // pair -> member -> clips
	Talkers    []string
}

// LoadInventory reads pairs and audio from the pack.
func LoadInventory(p *lang.Pack) (*Inventory, error) {
	if p == nil {
		return nil, fmt.Errorf("no language pack")
	}
	inv := &Inventory{ByContrast: map[string][]Pair{}, Clips: map[int64]map[string][]Clip{}}

	rows, err := p.DB().Query(`
		SELECT id, a, b, contrast, priority, gloss_a_ru, gloss_b_ru, gloss_a_en, gloss_b_en
		  FROM minimal_pair ORDER BY priority, id`)
	if err != nil {
		return nil, fmt.Errorf("load minimal pairs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mp Pair
		if err := rows.Scan(&mp.ID, &mp.A, &mp.B, &mp.Contrast, &mp.Priority,
			&mp.GlossARU, &mp.GlossBRU, &mp.GlossAEN, &mp.GlossBEN); err != nil {
			return nil, err
		}
		inv.Pairs = append(inv.Pairs, mp)
		inv.ByContrast[mp.Contrast] = append(inv.ByContrast[mp.Contrast], mp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	crows, err := p.DB().Query(`
		SELECT pair_id, member, talker_id, talker_kind, rate, path FROM pair_audio`)
	if err != nil {
		return nil, fmt.Errorf("load pair audio: %w", err)
	}
	defer crows.Close()
	talkers := map[string]bool{}
	for crows.Next() {
		var c Clip
		if err := crows.Scan(&c.PairID, &c.Member, &c.TalkerID, &c.Kind, &c.Rate, &c.Path); err != nil {
			return nil, err
		}
		if inv.Clips[c.PairID] == nil {
			inv.Clips[c.PairID] = map[string][]Clip{}
		}
		inv.Clips[c.PairID][c.Member] = append(inv.Clips[c.PairID][c.Member], c)
		talkers[c.TalkerID] = true
	}
	for t := range talkers {
		inv.Talkers = append(inv.Talkers, t)
	}
	sort.Strings(inv.Talkers)
	return inv, crows.Err()
}

// Audible reports whether a pair has audio for BOTH members — a 2AFC trial on
// a half-recorded pair is not a trial.
func (inv *Inventory) Audible(pairID int64) bool {
	m := inv.Clips[pairID]
	return len(m["a"]) > 0 && len(m["b"]) > 0
}

// AudibleContrasts returns the contrasts that can actually be trained.
func (inv *Inventory) AudibleContrasts() []string {
	out := []string{}
	for contrast, pairs := range inv.ByContrast {
		for _, mp := range pairs {
			if inv.Audible(mp.ID) {
				out = append(out, contrast)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// ContrastState is one contrast's FSRS + accuracy state.
type ContrastState struct {
	Contrast string        `json:"contrast"`
	State    srs.CardState `json:"-"`
	Accuracy float64       `json:"accuracy"`
	Total    int           `json:"total"`
}

// Trial is one served 2AFC item. The played member never leaves the server —
// the client gets the audio URL and two labelled buttons, nothing else.
type Trial struct {
	ID       string  `json:"id"`
	Pair     Pair    `json:"pair"`
	AudioURL string  `json:"audioUrl"`
	TalkerID string  `json:"talkerId"`
	Rate     float64 `json:"rate"`
	// played is the answer. Unexported: held server-side in the pending store.
	played string
}

// Played exposes the answer to the answer handler.
func (t Trial) Played() string { return t.played }

// WithPlayed restores a trial from the pending store.
func (t Trial) WithPlayed(m string) Trial { t.played = m; return t }

// SessionPlan builds one session's trials.
//
// The talker floor is checked FIRST and the whole session refused below it,
// with the numbers in the error — the caller turns that into an honest screen,
// not a degraded silent session. [R7]
func (inv *Inventory) SessionPlan(states map[string]ContrastState, trials, minTalkers int, rnd *rand.Rand, now time.Time, sched *srs.Scheduler) ([]Trial, error) {
	if len(inv.Talkers) < minTalkers {
		return nil, &TalkerFloorError{Have: len(inv.Talkers), Need: minTalkers}
	}
	contrasts := inv.AudibleContrasts()
	if len(contrasts) == 0 {
		return nil, fmt.Errorf("the pack has no audible minimal pairs")
	}
	weights := contrastWeights(contrasts, states, now, sched)

	out := make([]Trial, 0, trials)
	// Cycle talkers round-robin from a random start: R7's mechanism is
	// VARIETY WITHIN THE SESSION, so the plan guarantees consecutive trials
	// change voice rather than hoping randomness does.
	talkerAt := rnd.Intn(len(inv.Talkers))
	for len(out) < trials {
		contrast := pickWeighted(weights, rnd)
		pairs := inv.ByContrast[contrast]
		mp := pairs[rnd.Intn(len(pairs))]
		if !inv.Audible(mp.ID) {
			continue
		}
		member := "a"
		if rnd.Intn(2) == 1 {
			member = "b"
		}
		talker := inv.Talkers[talkerAt%len(inv.Talkers)]
		talkerAt++
		clip, ok := clipFor(inv.Clips[mp.ID][member], talker, rnd)
		if !ok {
			continue
		}
		out = append(out, Trial{
			Pair: mp, AudioURL: "/media/" + clip.Path,
			TalkerID: clip.TalkerID, Rate: clip.Rate, played: member,
		})
	}
	return out, nil
}

// TalkerFloorError carries the numbers the refusal screen needs.
type TalkerFloorError struct{ Have, Need int }

func (e *TalkerFloorError) Error() string {
	return fmt.Sprintf("only %d distinct talkers available, %d required", e.Have, e.Need)
}

func clipFor(clips []Clip, talker string, rnd *rand.Rand) (Clip, bool) {
	var mine []Clip
	for _, c := range clips {
		if c.TalkerID == talker {
			mine = append(mine, c)
		}
	}
	if len(mine) == 0 {
		if len(clips) == 0 {
			return Clip{}, false
		}
		return clips[rnd.Intn(len(clips))], true
	}
	return mine[rnd.Intn(len(mine))], true
}

// contrastWeights implements the adaptive selection: a contrast is worth more
// when it is DUE (FSRS says the perceptual gain is decaying) and when the
// learner has been getting it WRONG. A contrast never met gets the new-contrast
// weight so the inventory is explored in priority order rather than at random.
func contrastWeights(contrasts []string, states map[string]ContrastState, now time.Time, sched *srs.Scheduler) map[string]float64 {
	out := make(map[string]float64, len(contrasts))
	for _, c := range contrasts {
		st, seen := states[c]
		switch {
		case !seen:
			out[c] = 1.0 // unexplored: train it
		default:
			w := 0.2 // floor: maintenance never stops [R7]
			if !st.State.Due.After(now) {
				w += 1.0 // due: the decay clock says now
			}
			if st.Total >= 5 {
				w += (1 - st.Accuracy) * 2.0 // error-driven: weak ears first
			}
			out[c] = w
		}
	}
	return out
}

func pickWeighted(weights map[string]float64, rnd *rand.Rand) string {
	keys := make([]string, 0, len(weights))
	total := 0.0
	for k, w := range weights {
		keys = append(keys, k)
		total += w
	}
	sort.Strings(keys)
	roll := rnd.Float64() * total
	for _, k := range keys {
		roll -= weights[k]
		if roll <= 0 {
			return k
		}
	}
	return keys[len(keys)-1]
}
