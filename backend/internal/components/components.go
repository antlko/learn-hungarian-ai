// Package components installs and supervises the optional Python sidecars
// (services/mt, services/nlp) as CHILD PROCESSES of the Go binary — ADR-061.
//
// The idea in one paragraph: an administrator presses «Установить», the binary
// extracts the service source it carries (go:embed), builds a venv under
// `<data>/components/<id>/`, pip-installs the pinned requirements, prepares the
// model, and then runs `python app.py` bound to 127.0.0.1 on an EPHEMERAL port.
// The port lives in memory only; every client resolves the endpoint through
// Manager.Endpoint, so nothing about it is ever written to config.yaml.
//
// What this must never do:
//
//   - block boot or make the app depend on a child. AutoStart runs in a
//     goroutine, errors are logged, and with every component absent the app is
//     exactly what it was before this package existed.
//   - run anything but the embedded sources and the pinned requirements. No
//     route accepts a URL, a path or an argument; every command line here is a
//     constant or built from the data directory.
//   - hand a child the server's environment. Env is an ALLOWLIST (PATH, LANG,
//     HOME under the component dir, the service's own *_HOST/*_PORT/*_MODEL_DIR)
//     — TANULOM_* and every secret simply have no way in.
//   - leave a half-installed component behind. Installs happen in
//     `<id>.installing/` and are renamed on success; any failure removes it.
package components

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Component ids this package manages.
const (
	MT  = "mt"
	NLP = "nlp"
)

// State is the word the Components card draws.
type State string

const (
	StateAbsent     State = "absent"
	StateInstalling State = "installing"
	StateInstalled  State = "installed"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateFailed     State = "failed"
	StateStopped    State = "stopped"
)

// i18n keys. Step keys are for the progress line, error keys for failures.
const (
	StepVenv  = "component.step.venv"
	StepPip   = "component.step.pip"
	StepModel = "component.step.model"
	StepStart = "component.step.start"

	KeyNoPython      = "errors.componentNoPython"
	KeyReadOnly      = "errors.componentReadOnly"
	KeyBusy          = "errors.componentBusy"
	KeyUnknown       = "errors.componentUnknown"
	KeyNotInstalled  = "errors.componentNotInstalled"
	KeyInstalled     = "errors.componentInstalled"
	KeyDiskFull      = "errors.diskFull"
	KeyNetwork       = "errors.componentNetwork"
	KeyInstallFailed = "errors.componentInstallFailed"
	KeyStartFailed   = "errors.componentStartFailed"
	KeyCrashed       = "errors.componentCrashed"
	KeyPipBroken     = "errors.componentPipBroken"
)

// Error carries an i18n key for the API layer.
type Error struct {
	Key    string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Key
	}
	return e.Key + ": " + e.Detail
}

// Is makes errors.Is(err, ErrBusy) compare keys, so a wrapped *Error still matches.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Key == e.Key
}

// Sentinels for the state-machine refusals.
var (
	ErrBusy         = &Error{Key: KeyBusy}
	ErrUnknown      = &Error{Key: KeyUnknown}
	ErrNotInstalled = &Error{Key: KeyNotInstalled}
	ErrInstalled    = &Error{Key: KeyInstalled}
)

// spec is the static description of one component. Sizes are documented
// ESTIMATES (the UI says so) and the only thing the disk preflight is based on.
type spec struct {
	id            string
	files         []string // under src/<id>/
	diskNeedBytes uint64
	diskNeed      string
	ramNeed       string
	hostEnv       string
	portEnv       string
	modelEnv      string
	modelSub      string // under models/
	// torch: the model needs a one-time conversion that imports torch, which is
	// installed from the CPU index first and uninstalled afterwards (~800 MB).
	torch bool
	// prepare: `python -c <this>` run in src/ with the service's env.
	prepare string
	// warmup: the model is fetched by POST /warmup once the service runs.
	warmup bool
	// pipExpected: roughly how many "Collecting" lines a full pip run prints;
	// only used to draw coarse progress.
	pipExpected int
}

const gb = 1 << 30

