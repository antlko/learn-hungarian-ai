package api

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/stats"
)

// Reading time in the daily ledger.
//
// Every other loop reported its time and the reader reported none: an hour
// with a text moved no counter, so «21 / 90 мин» sat still and the day's
// obligation went unmet while the learner had been working the whole time.
// That is not a display bug — `daily_log.active_ms` is one half of what the
// streak requires (stats.Day.Met), so reading was literally not counted as
// study.
//
// It counts now, on the same terms as listening: clamped per report, and only
// for time the client can actually attest to. ADR-044.
//
// What the server can and cannot know, stated rather than assumed: it cannot
// see whether anyone is looking at the screen. So the clamp is the whole
// defence, and the client is the one that must not report time for a
// backgrounded tab or an idle one — see ReaderView. The server's job is to
// make a lying client cheap rather than impossible: 60 s per report, the same
// bound listening has, so a stuck timer mints a minute and not a night.

// maxReportSeconds bounds one progress report, for reading and listening
// alike. Sixty seconds because the client reports about once a second and a
// minute is the largest gap a real pause or a slow network explains.
const maxReportSeconds = 60

// ReadingProgressRequest is time spent with a text since the previous report.
type ReadingProgressRequest struct {
	// Seconds is active reading time — the client sends it only while the tab
	// is visible AND the learner has interacted recently.
	Seconds float64 `json:"seconds"`
}

// ReadingProgressResponse echoes what was actually credited.
//
// Echoed rather than assumed, because the clamp means the two can differ and
// a client that believes its own number would drift from the ledger.
type ReadingProgressResponse struct {
	Seconds  float64 `json:"seconds"`
	ActiveMs int64   `json:"activeMs"`
}

func (s *Server) handleReadingProgress(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req ReadingProgressRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.Seconds < 0 {
		req.Seconds = 0
	}
	if req.Seconds > maxReportSeconds {
		req.Seconds = maxReportSeconds
	}
	if req.Seconds == 0 {
		// Not an error: the client reports on a timer and an idle tick is the
		// normal case, not a failure.
		return c.JSON(ReadingProgressResponse{})
	}

	st := stats.NewStore(s.db.DB)
	if err := st.RecordReading(u.ID, localDay(time.Now()), req.Seconds); err != nil {
		return err
	}

	var activeMs int64
	if err := s.db.QueryRow(
		`SELECT active_ms FROM daily_log WHERE user_id = ? AND day = ?`,
		u.ID, localDay(time.Now()),
	).Scan(&activeMs); err != nil {
		// The write landed; failing to read the total back is not worth
		// failing the request over.
		activeMs = 0
	}
	return c.JSON(ReadingProgressResponse{Seconds: req.Seconds, ActiveMs: activeMs})
}
