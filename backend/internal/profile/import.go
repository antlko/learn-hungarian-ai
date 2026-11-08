package profile

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"tanulom/internal/db"
	"tanulom/internal/srs"
)

// maxJSONBytes caps the document read out of the archive. A year of heavy
// reading is a few tens of megabytes of text_token; 512 MB is far past any
// real profile and stops a crafted zip from being a memory bomb.
const maxJSONBytes = 512 << 20

// Import replaces one learner's profile with the contents of an archive.
//
// REPLACES. Not merges. Merging two FSRS histories means deciding which
// review_log wins for a card that exists on both sides, and there is no answer
// to that which does not quietly invent a learner's past. The confirm dialog in
// the UI names, by count, what is about to be erased.
//
// Everything happens in one transaction. A failure at any point leaves the
// learner exactly where they were — which matters more here than anywhere else
// in the app, because the alternative is a half-restored profile.
func Import(db *sql.DB, dataDir string, userID int64, r io.ReaderAt, size int64,
	desiredRetention float64) (*Report, error) {

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("this file is not a profile archive: %w", err)
	}

	env, err := readEnvelope(zr)
	if err != nil {
		return nil, err
	}
	if env.FormatVersion > FormatVersion {
		return nil, ErrFutureFormat{Found: env.FormatVersion, Supported: FormatVersion}
	}

	rep := &Report{
		RowsByTable:    map[string]int{},
		SecretsOmitted: env.SecretsOmitted,
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // a committed tx makes this a no-op

	// Children first, so foreign keys never see an orphan. The manifest's
	// order is the insert order; deleting is that order reversed.
	for i := len(manifest) - 1; i >= 0; i-- {
		t := manifest[i]
		if _, err := tx.Exec(`DELETE FROM "`+t.Name+`" WHERE `+t.Scope, userID); err != nil {
			return nil, fmt.Errorf("clear %s: %w", t.Name, err)
		}
	}

	// old id -> new id, per table, built as parents are inserted and read as
	// children are. The four tables with an IDColumn are the only entries.
	remap := map[string]map[int64]int64{}
	skipped := map[string]bool{}

	for _, t := range manifest {
		rows, ok := env.Tables[t.Name]
		if !ok {
			// An export from an older format simply has no such table. That
			// is not an error: the learner had no rows in a feature that did
			// not exist yet.
			continue
		}
		n, err := insertTable(tx, t, rows, userID, remap, skipped)
		if err != nil {
			return nil, fmt.Errorf("import %s: %w", t.Name, err)
		}
		rep.RowsByTable[t.Name] = n
	}

	if err := restoreAccount(tx, userID, env.User); err != nil {
		return nil, err
	}

	// The integrity check, inside the transaction: if the log does not produce
	// the state it shipped with, on this architecture, the profile is corrupt
	// and nothing is committed.
	checked, warning, err := verifyReplay(tx, userID, env.GOARCH, desiredRetention)
	if err != nil {
		return nil, err
	}
	rep.ReplayChecked, rep.ReplayWarning, rep.ReplayOK = checked, warning, warning == ""

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Media last and outside the transaction: files are not transactional, and
	// a profile whose audio failed to unpack is still a profile. The report
	// says how many landed.
	rep.MediaFiles, err = restoreMedia(zr, dataDir)
	if err != nil {
		return rep, fmt.Errorf("profile imported, but its audio did not: %w", err)
	}

	for c := range skipped {
		rep.SkippedColumns = append(rep.SkippedColumns, c)
	}
	sort.Strings(rep.SkippedColumns)
	return rep, nil
}

