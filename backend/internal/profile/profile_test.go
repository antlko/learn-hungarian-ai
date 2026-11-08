package profile

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"tanulom/internal/db"
	"tanulom/internal/srs"
)

func openDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	return d, dir
}

// seed builds a profile with something in every interesting corner: two notes
// with cards and a real review history, both state spaces, a text with tokens
// and audio, a tutor thread pinned to a note, a secret setting, and a day in
// the ledger.
func seed(t *testing.T, d *db.DB, dir string, email string) int64 {
	t.Helper()
	uid, err := d.CreateUser(email, "argon2-hash-"+email, db.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	e := srs.NewEngine(srs.NewStore(d.DB), 0.90, 20)
	store := e.Store()

	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	noteID, cards, err := store.CreateNote(srs.Note{
		UserID: uid, Kind: "sentence",
		HU: "A házaimban sok könyv van.", TargetForm: "házaimban",
		TargetStart: 2, TargetEnd: 11,
		TargetFeats: []string{"Case=Ine", "Number[psed]=Plur"},
		GlossRU:     "В моих домах много книг.",
		LemmaID:     ptr(int64(4242)),
	}, []srs.Direction{srs.Recognize, srs.Produce, srs.Cloze}, now)
	if err != nil {
		t.Fatal(err)
	}

	// A real history, so the replay check has something to verify. Answered
	// through the engine rather than by hand, so the fixture is a profile the app
	// could actually have produced.
	note, err := store.Note(uid, noteID)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cards {
		card := c
		for step := 0; step < 4+i; step++ {
			out := srs.Outcome{Correct: step != 2, Duration: 1200 * time.Millisecond, Typed: "hazaimban"}
			res, err := e.Answer(uid, card, note, out, "", now)
			if err != nil {
				t.Fatal(err)
			}
			card = res.Card
			now = now.Add(time.Duration(30+step*17) * time.Hour)
		}
	}

	exec(t, d.DB, `INSERT INTO lemma_state (user_id, lemma_id, status, stability, reps)
	               VALUES (?, 4242, 'learning', 3.5, 4)`, uid)
	exec(t, d.DB, `INSERT INTO morph_state (user_id, feature_id, stability, reps, recent_accuracy, recent_total)
	               VALUES (?, 'Mood=Cnd', 2.25, 3, 0.8, 5)`, uid)
	exec(t, d.DB, `INSERT INTO feature_review (user_id, feature_id, rating, correct, review_time, card_id)
	               VALUES (?, 'Case=Ine', 3, 1, '2026-03-01T09:00:00.000Z', ?)`, uid, cards[0].ID)
	exec(t, d.DB, `INSERT INTO daily_log (user_id, day, reviews, active_ms, due_left)
	               VALUES (?, '2026-03-01', 12, 480000, 0)`, uid)
	exec(t, d.DB, `INSERT INTO user_gloss (user_id, form, gloss, source, model)
	               VALUES (?, 'kinyitom', 'я открываю это', 'machine', 'nllb-200')`, uid)

	// Settings: one ordinary, one secret. The secret must not cross.
	exec(t, d.DB, `INSERT INTO settings (user_id, key, value, secret) VALUES (?, 'locale', 'ru', 0)`, uid)
	exec(t, d.DB, `INSERT INTO settings (user_id, key, value, secret) VALUES (?, 'llm.apiKey', 'sk-DO-NOT-EXPORT', 1)`, uid)

	// A text with a token and an audio file on disk.
	var textID int64
	if err := d.QueryRow(`INSERT INTO text (user_id, title, body, word_count, audio_path)
	                      VALUES (?, 'Egy nap', 'A ház nagy.', 3, 'listen/7.mp3') RETURNING id`,
		uid).Scan(&textID); err != nil {
		t.Fatal(err)
	}
	exec(t, d.DB, `INSERT INTO text_token (text_id, idx, sent_idx, form, start_b, end_b, is_word, lemma_id)
	               VALUES (?, 0, 0, 'A', 0, 1, 1, NULL)`, textID)
	exec(t, d.DB, `INSERT INTO note_origin (note_id, text_id) VALUES (?, ?)`, noteID, textID)
	exec(t, d.DB, `INSERT INTO listen_state (user_id, text_id, pass, position_s) VALUES (?, ?, 2, 41.5)`, uid, textID)
	if err := os.MkdirAll(filepath.Join(dir, "media", "listen"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "listen", "7.mp3"), []byte("ID3-fake-audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A tutor thread pinned to that note, plus a message pointing at a card —
	// the two NOT NULL DEFAULT 0 references that must survive remapping.
	var threadID int64
	if err := d.QueryRow(`INSERT INTO tutor_thread (user_id, title, kind, note_id)
	                      VALUES (?, 'házaimban', 'review', ?) RETURNING id`, uid, noteID).Scan(&threadID); err != nil {
		t.Fatal(err)
	}
	exec(t, d.DB, `INSERT INTO tutor_message (user_id, topic_code, role, content, thread_id, card_id)
	               VALUES (?, 'locative', 'user', 'почему -ban?', ?, ?)`, uid, threadID, cards[0].ID)
	// A second thread with no note and no card: the zero-ref path.
	exec(t, d.DB, `INSERT INTO tutor_thread (user_id, title, kind, note_id) VALUES (?, 'свободный', '', 0)`, uid)

	return uid
}

func ptr[T any](v T) *T { return &v }

func exec(t *testing.T, d *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := d.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// ── The coverage test that keeps the manifest from rotting ──────────────────

// Every table in the schema is either carried by a profile or excluded with a
// stated reason. A migration that adds a table fails here until someone
// decides which it is — which is the only thing standing between "we added a
// table" and "exports silently stopped being complete".
func TestManifestCoversEveryUserTable(t *testing.T) {
	d, _ := openDB(t)

	rows, err := d.Query(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	carried := map[string]bool{}
	for _, tb := range manifest {
		carried[tb.Name] = true
	}
	var missing []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if carried[name] || excluded[name] != "" {
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("these tables are in the schema but neither exported nor excluded: %v\n"+
			"Add them to `manifest` (with their scope and any id remapping) or to "+
			"`excluded` with the reason.", missing)
	}
}

// A manifest entry that names a column the schema does not have would fail at
// runtime on a learner's export, not here. So check it here.
func TestManifestColumnsExist(t *testing.T) {
	d, _ := openDB(t)
	for _, tb := range manifest {
		cols, err := columnsOf(d.DB, tb.Name)
		if err != nil {
			t.Fatalf("%s: %v", tb.Name, err)
		}
		has := map[string]bool{}
		for _, c := range cols {
			has[c] = true
		}
		for _, c := range append(keys(tb.Refs), tb.IDColumn, tb.UserColumn) {
			if c != "" && !has[c] {
				t.Errorf("%s names column %q, which the schema does not have", tb.Name, c)
			}
		}
		for c := range tb.ZeroRefs {
			if tb.Refs[c] == "" {
				t.Errorf("%s.%s is a ZeroRef but not a Ref", tb.Name, c)
			}
		}
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ── Round trip ──────────────────────────────────────────────────────────────

// The headline property: export, wipe, import, and every table is back exactly
// as it was. Compared by VALUE, not by id, because ids are deliberately
// re-assigned.
func TestRoundTripRestoresEveryTable(t *testing.T) {
	src, srcDir := openDB(t)
	uid := seed(t, src, srcDir, "learner@example.com")

	var buf bytes.Buffer
	if err := Export(src.DB, srcDir, uid, "test", &buf); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, src.DB, uid)

	dst, dstDir := openDB(t)
	dstUID := freshUser(t, dst, "elsewhere@example.com")

	rep := mustImport(t, dst, dstDir, dstUID, buf.Bytes())
	after := snapshot(t, dst.DB, dstUID)

	for _, tb := range manifest {
		if tb.Name == "settings" {
			continue // the secret row is missing on purpose; checked separately
		}
		if before[tb.Name] != after[tb.Name] {
			t.Errorf("%s did not survive the round trip:\n  before %s\n  after  %s",
				tb.Name, before[tb.Name], after[tb.Name])
		}
	}
	if !rep.ReplayOK {
		t.Errorf("the replay check failed: %s", rep.ReplayWarning)
	}
	if rep.ReplayChecked == 0 {
		t.Error("the replay check verified nothing — the fixture has no history")
	}
	if rep.MediaFiles != 1 {
		t.Errorf("media files restored = %d, want 1", rep.MediaFiles)
	}
	if len(rep.SkippedColumns) != 0 {
		t.Errorf("columns were dropped: %v", rep.SkippedColumns)
	}
	if got, err := os.ReadFile(filepath.Join(dstDir, "media", "listen", "7.mp3")); err != nil {
		t.Errorf("the audio did not land: %v", err)
	} else if string(got) != "ID3-fake-audio" {
		t.Errorf("the audio landed corrupt: %q", got)
	}
}

// The references that are NOT foreign keys — tutor_thread.note_id and
// tutor_message.card_id are NOT NULL DEFAULT 0 — get no help from SQLite, so
// a remapping bug there is silent. Check them by hand.
func TestZeroReferencesAreRemappedAndZerosStayZero(t *testing.T) {
	src, srcDir := openDB(t)
	uid := seed(t, src, srcDir, "learner@example.com")

	var buf bytes.Buffer
	if err := Export(src.DB, srcDir, uid, "test", &buf); err != nil {
		t.Fatal(err)
	}
	dst, dstDir := openDB(t)
	// Give the destination a head start on ids, so a preserved id would be a
	// different row and the test would notice.
	other := freshUser(t, dst, "other@example.com")
	for i := 0; i < 5; i++ {
		exec(t, dst.DB, `INSERT INTO note (user_id, kind, hu) VALUES (?, 'vocab', 'filler')`, other)
	}
	dstUID := freshUser(t, dst, "elsewhere@example.com")
	mustImport(t, dst, dstDir, dstUID, buf.Bytes())

	var pinned, free int
	if err := dst.QueryRow(`
		SELECT COUNT(*) FROM tutor_thread t JOIN note n ON n.id = t.note_id
		 WHERE t.user_id = ? AND n.user_id = ?`, dstUID, dstUID).Scan(&pinned); err != nil {
		t.Fatal(err)
	}
	if pinned != 1 {
		t.Errorf("the pinned thread lost its note: %d threads resolve to one of this user's notes", pinned)
	}
	if err := dst.QueryRow(`SELECT COUNT(*) FROM tutor_thread WHERE user_id = ? AND note_id = 0`,
		dstUID).Scan(&free); err != nil {
		t.Fatal(err)
	}
	if free != 1 {
		t.Errorf("the unpinned thread should still have note_id 0, got %d such rows", free)
	}

	var cardOK int
	if err := dst.QueryRow(`
		SELECT COUNT(*) FROM tutor_message m JOIN card c ON c.id = m.card_id
		  JOIN note n ON n.id = c.note_id
		 WHERE m.user_id = ? AND n.user_id = ?`, dstUID, dstUID).Scan(&cardOK); err != nil {
		t.Fatal(err)
	}
	if cardOK != 1 {
		t.Errorf("the message lost its card: %d resolve", cardOK)
	}
}

// Secrets are the one thing an export must not carry.
func TestSecretsNeverLeaveTheServer(t *testing.T) {
	src, srcDir := openDB(t)
	uid := seed(t, src, srcDir, "learner@example.com")

	var buf bytes.Buffer
	if err := Export(src.DB, srcDir, uid, "test", &buf); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("sk-DO-NOT-EXPORT")) {
		t.Fatal("the API key is in the archive")
	}

	dst, dstDir := openDB(t)
	dstUID := freshUser(t, dst, "elsewhere@example.com")
	rep := mustImport(t, dst, dstDir, dstUID, buf.Bytes())
	if !rep.SecretsOmitted {
		t.Error("the report must tell the learner their keys did not travel")
	}

	var n int
	if err := dst.QueryRow(`SELECT COUNT(*) FROM settings WHERE user_id = ?`, dstUID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("settings rows = %d, want 1 (the non-secret one)", n)
	}
}

// An import is one learner's business and must not touch anyone else's rows.
func TestImportLeavesOtherUsersAlone(t *testing.T) {
	src, srcDir := openDB(t)
	uid := seed(t, src, srcDir, "learner@example.com")
	var buf bytes.Buffer
	if err := Export(src.DB, srcDir, uid, "test", &buf); err != nil {
		t.Fatal(err)
	}

	dst, dstDir := openDB(t)
	bystander := seed(t, dst, dstDir, "bystander@example.com")
	beforeSnap := snapshot(t, dst.DB, bystander)
	beforeHash := scalar(t, dst.DB, `SELECT password_hash FROM user WHERE id = ?`, bystander)

	dstUID := freshUser(t, dst, "elsewhere@example.com")
	mustImport(t, dst, dstDir, dstUID, buf.Bytes())

	if after := snapshot(t, dst.DB, bystander); !equalSnapshots(beforeSnap, after) {
		for k := range beforeSnap {
			if beforeSnap[k] != after[k] {
				t.Errorf("the bystander's %s changed:\n  before %s\n  after  %s",
					k, beforeSnap[k], after[k])
			}
		}
	}
	if got := scalar(t, dst.DB, `SELECT password_hash FROM user WHERE id = ?`, bystander); got != beforeHash {
		t.Error("the bystander's password was overwritten")
	}
}

// An import replaces; whatever the learner had before is gone, not merged.
func TestImportReplacesRatherThanMerges(t *testing.T) {
	src, srcDir := openDB(t)
	uid := seed(t, src, srcDir, "learner@example.com")
	var buf bytes.Buffer
	if err := Export(src.DB, srcDir, uid, "test", &buf); err != nil {
		t.Fatal(err)
	}

	dst, dstDir := openDB(t)
	dstUID := seed(t, dst, dstDir, "elsewhere@example.com") // already has a profile
	mustImport(t, dst, dstDir, dstUID, buf.Bytes())

	srcNotes := scalar(t, src.DB, `SELECT COUNT(*) FROM note WHERE user_id = ?`, uid)
	dstNotes := scalar(t, dst.DB, `SELECT COUNT(*) FROM note WHERE user_id = ?`, dstUID)
	if srcNotes != dstNotes {
		t.Errorf("notes after import = %s, want %s — an import must replace, not add",
			dstNotes, srcNotes)
	}
}

// A file from a newer binary is refused rather than half-understood.
func TestFutureFormatIsRefused(t *testing.T) {
	src, srcDir := openDB(t)
	uid := seed(t, src, srcDir, "learner@example.com")
	var buf bytes.Buffer
	if err := Export(src.DB, srcDir, uid, "test", &buf); err != nil {
		t.Fatal(err)
	}
	bumped := rewriteEnvelope(t, buf.Bytes(), func(env map[string]any) {
		env["formatVersion"] = FormatVersion + 1
	})

	dst, dstDir := openDB(t)
	dstUID := freshUser(t, dst, "elsewhere@example.com")
	_, err := Import(dst.DB, dstDir, dstUID, bytes.NewReader(bumped), int64(len(bumped)), 0.9)
	var fe ErrFutureFormat
	if !errors.As(err, &fe) {
		t.Fatalf("want ErrFutureFormat, got %v", err)
	}
}

// A damaged log must not import as though it were fine.
func TestCorruptHistoryFailsTheReplayCheckAndRollsBack(t *testing.T) {
	src, srcDir := openDB(t)
	uid := seed(t, src, srcDir, "learner@example.com")
	var buf bytes.Buffer
	if err := Export(src.DB, srcDir, uid, "test", &buf); err != nil {
		t.Fatal(err)
	}
	// Drop one review from the log. The card's stored state now claims more
	// history than the log can account for.
	damaged := rewriteEnvelope(t, buf.Bytes(), func(env map[string]any) {
		tables := env["tables"].(map[string]any)
		rl := tables["review_log"].(map[string]any)
		rows := rl["rows"].([]any)
		rl["rows"] = rows[1:]
	})

	dst, dstDir := openDB(t)
	dstUID := freshUser(t, dst, "elsewhere@example.com")
	if _, err := Import(dst.DB, dstDir, dstUID, bytes.NewReader(damaged), int64(len(damaged)), 0.9); err == nil {
		t.Fatal("a profile whose log does not produce its state must be refused")
	}
	// And nothing may have landed.
	if n := scalar(t, dst.DB, `SELECT COUNT(*) FROM note WHERE user_id = ?`, dstUID); n != "0" {
		t.Errorf("the failed import left %s notes behind — it must roll back", n)
	}
}

// A zip whose media climbs out of the media directory is refused.
func TestUnsafeMediaPathIsRefused(t *testing.T) {
	if safeRelPath("../../etc/passwd") || safeRelPath("/etc/passwd") ||
		safeRelPath("a/../../b") || safeRelPath("") {
		t.Error("safeRelPath let an escaping path through")
	}
	if !safeRelPath("listen/7.mp3") {
		t.Error("safeRelPath refused an ordinary path")
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func freshUser(t *testing.T, d *db.DB, email string) int64 {
	t.Helper()
	uid, err := d.CreateUser(email, "hash", db.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	return uid
}

func mustImport(t *testing.T, d *db.DB, dir string, uid int64, archive []byte) *Report {
	t.Helper()
	rep, err := Import(d.DB, dir, uid, bytes.NewReader(archive), int64(len(archive)), 0.9)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return rep
}

// snapshot renders every table's rows as comparable text, with the columns
// that are ids or point at ids blanked out — those are re-assigned by design,
// and comparing them would test the opposite of what we mean.
func snapshot(t *testing.T, d *sql.DB, uid int64) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, tb := range manifest {
		cols, err := columnsOf(d, tb.Name)
		if err != nil {
			t.Fatal(err)
		}
		var sel []string
		for _, c := range cols {
			switch {
			case c == tb.IDColumn, tb.Refs[c] != "", c == tb.UserColumn:
				continue // re-assigned on import; not part of the value
			default:
				sel = append(sel, c)
			}
		}
		if len(sel) == 0 {
			sel = []string{"1"}
		}
		q := `SELECT "` + strings.Join(sel, `", "`) + `" FROM "` + tb.Name +
			`" WHERE ` + tb.Scope
		rows, err := d.Query(q, uid)
		if err != nil {
			t.Fatalf("%s: %v", tb.Name, err)
		}
		var lines []string
		for rows.Next() {
			vals := make([]any, len(sel))
			ptrs := make([]any, len(sel))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				if b, ok := v.([]byte); ok {
					v = string(b)
				}
				parts[i] = fmt.Sprintf("%v", v)
			}
			lines = append(lines, strings.Join(parts, "|"))
		}
		rows.Close()
		// Row ORDER is not part of the value either: ids are re-assigned, so
		// sorting is what makes the comparison about content.
		sort.Strings(lines)
		out[tb.Name] = fmt.Sprintf("%d rows %v", len(lines), lines)
	}
	return out
}

func equalSnapshots(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func scalar(t *testing.T, d *sql.DB, q string, args ...any) string {
	t.Helper()
	var v any
	if err := d.QueryRow(q, args...).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

// rewriteEnvelope unpacks the archive, lets fn mangle the document, and packs
// it again — the only way to build the damaged files these tests need.
func rewriteEnvelope(t *testing.T, archive []byte, fn func(map[string]any)) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open(FileName)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.NewDecoder(f).Decode(&env); err != nil {
		t.Fatal(err)
	}
	f.Close()
	fn(env)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(FileName)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(w).Encode(env); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
