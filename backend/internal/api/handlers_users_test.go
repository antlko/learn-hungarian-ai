package api

import (
	"bytes"
	"net/http"
	"strconv"
	"testing"
)

// newAdmin runs setup and returns an authenticated admin client.
func newAdmin(t *testing.T) (*Server, *client) {
	t.Helper()
	s, c := newTestServer(t)
	if resp := c.json(http.MethodPost, "/api/setup",
		SetupRequest{Email: "admin@example.com", Password: goodPassword}, nil); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: status %d", resp.StatusCode)
	}
	return s, c
}

// addUser creates an account through the API and returns it.
func addUser(t *testing.T, c *client, email, password, role string) UserDTO {
	t.Helper()
	var u UserDTO
	resp := c.json(http.MethodPost, "/api/users",
		CreateUserRequest{Email: email, Password: password, Role: role}, &u)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create %s: want 201, got %d", email, resp.StatusCode)
	}
	return u
}

// signIn returns a second client with its own session for an existing account.
func signIn(t *testing.T, s *Server, email, password string) *client {
	t.Helper()
	c := &client{t: t, app: s}
	if resp := c.json(http.MethodPost, "/api/login",
		LoginRequest{Email: email, Password: password}, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("login %s: got %d", email, resp.StatusCode)
	}
	return c
}

func listUsers(t *testing.T, c *client) []UserDTO {
	t.Helper()
	var got UsersDTO
	if resp := c.json(http.MethodGet, "/api/users", nil, &got); resp.StatusCode != http.StatusOK {
		t.Fatalf("list users: got %d", resp.StatusCode)
	}
	return got.Users
}

func TestUserLifecycle(t *testing.T) {
	t.Parallel()
	_, admin := newAdmin(t)

	users := listUsers(t, admin)
	if len(users) != 1 || users[0].Role != "admin" {
		t.Fatalf("fresh install should hold one admin, got %+v", users)
	}

	created := addUser(t, admin, "bea@example.com", goodPassword, "learner")
	if created.Role != "learner" || created.Email != "bea@example.com" || created.ID == 0 {
		t.Fatalf("created user looks wrong: %+v", created)
	}
	if created.CreatedAt == "" {
		t.Error("createdAt should be populated")
	}
	if got := listUsers(t, admin); len(got) != 2 {
		t.Fatalf("want 2 accounts after create, got %d", len(got))
	}

	// Promote, then demote.
	role := "admin"
	var updated UserDTO
	if resp := admin.json(http.MethodPatch, "/api/users/"+itoa(created.ID),
		UpdateUserRequest{Role: &role}, &updated); resp.StatusCode != http.StatusOK {
		t.Fatalf("promote: got %d", resp.StatusCode)
	}
	if updated.Role != "admin" {
		t.Errorf("promote: role = %q", updated.Role)
	}
	role = "learner"
	admin.json(http.MethodPatch, "/api/users/"+itoa(created.ID), UpdateUserRequest{Role: &role}, &updated)
	if updated.Role != "learner" {
		t.Errorf("demote: role = %q", updated.Role)
	}

	if resp, _ := admin.do(http.MethodDelete, "/api/users/"+itoa(created.ID), nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: got %d", resp.StatusCode)
	}
	if got := listUsers(t, admin); len(got) != 1 {
		t.Fatalf("want 1 account after delete, got %d", len(got))
	}

	// A missing account is a 404, on both verbs.
	if resp, _ := admin.do(http.MethodDelete, "/api/users/"+itoa(created.ID), nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("delete of a gone account: want 404, got %d", resp.StatusCode)
	}
	if resp, _ := admin.do(http.MethodPatch, "/api/users/"+itoa(created.ID), UpdateUserRequest{Role: &role}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("patch of a gone account: want 404, got %d", resp.StatusCode)
	}
}

