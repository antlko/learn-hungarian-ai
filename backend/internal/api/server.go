// Package api owns the HTTP server: the Fiber app, middleware, handlers, and the
// copy-on-write path for config changes.
package api

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"

	"tanulom/internal/config"
	"tanulom/internal/db"
	"tanulom/internal/lang"
	"tanulom/internal/morph"
	"tanulom/internal/web"
)

// Version is the application version, set at build time via -ldflags.
var Version = "dev"

// Server owns the config and database and applies config edits copy-on-write
// under a lock.
type Server struct {
	dataDir string

	mu  sync.RWMutex // guards cfg
	cfg config.Config

	db  *db.DB
	app *fiber.App

	// The language pack is optional at startup: the app must boot and let the
	// user download one rather than refusing to start. Features that need it
	// report unavailable, the same way an absent sidecar does.
	pack     *lang.Pack
	analyzer *morph.Analyzer
}

// New wires a server against a data directory, creating it if needed.
func New(dataDir string) (*Server, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	for _, sub := range []string{"media", "backups"} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("create %s dir: %w", sub, err)
		}
	}

	cfg, err := config.Load(dataDir)
	if err != nil {
		return nil, err
	}
	database, err := db.Open(db.Path(dataDir))
	if err != nil {
		return nil, err
	}
	if n, err := database.PurgeExpiredSessions(); err == nil && n > 0 {
		slog.Info("purged expired sessions", "count", n)
	}

	s := &Server{dataDir: dataDir, cfg: cfg, db: database}
	s.loadPack()
	s.app = s.buildApp()
	return s, nil
}

// loadPack opens the language pack if one is present. A missing or unusable pack
// is logged and the server starts anyway — the setup wizard exists precisely to
// let a user fetch one, and refusing to boot would leave them nowhere to do it.
func (s *Server) loadPack() {
	present, version := lang.Detect(s.dataDir)
	if !present {
		slog.Info("no language pack found", "data_dir", s.dataDir)
		return
	}
	p, err := lang.Open(lang.Path(s.dataDir, version))
	if err != nil {
		slog.Error("language pack could not be opened", "version", version, "error", err)
		return
	}
	s.pack = p
	s.analyzer = morph.NewAnalyzer(p)
}

// Pack returns the loaded language pack, or nil.
func (s *Server) Pack() *lang.Pack { return s.pack }

// App exposes the Fiber app for listening and for tests.
func (s *Server) App() *fiber.App { return s.app }

// Close releases resources.
func (s *Server) Close() error {
	if s.pack != nil {
		_ = s.pack.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Config returns a snapshot of the running config. The returned value is a deep
// copy, so callers cannot mutate the live one by accident.
func (s *Server) Config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Clone()
}

// updateConfig is the ONLY sanctioned way to change config.yaml:
// clone -> mutate -> validate -> atomic save -> swap, all under the write lock.
// Never assign to s.cfg anywhere else.
func (s *Server) updateConfig(mutate func(*config.Config)) ([]config.ErrClamped, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.cfg.Clone()
	mutate(&next)

	clamped, err := next.Validate()
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := config.Save(s.dataDir, next); err != nil {
		return nil, err
	}
	s.cfg = next
	return clamped, nil
}

func (s *Server) buildApp() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "tanulom",
		ErrorHandler: errorHandler,
		BodyLimit:    64 * 1024 * 1024, // EPUB uploads and audio recordings
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	})

	app.Use(recover.New())
	app.Use(requestLogger)

	api := app.Group("/api")

	// Unauthenticated.
	api.Get("/bootstrap", s.handleBootstrap)

	// Auth routes are rate-limited per IP: they are the only endpoints where
	// guessing is useful to an attacker.
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		LimitReached: func(c fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many attempts, try again shortly")
		},
	})
	api.Post("/setup", authLimiter, s.handleSetup)
	api.Post("/login", authLimiter, s.handleLogin)
	api.Post("/logout", s.handleLogout)

	// Authenticated. Middleware is attached per route rather than to the /api
	// group: a group-level Use would match every path under /api, so a typo'd
	// endpoint would answer 401 instead of 404 and the API 404 below would be
	// unreachable.
	//
	// Fiber v3 runs route handlers in argument order, so middleware goes FIRST.
	// Putting it last silently makes it run after the handler — which for a rate
	// limiter means no limiting at all.
	api.Get("/me", s.requireAuth, s.handleMe)
	api.Get("/settings", s.requireAuth, s.handleGetSettings)
	api.Get("/capabilities", s.requireAuth, s.handleCapabilities)

	// Language and morphology.
	api.Post("/analyze", s.requireAuth, s.handleAnalyze)
	api.Get("/lemma/:id", s.requireAuth, s.handleLemma)
	api.Get("/features", s.requireAuth, s.handleFeatures)

	// Admin-only: settings are global (config.yaml), so a learner must not edit them.
	api.Put("/settings", s.requireAuth, s.requireAdmin, s.handleUpdateSettings)

	// Anything else under /api is a 404 in the API's own error shape. Registered
	// last so real routes win, and before the SPA so an unknown endpoint never
	// returns index.html to a fetch() that expected JSON.
	api.Use(func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "no such endpoint")
	})

	// Learner-uploaded and generated media.
	app.Use("/media", static.New(filepath.Join(s.dataDir, "media")))

	// SPA: serve the embedded build, falling back to index.html so client-side
	// routes deep-link correctly. Registered last so /api always wins.
	spa := web.FS()
	app.Use("/", static.New("", static.Config{
		FS:         spa,
		IndexNames: []string{"index.html"},
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusOK).SendFile("index.html", fiber.SendFile{FS: spa})
		},
	}))

	return app
}

// requestLogger emits one structured line per request.
func requestLogger(c fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	slog.DebugContext(c.Context(), "request",
		"method", c.Method(),
		"path", c.Path(),
		"status", c.Response().StatusCode(),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return err
}
