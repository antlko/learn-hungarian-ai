package api

import (
	"database/sql"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/mining"
)

// Ready-made reading sets: «Живые примеры» (ADR-056).
//
// A set is a handful of sentences that ALREADY EXIST in the language pack — a
// person wrote each one — chosen at build time for one course unit and one
// CEFR band (langpack/pipeline/reading_sets.py). Importing one makes an
// ordinary text in the learner's library, through the same ingestion path as a
// paste. Nothing here writes Hungarian, and nothing here creates a note or a
// card: mining words out of the text stays the learner's explicit act in the
// reader.
//
// The pack is read-only. Which sets a learner has imported is recorded on the
// TEXT itself, as `source_url = "set:<code>"`; no table is added.

const setSourcePrefix = "set:"

// LibrarySetDTO is one row of GET /api/library/sets.
type LibrarySetDTO struct {
	Code  string `json:"code"`
	Level string `json:"level"`
	// Track is "core" (one of the 24 course units) or "theme".
	Track string `json:"track"`
	// Title and Theme are in the request's locale; the Ru/En pair is always sent
	// so the client can fall back without a second request.
	Title   string `json:"title"`
	TitleRU string `json:"titleRu"`
	TitleEN string `json:"titleEn"`
	Theme   string `json:"theme"`
	// Sentences is how many sentences the set holds.
	Sentences int  `json:"sentences"`
	Imported  bool `json:"imported"`
	// TextID is the learner's own copy, present only when Imported.
	TextID *int64 `json:"textId,omitempty"`
}

// ImportSetResponse is the body of POST /api/library/sets/:code/import.
type ImportSetResponse struct {
	TextID  int64 `json:"textId"`
	Created bool  `json:"created"`
}

// importLocks holds one mutex per (user, set) so a double click cannot create
// the text twice, while unrelated learners' imports (which include a network
// hint call) never wait on each other. Entries are tiny and bounded by
// users x sets, so they are never evicted.
var importLocks sync.Map // "uid:code" -> *sync.Mutex

func importLock(userID int64, code string) *sync.Mutex {
	m, _ := importLocks.LoadOrStore(strconv.FormatInt(userID, 10)+":"+code, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// hasReadingSets reports whether the pack carries the two tables. An older
// pack does not, and that is not an error: the list is simply empty.
func (s *Server) hasReadingSets() bool {
	if s.pack == nil {
		return false
	}
	var n int
	err := s.pack.DB().QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'
		   AND name IN ('reading_set', 'reading_set_sentence')`).Scan(&n)
	return err == nil && n == 2
}

// importedSets maps set code -> the learner's text id.
func (s *Server) importedSets(userID int64) (map[string]int64, error) {
	rows, err := s.db.Query(
		`SELECT source_url, MIN(id) FROM text
		  WHERE user_id = ? AND source_url LIKE 'set:%' GROUP BY source_url`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var src string
		var id int64
		if err := rows.Scan(&src, &id); err != nil {
			return nil, err
		}
		out[strings.TrimPrefix(src, setSourcePrefix)] = id
	}
	return out, rows.Err()
}

func (s *Server) handleLibrarySets(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	out := []LibrarySetDTO{}
	if !s.hasReadingSets() {
		return c.JSON(out)
	}
	imported, err := s.importedSets(u.ID)
	if err != nil {
		return err
	}
	lang := s.answerLang(c.Query("lang"))

	// Core units first, by their order, then the themes.
	rows, err := s.pack.DB().Query(`
		SELECT r.code, r.level, r.track, r.title_ru, r.title_en, r.theme_ru, r.theme_en,
		       (SELECT COUNT(*) FROM reading_set_sentence x WHERE x.set_code = r.code)
		  FROM reading_set r
		 ORDER BY (r.track <> 'core'), r.ord, r.code`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d LibrarySetDTO
		var themeRU, themeEN string
		if err := rows.Scan(&d.Code, &d.Level, &d.Track, &d.TitleRU, &d.TitleEN,
			&themeRU, &themeEN, &d.Sentences); err != nil {
			return err
		}
		d.Title, d.Theme = d.TitleRU, themeRU
		if lang == "en" {
			d.Title, d.Theme = d.TitleEN, themeEN
		}
		if id, ok := imported[d.Code]; ok {
			d.Imported = true
			d.TextID = &id
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return c.JSON(out)
}

func (s *Server) handleImportLibrarySet(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requirePack(); err != nil {
		return err
	}
	if !s.hasReadingSets() {
		// The pack is an older one: it has no sets at all, and says so — the
		// list is empty, so a client following the UI never reaches here.
		return fiber.NewError(fiber.StatusServiceUnavailable, "errors.packMissing")
	}
	code := c.Params("code")

	var titleRU, titleEN string
	err = s.pack.DB().QueryRow(
		`SELECT title_ru, title_en FROM reading_set WHERE code = ?`, code).Scan(&titleRU, &titleEN)
	if err == sql.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "no such reading set")
	}
	if err != nil {
		return err
	}

	mu := importLock(u.ID, code)
	mu.Lock()
	defer mu.Unlock()

	// Idempotent per learner: a second call hands back the first copy.
	existing, err := s.importedSets(u.ID)
	if err != nil {
		return err
	}
	if id, ok := existing[code]; ok {
		return c.JSON(ImportSetResponse{TextID: id, Created: false})
	}

	rows, err := s.pack.DB().Query(`
		SELECT s.hu FROM reading_set_sentence r JOIN sentence s ON s.id = r.sentence_id
		 WHERE r.set_code = ? ORDER BY r.ord`, code)
	if err != nil {
		return err
	}
	var sentences []string
	for rows.Next() {
		var hu string
		if err := rows.Scan(&hu); err != nil {
			rows.Close()
			return err
		}
		sentences = append(sentences, strings.TrimSpace(hu))
	}
	rows.Close()
	if len(sentences) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "reading set is empty")
	}

	// One sentence per paragraph. A blank line always ends a sentence in the
	// splitter, so a sentence with no closing punctuation cannot swallow the
	// next one. (The text model has no per-sentence translation slot, so the
	// pack's Russian is not stored with the text.)
	in, err := mining.FromPaste(s.setTitle(titleRU, titleEN), strings.Join(sentences, "\n\n"))
	if err != nil {
		return badRequest("%s", err.Error())
	}
	id, _, _, err := s.ingestText(c, u.ID, in, setSourcePrefix+code)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(ImportSetResponse{TextID: id, Created: true})
}

// setTitle names the imported text in the UI language the learner has set.
func (s *Server) setTitle(ru, en string) string {
	if s.Config().Locale.UI == "en" {
		return en
	}
	return ru
}
