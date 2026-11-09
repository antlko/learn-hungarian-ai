package api

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/mining"
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
	// TextID names the library text this word was mined from, so «повторить
	// слова из этого текста» can find it later (note_origin, ADR-022). Optional:
	// course notes and hand-typed ones have no text.
	TextID *int64 `json:"textId,omitempty"`
}

// CreateNoteResponse reports what was created.
type CreateNoteResponse struct {
	NoteID      int64    `json:"noteId"`
	Directions  []string `json:"directions"`
	TargetFeats []string `json:"targetFeats"`
	// Duplicate reports that this exact sentence and target were already saved,
	// so nothing new was created. The client says so rather than pretending.
	Duplicate bool `json:"duplicate"`
	// Restored reports that the duplicate was in the TRASH and has been put
	// back. Adding a word you once set aside is an unambiguous "I want this
	// again", and the alternative was a dead end: the button reported
	// «уже добавлено» while the word stayed invisible in every queue.
	Restored bool `json:"restored,omitempty"`
	// TargetStart/End locate the blank, so the client does not have to re-find it.
	TargetStart int `json:"targetStart"`
	TargetEnd   int `json:"targetEnd"`
	// LemmaStatus is the lemma's knowledge state AFTER this call.
	//
	// The reader recolours the word from this instead of guessing. `known` and
	// `ignored` are learner declarations that minting a card must never
	// override, and that rule lives in one ON CONFLICT clause — a client that
	// reimplemented it would be a second copy, and would recolour a known word
	// backwards the moment the two drifted.
	LemmaStatus string `json:"lemmaStatus,omitempty"`
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

	// A text link must point at the caller's own text; a stale or foreign id is
	// refused rather than stored as a claim nothing can honour.
	if req.TextID != nil {
		if _, err := mining.NewStore(s.db.DB).Get(u.ID, *req.TextID); err != nil {
			return badRequest("no such text")
		}
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

	// A note with no meaning has an EMPTY `produce` prompt — that direction's
	// entire question IS the gloss. The reader only sends a meaning the learner
	// typed by hand, and almost nobody types one, so every word added from a
	// text arrived unanswerable. The pack has a dictionary entry for nearly
	// every word the reader can resolve. Pack data compiled by people, not
	// machine translation, so ADR-014's labelling rule does not reach it.
	//
	// Per field, and never over a meaning the learner supplied.
	if note.LemmaID != nil && *note.LemmaID != 0 && (note.GlossRU == "" || note.GlossEN == "") {
		byID, err := s.lemmaData([]int64{*note.LemmaID})
		if err != nil {
			return err
		}
		if d, ok := byID[*note.LemmaID]; ok {
			if note.GlossRU == "" {
				note.GlossRU = d.GlossRU
			}
			if note.GlossEN == "" {
				note.GlossEN = d.GlossEN
			}
		}
	}

	store := srs.NewStore(s.db.DB)

	// A lemma with a card is a lemma in play: it enters the learning state,
	// which is what the lemma curve and the checkpoints count. A learner's own
	// `known`/`ignored` declaration is never overridden by this.
	//
	// Hoisted ABOVE the duplicate branch deliberately. The upsert is
	// idempotent, and "a note exists for this lemma" is true on both paths — so
	// doing it here removes the special case that left a re-added word with no
	// status to recolour from, rather than adding a second one.
	var lemmaStatus string
	if note.LemmaID != nil {
		if lemmaStatus, err = store.MarkLemmaLearning(u.ID, *note.LemmaID); err != nil {
			return err
		}
	}

	// Re-adding the same word from the reader must not double the review load.
	if existing, err := store.FindDuplicate(u.ID, note.HU, note.TargetForm); err != nil {
		return err
	} else if existing != 0 {
		// The duplicate branch links too: the same sentence met in a SECOND text
		// creates no new note, but that text's claim on the word must not vanish
		// — with a note column instead of the link table it silently would.
		if req.TextID != nil {
			if err := store.LinkNoteOrigin(existing, *req.TextID); err != nil {
				return err
			}
		}
		// A trashed duplicate comes BACK. Otherwise adding a word you had set
		// aside reported success and did nothing visible — FindDuplicate would
		// match the trashed note on every attempt, so the word could never be
		// re-added and the only route back was the trash screen the learner had
		// no reason to look at. Pressing «добавить» is the clearest possible
		// statement that the word is wanted again. Its FSRS state is untouched:
		// restore is not re-learning. See ADR-025.
		restored, err := store.Restore(u.ID, existing)
		if err != nil {
			return err
		}
		feats := note.TargetFeats
		if feats == nil {
			feats = []string{}
		}
		return c.JSON(CreateNoteResponse{
			NoteID: existing, Directions: []string{}, TargetFeats: feats,
			Duplicate: true, Restored: restored,
			TargetStart: note.TargetStart, TargetEnd: note.TargetEnd,
			LemmaStatus: lemmaStatus,
		})
	}

	cfg := s.Config()
	plan := srs.PlanFor(cfg.Review.Goal, directionWeights(cfg), s.audioAvailable())
	directions := plan.Directions

	// A cloze needs a SPAN to blank. Features are what it can ADDITIONALLY
	// grade, not what makes it a card.
	//
	// This used to demand features too, which denied the sentence exercise to
	// 23 of 37 words mined from a text — the exact words the learner chose to
	// keep — and to 21 course notes besides. A featureless cloze grades the
	// card's own FSRS state and emits nothing into the morphological state
	// space (srs.RequiredFeatures returns nil for empty feats, pinned by
	// srs.TestNoTargetFeaturesGradesNothing), so the two spaces stay exactly as
	// separate as before. See ADR-017.
	if !clozeable(note) {
		directions = withoutCloze(directions)
	}
	if len(directions) == 0 {
		return badRequest("no review directions are enabled")
	}

	id, cards, err := store.CreateNote(note, directions, time.Now())
	if err != nil {
		return err
	}
	if req.TextID != nil {
		if err := store.LinkNoteOrigin(id, *req.TextID); err != nil {
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
		LemmaStatus: lemmaStatus,
	})
}

// MasteryResponse is the per-feature heatmap.
type MasteryResponse struct {
	Features []MasteryFeature `json:"features"`
	// Window is `drill.gateWindow`: the number of answers a rolling accuracy
	// needs before `ready` turns true. It travels with the data so the screen
	// can say "a figure appears after 20 answers" with the configured number
	// rather than a copy of the default that drifts.
	Window int `json:"window"`
}

// MasteryFeature is one heatmap row plus the grammar topics that teach the
// feature, so a cell can link to its lesson. Topics is [] when the pack has no
// curriculum or no topic claims the feature.
type MasteryFeature struct {
	srs.FeatureMastery
	Topics []string `json:"topics"`
}

func (s *Server) handleMastery(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	got, err := s.mastery(u.ID)
	if err != nil {
		return err
	}
	byFeature := map[string][]string{}
	if s.dag != nil {
		for _, t := range s.dag.Topics() {
			for _, id := range t.FeatureIDs {
				byFeature[id] = append(byFeature[id], t.Code)
			}
		}
	}
	rows := make([]MasteryFeature, 0, len(got))
	for _, m := range got {
		topics := byFeature[m.FeatureID]
		if topics == nil {
			topics = []string{}
		}
		rows = append(rows, MasteryFeature{FeatureMastery: m, Topics: topics})
	}
	return c.JSON(MasteryResponse{Features: rows, Window: s.Config().Drill.GateWindow})
}

// clozeable reports whether a note has a span a cloze can blank.
//
// The single test both note creation and course enrolment apply, so the two
// paths cannot drift into disagreeing about what a cloze needs. It is the same
// validity rule srs.splitTarget enforces on the way out.
func clozeable(n srs.Note) bool {
	return n.TargetStart >= 0 && n.TargetEnd > n.TargetStart
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
