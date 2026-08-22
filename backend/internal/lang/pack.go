package lang

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// Pack is an opened, read-only language pack.
//
// It is deliberately a separate database from the learner's: a pack upgrade is a
// file swap that cannot put learner data at risk, and the pack is never written
// to. Learner corrections to a bad analysis live in `morph_correction` in the
// user database and are applied as an overlay at lookup time. See ADR-002.
type Pack struct {
	db      *sql.DB
	Version string
	Meta    Meta
}

// Meta is the pack's self-description, from the pack_meta table.
type Meta struct {
	Version       string   `json:"version"`
	SchemaVersion int      `json:"schema_version"`
	GlossLangs    []string `json:"gloss_langs"`
	// Edition is "starter" for the pack embedded in the binary and "full" for
	// the one built with `make langpack`. The UI names the installed edition off
	// this field rather than guessing from a row count. There is no in-app
	// upgrade path and the UI says so — [ADR-010].
	Edition string `json:"edition"`
}

// openQuiet opens a pack read-only and verifies it is one this binary
// understands, without announcing it. See Open.
func openQuiet(path string) (*Pack, error) {
	// mode=ro is belt and braces: nothing in the codebase writes here, and the
	// driver will refuse if something ever tries.
	dsn := "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)&_pragma=query_only(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open language pack: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("language pack unreadable: %w", err)
	}

	p := &Pack{db: db}
	if err := p.readMeta(); err != nil {
		db.Close()
		return nil, err
	}
	if p.Meta.SchemaVersion != SchemaVersion {
		db.Close()
		// Fail loudly at startup with the actual numbers, rather than producing
		// confusing lookup failures later.
		return nil, fmt.Errorf(
			"language pack schema version %d is not supported by this build (needs %d): "+
				"update the application, or remove this pack so the binary can "+
				"install the starter it carries",
			p.Meta.SchemaVersion, SchemaVersion)
	}
	return p, nil
}

// Open loads a pack and says so in the log.
//
// Split from openQuiet because two callers open a pack only to prove it is one
// — Detect vetting candidates, EnsurePack vetting what it just extracted — and
// their probes were reaching the log as "language pack loaded", announcing a
// pack before it was installed and again when it really loaded. A startup log
// that says a thing twice teaches the reader to stop trusting it.
func Open(path string) (*Pack, error) {
	p, err := openQuiet(path)
	if err != nil {
		return nil, err
	}
	slog.Info("language pack loaded", "version", p.Meta.Version,
		"edition", p.Meta.Edition, "schema", p.Meta.SchemaVersion,
		"glosses", p.Meta.GlossLangs)
	return p, nil
}

func (p *Pack) readMeta() error {
	rows, err := p.db.Query(`SELECT key, value FROM pack_meta`)
	if err != nil {
		return fmt.Errorf("read pack metadata: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return fmt.Errorf("scan pack metadata: %w", err)
		}
		switch k {
		case "version":
			p.Meta.Version = strings.Trim(v, `"`)
			p.Version = p.Meta.Version
		case "schema_version":
			n, err := strconv.Atoi(strings.Trim(v, `"`))
			if err != nil {
				return fmt.Errorf("pack schema_version %q is not a number", v)
			}
			p.Meta.SchemaVersion = n
		case "gloss_langs":
			_ = json.Unmarshal([]byte(v), &p.Meta.GlossLangs)
		case "edition":
			p.Meta.Edition = strings.Trim(v, `"`)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if p.Meta.Version == "" {
		return fmt.Errorf("language pack has no version in pack_meta")
	}
	if p.Meta.Edition == "" {
		p.Meta.Edition = EditionFull
	}
	return nil
}

// DB exposes the read-only handle for the morph and stats packages.
func (p *Pack) DB() *sql.DB { return p.db }

// Close releases the pack.
func (p *Pack) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Counts returns row counts for the tables the UI reports on.
func (p *Pack) Counts() (map[string]int, error) {
	out := map[string]int{}
	for _, t := range []string{
		"lemma", "wordform", "sentence", "morph_feature", "grammar_topic", "minimal_pair",
	} {
		var n int
		if err := p.db.QueryRow(`SELECT COUNT(*) FROM ` + t).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", t, err)
		}
		out[t] = n
	}
	return out, nil
}
