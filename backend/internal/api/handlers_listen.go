package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/db"
	"tanulom/internal/mining"
	"tanulom/internal/stats"
)

// The listening loop.
//
// A listening material is a TEXT — the transcript goes through the exact same
// ingest as a reading text, which is what makes the coverage pre-check free and
// honest: the same two coverage numbers, the same thresholds, computed by the
// same code. The audio file rides alongside in the media directory.
//
// The three-pass protocol ([R13] — .claude/research/13-shadowing.md) is UI
// state persisted per learner: 1 gist without the transcript, 2 with it, 3
// dictation/shadowing. The server never advances a pass on its own; passes are
// where the learner says they are.
//
// listen_log is the append-only record of what was played; the day's clamped
// total lands in daily_log alongside it, and that is what the listening-hours
// gate on word-order topics reads [R15]. Either way the hours accumulate from
// reported playback — they are never claimed.

const maxAudioBytes = 60 << 20 // within the Fiber body limit

var audioExts = map[string]string{
	"audio/mpeg": ".mp3", "audio/mp3": ".mp3", "audio/ogg": ".ogg",
	"audio/wav": ".wav", "audio/x-wav": ".wav", "audio/mp4": ".m4a",
	"audio/aac": ".m4a", "audio/webm": ".webm", "audio/flac": ".flac",
}

// handleListenUpload creates a listening material: multipart form with `audio`
// (the file), `title` and `transcript`.
func (s *Server) handleListenUpload(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requirePack(); err != nil {
		return err
	}
	title := strings.TrimSpace(c.FormValue("title"))
	transcript := strings.TrimSpace(c.FormValue("transcript"))
	if transcript == "" {
		return badRequest("a transcript is required — the coverage pre-check and " +
			"passes 2–3 are built on it")
	}
	fh, err := c.FormFile("audio")
	if err != nil {
		return badRequest("an audio file is required")
	}
	if fh.Size > maxAudioBytes {
		return badRequest("audio is too large (%d MB, limit %d)", fh.Size>>20, maxAudioBytes>>20)
	}
	ext := audioExts[fh.Header.Get("Content-Type")]
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(fh.Filename))
		if len(ext) > 5 || ext == "" {
			ext = ".mp3"
		}
	}

	// The transcript is ingested exactly like a pasted reading text.
	in, err := mining.FromPaste(title, transcript)
	if err != nil {
		return badRequest("%s", err.Error())
	}
	tokens, sentences, err := s.analyzer.AnalyzeText(in.Body)
	if err != nil {
		return err
	}
	stored := toStoredTokens(tokens, sentences)
	cov, err := s.coverageFor(u.ID, mining.AsTokenViews(stored))
	if err != nil {
		return err
	}
	store := mining.NewStore(s.db.DB)
	id, err := store.Create(u.ID, in, "", stored, cov)
	if err != nil {
		return err
	}

	// Audio lands under media/listen/<text id>.<ext>; the id makes the name
	// collision-free and the delete path obvious.
	rel := fmt.Sprintf("listen/%d%s", id, ext)
	dst := filepath.Join(s.dataDir, "media", rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE text SET audio_path = ? WHERE id = ? AND user_id = ?`,
		rel, id, u.ID); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id": id, "title": in.Title, "audioPath": rel, "coverage": cov,
		// The pre-check verdict rides along so the UI can warn BEFORE playback:
		// below ~90% element coverage, listening is decoding, not input. [R6]
		"verdict": cov.Judge(),
	})
}

// ListenStateDTO is the learner's position in the three-pass protocol.
type ListenStateDTO struct {
	Pass      int     `json:"pass"`
	PositionS float64 `json:"positionS"`
	// Seconds listened on this text, all passes.
	ListenedS float64 `json:"listenedS"`
}

func (s *Server) handleListenState(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("text id must be a number")
	}
	dto := ListenStateDTO{Pass: 1}
	_ = s.db.QueryRow(`
		SELECT pass, position_s FROM listen_state WHERE user_id = ? AND text_id = ?`,
		u.ID, id).Scan(&dto.Pass, &dto.PositionS)
	// Per-text, so this one genuinely needs the play log: the daily ledger
	// aggregates across materials and cannot answer "on this recording". It is
	// bounded by one text, unlike the lifetime total the gate reads.
	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(seconds),0) FROM listen_log WHERE user_id = ? AND text_id = ?`,
		u.ID, id).Scan(&dto.ListenedS)
	return c.JSON(dto)
}

// ListenProgressRequest reports playback: position/pass changes and seconds
// actually heard since the last report.
type ListenProgressRequest struct {
	Pass      int     `json:"pass"`
	PositionS float64 `json:"positionS"`
	// SecondsHeard is the audible playtime since the previous report. Bounded
	// per report so a stuck client cannot mint hours.
	SecondsHeard float64 `json:"secondsHeard"`
}

func (s *Server) handleListenProgress(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("text id must be a number")
	}
	var req ListenProgressRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.Pass < 1 || req.Pass > 3 {
		return badRequest("pass must be 1, 2 or 3")
	}
	// One report covers at most a minute of listening: hours accumulate from
	// many small true reports, and a fabricated giant one is clamped.
	if req.SecondsHeard < 0 {
		req.SecondsHeard = 0
	}
	if req.SecondsHeard > 60 {
		req.SecondsHeard = 60
	}

	var owned int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM text WHERE id = ? AND user_id = ?`,
		id, u.ID).Scan(&owned); err != nil || owned == 0 {
		return fiber.NewError(fiber.StatusNotFound, "no such text")
	}

	if _, err := s.db.Exec(`
		INSERT INTO listen_state (user_id, text_id, pass, position_s, updated_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(user_id, text_id) DO UPDATE SET
		  pass=excluded.pass, position_s=excluded.position_s, updated_at=excluded.updated_at`,
		u.ID, id, req.Pass, req.PositionS, db.FmtTime(time.Now())); err != nil {
		return err
	}
	if req.SecondsHeard > 0 {
		if _, err := s.db.Exec(`
			INSERT INTO listen_log (user_id, text_id, seconds, pass) VALUES (?,?,?,?)`,
			u.ID, id, req.SecondsHeard, req.Pass); err != nil {
			return err
		}
		// Listening is real work: it counts toward the day's minutes. The error
		// is not swallowed the way a ledger write elsewhere is, because the day
		// row is also where the listening-hours gate reads its total from —
		// dropping this write silently would quietly lose hours the learner
		// earned.
		if err := stats.NewStore(s.db.DB).RecordListening(
			u.ID, localDay(time.Now()), req.SecondsHeard); err != nil {
			return err
		}
	}

	return c.JSON(fiber.Map{"ok": true, "totalHours": s.listeningHours(u.ID)})
}