func readEnvelope(zr *zip.Reader) (*Envelope, error) {
	f, err := zr.Open(FileName)
	if err != nil {
		return nil, fmt.Errorf("the archive has no %s — is this a tanulom profile?", FileName)
	}
	defer f.Close()

	var env Envelope
	if err := json.NewDecoder(io.LimitReader(f, maxJSONBytes)).Decode(&env); err != nil {
		return nil, fmt.Errorf("%s is not readable: %w", FileName, err)
	}
	if env.User.Email == "" {
		return nil, fmt.Errorf("%s names no account", FileName)
	}
	return &env, nil
}

// insertTable writes one table's rows, rewriting the owning user id and every
// reference to a remapped key.
//
// Columns are intersected with what this schema actually has. A column the
// file carries and the schema does not is RECORDED in `skipped` and reported,
// never dropped in silence — a learner told "imported" while a column went
// missing is exactly the failure this package was written to avoid.
func insertTable(tx *sql.Tx, t table, rows Rows, userID int64,
	remap map[string]map[int64]int64, skipped map[string]bool) (int, error) {

	if len(rows.Rows) == 0 {
		return 0, nil
	}
	have, err := txColumns(tx, t.Name)
	if err != nil {
		return 0, err
	}
	// The surrogate key is always re-assigned, referenced or not.
	auto, err := rowidAlias(tx, t.Name)
	if err != nil {
		return 0, err
	}
	if t.IDColumn != "" && t.IDColumn != auto {
		return 0, fmt.Errorf("the manifest calls %q the id, but the schema's is %q",
			t.IDColumn, auto)
	}

	// Which of the file's columns we can write, and where each sits in a row.
	var cols []string
	var srcIdx []int
	for i, c := range rows.Columns {
		switch {
		case !have[c]:
			skipped[t.Name+"."+c] = true
		case c == auto:
			// Re-assigned by SQLite; its old value is captured for the map
			// when something references it.
		default:
			cols = append(cols, c)
			srcIdx = append(srcIdx, i)
		}
	}
	if len(cols) == 0 {
		return 0, fmt.Errorf("no usable columns")
	}

	oldIDAt := -1
	if t.IDColumn != "" {
		oldIDAt = indexOf(rows.Columns, t.IDColumn)
		if oldIDAt < 0 {
			return 0, fmt.Errorf("the file carries no %s", t.IDColumn)
		}
		remap[t.Name] = make(map[int64]int64, len(rows.Rows))
	}

	q := `INSERT INTO "` + t.Name + `" ("` + strings.Join(cols, `", "`) + `") VALUES (` +
		strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",") + `)`
	stmt, err := tx.Prepare(q)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, row := range rows.Rows {
		if len(row) != len(rows.Columns) {
			return 0, fmt.Errorf("a row has %d values for %d columns", len(row), len(rows.Columns))
		}
		args := make([]any, len(cols))
		for i, at := range srcIdx {
			c := cols[i]
			v := row[at]
			switch {
			case c == t.UserColumn:
				// The destination's user id, not the source's.
				v = userID
			case t.Refs[c] != "":
				v, err = rewriteRef(v, t, c, remap)
				if err != nil {
					return 0, err
				}
			}
			args[i] = normalise(v)
		}
		res, err := stmt.Exec(args...)
		if err != nil {
			return 0, err
		}
		if oldIDAt >= 0 {
			old, ok := asInt(row[oldIDAt])
			if !ok {
				return 0, fmt.Errorf("%s is not a number", t.IDColumn)
			}
			newID, err := res.LastInsertId()
			if err != nil {
				return 0, err
			}
			remap[t.Name][old] = newID
		}
	}
	return len(rows.Rows), nil
}

