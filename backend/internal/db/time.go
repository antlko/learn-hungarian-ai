package db

import (
	"database/sql"
	"time"
)

// The wire format for every timestamp stored in this database.
//
// SQLite has no time type, so timestamps are TEXT and the exact layout is a
// contract with the rows already on disk — including the migrations' own
// `strftime('%Y-%m-%dT%H:%M:%fZ','now')` defaults, which this layout matches.
// The milliseconds are load-bearing: the review log replays to stored state
// bit-for-bit, and a timestamp truncated to the second reorders reviews that
// happened inside the same second.
//
// It lives here, in the package that owns the handle, because the four copies
// that used to sit in srs, drill, hvpt and stats had already drifted apart —
// one wrote NULL where the others wrote the empty string for a zero time, and
// had lost a parse layout, so the same instant round-tripped differently
// depending on which table it went through.
const RFC3339ms = "2006-01-02T15:04:05.000Z07:00"

// FmtTime renders a timestamp for storage.
//
// A zero time is written as the empty string, not NULL. Both read back as the
// zero time, but only one of them is what the existing rows contain, and a
// column that is empty in some rows and NULL in others cannot be compared in
// SQL.
func FmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(RFC3339ms)
}

// ParseTime reads a stored timestamp back.
//
// NULL, the empty string and anything unparseable all yield the zero time: a
// column we cannot read is not a date we are entitled to guess at. The two
// looser layouts accept rows written before the format was pinned.
func ParseTime(s sql.NullString) time.Time {
	if !s.Valid || s.String == "" {
		return time.Time{}
	}
	for _, layout := range []string{RFC3339ms, time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s.String); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
