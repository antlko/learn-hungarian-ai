package api

import (
	"net/http"
	"testing"
)

func TestLLMProfilesCreateActivateEditDelete(t *testing.T) {
	t.Parallel()
	s, c := newAdmin(t)
	post := func(method, path string, body any) (SettingsDTO, int) {
		var st SettingsDTO
		resp := c.json(method, path, body, &st)
		return st, resp.StatusCode
	}

	home, code := post(http.MethodPost, "/api/admin/llm-profiles",
		LLMProfileRequest{Name: " Home PC ", Provider: "local", Endpoint: "http://pc.invalid:11434", Model: "big"})
	if code != 200 || len(home.LLMProfiles) != 1 || home.LLMProfiles[0].Name != "Home PC" {
		t.Fatalf("create: %d %+v", code, home)
	}
	homeID := home.LLMProfiles[0].ID
	if home.LLMActive != homeID || s.service("llm").Model != "big" {
		t.Fatalf("a new profile must become the active tutor: %+v / %+v", home.LLMActive, s.service("llm"))
	}

	laptop, _ := post(http.MethodPost, "/api/admin/llm-profiles",
		LLMProfileRequest{Name: "Laptop", Provider: "local", Endpoint: "http://localhost:11434", Model: "small"})
	laptopID := laptop.LLMActive
	if laptopID == homeID || s.service("llm").Model != "small" {
		t.Fatalf("second create: %+v", laptop)
	}

	if _, code := post(http.MethodPost, "/api/admin/llm-profiles",
		LLMProfileRequest{Name: "laptop", Provider: "off"}); code != 400 {
		t.Fatalf("duplicate name: want 400, got %d", code)
	}

	st, code := post(http.MethodPost, "/api/admin/llm-profiles/"+homeID+"/activate", nil)
	if code != 200 || st.LLMActive != homeID || st.Services["llm"].Model != "big" ||
		st.Services["llm"].Endpoint != "http://pc.invalid:11434" || s.service("llm").Model != "big" {
		t.Fatalf("activate: %d %+v", code, st)
	}

	// An edit through the ordinary settings path detaches the profile name.
	svc := st.Services["llm"]
	svc.Model = "tweaked"
	if resp, body := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{Services: map[string]ServiceDTO{"llm": svc}}); resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	c.json(http.MethodGet, "/api/settings", nil, &st)
	if st.LLMActive != "" {
		t.Fatalf("a hand edit must clear llmActive, got %q", st.LLMActive)
	}
	// ... but re-saving unchanged services keeps it.
	post(http.MethodPost, "/api/admin/llm-profiles/"+homeID+"/activate", nil)
	c.json(http.MethodGet, "/api/settings", nil, &st)
	c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{Services: st.Services})
	c.json(http.MethodGet, "/api/settings", nil, &st)
	if st.LLMActive != homeID {
		t.Fatalf("an unchanged save cleared llmActive: %q", st.LLMActive)
	}

	st, code = post(http.MethodPut, "/api/admin/llm-profiles/"+homeID,
		LLMProfileRequest{Name: "Desk", Provider: "local", Endpoint: "http://pc.invalid:11434", Model: "bigger"})
	if code != 200 || s.service("llm").Model != "bigger" || st.LLMActive != homeID {
		t.Fatalf("editing the active profile must move the tutor: %d %+v", code, st)
	}

	st, code = post(http.MethodDelete, "/api/admin/llm-profiles/"+homeID, nil)
	if code != 200 || len(st.LLMProfiles) != 1 || st.LLMActive != "" || s.service("llm").Model != "bigger" {
		t.Fatalf("delete: %d %+v", code, st)
	}
	if _, code := post(http.MethodPost, "/api/admin/llm-profiles/nope/activate", nil); code != 404 {
		t.Fatalf("unknown id: want 404, got %d", code)
	}
}

func TestLLMProfilesAreAdminOnly(t *testing.T) {
	t.Parallel()
	s, admin := newAdmin(t)
	addUser(t, admin, "bea@example.com", "bea elég hosszú jelszó", "learner")
	learner := signIn(t, s, "bea@example.com", "bea elég hosszú jelszó")
	if resp, _ := learner.do(http.MethodPost, "/api/admin/llm-profiles",
		LLMProfileRequest{Name: "x", Provider: "off"}); resp.StatusCode != 403 {
		t.Fatalf("learner create: %d", resp.StatusCode)
	}
}