// rewriteRef turns a reference to a source-side id into the destination's.
func rewriteRef(v any, t table, col string, remap map[string]map[int64]int64) (any, error) {
	if v == nil {
		return nil, nil // a nullable ref that was null (listen_log.text_id)
	}
	old, ok := asInt(v)
	if !ok {
		return nil, fmt.Errorf("%s.%s is not a number", t.Name, col)
	}
	// 0 means "none" on the NOT NULL DEFAULT 0 columns, and is not an id.
	if old == 0 && t.ZeroRefs[col] {
		return int64(0), nil
	}
	newID, ok := remap[t.Refs[col]][old]
	if !ok {
		if t.ZeroRefs[col] {
			// A tutor thread pointing at a note that is gone: the thread is
			// still a real conversation, it just loses its pin.
			return int64(0), nil
		}
		return nil, fmt.Errorf("%s.%s points at %s %d, which the file does not carry",
			t.Name, col, t.Refs[col], old)
	}
	return newID, nil
}

// restoreAccount puts the exported account's own fields back on the importing
// row — password included, so the learner logs in with the password they know.
//
// The EMAIL is deliberately not overwritten: it identifies the account on the
// destination install, and rewriting it could collide with another user there.
// Neither is the ROLE: admin is a property of the installation, not a thing a
// profile carries into somebody else's server.
func restoreAccount(tx *sql.Tx, userID int64, a AccountDTO) error {
	_, err := tx.Exec(`UPDATE user SET password_hash = ? WHERE id = ?`, a.PasswordHash, userID)
	return err
}