// A password hash must never reach the wire — not in a list, not in the object
// returned by the write that stored it.
func TestPasswordHashNeverLeavesTheServer(t *testing.T) {
	t.Parallel()
	_, admin := newAdmin(t)

	_, createBody := admin.do(http.MethodPost, "/api/users",
		CreateUserRequest{Email: "bea@example.com", Password: goodPassword, Role: "learner"})
	_, listBody := admin.do(http.MethodGet, "/api/users", nil)
	_, meBody := admin.do(http.MethodGet, "/api/me", nil)

	for name, body := range map[string][]byte{
		"create": createBody, "list": listBody, "me": meBody,
	} {
		for _, marker := range []string{"$2a$", "$2b$", "passwordHash", "password_hash"} {
			if bytes.Contains(body, []byte(marker)) {
				t.Errorf("%s response leaks a password hash (%q): %s", name, marker, body)
			}
		}
	}
}

func TestCreateUserRejectsBadInput(t *testing.T) {
	t.Parallel()
	_, admin := newAdmin(t)

	cases := []struct {
		name string
		req  CreateUserRequest
		want int
	}{
		{"short password", CreateUserRequest{Email: "b@example.com", Password: "short", Role: "learner"}, http.StatusBadRequest},
		{"no password", CreateUserRequest{Email: "b@example.com", Role: "learner"}, http.StatusBadRequest},
		{"bad email", CreateUserRequest{Email: "not-an-email", Password: goodPassword}, http.StatusBadRequest},
		{"empty email", CreateUserRequest{Password: goodPassword}, http.StatusBadRequest},
		{"unknown role", CreateUserRequest{Email: "b@example.com", Password: goodPassword, Role: "superuser"}, http.StatusBadRequest},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := admin.do(http.MethodPost, "/api/users", tt.req)
			if resp.StatusCode != tt.want {
				t.Fatalf("want %d, got %d (%s)", tt.want, resp.StatusCode, body)
			}
		})
	}

	// Omitting the role is not an error: it means learner, the safe default.
	if u := addUser(t, admin, "carla@example.com", goodPassword, ""); u.Role != "learner" {
		t.Errorf("default role = %q, want learner", u.Role)
	}
}

// The uniqueness index is COLLATE NOCASE, so a differently-cased duplicate hits
// the constraint. That has to surface as a 409, not as the 500 an unmapped
// constraint error would produce.
func TestDuplicateEmailIsConflictNotServerError(t *testing.T) {
	t.Parallel()
	_, admin := newAdmin(t)
	addUser(t, admin, "bea@example.com", goodPassword, "learner")

	resp, body := admin.do(http.MethodPost, "/api/users",
		CreateUserRequest{Email: "BEA@Example.com", Password: goodPassword, Role: "learner"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate email: want 409, got %d (%s)", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("already in use")) {
		t.Errorf("409 should say what went wrong, got %s", body)
	}
	if got := listUsers(t, admin); len(got) != 2 {
		t.Errorf("rejected create should not have added an account: %d users", len(got))
	}
}

// Losing the last admin locks everyone out of settings and of this screen, with
// no way back short of editing the database by hand.
func TestLastAdminCannotBeDemotedOrDeleted(t *testing.T) {
	t.Parallel()
	_, admin := newAdmin(t)
	me := listUsers(t, admin)[0]

	learner := "learner"
	resp, body := admin.do(http.MethodPatch, "/api/users/"+itoa(me.ID), UpdateUserRequest{Role: &learner})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("demoting the only admin: want 409, got %d (%s)", resp.StatusCode, body)
	}
	resp, body = admin.do(http.MethodDelete, "/api/users/"+itoa(me.ID), nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("deleting the only admin: want 409, got %d (%s)", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("only administrator")) {
		t.Errorf("409 should explain itself, got %s", body)
	}

	// The refusal must be a rollback, not a partial write.
	users := listUsers(t, admin)
	if len(users) != 1 || users[0].Role != "admin" {
		t.Fatalf("refused change leaked through: %+v", users)
	}

	// A second admin lifts the restriction — the rule is "at least one", not
	// "this one".
	addUser(t, admin, "bea@example.com", goodPassword, "admin")
	if resp, _ := admin.do(http.MethodPatch, "/api/users/"+itoa(me.ID), UpdateUserRequest{Role: &learner}); resp.StatusCode != http.StatusOK {
		t.Fatalf("demotion with a second admin present: got %d", resp.StatusCode)
	}

	// And having demoted themselves, they are no longer an admin anywhere.
	if resp, _ := admin.do(http.MethodGet, "/api/users", nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("demoted admin should lose access: got %d", resp.StatusCode)
	}
}

// A rejected half must reject the whole edit: landing the role change and then
// failing on the password would leave the caller unable to tell what applied.
func TestUpdateUserDoesNotApplyPartially(t *testing.T) {
	t.Parallel()
	s, admin := newAdmin(t)
	bea := addUser(t, admin, "bea@example.com", goodPassword, "learner")

	role, weak := "admin", "short"
	resp, body := admin.do(http.MethodPatch, "/api/users/"+itoa(bea.ID),
		UpdateUserRequest{Role: &role, Password: &weak})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", resp.StatusCode, body)
	}
	for _, u := range listUsers(t, admin) {
		if u.ID == bea.ID && u.Role != "learner" {
			t.Errorf("the role change was applied despite the rejection: %+v", u)
		}
	}
	// The old password must still be the one that works.
	signIn(t, s, "bea@example.com", goodPassword)
}

