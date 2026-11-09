package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tanulom/internal/components"
	"tanulom/internal/config"
)

// A stand-in for the OS: no Python, no network, no model. The "child" is a
// real httptest server whose port the manager is told to use, so the whole
// resolve -> client -> HTTP path is exercised.
type apiFakeRunner struct {
	mu    sync.Mutex
	procs []*apiFakeProc
	block chan struct{}
}

type apiFakeProc struct {
	once sync.Once
	done chan struct{}
}

func (p *apiFakeProc) Terminate()            { p.once.Do(func() { close(p.done) }) }
func (p *apiFakeProc) Pid() int              { return 0 }
func (p *apiFakeProc) Kill()                 { p.Terminate() }
func (p *apiFakeProc) Done() <-chan struct{} { return p.done }

func (r *apiFakeRunner) Run(ctx context.Context, c components.Cmd, onLine func(string)) error {
	joined := strings.Join(c.Args, " ")
	switch {
	case strings.Contains(joined, "version_info"):
		onLine("3.12.0")
	case strings.Contains(joined, "-m venv"):
		if r.block != nil {
			select {
			case <-r.block:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		target := c.Args[len(c.Args)-1]
		_ = os.MkdirAll(filepath.Join(target, "bin"), 0o755)
	case strings.Contains(joined, "pip install"):
		onLine("Collecting fastapi")
	}
	return nil
}

func (r *apiFakeRunner) Start(components.Cmd, func(string)) (components.Proc, error) {
	p := &apiFakeProc{done: make(chan struct{})}
	r.mu.Lock()
	r.procs = append(r.procs, p)
	r.mu.Unlock()
	return p, nil
}

type apiFakeProber struct{}

func (apiFakeProber) Alive(context.Context, int) bool   { return true }
func (apiFakeProber) Warmup(context.Context, int) error { return nil }
func (apiFakeProber) Health(context.Context, int) (components.Health, error) {
	return components.Health{Status: "ready"}, nil
}

type apiDisk struct{ free uint64 }

func (d apiDisk) Free(string) (uint64, error) { return d.free, nil }

// childServer is the "managed" mt service: it answers /health as mt does.
func childServer(t *testing.T) (port int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "ready", "model": "nllb-fake"})
	}))
	t.Cleanup(srv.Close)
	_, p, _ := net.SplitHostPort(srv.Listener.Addr().String())
	n := 0
	for _, ch := range p {
		n = n*10 + int(ch-'0')
	}
	return n
}

func withFakeManager(t *testing.T, s *Server, port int, mods ...func(*components.Options)) *apiFakeRunner {
	t.Helper()
	r := &apiFakeRunner{}
	s.components.Shutdown(context.Background())
	s.components = s.newComponents(func(o *components.Options) {
		o.Runner, o.Prober = r, apiFakeProber{}
		o.LookPath = func(n string) (string, error) { return "/usr/bin/" + n, nil }
		o.Disk = apiDisk{free: 100 << 30}
		o.PickPort = func() (int, error) { return port, nil }
		o.PollInterval = 2 * time.Millisecond
		o.StopGrace = 50 * time.Millisecond
		for _, m := range mods {
			m(o)
		}
	})
	t.Cleanup(func() {
		s.components.Shutdown(context.Background())
		InvalidateCapabilityCache() // the probe cache is process-wide; do not leak "managed" into the next test
	})
	return r
}

func managedOf(t *testing.T, c *client, id string) *ManagedDTO {
	t.Helper()
	var rows []ComponentDTO
	if resp := c.json(http.MethodGet, "/api/admin/components", nil, &rows); resp.StatusCode != 200 {
		t.Fatalf("components: %d", resp.StatusCode)
	}
	for _, r := range rows {
		if r.ID == id {
			return r.Managed
		}
	}
	t.Fatalf("no %s row", id)
	return nil
}