var specs = map[string]spec{
	MT: {
		id: MT, files: []string{"app.py", "requirements.txt", "requirements-convert.txt"},
		diskNeedBytes: 6 * gb, diskNeed: "≈6 GB (estimate)", ramNeed: "≈2–3 GB (estimate, not measured)",
		hostEnv: "MT_HOST", portEnv: "MT_PORT", modelEnv: "MT_MODEL_DIR", modelSub: "nllb-ct2",
		torch: true, prepare: "import app; app.prepare_model()", pipExpected: 30,
	},
	NLP: {
		id: NLP, files: []string{"app.py", "requirements.txt"},
		diskNeedBytes: gb + gb/2, diskNeed: "≈1.5 GB (estimate)", ramNeed: "≈2.4 GB (published, not measured)",
		hostEnv: "NLP_HOST", portEnv: "NLP_PORT", modelEnv: "NLP_MODEL_DIR", modelSub: "",
		warmup: true, pipExpected: 18,
	},
}

// IDs lists the managed component ids.
func IDs() []string { return []string{MT, NLP} }

// Status is what GET /api/admin/components reports under `managed`.
type Status struct {
	Available      bool
	UnavailableKey string
	PythonVersion  string
	State          State
	Step           string
	Progress       float64
	Port           int
	ErrorKey       string
	ErrorDetail    string
	Log            []string
	DiskNeed       string
	RAMNeed        string
}

// Options configure a Manager. Zero values are production defaults; tests
// replace the seams.
type Options struct {
	DataDir  string
	Runner   Runner
	LookPath func(string) (string, error)
	Disk     DiskInfo
	Prober   Prober
	// PickPort returns a free 127.0.0.1 port.
	PickPort func() (int, error)
	// OnInstalled runs after a successful install, once the child is reachable
	// (the API layer sets provider=managed here, through updateConfig).
	OnInstalled func(id string) error
	// OnChange runs after any state change (capability cache invalidation).
	OnChange func(id string)

	// ProcInfo / SignalPID are the pid-file reaper's seams.
	ProcInfo      func(pid int) (cmdline string, alive bool)
	SignalPID     func(pid int, force bool)
	GOOS          string        // default runtime.GOOS; picks the torch install path
	PollInterval  time.Duration // readiness polling, default 500 ms
	StopGrace     time.Duration // SIGTERM -> SIGKILL, default 5 s
	Backoff       func(n int) time.Duration
	MaxRestarts   int           // crashes allowed per RestartWindow, default 5
	RestartWindow time.Duration // default 10 min
	ReadyTimeout  time.Duration // child up but /health silent, default 5 min
	WarmupTimeout time.Duration // nlp model fetch, default 20 min
}

// Manager owns every managed component. Safe for concurrent use.
type Manager struct {
	dir string // <data>/components
	opt Options

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu    sync.Mutex
	comps map[string]*comp
	// installing: the id whose install runs now. Installs are manager-wide
	// single-flight — two venvs plus torch at once is how a disk fills.
	installing string

	pyMu    sync.Mutex
	pyAt    time.Time
	pyCache envInfo
	pyBusy  bool
}

// comp is one component's mutable state, guarded by Manager.mu.
type comp struct {
	spec spec

	state    State
	step     string
	progress float64
	port     int
	errKey   string
	errMsg   string
	log      ring
	noise    noiseFilter

	// busy: an install or an uninstall is in flight (single-flight).
	busy bool
	// Supervision of the running child.
	supCancel context.CancelFunc
	supDone   chan struct{}
	proc      Proc
	gen       int
	// claim: a Start is between its checks and its launch; Uninstall refuses.
	claim bool
	// timedOut: awaitReady gave up on this generation; the supervisor fails it.
	timedOut bool
}

