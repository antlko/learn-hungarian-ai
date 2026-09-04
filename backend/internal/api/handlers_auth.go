package api

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/auth"
	"tanulom/internal/config"
	"tanulom/internal/db"
	"tanulom/internal/lang"
)

// handleBootstrap tells the SPA what state the install is in. Unauthenticated by
// design: it is what the login/setup screen needs before anyone can sign in.
func (s *Server) handleBootstrap(c fiber.Ctx) error {
	n, err := s.db.CountUsers()
	if err != nil {
		return err
	}
	version, present := lang.Detect(s.dataDir)
	// Read off the loaded pack rather than reopening it: this endpoint is
	// unauthenticated and hit on every cold load of the SPA.
	edition := ""
	if p := s.Pack(); p != nil {
		edition = p.Meta.Edition
	}
	return c.JSON(BootstrapDTO{
		NeedsSetup:  n == 0,
		PackPresent: present,
		PackVersion: version,
		PackEdition: edition,
		Locales:     []string{"ru", "en"},
		Version:     Version,
		MinPassword: auth.MinPasswordLength,
	})
}

// handleSetup creates the first admin account. It is a one-shot endpoint: once
// any user exists it returns 409, so it cannot be used to add accounts.
func (s *Server) handleSetup(c fiber.Ctx) error {
	var req SetupRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	n, err := s.db.CountUsers()
	if err != nil {
		return err
	}
	if n > 0 {
		return fiber.NewError(fiber.StatusConflict, "setup has already been completed")
	}

	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		return badRequest("%s", err.Error())
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return badRequest("%s", err.Error())
	}

	uid, err := s.db.CreateUser(email, hash, db.RoleAdmin)
	if err != nil {
		return err
	}

	// Locale choices from the wizard become the initial config. Invalid values
	// are ignored rather than fatal — the wizard should never send them, and a
	// failed setup is a worse outcome than a default locale.
	if _, err := s.updateConfig(func(cfg *config.Config) {
		if req.UILocale == "ru" || req.UILocale == "en" {
			cfg.Locale.UI = req.UILocale
		}
		switch req.GlossLang {
		case "ru", "en", "both":
			cfg.Locale.Gloss = req.GlossLang
		}
		cfg.Locale.GlossFallback = true
	}); err != nil {
		slog.WarnContext(c.Context(), "setup: could not persist locale choice", "error", err)
	}

	slog.InfoContext(c.Context(), "setup completed", "user_id", uid, "email", email)
	return s.startSession(c, uid, fiber.StatusCreated)
}

// handleLogin authenticates and starts a session.
func (s *Server) handleLogin(c fiber.Ctx) error {
	var req LoginRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		// Do not echo the parse error: an unknown address and a malformed one
		// must look the same from outside.
		return fiber.NewError(fiber.StatusUnauthorized, "invalid email or password")
	}

	u, err := s.db.UserByEmail(email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			// Spend comparable time on a missing account so response timing does
			// not enumerate valid addresses.
			_ = auth.CheckPassword("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvalidin", req.Password)
			return fiber.NewError(fiber.StatusUnauthorized, "invalid email or password")
		}
		return err
	}
	if err := auth.CheckPassword(u.PasswordHash, req.Password); err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid email or password")
	}
	return s.startSession(c, u.ID, fiber.StatusOK)
}

// handleLogout ends the current session.
func (s *Server) handleLogout(c fiber.Ctx) error {
	if token := c.Cookies(sessionCookie); token != "" {
		if err := s.db.DeleteSession(auth.HashToken(token)); err != nil {
			slog.WarnContext(c.Context(), "logout: could not delete session", "error", err)
		}
	}
	s.clearSessionCookie(c)
	return c.SendStatus(fiber.StatusNoContent)
}

// handleMe returns the signed-in user plus the settings the UI needs at boot.
func (s *Server) handleMe(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	secrets, err := s.secretPresence(u.ID)
	if err != nil {
		return err
	}
	return c.JSON(MeDTO{
		User:     userDTO(u),
		Settings: settingsDTO(s.Config(), secrets),
	})
}

// startSession mints a token, stores its hash, and sets the cookie.
func (s *Server) startSession(c fiber.Ctx, userID int64, status int) error {
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	expires := auth.SessionExpiry()
	if err := s.db.CreateSession(hash, userID, expires, c.Get("User-Agent")); err != nil {
		return err
	}

	u, err := s.db.UserByID(userID)
	if err != nil {
		return err
	}
	s.setSessionCookie(c, token, expires)

	secrets, err := s.secretPresence(userID)
	if err != nil {
		return err
	}
	return c.Status(status).JSON(MeDTO{
		User:     userDTO(u),
		Settings: settingsDTO(s.Config(), secrets),
	})
}

func (s *Server) setSessionCookie(c fiber.Ctx, token string, expires time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
		// Secure only over TLS: forcing it on plain HTTP would break local
		// development and bare-IP LAN installs, where the cookie would be dropped.
		Secure: c.Protocol() == "https",
	})
}

func (s *Server) clearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}