func waitManaged(t *testing.T, c *client, id, state string) *ManagedDTO {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		m := managedOf(t, c, id)
		if m != nil && m.State == state {
			return m
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s managed state %+v, want %s", id, m, state)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func serviceOf(t *testing.T, c *client, id string) ServiceDTO {
	t.Helper()
	var st SettingsDTO
	c.json(http.MethodGet, "/api/settings", nil, &st)
	return st.Services[id]
}

func TestComponentRoutesAreAdminOnly(t *testing.T) {
	s, admin := newAdmin(t)
	addUser(t, admin, "bea@example.com", "bea elég hosszú jelszó", "learner")
	learner := signIn(t, s, "bea@example.com", "bea elég hosszú jelszó")
	for _, id := range []string{"mt", "nlp", "tts"} {
		for _, act := range []string{"install", "start", "stop", "uninstall"} {
			path := "/api/admin/components/" + id + "/" + act
			if resp, _ := learner.do(http.MethodPost, path, nil); resp.StatusCode != http.StatusForbidden {
				t.Errorf("learner POST %s = %d, want 403", path, resp.StatusCode)
			}
			anon := &client{t: t, app: s}
			if resp, _ := anon.do(http.MethodPost, path, nil); resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("anonymous POST %s = %d, want 401", path, resp.StatusCode)
			}
		}
	}
}

func TestComponentUnknownIDIs404(t *testing.T) {
	s, c := newAdmin(t)
	withFakeManager(t, s, 1)
	for _, id := range []string{"tts", "llm", "pack", "..%2Fetc"} {
		var e struct{ Error string }
		resp := c.json(http.MethodPost, "/api/admin/components/"+id+"/install", nil, &e)
		if resp.StatusCode != http.StatusNotFound || e.Error != "errors.componentUnknown" {
			t.Errorf("%s: %d %q", id, resp.StatusCode, e.Error)
		}
	}
}

func TestComponentsListCarriesManagedOnMTAndNLPOnly(t *testing.T) {
	s, c := newAdmin(t)
	withFakeManager(t, s, 1, func(o *components.Options) {
		o.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	})
	var rows []ComponentDTO
	c.json(http.MethodGet, "/api/admin/components", nil, &rows)
	for _, r := range rows {
		switch r.ID {
		case "mt", "nlp":
			m := r.Managed
			if m == nil || m.Available || m.UnavailableKey != "errors.componentNoPython" || m.State != "absent" || m.DiskNeed == "" || m.RAMNeed == "" {
				t.Errorf("%s: %+v", r.ID, m)
			}
		default:
			if r.Managed != nil {
				t.Errorf("%s must not carry managed", r.ID)
			}
		}
	}
	var e struct{ Error string }
	if resp := c.json(http.MethodPost, "/api/admin/components/mt/install", nil, &e); resp.StatusCode != 422 || e.Error != "errors.componentNoPython" {
		t.Fatalf("install without python: %d %q", resp.StatusCode, e.Error)
	}
}

func TestComponentInstallRefusedOnFullDisk(t *testing.T) {
	s, c := newAdmin(t)
	withFakeManager(t, s, 1, func(o *components.Options) { o.Disk = apiDisk{free: 1 << 20} })
	var e struct{ Error string }
	if resp := c.json(http.MethodPost, "/api/admin/components/nlp/install", nil, &e); resp.StatusCode != http.StatusInsufficientStorage || e.Error != "errors.diskFull" {
		t.Fatalf("%d %q", resp.StatusCode, e.Error)
	}
}

