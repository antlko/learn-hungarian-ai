package api

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/course"
	"tanulom/internal/morph"
)

// Unit phrases: a few short, attested pack sentences that show the unit's words
// in use — the greeting you would actually say, with its translation.
//
// Display-only. Nothing here creates a note or touches a schedule, and every
// sentence is one a person wrote (source <> 'authored', a human RU translation
// from the pack). No Hungarian is generated (ADR-072).

// PhraseDTO is one pack sentence for a unit, with whatever translations exist.
type PhraseDTO struct {
	ID int64  `json:"id"`
	HU string `json:"hu"`
	RU string `json:"ru"`
	EN string `json:"en,omitempty"`
}

const (
	maxUnitPhrases = 4
	// maxPhraseTokens keeps a phrase something to say, not something to parse.
	maxPhraseTokens = 6
	// Candidates read per unit word. A unit is ~40 words, so this bounds the
	// whole request at a few hundred sentences however common a word is.
	phrasePerLemma = 8
)

type phraseCand struct {
	hu   string
	len  int
	hits map[string]bool // unit lemma -> found by corpus lemma id (true) or surface (false)
}

func (s *Server) handleUnitPhrases(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	if err := s.requireCourse(); err != nil {
		return err
	}
	unit, ok := s.syllabus.Unit(c.Params("code"))
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such unit")
	}
	out := []PhraseDTO{}
	if s.pack != nil && s.analyzer != nil {
		var err error
		if out, err = s.unitPhrases(unit); err != nil {
			return err
		}
	}
	return c.JSON(fiber.Map{"phrases": out})
}

// unitPhrases picks up to maxUnitPhrases sentences ranked by how many distinct
// unit words they contain, then by length (shorter first).
func (s *Server) unitPhrases(unit course.Unit) ([]PhraseDTO, error) {
	cands := map[int64]*phraseCand{}
	idToLemma := map[int64]string{}
	add := func(sid int64, hu string, n int, lemma string, byID bool) {
		pc := cands[sid]
		if pc == nil {
			pc = &phraseCand{hu: hu, len: n, hits: map[string]bool{}}
			cands[sid] = pc
		}
		pc.hits[lemma] = pc.hits[lemma] || byID
	}

	for _, w := range unit.Words {
		ids, err := s.lemmaIDsFor(w.Lemma)
		if err != nil {
			return nil, err
		}
		found := 0
		for _, id := range ids {
			idToLemma[id] = w.Lemma
			rows, err := s.pack.DB().Query(phraseSQL(`t.lemma_id = ?`), id, phrasePerLemma)
			if err != nil {
				return nil, err
			}
			n, err := scanPhraseRows(rows, func(sid int64, hu string, ln int) { add(sid, hu, ln, w.Lemma, true) })
			if err != nil {
				return nil, err
			}
			found += n
		}
		// The tagger files some dictionary words under another lemma id
		// (handlers_course.go): search the attested spellings too.
		if found == 0 && len(ids) > 0 {
			variants, err := s.attestedVariants(ids[0])
			if err != nil {
				return nil, err
			}
			if len(variants) == 0 {
				continue
			}
			marks, args := morph.Placeholders(variants)
			args = append(args, phrasePerLemma)
			rows, err := s.pack.DB().Query(phraseSQL(`t.form IN (`+marks+`)`), args...)
			if err != nil {
				return nil, err
			}
			if _, err := scanPhraseRows(rows, func(sid int64, hu string, ln int) { add(sid, hu, ln, w.Lemma, false) }); err != nil {
				return nil, err
			}
		}
	}
	if len(cands) == 0 {
		return []PhraseDTO{}, nil
	}

	// A sentence found for one word may contain others the per-word limit never
	// reached; count those by lemma id too, so ranking sees the real overlap.
	sids := make([]int64, 0, len(cands))
	for sid := range cands {
		sids = append(sids, sid)
	}
	if err := s.addOverlap(cands, sids, idToLemma); err != nil {
		return nil, err
	}

	sort.Slice(sids, func(i, j int) bool {
		a, b := cands[sids[i]], cands[sids[j]]
		if len(a.hits) != len(b.hits) {
			return len(a.hits) > len(b.hits)
		}
		if a.len != b.len {
			return a.len < b.len
		}
		return sids[i] < sids[j]
	})

	out := []PhraseDTO{}
	for _, sid := range sids {
		if len(out) == maxUnitPhrases {
			break
		}
		pc := cands[sid]
		if !s.phraseConfirmed(pc, unit) {
			continue
		}
		p := PhraseDTO{ID: sid, HU: pc.hu}
		ex := ExampleDTO{}
		if err := s.fillTranslations(sid, &ex); err != nil {
			return nil, err
		}
		p.RU, p.EN = ex.RU, ex.EN
		out = append(out, p)
	}
	return out, nil
}

