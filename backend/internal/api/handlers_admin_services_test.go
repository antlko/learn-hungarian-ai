package api

import (
	"net/http"
	"testing"
)

func capOf(t *testing.T, c *client, name string) CapabilityDTO {
	t.Helper()
	var caps []CapabilityDTO
	c.json(http.MethodGet, "/api/capabilities", nil, &caps)
	for _, cp := range caps {
		if cp.Name == name {
			return cp
		}
	}
	t.Fatalf("no capability %q", name)
	return CapabilityDTO{}
}

func TestPausingAManagedServiceStopsTheChildAndResumingStartsIt(t *testing.T) {
	t.Parallel()
	s, c := newAdmin(t)
	r := withFakeManager(t, s, childServer(t))
	c.do(http.MethodPost, "/api/admin/components/mt/install", nil)
	waitManaged(t, c, "mt", "running")
	if cp := capOf(t, c, "mt"); !cp.Reachable || cp.Paused {
		t.Fatalf("before: %+v", cp)
	}

	if resp := c.json(http.MethodPost, "/api/admin/services/mt/paused", PausedRequest{Paused: true}, nil); resp.StatusCode != 200 {
		t.Fatalf("pause: %d", resp.StatusCode)
	}
	select {
	case <-r.procs[0].done:
	default:
		t.Fatal("the managed child kept running after the pause")
	}
	cp := capOf(t, c, "mt")
	if !cp.Paused || cp.Configured || cp.Reachable || cp.Provider != "managed" {
		t.Fatalf("paused row: %+v", cp)
	}
	if got := serviceOf(t, c, "mt"); got.Provider != "managed" || !got.Paused {
		t.Fatalf("settings must keep the provider and show the pause: %+v", got)
	}
	if got := s.service("mt"); got.Provider != "off" {
		t.Fatalf("resolveService of a paused service = %+v", got)
	}

	// A settings PUT (which carries no `paused`) must not resume it.
	if resp, body := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Services: map[string]ServiceDTO{"mt": {Provider: "managed"}}}); resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	if got := serviceOf(t, c, "mt"); !got.Paused {
		t.Fatalf("a settings PUT cleared the pause: %+v", got)
	}
	if m := managedOf(t, c, "mt"); m.State == "running" || m.State == "starting" {
		t.Fatalf("a settings PUT restarted a paused child: %+v", m)
	}

	if resp := c.json(http.MethodPost, "/api/admin/services/mt/paused", PausedRequest{Paused: false}, nil); resp.StatusCode != 200 {
		t.Fatalf("resume: %d", resp.StatusCode)
	}
	waitManaged(t, c, "mt", "running")
	if cp := capOf(t, c, "mt"); cp.Paused || !cp.Reachable {
		t.Fatalf("after resume: %+v", cp)
	}
}

func TestPausedLLMReadsAsOffEverywhere(t *testing.T) {
	t.Parallel()
	s, c := newAdmin(t)
	if resp, body := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Services: map[string]ServiceDTO{"llm": {Provider: "local", Endpoint: "http://llm.invalid:1", Model: "m"}}}); resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	c.json(http.MethodPost, "/api/admin/services/llm/paused", PausedRequest{Paused: true}, nil)
	if got := s.service("llm"); got.Provider != "off" {
		t.Fatalf("%+v", got)
	}
	got := serviceOf(t, c, "llm")
	if got.Endpoint != "http://llm.invalid:1" || got.Model != "m" || !got.Paused {
		t.Fatalf("pause must keep endpoint and model: %+v", got)
	}
}

func TestServicePausedRouteGuards(t *testing.T) {
	t.Parallel()
	s, admin := newAdmin(t)
	addUser(t, admin, "bea@example.com", "bea elég hosszú jelszó", "learner")
	learner := signIn(t, s, "bea@example.com", "bea elég hosszú jelszó")
	if resp := learner.json(http.MethodPost, "/api/admin/services/tts/paused", PausedRequest{Paused: true}, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("learner: %d, want 403", resp.StatusCode)
	}
	anon := &client{t: t, app: s}
	if resp := anon.json(http.MethodPost, "/api/admin/services/tts/paused", PausedRequest{Paused: true}, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous: %d, want 401", resp.StatusCode)
	}
	var e struct{ Error string }
	if resp := admin.json(http.MethodPost, "/api/admin/services/nlp/paused", PausedRequest{Paused: true}, &e); resp.StatusCode != http.StatusNotFound || e.Error != "errors.serviceUnknown" {
		t.Errorf("unknown name: %d %q", resp.StatusCode, e.Error)
	}
	if serviceOf(t, admin, "tts").Paused {
		t.Error("a refused request paused tts")
	}
}