// verifyReplay is the check no interchange format could make.
//
// Every card's review log is replayed through the scheduler and compared to
// the state that arrived with it. That the two agree bit-for-bit is already a
// property this app relies on for undo; here it proves the profile crossed
// intact.
//
// Bit-for-bit holds on ONE architecture. A profile exported on arm64 and
// imported on amd64 replays to a due date within the fuzz range rather than on
// it, so a mismatch after a cross-arch move is reported as a warning and the
// import stands. Same arch and a mismatch means corruption, and the caller
// rolls back. ARCHITECTURE.md §5.7.
func verifyReplay(tx *sql.Tx, userID int64, sourceArch string, desiredRetention float64) (int, string, error) {
	sched := srs.NewScheduler(desiredRetention)

	rows, err := tx.Query(`
		SELECT c.id, c.stability, c.reps, c.lapses, c.state
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.reps > 0
		 ORDER BY c.id`, userID)
	if err != nil {
		return 0, "", err
	}
	type want struct {
		id                  int64
		stability           float64
		reps, lapses, state int64
	}
	var cards []want
	for rows.Next() {
		var w want
		if err := rows.Scan(&w.id, &w.stability, &w.reps, &w.lapses, &w.state); err != nil {
			rows.Close()
			return 0, "", err
		}
		cards = append(cards, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, "", err
	}

	crossArch := sourceArch != "" && sourceArch != runtime.GOARCH
	checked := 0
	for _, w := range cards {
		steps, err := replaySteps(tx, w.id)
		if err != nil {
			return checked, "", err
		}
		if len(steps) == 0 {
			continue
		}
		got, err := sched.ReplayCard(steps)
		if err != nil {
			return checked, "", fmt.Errorf("card %d: %w", w.id, err)
		}
		checked++

		// Structural fields must match on every architecture: they are
		// integers and counters, nothing floating-point decides them.
		if int64(got.Reps) != w.reps || int64(got.Lapses) != w.lapses || int64(got.State) != w.state {
			return checked, "", fmt.Errorf(
				"card %d does not replay to the state it arrived with "+
					"(reps %d/%d, lapses %d/%d, state %d/%d) — the profile is damaged",
				w.id, got.Reps, w.reps, got.Lapses, w.lapses, got.State, w.state)
		}
		// Stability is floating point. Exact on one architecture; within a
		// hair across two.
		if math.Abs(got.Stability-w.stability) > 1e-9 {
			if !crossArch {
				return checked, "", fmt.Errorf(
					"card %d replays to stability %.6f but arrived with %.6f — the profile is damaged",
					w.id, got.Stability, w.stability)
			}
			return checked, fmt.Sprintf(
				"exported on %s and imported on %s: intervals replay within the fuzz range "+
					"rather than exactly on it. Everything is here and scheduling is unaffected; "+
					"an undo may land a due date a few hours from the original.",
				sourceArch, runtime.GOARCH), nil
		}
	}
	return checked, "", nil
}

func replaySteps(tx *sql.Tx, cardID int64) ([]srs.ReplayStep, error) {
	rows, err := tx.Query(`
		SELECT rating, review_time, correct, duration_ms
		  FROM review_log WHERE card_id = ? ORDER BY review_time, id`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []srs.ReplayStep
	for rows.Next() {
		var rating int
		var reviewTime sql.NullString
		var correct int
		var durationMs int64
		if err := rows.Scan(&rating, &reviewTime, &correct, &durationMs); err != nil {
			return nil, err
		}
		out = append(out, srs.ReplayStep{
			Rating:     srs.Rating(rating),
			ReviewTime: db.ParseTime(reviewTime),
			Correct:    correct == 1,
			DurationMs: durationMs,
		})
	}
	return out, rows.Err()
}

// restoreMedia unpacks the archive's media into <dataDir>/media.
func restoreMedia(zr *zip.Reader, dataDir string) (int, error) {
	n := 0
	for _, f := range zr.File {
		rel, ok := strings.CutPrefix(f.Name, MediaDir)
		if !ok || f.FileInfo().IsDir() {
			continue
		}
		// The archive may have come from anywhere. A name that climbs out of
		// the media directory is refused rather than sanitised.
		if !safeRelPath(rel) {
			return n, fmt.Errorf("the archive contains an unsafe path: %q", f.Name)
		}
		dst := filepath.Join(dataDir, "media", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return n, err
		}
		if err := writeOne(f, dst); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func writeOne(f *zip.File, dst string) error {
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func txColumns(tx *sql.Tx, name string) (map[string]bool, error) {
	rows, err := tx.Query(`SELECT name FROM pragma_table_info(?)`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out[c] = true
	}
	return out, rows.Err()
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

// asInt accepts what JSON and the driver each produce for an integer.
func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		// encoding/json decodes every number as float64. Ids are well inside
		// the range float64 represents exactly.
		return int64(n), n == math.Trunc(n)
	}
	return 0, false
}

// normalise turns a JSON-decoded value into something the driver will bind.
//
// json.Unmarshal into `any` gives float64 for every number, and binding a
// float64 into an INTEGER column stores a float — which SQLite is happy to do
// and which then reads back as 3.0 where the app expects 3. Whole numbers go
// back to int64.
func normalise(v any) any {
	if f, ok := v.(float64); ok && f == math.Trunc(f) && math.Abs(f) < 1<<53 {
		return int64(f)
	}
	return v
}

// rowidAlias names the column that is this table's INTEGER PRIMARY KEY — the
// rowid alias SQLite assigns automatically — or "" when there is none.
//
// Every such column is dropped on insert and re-assigned, whether or not
// anything references it. `review_log.id` taught this lesson: it is referenced
// by nothing, so it carried no IDColumn, so it was inserted verbatim and
// collided the moment the destination already had review logs of its own.
// Discovering it is better than a second hand-maintained flag beside IDColumn,
// for the same reason the columns are discovered at all.
//
// A composite primary key (text_token's (text_id, idx), settings' (user_id,
// key)) is NOT a rowid alias: those values are real data and must be kept.
func rowidAlias(tx *sql.Tx, name string) (string, error) {
	rows, err := tx.Query(`SELECT name, type, pk FROM pragma_table_info(?)`, name)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var candidate string
	pkCount := 0
	for rows.Next() {
		var col, typ string
		var pk int
		if err := rows.Scan(&col, &typ, &pk); err != nil {
			return "", err
		}
		if pk == 0 {
			continue
		}
		pkCount++
		if strings.EqualFold(typ, "INTEGER") {
			candidate = col
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if pkCount != 1 {
		return "", nil
	}
	return candidate, nil
}
