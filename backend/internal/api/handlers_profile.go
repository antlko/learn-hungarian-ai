package api

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/profile"
)

// Export and import of one learner's whole profile.
//
// ADR-011 removed the old `.apkg` export and was right to: that format could
// not carry the morphological state space, so what it handed a learner was a
// vocabulary list labelled as their progress. It also named the honest shape
// in the same breath — a full dump of both state spaces plus the review log —
// and ADR-040 is that shape being built. See internal/profile.
//
// Both routes act on the ACTING user and nobody else. An admin exporting is
// exporting their own profile; there is deliberately no endpoint that reads
// another account's data, because "self-hosted" does not mean "the first
// account owns everyone".

// maxImportBytes caps an upload. A learner with a year of texts and a few
// audio materials is comfortably inside this; it exists so a bad upload fails
// fast rather than filling the disk.
const maxImportBytes = 2 << 30 // 2 GiB

// ImportReport is the wire shape of what an import did. It mirrors
// profile.Report, which is deliberately not a DTO: the package should not have
// to know how it is rendered.
type ImportReport struct {
	Tables         map[string]int `json:"tables"`
	TotalRows      int            `json:"totalRows"`
	MediaFiles     int            `json:"mediaFiles"`
	SkippedColumns []string       `json:"skippedColumns,omitempty"`
	ReplayOK       bool           `json:"replayOk"`
	ReplayChecked  int            `json:"replayChecked"`
	ReplayWarning  string         `json:"replayWarning,omitempty"`
	SecretsOmitted bool           `json:"secretsOmitted"`
}

// handleProfileExport streams the acting learner's profile as a zip.
func (s *Server) handleProfileExport(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("tanulom-profile-%s.zip", time.Now().Format("2006-01-02"))
	c.Set(fiber.HeaderContentType, "application/zip")
	// The filename is quoted and percent-encoded: an email is not in the name,
	// but a future one might be, and a bare quote in a header is a bug waiting.
	c.Set(fiber.HeaderContentDisposition,
		`attachment; filename="`+name+`"; filename*=UTF-8''`+url.PathEscape(name))

	// Written straight to the response: a profile can be tens of megabytes and
	// there is nothing to gain from holding it in memory first. The cost is
	// that a mid-stream failure cannot become a clean error — the status is
	// already sent — so it is logged and the archive ends truncated, which the
	// zip reader on the other side will refuse rather than half-import.
	if err := profile.Export(s.db.DB, s.dataDir, u.ID, Version, c.Response().BodyWriter()); err != nil {
		slog.ErrorContext(c.Context(), "profile export failed mid-stream",
			"user", u.ID, "error", err)
		return err
	}
	slog.InfoContext(c.Context(), "profile exported", "user", u.ID)
	return nil
}

// handleProfileImport replaces the acting learner's profile from an upload.
//
// This is the most destructive thing a learner can ask the app to do, so the
// confirm dialog on the client names by count what is about to go. The server
// does not second-guess a confirmed request, but it does refuse a file it
// cannot fully understand — see profile.ErrFutureFormat.
func (s *Server) handleProfileImport(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	fh, err := c.FormFile("profile")
	if err != nil || fh == nil {
		return badRequest("a profile archive is required")
	}
	if fh.Size > maxImportBytes {
		return badRequest("that file is %d MB; the limit is %d MB",
			fh.Size>>20, maxImportBytes>>20)
	}

	// Spooled to a temp file rather than held in memory: zip needs an
	// io.ReaderAt, and a multipart part is a stream.
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	tmp, err := os.CreateTemp(s.dataDir, "import-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	n, err := io.Copy(tmp, io.LimitReader(src, maxImportBytes))
	if err != nil {
		return err
	}

	cfg := s.Config()
	rep, err := profile.Import(s.db.DB, s.dataDir, u.ID, tmp, n, cfg.Review.DesiredRetention)
	if err != nil {
		var future profile.ErrFutureFormat
		if errors.As(err, &future) {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		slog.ErrorContext(c.Context(), "profile import failed", "user", u.ID, "error", err)
		// The transaction rolled back; the learner still has what they had.
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	total := 0
	for _, v := range rep.RowsByTable {
		total += v
	}
	slog.InfoContext(c.Context(), "profile imported",
		"user", u.ID, "rows", total, "media", rep.MediaFiles,
		"replay_ok", rep.ReplayOK, "replay_checked", rep.ReplayChecked)

	return c.JSON(ImportReport{
		Tables:         rep.RowsByTable,
		TotalRows:      total,
		MediaFiles:     rep.MediaFiles,
		SkippedColumns: rep.SkippedColumns,
		ReplayOK:       rep.ReplayOK,
		ReplayChecked:  rep.ReplayChecked,
		ReplayWarning:  rep.ReplayWarning,
		SecretsOmitted: rep.SecretsOmitted,
	})
}

// ProfileSizeResponse is what the confirm dialog counts.
//
// The dialog has to say what is about to be erased, and it must say it in the
// learner's own numbers — words, reviews, texts — not in table names. Counting
// on the server rather than reusing the review breakdowns is deliberate: those
// are post-burial and day-granular, which is right for "what will I study
// today" and wrong for "what am I about to delete".
type ProfileSizeResponse struct {
	Notes   int `json:"notes"`
	Cards   int `json:"cards"`
	Reviews int `json:"reviews"`
	Texts   int `json:"texts"`
	Words   int `json:"words"`
	Threads int `json:"threads"`
	Days    int `json:"days"`
}

func (s *Server) handleProfileSize(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var out ProfileSizeResponse
	count := func(q string, into *int) error {
		return s.db.QueryRow(q, u.ID).Scan(into)
	}
	for _, pair := range []struct {
		q    string
		into *int
	}{
		{`SELECT COUNT(*) FROM note WHERE user_id = ?`, &out.Notes},
		{`SELECT COUNT(*) FROM card c JOIN note n ON n.id = c.note_id WHERE n.user_id = ?`, &out.Cards},
		{`SELECT COUNT(*) FROM review_log r JOIN card c ON c.id = r.card_id
		    JOIN note n ON n.id = c.note_id WHERE n.user_id = ?`, &out.Reviews},
		{`SELECT COUNT(*) FROM text WHERE user_id = ?`, &out.Texts},
		{`SELECT COUNT(*) FROM lemma_state WHERE user_id = ?`, &out.Words},
		{`SELECT COUNT(*) FROM tutor_thread WHERE user_id = ?`, &out.Threads},
		{`SELECT COUNT(*) FROM daily_log WHERE user_id = ?`, &out.Days},
	} {
		if err := count(pair.q, pair.into); err != nil {
			return err
		}
	}
	return c.JSON(out)
}