// New builds a Manager. It does no work and touches no disk until used.
func New(o Options) *Manager {
	if o.Runner == nil {
		o.Runner = ExecRunner{}
	}
	if o.LookPath == nil {
		o.LookPath = lookPath
	}
	if o.Disk == nil {
		o.Disk = osDisk{}
	}
	if o.Prober == nil {
		o.Prober = HTTPProber{}
	}
	if o.PickPort == nil {
		o.PickPort = pickPort
	}
	if o.ProcInfo == nil {
		o.ProcInfo = procInfo
	}
	if o.SignalPID == nil {
		o.SignalPID = signalPID
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.PollInterval <= 0 {
		o.PollInterval = 500 * time.Millisecond
	}
	if o.StopGrace <= 0 {
		o.StopGrace = 5 * time.Second
	}
	if o.Backoff == nil {
		o.Backoff = func(n int) time.Duration {
			d := time.Second << min(n-1, 5)
			return min(d, 30*time.Second)
		}
	}
	if o.MaxRestarts <= 0 {
		o.MaxRestarts = 5
	}
	if o.RestartWindow <= 0 {
		o.RestartWindow = 10 * time.Minute
	}
	if o.ReadyTimeout <= 0 {
		o.ReadyTimeout = 5 * time.Minute
	}
	if o.WarmupTimeout <= 0 {
		o.WarmupTimeout = 20 * time.Minute
	}
	m := &Manager{dir: filepath.Join(o.DataDir, "components"), opt: o, comps: map[string]*comp{}}
	m.ctx, m.cancel = context.WithCancel(context.Background())
	for id, sp := range specs {
		m.comps[id] = &comp{spec: sp}
	}
	m.reapStale()
	return m
}

func pickPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Paths. `final` is the live directory, `stage` the atomic install target.
func (m *Manager) final(id string) string { return filepath.Join(m.dir, id) }
func (m *Manager) stage(id string) string { return filepath.Join(m.dir, id+".installing") }
func marker(dir string) string            { return filepath.Join(dir, ".installed") }

func (m *Manager) installedOnDisk(id string) bool {
	_, err := os.Stat(marker(m.final(id)))
	return err == nil
}

// Installed reports whether a finished install is on disk.
func (m *Manager) Installed(id string) bool {
	if _, ok := specs[id]; !ok {
		return false
	}
	return m.installedOnDisk(id)
}

func (m *Manager) get(id string) (*comp, error) {
	c, ok := m.comps[id]
	if !ok {
		return nil, ErrUnknown
	}
	return c, nil
}

func (m *Manager) changed(id string) {
	if m.opt.OnChange != nil {
		m.opt.OnChange(id)
	}
}

// set mutates a component under the lock and then notifies.
func (m *Manager) set(c *comp, f func(*comp)) {
	m.mu.Lock()
	f(c)
	m.mu.Unlock()
	m.changed(c.spec.id)
}

func (m *Manager) progress(c *comp, p float64) {
	m.set(c, func(c *comp) {
		if p > c.progress {
			c.progress = min(p, 1)
		}
	})
}

func (m *Manager) addLog(c *comp, line string) {
	line = m.redact(line)
	if line == "" {
		return
	}
	m.mu.Lock()
	if c.noise.skip(line) {
		m.mu.Unlock()
		return
	}
	c.log.add(line)
	m.mu.Unlock()
}

// Status reports one component. An unknown id yields a zero Status.
func (m *Manager) Status(id string) Status {
	c, err := m.get(id)
	if err != nil {
		return Status{}
	}
	env := m.environment(false)
	py := env.py
	avail, key := py.ok, py.key
	if env.wkey != "" {
		avail, key = false, env.wkey
	}
	onDisk := m.installedOnDisk(id)

	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{
		Available: avail, UnavailableKey: key, PythonVersion: py.version,
		State: c.state, Step: c.step, Progress: c.progress, Port: c.port,
		ErrorKey: c.errKey, ErrorDetail: c.errMsg, Log: c.log.last(40),
		DiskNeed: c.spec.diskNeed, RAMNeed: c.spec.ramNeed,
	}
	if st.Available {
		st.UnavailableKey = ""
	}
	if st.State == "" {
		st.State = StateAbsent
		if onDisk {
			st.State = StateInstalled
		}
	}
	if st.State != StateInstalling {
		st.Step = ""
	}
	return st
}

// Endpoint is the ONE place a managed endpoint is resolved. It returns
// "http://127.0.0.1:<port>" while the child has a port, and "" otherwise —
// callers treat "" as "not reachable now", never as "use the stored endpoint".
// Nil-safe so a Server built without a manager resolves to nothing.
func (m *Manager) Endpoint(id string) string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.comps[id]
	if !ok || c.port == 0 || (c.state != StateRunning && c.state != StateStarting) {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", c.port)
}

// writable checks that the components directory can be created and written to.
// Returns "" when it can, KeyReadOnly otherwise.
func (m *Manager) writable() string {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return KeyReadOnly
	}
	f, err := os.CreateTemp(m.dir, ".w-*")
	if err != nil {
		return KeyReadOnly
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return ""
}

// redact rewrites paths so a log line shows `<data>/components/mt/...` and
// never a path outside the data directory.
func (m *Manager) redact(text string) string {
	root := filepath.Dir(m.dir)
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = redactLine(l, root)
	}
	return strings.Join(lines, "\n")
}

