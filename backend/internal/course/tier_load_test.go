package course

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"tanulom/internal/lang"
)

// loadFrom builds a pack file whose course_unit has exactly the given extra
// columns, opens it the way the app does, and loads the syllabus.
func loadFrom(t *testing.T, extraCols string, extraVals string) *Syllabus {
	t.Helper()
	path := filepath.Join(t.TempDir(), "p.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT)`,
		fmt.Sprintf(`INSERT INTO pack_meta VALUES ('version','t'),('schema_version','%d')`, lang.SchemaVersion),
		`CREATE TABLE course_unit (id INTEGER PRIMARY KEY, code TEXT, month INTEGER, ord INTEGER,
			title_ru TEXT, title_en TEXT, goal_ru TEXT, goal_en TEXT, situation_ru TEXT,
			situation_en TEXT, topic_codes TEXT` + extraCols + `)`,
		`CREATE TABLE course_word (unit_code TEXT, lemma TEXT, upos TEXT, gloss_ru TEXT,
			gloss_en TEXT, note TEXT, ord INTEGER)`,
		`INSERT INTO course_unit (code, month, ord, title_ru, title_en, goal_ru, goal_en,
			situation_ru, situation_en, topic_codes` + colNames(extraCols) + `)
			VALUES ('u1', 1, 101, 'a','a','a','a','a','a','' ` + extraVals + `)`,
		`INSERT INTO course_word VALUES ('u1','szia','INTJ','привет','hi','',0)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	db.Close()
	pack, err := lang.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pack.Close()
	s, err := Load(pack)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func colNames(extraCols string) string {
	switch extraCols {
	case "":
		return ""
	case `, track TEXT`:
		return ", track"
	default:
		return ", track, tier"
	}
}

// A pack built before `tier` (or before `track`) existed still loads: every
// unit is core and basic, so the course degrades to what it was.
func TestLoadFromAnOlderPackDefaultsToCoreAndBasic(t *testing.T) {
	cases := map[string]struct{ cols, vals string }{
		"neither column": {"", ""},
		"track only":     {`, track TEXT`, `, 'core'`},
		"empty strings":  {`, track TEXT, tier TEXT`, `, '', ''`},
	}
	for name, c := range cases {
		s := loadFrom(t, c.cols, c.vals)
		if len(s.Units) != 1 {
			t.Fatalf("%s: %d units", name, len(s.Units))
		}
		u := s.Units[0]
		if u.Track != TrackCore || u.Tier != TierBasic || len(u.Words) != 1 {
			t.Errorf("%s: track=%q tier=%q words=%d", name, u.Track, u.Tier, len(u.Words))
		}
	}
}

func TestLoadReadsTheTier(t *testing.T) {
	s := loadFrom(t, `, track TEXT, tier TEXT`, `, 'core', 'advanced'`)
	if u := s.Units[0]; u.Tier != TierAdvanced || u.Track != TrackCore {
		t.Errorf("track=%q tier=%q", u.Track, u.Tier)
	}
}