// Deleting an account must take its learner data and its live sessions with it.
func TestDeleteUserCascadesDataAndSessions(t *testing.T) {
	t.Parallel()
	s, admin := newAdmin(t)
	bea := addUser(t, admin, "bea@example.com", goodPassword, "learner")
	beaClient := signIn(t, s, "bea@example.com", goodPassword)

	// Rows in both state spaces plus a note, written directly: the point under
	// test is the schema's ON DELETE CASCADE, not the handlers that fill them.
	mustExec(t, s, `INSERT INTO lemma_state (user_id, lemma_id) VALUES (?, 1)`, bea.ID)
	mustExec(t, s, `INSERT INTO morph_state (user_id, feature_id, due) VALUES (?, 'Case=Ine', '')`, bea.ID)
	mustExec(t, s, `INSERT INTO note (user_id, kind, hu) VALUES (?, 'sentence', 'A ház nagy.')`, bea.ID)

	if resp, _ := admin.do(http.MethodDelete, "/api/users/"+itoa(bea.ID), nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: got %d", resp.StatusCode)
	}

	for _, table := range []string{"lemma_state", "morph_state", "note", "session"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, bea.ID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s still holds %d row(s) for the deleted user", table, n)
		}
	}

	// The cookie the deleted user is still holding must be dead immediately.
	if resp, _ := beaClient.do(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("deleted user's session still works: got %d", resp.StatusCode)
	}
}

// Route guards in the SPA are UX only; this is the boundary that matters.
func TestLearnerIsForbiddenOnEveryAdminRoute(t *testing.T) {
	t.Parallel()
	s, admin := newAdmin(t)
	bea := addUser(t, admin, "bea@example.com", goodPassword, "learner")
	beaClient := signIn(t, s, "bea@example.com", goodPassword)

	role := "admin"
	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/users", nil},
		{http.MethodPost, "/api/users", CreateUserRequest{Email: "x@example.com", Password: goodPassword}},
		{http.MethodPatch, "/api/users/" + itoa(bea.ID), UpdateUserRequest{Role: &role}},
		{http.MethodDelete, "/api/users/1", nil},
	}
	for _, tt := range cases {
		resp, body := beaClient.do(tt.method, tt.path, tt.body)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s as learner: want 403, got %d (%s)", tt.method, tt.path, resp.StatusCode, body)
		}
	}

	// None of the rejected calls may have had an effect.
	users := listUsers(t, admin)
	if len(users) != 2 {
		t.Fatalf("forbidden calls changed the account list: %+v", users)
	}
	for _, u := range users {
		if u.Email == "bea@example.com" && u.Role != "learner" {
			t.Errorf("forbidden PATCH promoted the caller: %+v", u)
		}
	}
}

