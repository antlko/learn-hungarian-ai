package components

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// DiskInfo reports free bytes on the filesystem holding path. An interface so
// tests can claim a full disk without filling one.
type DiskInfo interface {
	Free(path string) (uint64, error)
}

// Prober talks to a running service over loopback.
type Prober interface {
	// Alive: the service answered /health with ANY HTTP status. A model still
	// loading answers 503, and that is "up".
	Alive(ctx context.Context, port int) bool
	// Health reads the service's own account of its model.
	Health(ctx context.Context, port int) (Health, error)
}

// Health is the subset of /health the install job reads.
type Health struct {
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Error    string  `json:"error"`
}

// HTTPProber is the production Prober.
type HTTPProber struct{}

var probeClient = &http.Client{Timeout: 3 * time.Second}

func url(port int, path string) string { return fmt.Sprintf("http://127.0.0.1:%d%s", port, path) }

// Alive implements Prober.
func (HTTPProber) Alive(ctx context.Context, port int) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url(port, "/health"), nil)
	if err != nil {
		return false
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// Health implements Prober.
func (HTTPProber) Health(ctx context.Context, port int) (Health, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url(port, "/health"), nil)
	if err != nil {
		return Health{}, err
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return Health{}, err
	}
	defer resp.Body.Close()
	var h Health
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<10)).Decode(&h); err != nil {
		return Health{}, err
	}
	return h, nil
}

// ---- log ring ----

const ringCap = 200

// ring keeps the last ringCap lines. Guarded by Manager.mu.
type ring struct{ lines []string }

func (r *ring) add(s string) {
	if len(s) > 300 {
		cut := 300
		for cut > 0 && !utf8.RuneStart(s[cut]) { // never split a multi-byte rune
			cut--
		}
		s = s[:cut] + "…"
	}
	r.lines = append(r.lines, s)
	if len(r.lines) > ringCap {
		r.lines = append(r.lines[:0], r.lines[len(r.lines)-ringCap:]...)
	}
}

func (r *ring) last(n int) []string {
	if len(r.lines) == 0 {
		return nil
	}
	start := max(0, len(r.lines)-n)
	return append([]string(nil), r.lines[start:]...)
}

func (r *ring) reset() { r.lines = nil }

// ---- redaction ----

// absPath matches a filesystem path that starts a token. URLs are not matched:
// their second slash follows ':' or '/', not whitespace, a quote or '='.
var absPath = regexp.MustCompile(`(^|[\s'"=(])(/[^\s'")]+)`)

// redactLine keeps paths inside dataDir (shown as <data>/…) and reduces every
// other absolute path to its last element, so a log never reveals the layout
// of the host outside the data directory.
func redactLine(line, dataDir string) string {
	line = strings.TrimRight(line, " \t")
	return absPath.ReplaceAllStringFunc(line, func(m string) string {
		i := strings.Index(m, "/")
		prefix, p := m[:i], m[i:]
		if dataDir != "" && (p == dataDir || strings.HasPrefix(p, dataDir+"/")) {
			return prefix + "<data>" + strings.TrimPrefix(p, dataDir)
		}
		base := p[strings.LastIndex(p, "/")+1:]
		return prefix + "…/" + base
	})
}

// tail returns the last n non-empty lines joined, for errorDetail.
func tail(lines []string, n int) string {
	start := max(0, len(lines)-n)
	return strings.Join(lines[start:], "\n")
}

// noiseFilter drops the block a SYSTEM python prints when one of its
// site-packages .pth files is broken ("Error processing line 1 of …pth:", an
// indented traceback, "Remainder of file ignored"). It is harmless, not ours,
// and read as the cause of the failure it preceded.
type noiseFilter struct {
	in bool
	n  int
}

var rePthNoise = regexp.MustCompile(`^Error processing line \d+ of .*\.pth:?$`)

// skip reports whether line belongs to the noise block.
func (f *noiseFilter) skip(line string) bool {
	t := strings.TrimSpace(line)
	// Left on purpose after conversion (torch is uninstalled): transformers'
	// advisory that no deep-learning framework is installed. Harmless, alarming.
	if strings.Contains(t, "None of PyTorch, TensorFlow >= 2.0, or Flax have been found") {
		return true
	}
	if !f.in {
		if rePthNoise.MatchString(t) {
			f.in, f.n = true, 0
			return true
		}
		return false
	}
	f.n++
	if strings.HasPrefix(t, "Remainder of file ignored") || f.n > 30 {
		f.in = false
		return strings.HasPrefix(t, "Remainder of file ignored")
	}
	return true
}
