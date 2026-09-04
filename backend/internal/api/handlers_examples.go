package api

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/morph"
)

// Example sentences for one lemma, with their translations.
//
// This is what makes a word clickable anywhere in the app: the inspector panel
// asks "what does this word mean, what shape is it in, and where would I meet
// it", and the third question needs real sentences. Every one of them is a
// sentence somebody wrote — nothing here is generated.
//
// The candidate search is the same two-pass one the course uses, and for the
// same reason: the corpus files tokens under the tagger's lemma id while the
// dictionary uses its own headword, so `nagyon` looks absent from 820 sentences
// if you only ever join on lemma id. See handlers_course.go.

// ExampleDTO is one attested sentence, with whatever translations exist.
type ExampleDTO struct {
	HU string `json:"hu"`
	RU string `json:"ru,omitempty"`
	EN string `json:"en,omitempty"`
	// Form is the actual surface form of the lemma in this sentence, so the UI
	// can point at it: seeing `nagyon` in `nagy`'s examples is the lesson.
	Form string `json:"form,omitempty"`
}

const (
	defaultExamples = 3
	maxExamples     = 10
	// Candidates examined before giving up. Each one costs a tokenise plus a
	// lookup per word, so this bounds the work rather than the result.
	exampleCandidates = 40
)

func (s *Server) handleLemmaExamples(c fiber.Ctx) error {
	if err := s.requirePack(); err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("lemma id must be a number")
	}
	limit := defaultExamples
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxExamples {
			return badRequest("limit must be between 1 and %d", maxExamples)
		}
		limit = n
	}

	out, err := s.examplesFor(id, limit)
	if err != nil {
		return err
	}
	// Always an array: `null` here would make the panel render "no examples"
	// differently depending on which branch produced it.
	if out == nil {
		out = []ExampleDTO{}
	}
	return c.JSON(fiber.Map{"examples": out})
}

func (s *Server) examplesFor(lemmaID int64, limit int) ([]ExampleDTO, error) {
	seen := map[int64]bool{}
	var out []ExampleDTO

	for _, find := range []func(int64) (*sql.Rows, error){
		s.exampleRowsByLemmaID,
		s.exampleRowsBySurfaceForm,
	} {
		rows, err := find(lemmaID)
		if err != nil {
			return nil, err
		}
		if rows == nil {
			continue
		}
		got, err := s.collectExamples(rows, lemmaID, limit-len(out), seen)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// collectExamples verifies each candidate and pulls its translations.
//
// Shortest first: an example is there to show the word in use, and a
// twenty-word sentence shows it surrounded by four other things you do not know
// yet.
func (s *Server) collectExamples(
	rows *sql.Rows, lemmaID int64, want int, seen map[int64]bool,
) ([]ExampleDTO, error) {
	defer rows.Close()
	var out []ExampleDTO

	for rows.Next() && len(out) < want {
		var id int64
		var hu string
		if err := rows.Scan(&id, &hu); err != nil {
			return nil, err
		}
		if seen[id] {
			continue
		}
		seen[id] = true

		form, ok, err := s.formOfLemmaIn(hu, lemmaID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		ex := ExampleDTO{HU: hu, Form: form}
		if err := s.fillTranslations(id, &ex); err != nil {
			return nil, err
		}
		out = append(out, ex)
	}
	return out, rows.Err()
}

// formOfLemmaIn reports which token of the sentence reads as this lemma.
// A candidate whose tokens the analyser does not agree about is not an example
// of this word, whatever the corpus index said.
func (s *Server) formOfLemmaIn(hu string, lemmaID int64) (string, bool, error) {
	for _, tok := range morph.Tokenize(hu) {
		if !tok.IsWord {
			continue
		}
		readings, err := s.analyzer.Lookup(tok.Text)
		if err != nil {
			return "", false, err
		}
		for _, r := range readings {
			if r.LemmaID == lemmaID {
				return tok.Text, true, nil
			}
		}
	}
	return "", false, nil
}

func (s *Server) fillTranslations(sentenceID int64, ex *ExampleDTO) error {
	rows, err := s.pack.DB().Query(
		`SELECT lang, text FROM sentence_trans WHERE sentence_id = ? AND lang IN ('ru','en')`,
		sentenceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var lang, text string
		if err := rows.Scan(&lang, &text); err != nil {
			return err
		}
		if lang == "ru" {
			ex.RU = text
		} else {
			ex.EN = text
		}
	}
	return rows.Err()
}

func (s *Server) exampleRowsByLemmaID(lemmaID int64) (*sql.Rows, error) {
	return s.pack.DB().Query(`
		SELECT s.id, s.hu FROM sentence s
		  JOIN sentence_token t ON t.sentence_id = s.id
		 WHERE t.lemma_id = ? AND s.len_tokens BETWEEN 3 AND 14
		 ORDER BY s.len_tokens LIMIT ?`, lemmaID, exampleCandidates)
}

func (s *Server) exampleRowsBySurfaceForm(lemmaID int64) (*sql.Rows, error) {
	forms, err := s.analyzer.FormsOf(lemmaID)
	if err != nil {
		return nil, err
	}
	var variants []string
	seen := map[string]bool{}
	for _, f := range forms {
		if !f.Attested() {
			continue
		}
		for _, v := range morph.CaseVariants(f.Form) {
			if !seen[v] {
				seen[v] = true
				variants = append(variants, v)
			}
		}
		if len(variants) >= maxFormsSearched {
			break
		}
	}
	if len(variants) == 0 {
		return nil, nil
	}
	marks, args := morph.Placeholders(variants)
	args = append(args, exampleCandidates)
	return s.pack.DB().Query(`
		SELECT s.id, s.hu FROM sentence s
		  JOIN sentence_token t ON t.sentence_id = s.id
		 WHERE t.form IN (`+marks+`) AND s.len_tokens BETWEEN 3 AND 14
		 ORDER BY s.len_tokens LIMIT ?`, args...)
}