// phraseConfirmed: a hit made through the corpus's own lemma id stands; one made
// only by spelling must be confirmed by the analyser, like an example sentence.
func (s *Server) phraseConfirmed(pc *phraseCand, unit course.Unit) bool {
	for _, byID := range pc.hits {
		if byID {
			return true
		}
	}
	for lemma := range pc.hits {
		ids, err := s.lemmaIDsFor(lemma)
		if err != nil {
			continue
		}
		for _, id := range ids {
			if _, ok, err := s.formOfLemmaIn(pc.hu, id); err == nil && ok {
				return true
			}
		}
		// The tagger may file the token elsewhere; the headword spelled out is
		// what the course lookup accepts too.
		for _, tok := range morph.Tokenize(pc.hu) {
			if tok.IsWord && strings.EqualFold(tok.Text, lemma) {
				return true
			}
		}
	}
	return false
}

// phraseSQL: candidate sentences for one word, human RU translation required,
// shortest first. The `where` clause hits idx_stoken_lemma / idx_stoken_form.
func phraseSQL(where string) string {
	return `
		SELECT s.id, s.hu, s.len_tokens FROM sentence s
		  JOIN sentence_token t ON t.sentence_id = s.id
		 WHERE ` + where + ` AND s.len_tokens BETWEEN 1 AND ` + strconv.Itoa(maxPhraseTokens) + `
		   AND s.source <> 'authored'
		   AND EXISTS (SELECT 1 FROM sentence_trans r WHERE r.sentence_id = s.id AND r.lang = 'ru')
		 ORDER BY s.len_tokens LIMIT ?`
}

// idPlaceholders renders "?,?" and the ids as query args.
func idPlaceholders(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, v := range ids {
		args[i] = v
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

func scanPhraseRows(rows *sql.Rows, fn func(sid int64, hu string, n int)) (int, error) {
	defer rows.Close()
	n := 0
	for rows.Next() {
		var sid int64
		var hu string
		var ln int
		if err := rows.Scan(&sid, &hu, &ln); err != nil {
			return n, err
		}
		fn(sid, hu, ln)
		n++
	}
	return n, rows.Err()
}

func (s *Server) lemmaIDsFor(lemma string) ([]int64, error) {
	rows, err := s.pack.DB().Query(`SELECT id FROM lemma WHERE lemma = ?`, lemma)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// attestedVariants: the case variants of the lemma's attested spellings.
func (s *Server) attestedVariants(lemmaID int64) ([]string, error) {
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
	return variants, nil
}

func (s *Server) addOverlap(cands map[int64]*phraseCand, sids []int64, idToLemma map[int64]string) error {
	if len(idToLemma) == 0 {
		return nil
	}
	sm, sa := idPlaceholders(sids)
	im, ia := idPlaceholders(keysOf(idToLemma))
	rows, err := s.pack.DB().Query(
		`SELECT sentence_id, lemma_id FROM sentence_token
		  WHERE sentence_id IN (`+sm+`) AND lemma_id IN (`+im+`)`, append(sa, ia...)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sid, lid int64
		if err := rows.Scan(&sid, &lid); err != nil {
			return err
		}
		if pc := cands[sid]; pc != nil {
			pc.hits[idToLemma[lid]] = true
		}
	}
	return rows.Err()
}

func keysOf(m map[int64]string) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