// Shutdown stops every child and cancels installs, waiting for them. Called
// before the database closes.
func (m *Manager) Shutdown(ctx context.Context) {
	if m == nil {
		return
	}
	m.cancel()
	done := make(chan struct{})
	go func() {
		m.mu.Lock()
		var all []*comp
		for _, c := range m.comps {
			all = append(all, c)
		}
		m.mu.Unlock()
		for _, c := range all {
			m.stopSupervisor(c)
		}
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("components: shutdown timed out; children may outlive the server")
	}
}

// AutoStart starts every installed component for which wanted(id) is true.
// Never blocks on a child and only logs failures — boot must not depend on it.
func (m *Manager) AutoStart(wanted func(id string) bool) {
	for _, id := range IDs() {
		if !wanted(id) || !m.installedOnDisk(id) {
			continue
		}
		if err := m.Start(id); err != nil {
			slog.Warn("components: autostart failed", "component", id, "error", err)
		}
	}
}

// ---- python detection ----

type pyInfo struct {
	ok      bool
	path    string
	version string
	key     string
}

// detectScript exits 0 only on Python >= 3.10 with venv and ensurepip, and
// prints the version.
const detectScript = `import sys
if sys.version_info < (3, 10):
    sys.exit(2)
import venv, ensurepip
print("%d.%d.%d" % sys.version_info[:3])`

var pyCandidates = []string{"python3", "python3.13", "python3.12", "python3.11", "python3.10"}

// envInfo is what the Components card needs to know about the machine.
type envInfo struct {
	py   pyInfo
	wkey string // "" or KeyReadOnly
}

// environment returns the cached machine probe. A request never waits for the
// probe except the very first one (or force, used by Install): a stale value is
// returned at once and refreshed in the background, so polling the card costs a
// map read, not a python subprocess and a temp-file write.
func (m *Manager) environment(force bool) envInfo {
	m.pyMu.Lock()
	if !force && !m.pyAt.IsZero() {
		e := m.pyCache
		if time.Since(m.pyAt) > 30*time.Second && !m.pyBusy && m.ctx.Err() == nil {
			m.pyBusy = true
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				e := m.probeEnvironment()
				m.pyMu.Lock()
				m.pyAt, m.pyCache, m.pyBusy = time.Now(), e, false
				m.pyMu.Unlock()
			}()
		}
		m.pyMu.Unlock()
		return e
	}
	m.pyMu.Unlock()
	e := m.probeEnvironment()
	m.pyMu.Lock()
	m.pyAt, m.pyCache = time.Now(), e
	m.pyMu.Unlock()
	return e
}

func (m *Manager) python() pyInfo { return m.environment(true).py }

func (m *Manager) probeEnvironment() envInfo {
	e := envInfo{py: m.detectPython(), wkey: m.writable()}
	return e
}

// detectPython finds a usable interpreter. The probe runs isolated (-I) from
// the temp directory, never from the data directory or the server's cwd.
func (m *Manager) detectPython() pyInfo {
	info := pyInfo{key: KeyNoPython}
	if !pythonSupported {
		return info
	}
	for _, name := range pyCandidates {
		p, err := m.opt.LookPath(name)
		if err != nil {
			continue
		}
		var last string
		ctx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
		err = m.opt.Runner.Run(ctx, Cmd{Path: p, Args: []string{"-I", "-c", detectScript}, Dir: os.TempDir(), Env: baseEnv(os.TempDir())},
			func(l string) {
				if l = strings.TrimSpace(l); l != "" {
					last = l
				}
			})
		cancel()
		if err == nil && last != "" {
			return pyInfo{ok: true, path: p, version: last}
		}
	}
	return info
}
