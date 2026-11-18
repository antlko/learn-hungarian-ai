package api

import (
	"context"
	"errors"
	"os"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/speech"
)

// Piper speech (ADR-079). The browser asks for a sentence, gets a URL and plays
// it; the server renders it with the managed `tts` child once and caches it.
//
// Every failure the child can cause is a 503/504 with an i18n key. The client
// treats both as "use the device voice" and shows nothing: speech is an
// enhancement, never a requirement (ADR-012).

// speechEndpoint is the tts child's address while it can answer, "" otherwise.
// A paused service resolves to off (ADR-068) and so to "".
func (s *Server) speechEndpoint() string {
	return s.service("tts").Endpoint
}

// handleSpeechVoices lists the personas the running child can render.
func (s *Server) handleSpeechVoices(c fiber.Ctx) error {
	out := SpeechVoicesDTO{Voices: []SpeechVoiceDTO{}}
	ep := s.speechEndpoint()
	if ep == "" {
		return c.JSON(out)
	}
	have, err := s.speech.ChildVoices(c.Context(), ep)
	if err != nil {
		return c.JSON(out)
	}
	ok := map[string]bool{}
	for _, v := range have {
		ok[v.ID] = true
	}
	for _, p := range speech.Personas {
		if ok[p.ID] {
			out.Voices = append(out.Voices, SpeechVoiceDTO{ID: p.ID, Name: p.Name, Gender: p.Gender})
		}
	}
	out.Available = len(out.Voices) > 0
	return c.JSON(out)
}

// handleSpeechTTS renders (or finds) one sentence or word.
func (s *Server) handleSpeechTTS(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	var req SpeechTTSRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.Style == "" {
		req.Style = speech.StyleSentence
	}
	res, err := s.speech.Synthesize(c.Context(), s.speechEndpoint(),
		speech.Request{Text: req.Text, Voice: req.Voice, Style: req.Style})
	switch {
	case err == nil:
		return c.JSON(SpeechTTSResponse{URL: "/api/speech/audio/" + res.File, DurationS: res.DurationS})
	case errors.Is(err, speech.ErrBadText):
		return badRequest("text must be 1 to %d bytes", speech.MaxTextBytes)
	case errors.Is(err, speech.ErrBadVoice):
		return badRequest("unknown voice")
	case errors.Is(err, speech.ErrBadStyle):
		return badRequest("style must be sentence or word")
	case errors.Is(err, speech.ErrTimeout):
		return fiber.NewError(fiber.StatusGatewayTimeout, "errors.ttsTimeout")
	case errors.Is(err, speech.ErrUnavailable), errors.Is(err, context.Canceled):
		return fiber.NewError(fiber.StatusServiceUnavailable, "errors.ttsUnavailable")
	}
	return err
}

// handleSpeechAudio serves one cached WAV. The name must be exactly
// "<64 hex>.wav": speech.Path returns "" for anything else, so no request can
// name another file. Content is addressed by hash, hence immutable.
func (s *Server) handleSpeechAudio(c fiber.Ctx) error {
	p := s.speech.Path(c.Params("file"))
	if p == "" {
		return fiber.NewError(fiber.StatusNotFound, "no such audio")
	}
	if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
		return fiber.NewError(fiber.StatusNotFound, "no such audio")
	}
	c.Set(fiber.HeaderCacheControl, "private, max-age=31536000, immutable")
	c.Set(fiber.HeaderContentType, "audio/wav")
	return c.SendFile(p, fiber.SendFile{ByteRange: true})
}

// speechBodyLimit bounds POST /api/speech/tts: the text is at most 1000 bytes,
// so 8 KiB leaves room for JSON and nothing more. The global limit (64 MB) is
// for uploads.
const speechBodyLimit = 8 << 10

// limitBody refuses a request whose declared or actual body exceeds max, with
// 413, before any handler parses it.
func limitBody(max int) fiber.Handler {
	return func(c fiber.Ctx) error {
		if n := c.Request().Header.ContentLength(); n > max || len(c.Body()) > max {
			return fiber.NewError(fiber.StatusRequestEntityTooLarge, "request body too large")
		}
		return c.Next()
	}
}
