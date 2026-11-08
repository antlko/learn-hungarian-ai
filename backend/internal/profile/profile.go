// Package profile moves one learner's entire profile between installations.
//
// # Why this exists again
//
// v1.0 shipped `internal/export` and ADR-011 removed it, because what it
// exported was an `.apkg`: one card, one interval, a vocabulary list. The
// morphological half of what this app knows — the thing the whole product is
// about — could not cross that format, and handing a learner half their
// progress while calling it their progress is worse than handing them nothing.
//
// ADR-011 named the honest shape in the same breath: "a full JSON dump of both
// state spaces plus the review log". That is exactly this package. Nothing is
// summarised, nothing is flattened, and the format is not an interchange
// format with another tool — it is this app's own state, whole, moving to
// another copy of this app. See ADR-040.
//
// # The two rules that shape the code
//
//  1. **Columns are discovered, never listed.** Every table is read through
//     `PRAGMA table_info`. A hand-maintained column list is a data-loss bug
//     waiting for the next migration: someone adds a column, forgets this
//     file, and exports quietly stop carrying it. Discovery means a new column
//     rides along the day it is added, and an export written by an older
//     binary still imports (its missing columns take their schema defaults).
//
//  2. **Surrogate ids are remapped, not preserved.** `note.id` is global and
//     autoincrementing; on the destination install another learner may already
//     hold id 5. So ids are re-assigned on insert and the four tables whose ids
//     are pointed at — note, card, text, tutor_thread — carry an old→new map
//     that every referencing column is rewritten through.
//
// # What is deliberately left out
//
//   - `session` — live tokens. Moving them would move a logged-in session to a
//     machine the learner may not control.
//   - `app_meta` — instance-level bookkeeping (the FSRS steps reconciliation
//     marker), not learner state.
//   - `settings` rows with `secret = 1` — API keys. "Secrets never leave the
//     server" is an invariant, and an export is a file that leaves the server
//     by definition. The learner re-enters them, and both the export screen and
//     the import report say so.
package profile

import "fmt"

// FormatVersion is the profile format this binary writes and the newest it can
// read. It is deliberately separate from the goose migration number: a
// migration that only adds a column changes nothing here, because columns are
// discovered. Bump this only when the FILE's shape changes — the manifest, the
// remapping rules, the envelope.
const FormatVersion = 1

// FileName is the JSON document inside the archive.
const FileName = "profile.json"

// MediaDir is the archive directory mirroring <dataDir>/media.
const MediaDir = "media/"

// table describes one user-scoped table: how to find this learner's rows, which
// column is a surrogate key worth remapping, and which columns point at the
// remapped keys of other tables.
//
// The slice order is INSERT order — parents before children. Deletes walk it
// backwards. Foreign keys are on (`db.Open` sets `_pragma=foreign_keys(1)`), so
// getting this order wrong is a loud failure rather than a silent one, which is
// the reason to rely on it.
type table struct {
	// Name is the SQL table.
	Name string
	// UserColumn holds the owning user id, or "" when the table is reached
	// through a join instead (card, review_log, text_token, note_origin).
	UserColumn string
	// Scope is the WHERE clause selecting one learner's rows, with exactly one
	// placeholder for the user id. Set for every table, join or not, so the
	// export and the delete-before-import cannot disagree about what "this
	// learner's rows" means.
	Scope string
	// IDColumn is the surrogate key remapped on import, or "" when the table
	// has no id anything else points at. A table with an id that nothing
	// references (review_log, drill_attempt, …) leaves this empty and simply
	// lets SQLite assign a fresh one.
	IDColumn string
	// Refs maps a column to the table whose remapped ids it carries.
	Refs map[string]string
	// ZeroRefs are Refs columns where 0 means "none" rather than a real id —
	// `tutor_thread.note_id` and `tutor_message.card_id` are NOT NULL DEFAULT 0
	// instead of nullable. A 0 stays 0; anything else is remapped.
	ZeroRefs map[string]bool
	// Extra is appended to Scope on export only: the one place a row is
	// filtered out for a reason other than ownership (secret settings).
	Extra string
}