func TestChangeOwnPassword(t *testing.T) {
	t.Parallel()
	s, admin := newAdmin(t)
	addUser(t, admin, "bea@example.com", goodPassword, "learner")

	const newPassword = "másik hosszú jelszó"
	phone := signIn(t, s, "bea@example.com", goodPassword)
	laptop := signIn(t, s, "bea@example.com", goodPassword)

	// The current password is required, and getting it wrong is not a dead session.
	resp, _ := laptop.do(http.MethodPost, "/api/me/password",
		ChangePasswordRequest{CurrentPassword: "not the password", NewPassword: newPassword})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong current password: want 403, got %d", resp.StatusCode)
	}
	if resp, _ := laptop.do(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("a rejected password change must not end the session: got %d", resp.StatusCode)
	}

	// The new password faces the same strength floor as every other.
	resp, _ = laptop.do(http.MethodPost, "/api/me/password",
		ChangePasswordRequest{CurrentPassword: goodPassword, NewPassword: "short"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("weak new password: want 400, got %d", resp.StatusCode)
	}

	if resp, _ := laptop.do(http.MethodPost, "/api/me/password",
		ChangePasswordRequest{CurrentPassword: goodPassword, NewPassword: newPassword}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("password change: got %d", resp.StatusCode)
	}

	// The browser it was typed into keeps working; every other session dies.
	if resp, _ := laptop.do(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("current session should survive its own password change: got %d", resp.StatusCode)
	}
	if resp, _ := phone.do(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("other sessions should be evicted: got %d", resp.StatusCode)
	}

	fresh := &client{t: t, app: s}
	if resp, _ := fresh.do(http.MethodPost, "/api/login",
		LoginRequest{Email: "bea@example.com", Password: goodPassword}); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("old password still works: got %d", resp.StatusCode)
	}
	if resp, _ := fresh.do(http.MethodPost, "/api/login",
		LoginRequest{Email: "bea@example.com", Password: newPassword}); resp.StatusCode != http.StatusOK {
		t.Errorf("new password rejected: got %d", resp.StatusCode)
	}
}

// An admin reset is the answer to a compromised account, so it has to evict the
// account everywhere — while leaving the admin's own session alone.
func TestAdminPasswordResetEndsThatUsersSessions(t *testing.T) {
	t.Parallel()
	s, admin := newAdmin(t)
	bea := addUser(t, admin, "bea@example.com", goodPassword, "learner")
	beaClient := signIn(t, s, "bea@example.com", goodPassword)

	const reset = "friss hosszú jelszó"
	pw := reset
	if resp, _ := admin.do(http.MethodPatch, "/api/users/"+itoa(bea.ID), UpdateUserRequest{Password: &pw}); resp.StatusCode != http.StatusOK {
		t.Fatalf("admin reset: got %d", resp.StatusCode)
	}
	if resp, _ := beaClient.do(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("reset user's session should be gone: got %d", resp.StatusCode)
	}
	if resp, _ := admin.do(http.MethodGet, "/api/users", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("the admin's own session must be untouched: got %d", resp.StatusCode)
	}

	fresh := &client{t: t, app: s}
	if resp, _ := fresh.do(http.MethodPost, "/api/login",
		LoginRequest{Email: "bea@example.com", Password: reset}); resp.StatusCode != http.StatusOK {
		t.Errorf("reset password does not sign in: got %d", resp.StatusCode)
	}
}

// An admin changing their own password through the admin route keeps the browser
// they are holding, exactly as /api/me/password does.
func TestAdminChangingOwnPasswordKeepsCurrentSession(t *testing.T) {
	t.Parallel()
	_, admin := newAdmin(t)
	me := listUsers(t, admin)[0]

	pw := "saját hosszú jelszó"
	if resp, _ := admin.do(http.MethodPatch, "/api/users/"+itoa(me.ID), UpdateUserRequest{Password: &pw}); resp.StatusCode != http.StatusOK {
		t.Fatalf("self reset: got %d", resp.StatusCode)
	}
	if resp, _ := admin.do(http.MethodGet, "/api/users", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("admin signed themselves out changing their own password: got %d", resp.StatusCode)
	}
}

func TestUserRoutesRequireAuthentication(t *testing.T) {
	t.Parallel()
	_, c := newTestServer(t)
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "admin@example.com", Password: goodPassword}, nil)
	c.cookie = ""

	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/users"},
		{http.MethodPatch, "/api/users/1"},
		{http.MethodDelete, "/api/users/1"},
		{http.MethodPost, "/api/me/password"},
	} {
		resp, _ := c.do(tt.method, tt.path, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without a session: want 401, got %d", tt.method, tt.path, resp.StatusCode)
		}
	}
}

func mustExec(t *testing.T, s *Server, query string, args ...any) {
	t.Helper()
	if _, err := s.db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
