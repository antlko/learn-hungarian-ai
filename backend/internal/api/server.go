// Package api owns the HTTP server: the Fiber app, middleware, handlers, and the
// copy-on-write path for config changes.
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"

	"tanulom/internal/config"
	"tanulom/internal/course"
	"tanulom/internal/curriculum"
	"tanulom/internal/db"
	"tanulom/internal/drill"
	"tanulom/internal/hvpt"
	"tanulom/internal/lang"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
	"tanulom/internal/tutor"
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

	// The language pack is installed from the binary on first boot, so it is
	// present in every normal case. It is still nil-able: if the embedded pack
	// cannot be written or opened, the app boots and the features that need it
	// report unavailable, the same way an absent sidecar does — a genuine
	// failure path, not the first-run state. See [ADR-010].
	pack     *lang.Pack
	analyzer *morph.Analyzer
	dag      *curriculum.DAG
	drills   *drill.Generator

	// pending holds drill items that have been served and not yet answered. In
	// memory on purpose — see internal/drill/pending.go.
	pending *drill.Pending

	// cribFlights guards one background translation flight per learner, and bg
	// tracks every detached goroutine so Close can wait for it: a goroutine
	// still writing to s.db after Close fails the race detector.
	//
	// bgCtx is cancelled FIRST, so Close does not sit through a translation
	// that may legitimately take minutes — waiting without cancelling would
	// turn every shutdown into a two-minute hang. See handlers_crib.go.
	cribFlights sync.Map
	bg          sync.WaitGroup
	bgCtx       context.Context
	bgCancel    context.CancelFunc

	// The guided syllabus: the pack's answer to "what do I do today".
	syllabus *course.Syllabus

	// The ear trainer's material and its own served-trial store.
	hvptInv     *hvpt.Inventory
	hvptPending *hvpt.Pending

	// The grammar tutor's reference corpus, embedded in the binary, and the
	// one-question-in-flight guard per learner. A corpus that fails to load
	// leaves the tutor unavailable with a reason; nothing else is affected.
	// ADR-027.
	tutorKB      *tutor.KB
	tutorFlights sync.Map
}