// manifest is every table a profile carries, in insert order.
//
// Anything NOT here is either instance-level or excluded on purpose; see the
// package doc. When a migration adds a table, it belongs here the same day —
// `TestManifestCoversEveryUserTable` fails otherwise, which is what makes this
// list maintainable rather than a thing that rots.
var manifest = []table{
	{
		Name: "settings", UserColumn: "user_id", Scope: "user_id = ?",
		// The one export-side filter in the whole package. An API key is not
		// the learner's progress and must not ride in a file.
		Extra: " AND secret = 0",
	},
	{Name: "note", UserColumn: "user_id", Scope: "user_id = ?", IDColumn: "id"},
	{
		Name:  "card",
		Scope: "note_id IN (SELECT id FROM note WHERE user_id = ?)",
		// Referenced by review_log, feature_review and practice_log.
		IDColumn: "id",
		Refs:     map[string]string{"note_id": "note"},
	},
	{
		Name:  "review_log",
		Scope: "card_id IN (SELECT c.id FROM card c JOIN note n ON n.id = c.note_id WHERE n.user_id = ?)",
		Refs:  map[string]string{"card_id": "card"},
	},
	{Name: "lemma_state", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "morph_state", UserColumn: "user_id", Scope: "user_id = ?"},
	{
		Name: "feature_review", UserColumn: "user_id", Scope: "user_id = ?",
		Refs: map[string]string{"card_id": "card"},
	},
	{Name: "morph_correction", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "text", UserColumn: "user_id", Scope: "user_id = ?", IDColumn: "id"},
	{
		Name:  "text_token",
		Scope: "text_id IN (SELECT id FROM text WHERE user_id = ?)",
		Refs:  map[string]string{"text_id": "text"},
	},
	{
		Name:  "note_origin",
		Scope: "note_id IN (SELECT id FROM note WHERE user_id = ?)",
		Refs:  map[string]string{"note_id": "note", "text_id": "text"},
	},
	{Name: "topic_progress", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "drill_attempt", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "hvpt_contrast_state", UserColumn: "user_id", Scope: "user_id = ?"},
	// hvpt_trial.pair_id points into the read-only pack, not at anything here,
	// so it is carried verbatim and not remapped.
	{Name: "hvpt_trial", UserColumn: "user_id", Scope: "user_id = ?"},
	{
		Name: "listen_state", UserColumn: "user_id", Scope: "user_id = ?",
		Refs: map[string]string{"text_id": "text"},
	},
	{
		Name: "listen_log", UserColumn: "user_id", Scope: "user_id = ?",
		Refs: map[string]string{"text_id": "text"},
	},
	{Name: "daily_log", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "checkpoint", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "course_progress", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "translation_cache", UserColumn: "user_id", Scope: "user_id = ?"},
	{Name: "user_gloss", UserColumn: "user_id", Scope: "user_id = ?"},
	{
		Name: "practice_log", UserColumn: "user_id", Scope: "user_id = ?",
		Refs: map[string]string{"card_id": "card"},
	},
	{
		Name: "tutor_thread", UserColumn: "user_id", Scope: "user_id = ?",
		IDColumn: "id",
		Refs:     map[string]string{"note_id": "note"},
		ZeroRefs: map[string]bool{"note_id": true},
	},
	{
		Name: "tutor_message", UserColumn: "user_id", Scope: "user_id = ?",
		Refs:     map[string]string{"thread_id": "tutor_thread", "card_id": "card"},
		ZeroRefs: map[string]bool{"thread_id": true, "card_id": true},
	},
}

// excluded names the tables a profile deliberately does not carry, with the
// reason, so the coverage test can assert the set is complete rather than
// merely large. A new table must be listed here or in the manifest.
var excluded = map[string]string{
	"user":             "the envelope carries the account itself, not a row",
	"session":          "live tokens; moving them moves a logged-in session",
	"app_meta":         "instance-level bookkeeping, not learner state",
	"goose_db_version": "the migration ledger of the destination install",
}

// Envelope is the profile document: a header saying what wrote it and on what,
// the account, and every table.
type Envelope struct {
	// FormatVersion is refused when newer than this binary's.
	FormatVersion int `json:"formatVersion"`
	// AppVersion and ExportedAt are for the learner and for support, never
	// for a decision this code makes.
	AppVersion string `json:"appVersion"`
	ExportedAt string `json:"exportedAt"`
	// GOARCH is load-bearing. The review log replays bit-for-bit only on one
	// CPU architecture — go-fsrs's fuzz is floating point and Go fuses
	// multiply-adds on arm64 only — so a replay mismatch after a cross-arch
	// move is expected and is reported as a warning, not a failure.
	// ARCHITECTURE.md §5.7.
	GOARCH string `json:"goarch"`
	// SecretsOmitted is always true today. It is a field rather than a
	// constant so the import report states it as a fact read from the file.
	SecretsOmitted bool `json:"secretsOmitted"`

	User   AccountDTO      `json:"user"`
	Media  []MediaFile     `json:"media"`
	Tables map[string]Rows `json:"tables"`
}

// AccountDTO is the account itself. The password hash travels so the learner
// can log in on the destination with the password they already know; it is a
// hash, and the file is theirs.
type AccountDTO struct {
	Email        string `json:"email"`
	PasswordHash string `json:"passwordHash"`
	Role         string `json:"role"`
	CreatedAt    string `json:"createdAt"`
}

// MediaFile records one file carried in the archive, by its path relative to
// <dataDir>/media.
type MediaFile struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// Rows is one table: the column names, then the values positionally.
//
// Positional rather than a map per row because a text with ten thousand tokens
// would otherwise repeat ten column names ten thousand times. The file stays
// readable — the columns are right there at the top of each table.
type Rows struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// Report is what an import did, for the screen that shows it.
type Report struct {
	// RowsByTable is what actually landed, per table.
	RowsByTable map[string]int `json:"rowsByTable"`
	// MediaFiles restored.
	MediaFiles int `json:"mediaFiles"`
	// SkippedColumns names columns present in the file but not in this
	// schema — an export from a NEWER minor version, or a column since
	// dropped. Named rather than counted: silently discarding a learner's
	// data is the thing this package exists not to do.
	SkippedColumns []string `json:"skippedColumns,omitempty"`
	// ReplayOK is the integrity check: the review log replayed to the card
	// state that came with it. ReplayWarning explains a mismatch that is
	// expected (a cross-architecture move) rather than a corruption.
	ReplayOK      bool   `json:"replayOk"`
	ReplayChecked int    `json:"replayChecked"`
	ReplayWarning string `json:"replayWarning,omitempty"`
	// SecretsOmitted repeats the file's own claim so the UI can tell the
	// learner to re-enter their API keys.
	SecretsOmitted bool `json:"secretsOmitted"`
}

// ErrFutureFormat is returned when the file was written by a newer binary.
// Importing it would mean guessing at a shape this code does not know, and a
// half-understood profile is worse than a refused one.
type ErrFutureFormat struct {
	Found, Supported int
}

func (e ErrFutureFormat) Error() string {
	return fmt.Sprintf("this profile was written in format %d; this version reads up to %d",
		e.Found, e.Supported)
}
