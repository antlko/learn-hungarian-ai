package api

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/auth"
	"tanulom/internal/db"
)

// sessionCookie is the cookie carrying the session token.
const sessionCookie = "tanulom_session"

// localsUser is the Locals key holding the authenticated user.
const localsUser = "user"

// requireAuth rejects unauthenticated requests and stashes the user in Locals.
func (s *Server) requireAuth(c fiber.Ctx) error {
	token := c.Cookies(sessionCookie)
	if token == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	u, err := s.db.UserBySession(auth.HashToken(token))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			// Expired or unknown: clear the stale cookie so the client stops sending it.
			s.clearSessionCookie(c)
			return fiber.NewError(fiber.StatusUnauthorized, "session expired")
		}
		return err
	}
	c.Locals(localsUser, u)
	return c.Next()
}

// requireAdmin gates admin-only routes. It runs after requireAuth.
//
// Router guards in the SPA are UX only — this is the actual boundary.
func (s *Server) requireAdmin(c fiber.Ctx) error {
	u, ok := userLocal(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	if u.Role != db.RoleAdmin {
		return fiber.NewError(fiber.StatusForbidden, "administrator access required")
	}
	return c.Next()
}

// userLocal reads the authenticated user stashed by requireAuth.
func userLocal(c fiber.Ctx) (db.User, bool) {
	u, ok := c.Locals(localsUser).(db.User)
	return u, ok
}

// mustUser reads the authenticated user, panicking is not an option — routes
// using it are always behind requireAuth, but we fail closed if that changes.
func mustUser(c fiber.Ctx) (db.User, error) {
	u, ok := userLocal(c)
	if !ok {
		return db.User{}, fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	return u, nil
}
