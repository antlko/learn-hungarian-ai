package api

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/db"
	"tanulom/internal/srs"
	"tanulom/internal/stats"
)

// The estimated level and text coverage (ADR-047). Computed per request from
// the learner's deck, never stored; the pure model is stats.EstimateLevel and
// this file only gathers its inputs. Words are counted by LEMMA — a Hungarian
// word-family unit would inflate the figure by an order of magnitude [R6].

// packBands is what the read-only pack says about CEFR bands, cached per boot:
// the pack never changes under a running server, and the two aggregates scan
// every lemma.
type packBands struct {
	sizes     map[string]int // lemmas per band A1..C1
	freqTotal float64        // sum of freq_pm over every frequency-ranked lemma
}

func (s *Server) bands() (*packBands, error) {
	s.bandsMu.Lock()
	defer s.bandsMu.Unlock()
	if s.bandsCache != nil {
		return s.bandsCache, nil
	}
	pb := &packBands{sizes: map[string]int{}}
	if s.pack != nil {
		rows, err := s.pack.DB().Query(`
			SELECT cefr, COUNT(*) FROM lemma
			 WHERE cefr IN ('A1','A2','B1','B2','C1') GROUP BY cefr`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var band string
			var n int
			if err := rows.Scan(&band, &n); err != nil {
				rows.Close()
				return nil, err
			}
			pb.sizes[band] = n
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		var total sql.NullFloat64
		if err := s.pack.DB().QueryRow(
			`SELECT SUM(freq_pm) FROM lemma WHERE freq_pm > 0`).Scan(&total); err != nil {
			return nil, err
		}
		pb.freqTotal = total.Float64
	}
	s.bandsCache = pb
	return pb, nil
}

// countedLemmas returns the lemma ids that COUNT toward the estimate: status
// `known`, or a graduated card (FSRS state Review) whose retrievability right
// now is at least mining.knownThreshold — the same line the miner uses to call
// a word known. A word still on its learning steps does not count, and a
// decayed one stops counting, so the figures can go down.
func (s *Server) countedLemmas(userID int64, now time.Time) (map[int64]bool, error) {
	set, _, err := s.countedAndSettled(userID, now)
	return set, err
}

// countedAndSettled returns the counted set and the SETTLED set from one scan
// and one cache entry, so the two figures on the plan screen cannot disagree
// about when they were measured and answering a card drops both.
func (s *Server) countedAndSettled(userID int64, now time.Time) (counted, settled map[int64]bool, err error) {
	s.countedMu.Lock()
	if e, ok := s.countedCache[userID]; ok && now.Sub(e.at) < countedTTL {
		s.countedMu.Unlock()
		return e.set, e.settled, nil
	}
	s.countedMu.Unlock()
	set, settledSet, err := s.scanCountedLemmas(userID, now)
	if err != nil {
		return nil, nil, err
	}
	s.countedMu.Lock()
	if s.countedCache == nil {
		s.countedCache = map[int64]countedEntry{}
	}
	s.countedCache[userID] = countedEntry{at: now, set: set, settled: settledSet}
	s.countedMu.Unlock()
	return set, settledSet, nil
}

// countedTTL bounds how stale the cached counted set can be; answering a card
// drops it at once (recordInteraction), so the TTL only covers other changes
// («Знаю», decay with time). Callers must treat the returned map as read-only.
const countedTTL = 30 * time.Second

type countedEntry struct {
	at      time.Time
	set     map[int64]bool
	settled map[int64]bool // see scanCountedLemmas
	deck    *DeckStages    // GET /api/stats/deck, filled lazily; dies with the entry
}

func (s *Server) invalidateCounted(userID int64) {
	s.countedMu.Lock()
	delete(s.countedCache, userID)
	s.countedMu.Unlock()
}

// scanCountedLemmas also returns the SETTLED lemmas (course.UnlockShare): status
// `known`, or a non-suspended, non-trashed card whose stage is mature
// (srs.StageOf: graduated with an interval of srs.MatureDays or more). The
// stage machinery is the Words list's own; no FSRS maths is repeated here.
func (s *Server) scanCountedLemmas(userID int64, now time.Time) (map[int64]bool, map[int64]bool, error) {
	counted := map[int64]bool{}
	settled := map[int64]bool{}
	rows, err := s.db.Query(`
		SELECT lemma_id, status FROM lemma_state
		 WHERE user_id = ? AND status IN ('known','learning')`, userID)
	if err != nil {
		return nil, nil, err
	}
	learning := map[int64]bool{}
	for rows.Next() {
		var id int64
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if status == "known" {
			counted[id] = true
			settled[id] = true
		} else {
			learning[id] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}

	threshold := s.Config().Mining.KnownThreshold
	sched := s.engine().Scheduler()
	crows, err := s.db.Query(`
		SELECT n.lemma_id, c.due, c.stability, c.difficulty, c.scheduled_days,
		       c.reps, c.lapses, c.state, c.last_review, c.remaining_steps
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND n.lemma_id IS NOT NULL AND c.suspended = 0
		   AND c.state = 2`+srs.NotTrashed, userID)
	if err != nil {
		return nil, nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var lemmaID int64
		var st srs.CardState
		var due string
		var last sql.NullString
		if err := crows.Scan(&lemmaID, &due, &st.Stability, &st.Difficulty,
			&st.ScheduledDays, &st.Reps, &st.Lapses, &st.State, &last,
			&st.RemainingSteps); err != nil {
			return nil, nil, err
		}
		if srs.StageOf(st) == srs.StageMature {
			settled[lemmaID] = true
		}
		if !learning[lemmaID] || counted[lemmaID] {
			continue
		}
		st.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
		st.LastReview = db.ParseTime(last)
		if sched.Retrievability(st, now) >= threshold {
			counted[lemmaID] = true
		}
	}
	return counted, settled, crows.Err()
}

func (s *Server) handleStatsLevel(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	pb, err := s.bands()
	if err != nil {
		return err
	}
	counted, err := s.countedLemmas(u.ID, time.Now())
	if err != nil {
		return err
	}

	in := stats.LevelInput{
		Counted: map[string]int{}, Sizes: pb.sizes,
		CountedLemmas: len(counted), FreqTotal: pb.freqTotal,
		Blocked: map[string]bool{},
	}
	if s.pack != nil {
		ids := make([]int64, 0, len(counted))
		for id := range counted {
			ids = append(ids, id)
		}
		for start := 0; start < len(ids); start += 500 {
			part := ids[start:min(start+500, len(ids))]
			args := make([]any, len(part))
			for i, id := range part {
				args[i] = id
			}
			rows, err := s.pack.DB().Query(`
				SELECT COALESCE(cefr, ''), COALESCE(freq_pm, 0) FROM lemma
				 WHERE id IN (?`+repeatComma(len(part)-1)+`)`, args...)
			if err != nil {
				return err
			}
			for rows.Next() {
				var band string
				var pm float64
				if err := rows.Scan(&band, &pm); err != nil {
					rows.Close()
					return err
				}
				in.Counted[band]++
				if pm > 0 {
					in.FreqCounted += pm
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
	}

	history, err := stats.NewStore(s.db.DB).Checkpoints(u.ID)
	if err != nil {
		return err
	}
	in.Blocked = stats.BlockedLevels(history)

	// Grammar is read only where the pack has a curriculum; without one no
	// band is held back by it.
	if s.dag != nil && s.drills != nil {
		states, _, err := s.computeCurriculum(u.ID) // read-only: never drill.Persist on a GET
		if err != nil {
			return err
		}
		for _, st := range states {
			in.Topics = append(in.Topics, stats.TopicPhase{
				Level: st.Topic.Level, Drillable: st.Availability.Available,
				Phase: string(st.Decision.Phase),
			})
		}
	}
	return c.JSON(stats.EstimateLevel(in))
}

// BandWord is a word of a CEFR band the learner does not yet count.
type BandWord struct {
	LemmaID  int64  `json:"lemmaId"`
	Lemma    string `json:"lemma"`
	GlossRU  string `json:"glossRu"`
	GlossEN  string `json:"glossEn"`
	FreqRank int    `json:"freqRank"`
}

// BandWordsResponse is GET /api/stats/level/band/:band.
type BandWordsResponse struct {
	Words []BandWord `json:"words"`
}

// handleStatsLevelBand lists the most frequent lemmas of a band that do not
// yet count — what to learn next to move that band's bar. Read-only: nothing
// here adds a note or touches a state space.
func (s *Server) handleStatsLevelBand(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	band := strings.ToUpper(c.Params("band"))
	valid := false
	for _, b := range stats.LevelBands {
		valid = valid || b == band
	}
	if !valid {
		return badRequest("band must be one of %s", strings.Join(stats.LevelBands, ", "))
	}
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	limit = min(max(limit, 1), 200)

	out := BandWordsResponse{Words: []BandWord{}}
	if s.pack == nil {
		return c.JSON(out)
	}
	counted, err := s.countedLemmas(u.ID, time.Now())
	if err != nil {
		return err
	}
	rows, err := s.pack.DB().Query(`
		SELECT id, lemma, COALESCE(freq_rank, 0) FROM lemma
		 WHERE cefr = ?
		 ORDER BY freq_rank IS NULL, freq_rank, id`, band)
	if err != nil {
		return err
	}
	// Homonyms (hogy x2, mi x3, egy x3) are one word to a learner. Rows arrive
	// best-rank first, so a group's first row is its best-ranked one; the groups
	// keep that order. Over-fetch every uncounted row, then trim to `limit`
	// DISTINCT lemmas.
	var order []string
	groups := map[string][]BandWord{}
	for rows.Next() {
		var w BandWord
		if err := rows.Scan(&w.LemmaID, &w.Lemma, &w.FreqRank); err != nil {
			rows.Close()
			return err
		}
		if counted[w.LemmaID] {
			continue
		}
		k := strings.ToLower(w.Lemma)
		if _, seen := groups[k]; !seen {
			if len(order) >= limit {
				continue // beyond the limit and not a homonym of anything kept
			}
			order = append(order, k)
		}
		groups[k] = append(groups[k], w)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var ids []int64
	for _, k := range order {
		for _, w := range groups[k] {
			ids = append(ids, w.LemmaID)
		}
	}
	data, err := s.lemmaData(ids)
	if err != nil {
		return err
	}
	lang := s.answerLang(c.Query("lang"))
	hasGloss := func(id int64) bool {
		if lang == "en" {
			return data[id].GlossEN != ""
		}
		return data[id].GlossRU != ""
	}
	for _, k := range order {
		pick := groups[k][0]
		for _, w := range groups[k] {
			if hasGloss(w.LemmaID) {
				pick = w // first = best-ranked among those with a gloss
				break
			}
		}
		pick.GlossRU, pick.GlossEN = data[pick.LemmaID].GlossRU, data[pick.LemmaID].GlossEN
		out.Words = append(out.Words, pick)
	}
	return c.JSON(out)
}
