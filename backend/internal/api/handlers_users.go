package api

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/auth"
	"tanulom/internal/db"
)

// Account management. /api/setup creates the first admin and then closes itself;
// everything after that happens here.
//
// Two rules shape this file. Nothing on these routes ever renders a password
// hash — UserDTO has no field for one, and ListUsers does not even select the
// column. And the install must always keep at least one admin: the last one can
// be neither deleted nor demoted, because settings and this screen are the only
// way back and there is none once they are gone. The invariant is enforced in
// internal/db inside the write transaction; these handlers only translate it.

// CreateUserRequest adds an account. Role defaults to learner — an admin has to
// ask for an admin.
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateUserRequest is a partial edit: nil means "leave this alone", which is
// what lets the UI change a role without resending a password it never had.
type UpdateUserRequest struct {
	Role     *string `json:"role"`
	Password *string `json:"password"`
}

// ChangePasswordRequest is a learner changing their own password. The current
// one is required: a borrowed unlocked browser must not become a stolen account.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// UsersDTO is the account list.
type UsersDTO struct {
	Users []UserDTO `json:"users"`
}

func (s *Server) handleListUsers(c fiber.Ctx) error {
	users, err := s.db.ListUsers()
	if err != nil {
		return err
	}
	out := make([]UserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, userDTO(u))
	}
	return c.JSON(UsersDTO{Users: out})
}

func (s *Server) handleCreateUser(c fiber.Ctx) error {
	var req CreateUserRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		return badRequest("%s", err.Error())
	}
	role := req.Role
	if role == "" {
		role = db.RoleLearner
	}
	if !db.ValidRole(role) {
		return badRequest("role must be %q or %q", db.RoleAdmin, db.RoleLearner)
	}
	// Same strength floor as the setup wizard: HashPassword validates before it
	// hashes, so there is no path that stores a password weaker than the first
	// admin's.
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return badRequest("%s", err.Error())
	}

	id, err := s.db.CreateUser(email, hash, role)
	if err != nil {
		if errors.Is(err, db.ErrDuplicateEmail) {
			return fiber.NewError(fiber.StatusConflict, "that email address is already in use")
		}
		return err
	}
	u, err := s.db.UserByID(id)
	if err != nil {
		return err
	}
	slog.InfoContext(c.Context(), "user created", "user_id", id, "email", email, "role", role)
	return c.Status(fiber.StatusCreated).JSON(userDTO(u))
}

func (s *Server) handleUpdateUser(c fiber.Ctx) error {
	actor, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("user id must be a number")
	}
	var req UpdateUserRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if _, err := s.db.UserByID(id); err != nil {
		return err // ErrNotFound renders as 404
	}

	// Validate and hash before writing anything. An edit carrying both a role and
	// a password the strength floor rejects must not land the role change and then
	// fail: the settings endpoint refuses to apply partially for the same reason.
	var hash string
	if req.Role != nil && !db.ValidRole(*req.Role) {
		return badRequest("role must be %q or %q", db.RoleAdmin, db.RoleLearner)
	}
	if req.Password != nil {
		if hash, err = auth.HashPassword(*req.Password); err != nil {
			return badRequest("%s", err.Error())
		}
	}

	if req.Role != nil {
		if err := s.db.SetUserRole(id, *req.Role); err != nil {
			return lastAdminError(err)
		}
		slog.InfoContext(c.Context(), "user role changed",
			"user_id", id, "role", *req.Role, "by", actor.ID)
	}

	if req.Password != nil {
		if err := s.db.SetUserPassword(id, hash); err != nil {
			return err
		}
		// An admin resetting someone else's password is normally a response to
		// that account being compromised, so every session it has must die. When
		// the admin is editing themselves, the browser in front of them survives —
		// same rule as /api/me/password.
		keep := ""
		if id == actor.ID {
			keep = auth.HashToken(c.Cookies(sessionCookie))
		}
		if n, err := s.db.DeleteOtherSessions(id, keep); err != nil {
			slog.WarnContext(c.Context(), "could not end sessions after password reset", "error", err)
		} else {
			slog.InfoContext(c.Context(), "password reset", "user_id", id, "sessions_ended", n, "by", actor.ID)
		}
	}

	u, err := s.db.UserByID(id)
	if err != nil {
		return err
	}
	return c.JSON(userDTO(u))
}

func (s *Server) handleDeleteUser(c fiber.Ctx) error {
	actor, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("user id must be a number")
	}

	if err := s.db.DeleteUser(id); err != nil {
		return lastAdminError(err)
	}
	slog.InfoContext(c.Context(), "user deleted", "user_id", id, "by", actor.ID)

	// An admin who deleted their own account still holds a cookie for a session
	// that no longer exists. Clearing it here turns the next request into a login
	// screen instead of a 401 the SPA has to discover.
	if id == actor.ID {
		s.clearSessionCookie(c)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleChangeOwnPassword is the one account route open to every signed-in user.
func (s *Server) handleChangeOwnPassword(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req ChangePasswordRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if err := auth.CheckPassword(u.PasswordHash, req.CurrentPassword); err != nil {
		// 403, not 401: the session is perfectly valid and the client must not
		// conclude it has expired and throw the user back to the login screen.
		return fiber.NewError(fiber.StatusForbidden, "current password is incorrect")
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return badRequest("%s", err.Error())
	}
	if err := s.db.SetUserPassword(u.ID, hash); err != nil {
		return err
	}

	// Every other session dies; this one does not. Changing a password is how you
	// evict a device you no longer control, and being signed out of the browser
	// you just typed the new password into would be a bug.
	n, err := s.db.DeleteOtherSessions(u.ID, auth.HashToken(c.Cookies(sessionCookie)))
	if err != nil {
		slog.WarnContext(c.Context(), "could not end other sessions", "error", err)
	}
	slog.InfoContext(c.Context(), "password changed", "user_id", u.ID, "sessions_ended", n)
	return c.SendStatus(fiber.StatusNoContent)
}

// lastAdminError turns the database's refusal into the 409 both the demote and
// the delete path answer with. The message names the way out, because "conflict"
// on its own tells an admin nothing about what to do next.
func lastAdminError(err error) error {
	if errors.Is(err, db.ErrLastAdmin) {
		return fiber.NewError(fiber.StatusConflict,
			"this is the only administrator; promote someone else first")
	}
	return err
}