// New wires a server against a data directory, creating it if needed.
func New(dataDir string) (*Server, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	// media only. `backups/` used to be created here for the staged restore that
	// went with internal/export ([ADR-011]); nothing has written to it since, and
	// an empty directory named `backups` in the learner's data dir is a promise
	// the app does not keep. A backup is a tar of this whole directory.
	if err := os.MkdirAll(filepath.Join(dataDir, "media"), 0o755); err != nil {
		return nil, fmt.Errorf("create media dir: %w", err)
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

	s := &Server{dataDir: dataDir, cfg: cfg, db: database,
		pending: drill.NewPending(), hvptPending: hvpt.NewPending()}
	s.bgCtx, s.bgCancel = context.WithCancel(context.Background())
	s.reconcileStepsOnce()
	s.backfillNoteOriginsOnce()
	s.loadPack()
	if kb, err := tutor.LoadKB(); err != nil {
		slog.Error("tutor corpus unavailable", "error", err)
	} else {
		s.tutorKB = kb
	}
	s.app = s.buildApp()
	return s, nil
}

// reconcileStepsOnce runs the one-time replay repair behind migration 00011.
//
// It lives here and not in the migration because replay needs the configured
// desiredRetention and gate window, and migrations run inside db.Open before
// any config has been read. Marker-gated in app_meta; on failure the marker is
// NOT set, the server boots anyway (stored state is stale but self-consistent),
// and the next start retries. Replay is deterministic, so a re-run after a
// half-recorded success changes nothing. See ADR-020.
func (s *Server) reconcileStepsOnce() {
	cfg := s.Config()
	sched := srs.NewScheduler(cfg.Review.DesiredRetention)

	if _, done, err := s.db.Meta("steps_reconciled"); err == nil && !done {
		cards, features, err := srs.ReconcileSteps(s.db.DB, sched, cfg.Drill.GateWindow)
		if err != nil {
			slog.Error("FSRS steps reconciliation failed; will retry next start", "error", err)
		} else {
			slog.Info("FSRS steps reconciled", "cards_changed", cards, "features_changed", features)
			if err := s.db.SetMeta("steps_reconciled", "v1"); err != nil {
				slog.Error("could not record steps reconciliation", "error", err)
			}
		}
	}

	if _, done, err := s.db.Meta("hvpt_steps_reconciled"); err == nil && !done {
		contrasts, err := hvpt.ReconcileSteps(s.db.DB, sched, cfg.Drill.GateWindow)
		if err != nil {
			slog.Error("HVPT steps reconciliation failed; will retry next start", "error", err)
		} else {
			slog.Info("HVPT steps reconciled", "contrasts_changed", contrasts)
			if err := s.db.SetMeta("hvpt_steps_reconciled", "v1"); err != nil {
				slog.Error("could not record hvpt steps reconciliation", "error", err)
			}
		}
	}
}

// loadPack installs the embedded starter pack if the data directory has no
// usable one, then opens it. A pack that cannot be installed or opened is
// logged and the server starts anyway: there is nowhere better to report it
// than a running UI, and refusing to boot would take the report down with it.
func (s *Server) loadPack() {
	// The binary carries a starter pack, so "no language pack" is a state the
	// learner should never reach: on a fresh data directory — or one holding a
	// truncated download — we install the embedded pack and carry on. Failing
	// that, the app still boots and every language feature reports unavailable,
	// the same way an absent sidecar does.
	version, installed, err := lang.EnsurePack(s.dataDir)
	if err != nil {
		slog.Error("no usable language pack and the starter could not be installed",
			"data_dir", s.dataDir, "error", err)
		return
	}
	if installed {
		slog.Info("starter language pack installed", "version", version)
	}
	p, err := lang.Open(lang.Path(s.dataDir, version))
	if err != nil {
		slog.Error("language pack could not be opened", "version", version, "error", err)
		return
	}
	s.pack = p
	s.analyzer = morph.NewAnalyzer(p)

	// A pack whose curriculum DAG is broken still serves reading and review;
	// only the grammar screen goes unavailable, and it says why. Refusing to
	// boot over it would take out four working loops to protect one.
	dag, err := curriculum.Load(p)
	if err != nil {
		slog.Error("curriculum unavailable", "error", err)
		return
	}
	s.dag = dag
	s.drills = drill.NewGenerator(p, s.analyzer, dag)

	// The HVPT inventory is optional the same way the curriculum is: a pack
	// without audio still serves everything else, and the Ear screen says what
	// is missing instead of erroring.
	// A pack without a course still serves everything else; the plan screen
	// says the pack carries no syllabus rather than the app refusing to boot.
	syl, err := course.Load(p)
	if err != nil {
		slog.Error("course unavailable", "error", err)
	} else {
		s.syllabus = syl
	}

	inv, err := hvpt.LoadInventory(p)
	if err != nil {
		slog.Error("hvpt inventory unavailable", "error", err)
		return
	}
	s.hvptInv = inv
}

// Pack returns the loaded language pack, or nil.
func (s *Server) Pack() *lang.Pack { return s.pack }

// App exposes the Fiber app for listening and for tests.
func (s *Server) App() *fiber.App { return s.app }

// Close releases resources.
func (s *Server) Close() error {
	// Cancel first, then wait: detached work writes to s.db, so closing under
	// it is a race — but waiting on an uncancelled network call is a hang.
	if s.bgCancel != nil {
		s.bgCancel()
	}
	s.bg.Wait()
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
	api.Post("/me/password", s.requireAuth, s.handleChangeOwnPassword)
	api.Get("/settings", s.requireAuth, s.handleGetSettings)
	api.Get("/capabilities", s.requireAuth, s.handleCapabilities)

	// Language and morphology.
	api.Post("/analyze", s.requireAuth, s.handleAnalyze)
	// The learner's own dictionary, for the words the pack does not carry.
	api.Get("/glosses/:form", s.requireAuth, s.handleGetGloss)
	api.Put("/glosses/:form", s.requireAuth, s.handlePutGloss)
	api.Delete("/glosses/:form", s.requireAuth, s.handleDeleteGloss)
	api.Get("/lemma/:id", s.requireAuth, s.handleLemma)
	api.Get("/lemma/:id/examples", s.requireAuth, s.handleLemmaExamples)
	api.Get("/features", s.requireAuth, s.handleFeatures)
	api.Get("/breakdown", s.requireAuth, s.handleBreakdown)
	api.Post("/morph/correction", s.requireAuth, s.handleCreateCorrection)
	api.Delete("/morph/correction", s.requireAuth, s.handleDeleteCorrection)
	api.Get("/morph/corrections", s.requireAuth, s.handleListCorrections)

	// Review.
	api.Get("/review/queue", s.requireAuth, s.handleQueue)
	api.Post("/review/answer", s.requireAuth, s.handleAnswer)
	api.Post("/review/undo", s.requireAuth, s.handleUndo)
	// Free practice is a separate mode, not a second queue: it schedules
	// nothing. Its own routes so that can never be confused. See ADR-019.
	api.Get("/practice/queue", s.requireAuth, s.handlePracticeQueue)
	api.Post("/practice/answer", s.requireAuth, s.handlePracticeAnswer)
	api.Get("/review/forecast", s.requireAuth, s.handleForecast)
	api.Post("/notes", s.requireAuth, s.handleCreateNote)
	// The trash: a word the learner set aside. Not a delete — see
	// handlers_trash.go and ADR-025.
	api.Get("/trash", s.requireAuth, s.handleTrashList)
	api.Post("/notes/:id/trash", s.requireAuth, s.handleTrashNote)
	api.Post("/notes/:id/restore", s.requireAuth, s.handleRestoreNote)
	api.Get("/stats/mastery", s.requireAuth, s.handleMastery)
	// The words browser: «что выучилось» as a readable, searchable list.
	api.Get("/words", s.requireAuth, s.handleWords)

	// Progress.
	api.Get("/stats/overview", s.requireAuth, s.handleOverview)
	api.Post("/checkpoints", s.requireAuth, s.handleCheckpoint)
	api.Post("/today/human", s.requireAuth, s.handleAttestHuman)

	// Grammar and curriculum.
	api.Get("/curriculum", s.requireAuth, s.handleCurriculum)
	api.Get("/curriculum/:code", s.requireAuth, s.handleTopic)
	api.Get("/drill/next", s.requireAuth, s.handleDrillNext)
	api.Post("/drill/answer", s.requireAuth, s.handleDrillAnswer)
	api.Get("/placement", s.requireAuth, s.handlePlacement)
	api.Post("/placement", s.requireAuth, s.handlePlacementAction)
	// The grammar tutor: threads are conversations with an optional pinned
	// topic, the lesson is found per question, answers stream as server-sent
	// events, and nothing it says becomes a card. ADR-027, ADR-029.
	api.Get("/tutor", s.requireAuth, s.handleTutorList)
	api.Post("/tutor", s.requireAuth, s.handleTutorCreate)
	// The lesson panel's route is registered before the id route: Fiber
	// matches in order, and "topic" is not a thread id.
	api.Get("/tutor/new", s.requireAuth, s.handleTutorNew)
	api.Get("/tutor/review", s.requireAuth, s.handleTutorReviewToday)
	api.Get("/tutor/topic/:code", s.requireAuth, s.handleTutorForTopic)
	api.Get("/tutor/:id", s.requireAuth, s.handleTutorThread)
	api.Post("/tutor/:id", s.requireAuth, s.handleTutorAsk)
	api.Post("/tutor/:id/stop", s.requireAuth, s.handleTutorStop)
	api.Delete("/tutor/:id", s.requireAuth, s.handleTutorDelete)

	// Listening.
	api.Post("/listen/material", s.requireAuth, s.handleListenUpload)
	api.Get("/listen/:id/state", s.requireAuth, s.handleListenState)
	api.Post("/listen/:id/progress", s.requireAuth, s.handleListenProgress)

	// The guided course and the hand-off catalogue.
	api.Get("/course", s.requireAuth, s.handleCourse)
	api.Post("/course/:code/enrol", s.requireAuth, s.handleEnrolUnit)
	api.Get("/resources", s.requireAuth, s.handleResources)

	// Ear trainer.
	api.Get("/hvpt/session", s.requireAuth, s.handleHVPTSession)
	api.Post("/hvpt/answer", s.requireAuth, s.handleHVPTAnswer)
	api.Get("/hvpt/contrasts", s.requireAuth, s.handleHVPTContrasts)

	// Reading library.
	api.Get("/texts", s.requireAuth, s.handleListTexts)
	api.Post("/texts", s.requireAuth, s.handleCreateText)
	api.Get("/texts/:id", s.requireAuth, s.handleGetText)
	api.Delete("/texts/:id", s.requireAuth, s.handleDeleteText)
	api.Get("/texts/:id/next", s.requireAuth, s.handleNextSentences)
	// A text's estimated level: the learner's own verdict, or a labelled second
	// opinion from the configured model. ADR-023.
	api.Put("/texts/:id/level", s.requireAuth, s.handleSetLevel)
	api.Post("/texts/:id/level", s.requireAuth, s.handleLLMLevel)
	api.Post("/knowledge/lemma", s.requireAuth, s.handleSetKnown)
	api.Post("/translate", s.requireAuth, s.handleTranslate)
	// The translation sidecar's own state, which the capability probe cannot
	// carry: it flattens the 503 a downloading model returns into "unreachable".
	api.Get("/mt/status", s.requireAuth, s.handleMTStatus)
	api.Post("/mt/warmup", s.requireAuth, s.requireAdmin, s.handleMTWarmup)

	// Admin-only: settings are global (config.yaml), so a learner must not edit them.
	api.Put("/settings", s.requireAuth, s.requireAdmin, s.handleUpdateSettings)

	// Admin-only: account management. Changing your own password is above, on
	// /me/password, because it is the one thing here every learner may do.
	api.Get("/users", s.requireAuth, s.requireAdmin, s.handleListUsers)
	api.Post("/users", s.requireAuth, s.requireAdmin, s.handleCreateUser)
	api.Patch("/users/:id", s.requireAuth, s.requireAdmin, s.handleUpdateUser)
	api.Delete("/users/:id", s.requireAuth, s.requireAdmin, s.handleDeleteUser)

	// Anything else under /api is a 404 in the API's own error shape. Registered
	// last so real routes win, and before the SPA so an unknown endpoint never
	// returns index.html to a fetch() that expected JSON.
	api.Use(func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "no such endpoint")
	})

	// Learner-uploaded and generated media.
	app.Use("/media", static.New(filepath.Join(s.dataDir, "media")))

	// Cache policy for the SPA.
	//
	// Vite fingerprints every asset, so `/assets/*` is immutable and can be
	// cached for a year. `index.html` is NOT fingerprinted — it is the file that
	// names the current asset hashes — so it must never be cached. Getting this
	// wrong means a user who has visited before keeps loading an old index.html
	// after an update, and therefore keeps running the previous build while the
	// server happily serves the new assets nobody asks for.
	app.Use("/", func(c fiber.Ctx) error {
		path := c.Path()
		switch {
		case strings.HasPrefix(path, "/assets/"):
			c.Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasPrefix(path, "/api/"), strings.HasPrefix(path, "/media/"):
			// handled elsewhere
		default:
			c.Set("Cache-Control", "no-cache, must-revalidate")
		}
		return c.Next()
	})

	// SPA: serve the embedded build, falling back to index.html so client-side
	// routes deep-link correctly. Registered last so /api always wins.
	spa := web.FS()
	serveIndex := indexHandler(spa)

	// `/` and `/index.html` are served by hand rather than by the static
	// middleware — see indexHandler for why the middleware cannot do it. Both
	// registered BEFORE the catch-all so they win on exact match.
	// HEAD as well as GET: `app.Get` does not imply HEAD, and without this the
	// two methods take different code paths — HEAD would fall through to the
	// static middleware and answer with the modtime semantics this handler
	// exists to avoid. Browsers navigate with GET, but proxies and health
	// checks probe with HEAD, and a divergence here is a bug waiting for one
	// of them.
	for _, path := range []string{"/", "/index.html"} {
		app.Get(path, serveIndex)
		app.Head(path, serveIndex)
	}

	app.Use("/", static.New("", static.Config{
		FS:         spa,
		IndexNames: []string{"index.html"},
		NotFoundHandler: func(c fiber.Ctx) error {
			// A missing fingerprinted asset is a 404, not the shell. Handing
			// back index.html with a 200 for a `.js` request makes the browser
			// parse HTML as JavaScript: it dies on `<`, and the symptom is a
			// blank page rather than the missing file it actually is.
			if strings.HasPrefix(c.Path(), "/assets/") {
				return fiber.NewError(fiber.StatusNotFound, "no such asset")
			}
			return serveIndex(c)
		},
	}))

	return app
}

