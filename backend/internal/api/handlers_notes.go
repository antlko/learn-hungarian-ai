package api

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/srs"
)

// CreateNoteRequest adds something to study.
//
// A note is a sentence, not a word: the brief is explicit that a bare lemma
// teaches nothing about the morphology that carries the meaning, and the cloze
// direction — the only one that grades features — needs a span to blank.
type CreateNoteRequest struct {
	HU         string `json:"hu"`
	TargetForm string `json:"targetForm"`
	GlossRU    string `json:"glossRu"`
	GlossEN    string `json:"glossEn"`
	LemmaID    *int64 `json:"lemmaId,omitempty"`
	SentenceID *int64 `json:"sentenceId,omitempty"`
	Notes      string `json:"notes"`
	Source     string `json:"source"`
}

// CreateNoteResponse reports what was created.
type CreateNoteResponse struct {
	NoteID      int64    `json:"noteId"`
	Directions  []string `json:"directions"`
	TargetFeats []string `json:"targetFeats"`
	// Duplicate reports that this exact sentence and target were already saved,
	// so nothing new was created. The client says so rather than pretending.
	Duplicate bool `json:"duplicate"`
	// TargetStart/End locate the blank, so the client does not have to re-find it.
	TargetStart int `json:"targetStart"`
	TargetEnd   int `json:"targetEnd"`
}

func (s *Server) handleCreateNote(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req CreateNoteRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.HU == "" {
		return badRequest("hu is required")
	}

	note := srs.Note{
		UserID: u.ID, Kind: "sentence", HU: req.HU,
		TargetForm: req.TargetForm, TargetStart: -1, TargetEnd: -1,
		GlossRU: req.GlossRU, GlossEN: req.GlossEN,
		LemmaID: req.LemmaID, SentenceID: req.SentenceID,
		Notes: req.Notes, Source: req.Source,
	}

	// Locate the target span and read its features off the analysis. Without a
	// pack the note is still created — it simply spawns no cloze card, because
	// there is nothing to attribute a feature error to.
	if s.analyzer != nil && req.TargetForm != "" {
		tokens, _, err := s.analyzer.AnalyzeText(req.HU)
		if err != nil {
			return err
		}
		for _, t := range tokens {
			if !t.IsWord || !equalFold(t.Text, req.TargetForm) {
				continue
			}
			note.TargetStart, note.TargetEnd = t.Start, t.End
			note.TargetFeats = t.Analysis.FeatureID
			if note.LemmaID == nil && t.Analysis.LemmaID != 0 {
				id := t.Analysis.LemmaID
				note.LemmaID = &id
			}
			break
		}
		if note.TargetStart < 0 {
			return badRequest("targetForm %q does not occur in the sentence", req.TargetForm)
		}
	}

	store := srs.NewStore(s.db.DB)

	// Re-adding the same word from the reader must not double the review load.
	if existing, err := store.FindDuplicate(u.ID, note.HU, note.TargetForm); err != nil {
		return err
	} else if existing != 0 {
		feats := note.TargetFeats
		if feats == nil {
			feats = []string{}
		}
		return c.JSON(CreateNoteResponse{
			NoteID: existing, Directions: []string{}, TargetFeats: feats,
			Duplicate: true, TargetStart: note.TargetStart, TargetEnd: note.TargetEnd,
		})
	}

	cfg := s.Config()
	plan := srs.PlanFor(cfg.Review.Goal, directionWeights(cfg), s.audioAvailable())
	directions := plan.Directions

	// A cloze card with no blanked span and no features would grade nothing.
	if len(note.TargetFeats) == 0 {
		directions = withoutCloze(directions)
	}
	if len(directions) == 0 {
		return badRequest("no review directions are enabled")
	}

	id, cards, err := store.CreateNote(note, directions, time.Now())
	if err != nil {
		return err
	}
	// A lemma with a card is a lemma in play: it enters the learning state,
	// which is what the lemma curve and the checkpoints count. A learner's own
	// `known`/`ignored` declaration is never overridden by this.
	if note.LemmaID != nil {
		if err := store.MarkLemmaLearning(u.ID, *note.LemmaID); err != nil {
			return err
		}
	}

	names := make([]string, 0, len(cards))
	for _, c := range cards {
		names = append(names, string(c.Direction))
	}
	feats := note.TargetFeats
	if feats == nil {
		feats = []string{}
	}
	return c.Status(fiber.StatusCreated).JSON(CreateNoteResponse{
		NoteID: id, Directions: names, TargetFeats: feats,
		TargetStart: note.TargetStart, TargetEnd: note.TargetEnd,
	})
}

// MasteryResponse is the per-feature heatmap.
type MasteryResponse struct {
	Features []srs.FeatureMastery `json:"features"`
}

func (s *Server) handleMastery(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	cfg := s.Config()
	got, err := s.engine().Mastery(u.ID, cfg.Drill.HarmonyLatencyMs, time.Now())
	if err != nil {
		return err
	}
	if got == nil {
		got = []srs.FeatureMastery{}
	}
	return c.JSON(MasteryResponse{Features: got})
}

func withoutCloze(in []srs.Direction) []srs.Direction {
	out := make([]srs.Direction, 0, len(in))
	for _, d := range in {
		if d != srs.Cloze {
			out = append(out, d)
		}
	}
	return out
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		// Hungarian case folding never changes byte length for the letters we
		// care about, so a length mismatch is a fast reject.
		if len([]rune(a)) != len([]rune(b)) {
			return false
		}
	}
	ar, br := []rune(a), []rune(b)
	if len(ar) != len(br) {
		return false
	}
	for i := range ar {
		if ar[i] != br[i] && lowerRune(ar[i]) != lowerRune(br[i]) {
			return false
		}
	}
	return true
}

func lowerRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	// Hungarian accented capitals.
	switch r {
	case 'Á':
		return 'á'
	case 'É':
		return 'é'
	case 'Í':
		return 'í'
	case 'Ó':
		return 'ó'
	case 'Ö':
		return 'ö'
	case 'Ő':
		return 'ő'
	case 'Ú':
		return 'ú'
	case 'Ü':
		return 'ü'
	case 'Ű':
		return 'ű'
	}
	return r
}
