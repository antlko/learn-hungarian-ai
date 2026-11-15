package api

import (
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/mining"
)

// Ready-made reading texts (ADR-056, ADR-066, ADR-076).
//
// The texts live in a catalogue embedded in the binary (internal/readsets), not
// in the language pack: shipping a new binary updates them for the full and the
// starter edition alike, and the pack is never written. Per course unit there is
// a short, a medium and a long text (and, later, talks and mini-dialogues), all
// written by an AI for the app and not checked by a native speaker, and every
// surface says so. Importing a set makes an ordinary text in the learner's
// library through the same ingestion path as a paste. Nothing here creates a
// note or a card: mining words out of the text stays the learner's explicit act.
//
// Which sets a learner has imported is recorded on the TEXT itself, as
// `source_url = "set:<code>"`; no table is added.

const setSourcePrefix = "set:"

// LibrarySetDTO is one row of GET /api/library/sets.
type LibrarySetDTO struct {
	Code string `json:"code"`
	// Unit is the course unit (or theme) code the text belongs to; texts of one
	// unit share it.
	Unit  string `json:"unit"`
	Level string `json:"level"`
	// Track is "core" (one of the course units) or "theme".
	Track string `json:"track"`
	// Register is "standard" or "colloquial" (spoken, informal language).
	Register string `json:"register"`
	// Title and Theme are in the request's locale; the Ru/En pair is always sent
	// so the client can fall back without a second request.
	Title   string `json:"title"`
	TitleRU string `json:"titleRu"`
	TitleEN string `json:"titleEn"`
	Theme   string `json:"theme"`
	// Scene is a mini-dialogue's one-line situation in the request's locale;
	// empty for other texts. It is shown in the UI only, never imported.
	Scene string `json:"scene,omitempty"`
	// SceneRU/SceneEN are always sent so the client picks the scene exactly like the title.
	SceneRU string `json:"sceneRu,omitempty"`
	SceneEN string `json:"sceneEn,omitempty"`
	// Sentences is how many lines the set holds.
	Sentences int `json:"sentences"`
	// Tier is "short" | "medium" | "long" | "talk" | "mini"; Genre is the text's
	// genre id ("sms", "note", "dialogue", "story" ...); Words is its word
	// count; Checked says who vetted it ("ai" until a human does).
	Tier    string `json:"tier,omitempty"`
	Genre   string `json:"genre,omitempty"`
	Words   int    `json:"words,omitempty"`
	Checked string `json:"checked,omitempty"`
	// Authored: written by an AI for the app, not checked by a native speaker.
	// Always shown to the learner as such; true for every catalogue text.
	Authored bool `json:"authored"`
	Imported bool `json:"imported"`
	// TextID is the learner's own copy, present only when Imported.
	TextID *int64 `json:"textId,omitempty"`
}

// LibrarySetLineDTO is one line of a set, for the preview. RU and EN are the
// catalogue's own translations and may be empty: the client picks one by the
// learner's gloss language and labels a fallback.
type LibrarySetLineDTO struct {
	HU string `json:"hu"`
	RU string `json:"ru,omitempty"`
	EN string `json:"en,omitempty"`
	// Who is the speaker of a dialogue turn, when the catalogue names one.
	Who string `json:"who,omitempty"`
}

// LibrarySetDetailDTO is GET /api/library/sets/:code: the set as it would be
// imported, without importing it.
type LibrarySetDetailDTO struct {
	Code      string              `json:"code"`
	Unit      string              `json:"unit"`
	Level     string              `json:"level"`
	Track     string              `json:"track"`
	Register  string              `json:"register"`
	Title     string              `json:"title"`
	TitleRU   string              `json:"titleRu"`
	TitleEN   string              `json:"titleEn"`
	Theme     string              `json:"theme"`
	Scene     string              `json:"scene,omitempty"`
	SceneRU   string              `json:"sceneRu,omitempty"`
	SceneEN   string              `json:"sceneEn,omitempty"`
	Authored  bool                `json:"authored"`
	Tier      string              `json:"tier,omitempty"`
	Genre     string              `json:"genre,omitempty"`
	Words     int                 `json:"words,omitempty"`
	Checked   string              `json:"checked,omitempty"`
	Sentences []LibrarySetLineDTO `json:"sentences"`
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
	imported, err := s.importedSets(u.ID)
	if err != nil {
		return err
	}
	en := s.answerLang(c.Query("lang")) == "en"
	out := make([]LibrarySetDTO, 0, len(s.sets.Sets()))
	for _, set := range s.sets.Sets() {
		d := LibrarySetDTO{
			Code: set.Code, Unit: set.Unit, Level: set.Level, Track: set.Track,
			Register: set.Register, Title: set.TitleRU, TitleRU: set.TitleRU,
			TitleEN: set.TitleEN, Theme: set.ThemeRU, Sentences: len(set.Lines),
			Tier: set.Tier, Genre: set.Genre, Words: set.Words, Checked: set.Checked,
			Scene: set.SceneRU, SceneRU: set.SceneRU, SceneEN: set.SceneEN, Authored: true,
		}
		if en {
			d.Title, d.Theme, d.Scene = set.TitleEN, set.ThemeEN, set.SceneEN
		}
		if id, ok := imported[set.Code]; ok {
			d.Imported = true
			d.TextID = &id
		}
		out = append(out, d)
	}
	return c.JSON(out)
}

// handleLibrarySet previews one set: every line, in order, with the catalogue's
// translations, so a learner can read it before adding it.
func (s *Server) handleLibrarySet(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	set, ok := s.sets.Get(c.Params("code"))
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such reading set")
	}
	en := s.answerLang(c.Query("lang")) == "en"
	d := LibrarySetDetailDTO{
		Code: set.Code, Unit: set.Unit, Level: set.Level, Track: set.Track,
		Register: set.Register, Title: set.TitleRU, TitleRU: set.TitleRU,
		TitleEN: set.TitleEN, Theme: set.ThemeRU, Authored: true, Tier: set.Tier,
		Genre: set.Genre, Words: set.Words, Checked: set.Checked,
		Scene: set.SceneRU, SceneRU: set.SceneRU, SceneEN: set.SceneEN, Sentences: make([]LibrarySetLineDTO, 0, len(set.Lines)),
	}
	if en {
		d.Title, d.Theme, d.Scene = set.TitleEN, set.ThemeEN, set.SceneEN
	}
	for _, l := range set.Lines {
		d.Sentences = append(d.Sentences, LibrarySetLineDTO{
			HU: strings.TrimSpace(l.HU), RU: l.RU, EN: l.EN, Who: l.Who})
	}
	// The catalogue changes only with the binary: its hash is the validator.
	tag := `"` + s.sets.Hash() + `-` + strconv.FormatBool(en) + `"`
	c.Set("ETag", tag)
	c.Set("Cache-Control", "private, no-cache")
	if c.Get("If-None-Match") == tag {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return c.JSON(d)
}

func (s *Server) handleImportLibrarySet(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requirePack(); err != nil {
		return err
	}
	code := c.Params("code")
	set, ok := s.sets.Get(code)
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such reading set")
	}
	title, body := s.setTitle(set.TitleRU, set.TitleEN), set.Body()

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

	in, err := mining.FromPaste(title, body)
	if err != nil {
		return badRequest("%s", err.Error())
	}
	// The text model's own mark for AI-written text, so the shelf can say so.
	in.Kind = mining.KindGenerated
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
