package components

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Cmd is one command line. Env is the COMPLETE environment: the runner never
// adds the parent's.
type Cmd struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

// Proc is a long-lived child process.
type Proc interface {
	// Terminate asks the whole process group to stop (SIGTERM).
	Terminate()
	// Kill ends the whole process group (SIGKILL).
	Kill()
	// Pid is the child's process id (and process-group id), 0 if unknown.
	Pid() int
	// Done is closed when the process has exited.
	Done() <-chan struct{}
}

// Runner is the seam between the manager and the operating system. Production
// uses ExecRunner; tests substitute an in-memory fake, so no test needs a real
// Python, a network or a model.
type Runner interface {
	// Run runs a command to completion, streaming each output line to onLine.
	Run(ctx context.Context, c Cmd, onLine func(string)) error
	// Start launches a long-lived command and returns at once.
	Start(c Cmd, onLine func(string)) (Proc, error)
}

// ExecRunner is the os/exec implementation. Every child gets its own process
// group so stopping it also stops anything it spawned.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, c Cmd, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Dir, cmd.Env = c.Dir, nonEmptyEnv(c.Env)
	setProcAttr(cmd)
	cmd.Cancel = func() error { killGroup(cmd.Process); return nil }
	cmd.WaitDelay = 3 * time.Second
	w := &lineWriter{emit: onLine}
	cmd.Stdout, cmd.Stderr = w, w
	err := cmd.Run()
	w.flush()
	return err
}

type execProc struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func (p *execProc) Pid() int              { return p.cmd.Process.Pid }
func (p *execProc) Terminate()            { termGroup(p.cmd.Process) }
func (p *execProc) Kill()                 { killGroup(p.cmd.Process) }
func (p *execProc) Done() <-chan struct{} { return p.done }

// ExitCode is valid once Done is closed; -1 when the child died on a signal.
func (p *execProc) ExitCode() int {
	if p.cmd.ProcessState == nil {
		return -1
	}
	return p.cmd.ProcessState.ExitCode()
}

// Start implements Runner.
func (ExecRunner) Start(c Cmd, onLine func(string)) (Proc, error) {
	cmd := exec.Command(c.Path, c.Args...)
	cmd.Dir, cmd.Env = c.Dir, nonEmptyEnv(c.Env)
	setProcAttr(cmd)
	cmd.WaitDelay = 2 * time.Second
	w := &lineWriter{emit: onLine}
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &execProc{cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		w.flush()
		close(p.done)
	}()
	return p, nil
}

// nonEmptyEnv: an empty Env means "inherit the parent's" to os/exec, which is
// exactly what this package exists to avoid.
func nonEmptyEnv(env []string) []string {
	if len(env) == 0 {
		return []string{"LANG=C"}
	}
	return env
}

// lineWriter splits output on \n and \r (pip redraws its progress with \r).
type lineWriter struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	emit func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		b := w.buf.Bytes()
		i := bytes.IndexAny(b, "\r\n")
		if i < 0 {
			if w.buf.Len() > 64<<10 { // never grow without bound on a line-less stream
				w.emitLine(string(b))
				w.buf.Reset()
			}
			break
		}
		line := string(b[:i])
		w.buf.Next(i + 1)
		w.emitLine(line)
	}
	return len(p), nil
}

func (w *lineWriter) emitLine(s string) {
	if w.emit != nil && strings.TrimSpace(s) != "" {
		w.emit(s)
	}
}

func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		w.emitLine(w.buf.String())
		w.buf.Reset()
	}
}

// ---- environments (allowlists, never the parent's environment) ----

// baseEnv is what any child may see: PATH, a UTF-8 locale and a HOME under the
// component. Nothing else is inherited — TANULOM_* and every secret have no way in.
func baseEnv(home string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"LANG=C.UTF-8",
		"PYTHONUNBUFFERED=1",
		"PYTHONNOUSERSITE=1",
	}
}

// installEnvKeys are the extra variables a pip/model-download step may need to
// get through a proxy or a private CA. They are passed to INSTALL commands
// only, never to the running service.
var installEnvKeys = []string{
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "REQUESTS_CA_BUNDLE", "CURL_CA_BUNDLE",
}

func installEnv(home string, extra ...string) []string {
	env := baseEnv(home)
	for _, k := range installEnvKeys {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return append(env, extra...)
}