func TestManagedInstallSelectsProviderAndTheClientsResolveThePort(t *testing.T) {
	s, c := newAdmin(t)
	port := childServer(t)
	r := withFakeManager(t, s, port)
	r.block = make(chan struct{})

	if resp, _ := c.do(http.MethodPost, "/api/admin/components/mt/install", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("install = %d", resp.StatusCode)
	}
	// Single-flight, observable over HTTP.
	var e struct{ Error string }
	for _, act := range []string{"install", "uninstall", "stop"} {
		if resp := c.json(http.MethodPost, "/api/admin/components/mt/"+act, nil, &e); resp.StatusCode != http.StatusConflict || e.Error != "errors.componentBusy" {
			t.Errorf("%s while installing: %d %q", act, resp.StatusCode, e.Error)
		}
	}
	if m := managedOf(t, c, "mt"); m.State != "installing" || m.Step == "" {
		t.Fatalf("%+v", m)
	}
	if got := serviceOf(t, c, "mt"); got.Provider != "off" {
		t.Fatalf("provider changed before the install finished: %+v", got)
	}
	close(r.block)
	m := waitManaged(t, c, "mt", "running")
	if m.Port != port || m.Progress != 1 {
		t.Fatalf("%+v", m)
	}

	// Provider selected automatically; no endpoint stored anywhere.
	if got := serviceOf(t, c, "mt"); got.Provider != "managed" || got.Endpoint != "" {
		t.Fatalf("service = %+v", got)
	}
	raw, _ := os.ReadFile(filepath.Join(s.dataDir, "config.yaml"))
	if strings.Contains(string(raw), "127.0.0.1") || strings.Contains(string(raw), "http://") && strings.Contains(string(raw), "mt:") && strings.Contains(string(raw), "endpoint: http://127") {
		t.Fatalf("a managed endpoint reached config.yaml:\n%s", raw)
	}

	// ONE resolver: the mt client, the probe and the row all see the port.
	if svc := s.service("mt"); svc.Provider != "local" || svc.Endpoint != "http://127.0.0.1:"+itoa(int64(port)) {
		t.Fatalf("resolved = %+v", svc)
	}
	var st MTStatusResponse
	c.json(http.MethodGet, "/api/mt/status", nil, &st)
	if !st.Configured || !st.Reachable || st.State != "ready" || st.Model != "nllb-fake" {
		t.Fatalf("mt status through the managed port: %+v", st)
	}
	var rows []ComponentDTO
	c.json(http.MethodGet, "/api/admin/components", nil, &rows)
	for _, row := range rows {
		if row.ID == "mt" && (!row.Configured || !row.Reachable || row.State != "ready") {
			t.Fatalf("mt row: %+v", row)
		}
	}
	var caps []CapabilityDTO
	c.json(http.MethodGet, "/api/capabilities", nil, &caps)
	for _, cp := range caps {
		if cp.Name == "mt" && (cp.Provider != "managed" || !cp.Configured || !cp.Reachable) {
			t.Fatalf("capability: %+v", cp)
		}
	}
}

func TestStopMakesManagedServiceUnreachableNotBroken(t *testing.T) {
	s, c := newAdmin(t)
	r := withFakeManager(t, s, childServer(t))
	c.do(http.MethodPost, "/api/admin/components/mt/install", nil)
	waitManaged(t, c, "mt", "running")

	if resp, _ := c.do(http.MethodPost, "/api/admin/components/mt/stop", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatal(resp.StatusCode)
	}
	if m := managedOf(t, c, "mt"); m.State != "stopped" || m.Port != 0 {
		t.Fatalf("%+v", m)
	}
	select {
	case <-r.procs[0].done:
	default:
		t.Fatal("child not stopped")
	}
	// provider stays managed; the endpoint is gone; clients see "not configured".
	if svc := s.service("mt"); svc.Provider != "managed" || svc.Endpoint != "" {
		t.Fatalf("%+v", svc)
	}
	var st MTStatusResponse
	c.json(http.MethodGet, "/api/mt/status", nil, &st)
	if st.Reachable {
		t.Fatalf("%+v", st)
	}
	var rows []ComponentDTO
	c.json(http.MethodGet, "/api/admin/components", nil, &rows)
	for _, row := range rows {
		if row.ID == "mt" && (!row.Configured || row.Reachable || row.State != "unreachable") {
			t.Fatalf("mt row after stop: %+v", row)
		}
	}
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/mt/start", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatal(resp.StatusCode)
	}
	waitManaged(t, c, "mt", "running")
}