// indexHandler serves index.html with CONTENT-based revalidation.
//
// The embedded filesystem has no modification times: every file reports the
// zero instant. Fiber's static middleware does its 304 by comparing that
// against `If-Modified-Since`, so index.html answered "not modified" to every
// conditional request any browser ever made — including requests carrying a
// date decades in the future.
//
// The consequence was a black screen after an update. `index.html` is the one
// file that is NOT fingerprinted; it is the file that names the current asset
// hashes. A browser that had opened the app once kept its first copy forever,
// that copy pointed at asset filenames the new build no longer contains, and
// the app booted into nothing until a hard reload. The `no-cache` header above
// was correct and did not help: no-cache means "revalidate", and revalidation
// was the broken part.
//
// An ETag over the bytes fixes it because the bytes are what actually change.
// The file is read once at startup — it is embedded, it cannot change under a
// running process, and re-reading it per request would buy nothing.
func indexHandler(spa fs.FS) fiber.Handler {
	body, err := fs.ReadFile(spa, "index.html")
	if err != nil {
		// A build without an index is a broken build, and every route below
		// depends on it. Serving a clear error beats serving a blank page.
		slog.Error("embedded SPA has no index.html", "error", err)
		return func(c fiber.Ctx) error {
			return fiber.NewError(fiber.StatusInternalServerError, "the UI build is missing")
		}
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	return func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-cache, must-revalidate")
		c.Set("ETag", etag)
		if c.Get("If-None-Match") == etag {
			return c.SendStatus(fiber.StatusNotModified)
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusOK).Send(body)
	}
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
