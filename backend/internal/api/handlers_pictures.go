package api

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/pictures"
)

// handlePicture serves one embedded picture (GET /api/pictures/:id.svg). The
// SVG is static, so it is cached for a year and tagged by content; the strict
// CSP and nosniff keep it inert even if opened directly in a tab.
func (s *Server) handlePicture(c fiber.Ctx) error {
	id, ok := strings.CutSuffix(c.Params("file"), ".svg")
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such picture")
	}
	cat, err := pictures.Default()
	if err != nil {
		return err
	}
	body, found := cat.SVG(id)
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no such picture")
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	c.Set("ETag", etag)
	c.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	c.Set("X-Content-Type-Options", "nosniff")
	if c.Get("If-None-Match") == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Set("Content-Type", "image/svg+xml")
	return c.Status(fiber.StatusOK).Send(body)
}