func TestStartNeedsAnInstall(t *testing.T) {
	s, c := newAdmin(t)
	withFakeManager(t, s, 1)
	var e struct{ Error string }
	if resp := c.json(http.MethodPost, "/api/admin/components/nlp/start", nil, &e); resp.StatusCode != http.StatusConflict || e.Error != "errors.componentNotInstalled" {
		t.Fatalf("%d %q", resp.StatusCode, e.Error)
	}
}

func TestSwitchingProviderAwayStopsTheChild(t *testing.T) {
	s, c := newAdmin(t)
	r := withFakeManager(t, s, childServer(t))
	c.do(http.MethodPost, "/api/admin/components/mt/install", nil)
	waitManaged(t, c, "mt", "running")

	resp, body := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Services: map[string]ServiceDTO{"mt": {Provider: "local", Endpoint: "http://elsewhere:5003"}}})
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	select {
	case <-r.procs[0].done:
	default:
		t.Fatal("the managed child kept running after the learner chose another provider")
	}
	if m := managedOf(t, c, "mt"); m.State != "stopped" {
		t.Fatalf("%+v", m)
	}

	// And back: choosing managed again starts the installed child and drops any endpoint.
	resp, body = c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Services: map[string]ServiceDTO{"mt": {Provider: "managed", Endpoint: "http://leftover:1"}}})
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	if got := serviceOf(t, c, "mt"); got.Provider != "managed" || got.Endpoint != "" {
		t.Fatalf("%+v", got)
	}
	waitManaged(t, c, "mt", "running")
}

func TestUninstallResetsManagedProviderAndDeletesFiles(t *testing.T) {
	s, c := newAdmin(t)
	withFakeManager(t, s, childServer(t))
	c.do(http.MethodPost, "/api/admin/components/nlp/install", nil)
	waitManaged(t, c, "nlp", "running")
	if got := serviceOf(t, c, "nlp"); got.Provider != "managed" {
		t.Fatalf("%+v", got)
	}
	dir := filepath.Join(s.dataDir, "components", "nlp")
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/nlp/uninstall", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatal(resp.StatusCode)
	}
	if got := serviceOf(t, c, "nlp"); got.Provider != "off" || got.Endpoint != "" {
		t.Fatalf("provider after uninstall: %+v", got)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("component files remain")
	}
	if m := managedOf(t, c, "nlp"); m.State != "absent" {
		t.Fatalf("%+v", m)
	}
}

func TestUninstallLeavesAForeignProviderAlone(t *testing.T) {
	s, c := newAdmin(t)
	withFakeManager(t, s, childServer(t))
	c.do(http.MethodPost, "/api/admin/components/mt/install", nil)
	waitManaged(t, c, "mt", "running")
	c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Services: map[string]ServiceDTO{"mt": {Provider: "local", Endpoint: "http://elsewhere:5003"}}})
	c.do(http.MethodPost, "/api/admin/components/mt/uninstall", nil)
	if got := serviceOf(t, c, "mt"); got.Provider != "local" || got.Endpoint != "http://elsewhere:5003" {
		t.Fatalf("uninstall rewrote an unrelated provider: %+v", got)
	}
}

func TestBootDoesNotBlockOnManagedComponents(t *testing.T) {
	// Nothing installed + provider managed in config: New must come up, the
	// autostart finds nothing and the app is simply without the service.
	s, c := newAdmin(t)
	if _, err := s.updateConfig(func(cfg *config.Config) { cfg.Services.MT = config.Service{Provider: config.ProviderManaged} }); err != nil {
		t.Fatal(err)
	}
	withFakeManager(t, s, 1)
	s.components.AutoStart(func(string) bool { return true })
	if m := managedOf(t, c, "mt"); m.State != "absent" {
		t.Fatalf("%+v", m)
	}
	var st MTStatusResponse
	if resp := c.json(http.MethodGet, "/api/mt/status", nil, &st); resp.StatusCode != 200 || st.Reachable {
		t.Fatalf("%d %+v", resp.StatusCode, st)
	}
}
