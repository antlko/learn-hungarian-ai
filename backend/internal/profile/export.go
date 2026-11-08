package profile

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Export writes one learner's whole profile into w as a zip archive.
//
// Streamed rather than buffered: a learner with a year of reading has a
// text_token table in the hundreds of thousands of rows, and holding the whole
// document in memory to measure it first buys nothing.
func Export(db *sql.DB, dataDir string, userID int64, appVersion string, w io.Writer) error {
	env := Envelope{
		FormatVersion:  FormatVersion,
		AppVersion:     appVersion,
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		GOARCH:         runtime.GOARCH,
		SecretsOmitted: true,
		Tables:         make(map[string]Rows, len(manifest)),
	}

	acct, err := account(db, userID)
	if err != nil {
		return err
	}
	env.User = acct

	for _, t := range manifest {
		rows, err := dumpTable(db, t, userID)
		if err != nil {
			return fmt.Errorf("export %s: %w", t.Name, err)
		}
		env.Tables[t.Name] = rows
	}

	// Media is listed in the envelope AND carried in the archive, so an import
	// can report a file the archive lost rather than discovering it missing
	// the next time the learner presses play.
	media, err := mediaFiles(db, dataDir, userID)
	if err != nil {
		return err
	}
	env.Media = media

	zw := zip.NewWriter(w)
	f, err := zw.Create(FileName)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	if err := enc.Encode(env); err != nil {
		return err
	}
	for _, m := range media {
		if err := addFile(zw, filepath.Join(dataDir, "media", m.Path), MediaDir+m.Path); err != nil {
			return fmt.Errorf("export media %s: %w", m.Path, err)
		}
	}
	return zw.Close()
}

// account reads the row the envelope carries instead of a `user` table dump.
func account(db *sql.DB, userID int64) (AccountDTO, error) {
	var a AccountDTO
	err := db.QueryRow(
		`SELECT email, password_hash, role, created_at FROM user WHERE id = ?`, userID,
	).Scan(&a.Email, &a.PasswordHash, &a.Role, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return a, fmt.Errorf("no such user")
	}
	return a, err
}

// columnsOf asks SQLite what a table actually has, in declared order.
//
// This is the whole reason a migration cannot silently drop a column out of
// the export: there is no list to forget to update.
func columnsOf(db *sql.DB, name string) ([]string, error) {
	// PRAGMA takes no placeholders; every name here is a compile-time constant
	// from the manifest, never anything a request supplies.
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("table %q has no columns — is it in the schema?", name)
	}
	return out, nil
}

// dumpTable reads one table's rows for one learner.
func dumpTable(db *sql.DB, t table, userID int64) (Rows, error) {
	cols, err := columnsOf(db, t.Name)
	if err != nil {
		return Rows{}, err
	}
	out := Rows{Columns: cols, Rows: [][]any{}}

	// Ordered by rowid so the file is stable between two exports of an
	// unchanged profile — which is what makes a round-trip test meaningful and
	// a diff of two exports readable.
	order := " ORDER BY rowid"
	if t.Name == "translation_cache" || t.Name == "user_gloss" {
		// WITHOUT ROWID tables have no rowid to order by.
		order = ""
	}
	q := `SELECT "` + strings.Join(cols, `", "`) + `" FROM "` + t.Name + `" WHERE ` +
		t.Scope + t.Extra + order

	rows, err := db.Query(q, userID)
	if err != nil {
		return Rows{}, err
	}
	defer rows.Close()

	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return Rows{}, err
		}
		// []byte is how the driver hands back TEXT and BLOB; JSON would encode
		// it as base64 and the import would read back a different string.
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		out.Rows = append(out.Rows, vals)
	}
	return out, rows.Err()
}

// mediaFiles lists the audio this learner's texts point at.
//
// Driven off `text.audio_path` rather than by walking <dataDir>/media, because
// that directory also holds the HVPT clips, which belong to the language pack
// and not to any learner.
func mediaFiles(db *sql.DB, dataDir string, userID int64) ([]MediaFile, error) {
	rows, err := db.Query(
		`SELECT audio_path FROM text WHERE user_id = ? AND audio_path <> ''`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaFile
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			return nil, err
		}
		if !safeRelPath(rel) {
			continue
		}
		st, err := os.Stat(filepath.Join(dataDir, "media", rel))
		if err != nil {
			// A row pointing at a file that is gone is a fact about this
			// install, not a reason to refuse the whole export.
			continue
		}
		out = append(out, MediaFile{Path: rel, Bytes: st.Size()})
	}
	return out, rows.Err()
}

// safeRelPath refuses anything that could escape the media directory. It
// guards the export (a stored path) and the import (a path from a file a
// learner was handed by someone else), so it is deliberately strict.
func safeRelPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return false
	}
	if filepath.IsAbs(p) {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func addFile(zw *zip.Writer, src, name string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// Store, not deflate: these are mp3/m4a, already compressed.
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}
