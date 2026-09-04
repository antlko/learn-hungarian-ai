package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A sidecar that can be in each of the states the Settings row has to draw.
//
// The whole point of `/api/mt/status` is telling those states apart, and the
// one that used to be indistinguishable — a service answering 503 because it
// is downloading 2.4 GB of weights — is the one these tests are mostly about.
type fakeMTLifecycle struct {
	*httptest.Server
	state   atomic.Value // string: loading | ready | failed
	warmups atomic.Int32
}

func newFakeMTLifecycle(t *testing.T, initial string) *fakeMTLifecycle {
	t.Helper()
	f := &fakeMTLifecycle{}
	f.state.Store(initial)

	body := func(w http.ResponseWriter) {
		state, _ := f.state.Load().(string)
		out := map[string]any{"status": state, "detail": "", "model": "test/nllb-200"}
		if state == "failed" {
			out["detail"] = "No space left on device"
		}
		if state == "loading" {
			out["downloadedBytes"], out["totalBytes"] = 1_200_000_000, 2_400_000_000
		}
		w.Header().Set("Content-Type", "application/json")
		code := http.StatusOK
		if state != "ready" {
			code = http.StatusServiceUnavailable
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(out)
	}

	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			body(w)
		case "/warmup":
			f.warmups.Add(1)
			f.state.Store("loading")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "loading", "detail": "", "model": "test/nllb-200",
				"downloadedBytes": 0, "totalBytes": 2_400_000_000, "started": true,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

// The regression this file exists for. `/health` answers 503 while the model
// downloads, and the capability probe maps every 5xx to "unreachable" — so a
// downloading service and an undeployed one reached Settings identically. The
// status route must report the difference.
func TestMTStatusDistinguishesLoadingFromUnreachable(t *testing.T) {
	fake := newFakeMTLifecycle(t, "loading")
	_, c := setupWithMT(t, fake.URL)

	var loading MTStatusResponse
	if resp := c.json(http.MethodGet, "/api/mt/status", nil, &loading); resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if !loading.Configured || !loading.Reachable {
		t.Fatalf("a loading service is configured and answering: %+v", loading)
	}
	if loading.State != "loading" {
		t.Fatalf("state = %q, want loading", loading.State)
	}
	if loading.TotalBytes == 0 || loading.DownloadedBytes == 0 {
		t.Fatalf("download progress must survive the trip: %+v", loading)
	}

	// Same configuration, nothing listening: reachable must go false while
	// configured stays true, because those two send the learner to different
	// remedies — deploy the container versus turn the setting on.
	fake.Close()
	var down MTStatusResponse
	if resp := c.json(http.MethodGet, "/api/mt/status", nil, &down); resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if !down.Configured || down.Reachable {
		t.Fatalf("want configured-but-unreachable, got %+v", down)
	}
	if down.State != "" {
		t.Fatalf("state must be empty when we cannot ask: %+v", down)
	}
}

func TestMTStatusReportsOffWhenNotConfigured(t *testing.T) {
	_, c := setupWithMT(t, "")

	var out MTStatusResponse
	if resp := c.json(http.MethodGet, "/api/mt/status", nil, &out); resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if out.Configured || out.Reachable {
		t.Fatalf("provider off must read as not configured: %+v", out)
	}
}

// A failed load has a reason, and the reason is the service's own words. A
// sentence we invent about "the model could not be loaded" would lose the one
// piece of information that lets anybody fix it.
func TestMTStatusCarriesTheServicesOwnFailureReason(t *testing.T) {
	fake := newFakeMTLifecycle(t, "failed")
	_, c := setupWithMT(t, fake.URL)

	var out MTStatusResponse
	if resp := c.json(http.MethodGet, "/api/mt/status", nil, &out); resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if out.State != "failed" || out.Detail != "No space left on device" {
		t.Fatalf("want the sidecar's own detail, got %+v", out)
	}
}

// Warmup is what makes a failed download retryable without a container
// restart — which on a deployed instance means without SSH.
func TestMTWarmupRestartsAFailedLoad(t *testing.T) {
	fake := newFakeMTLifecycle(t, "failed")
	_, c := setupWithMT(t, fake.URL)

	var out MTStatusResponse
	if resp := c.json(http.MethodPost, "/api/mt/warmup", nil, &out); resp.StatusCode != http.StatusOK {
		t.Fatalf("warmup: %d", resp.StatusCode)
	}
	if fake.warmups.Load() != 1 {
		t.Fatalf("warmups = %d, want 1", fake.warmups.Load())
	}
	if out.State != "loading" {
		t.Fatalf("state after warmup = %q, want loading", out.State)
	}
}

func TestMTWarmupRefusesWhenNotConfigured(t *testing.T) {
	_, c := setupWithMT(t, "")

	var body map[string]string
	if resp := c.json(http.MethodPost, "/api/mt/warmup", nil, &body); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("warmup with no service: %d, want 503", resp.StatusCode)
	}
	// An i18n key, not a sentence: only the client knows the learner's language.
	if body["error"] != "translation.notConfigured" {
		t.Fatalf("error = %q, want the shared i18n key", body["error"])
	}
}
