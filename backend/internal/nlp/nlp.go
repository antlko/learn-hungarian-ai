// Package nlp talks to the optional HuSpaCy sidecar (services/nlp).
//
// What it is for: choosing, in context, between readings the language pack
// ALREADY attests for an ambiguous form (`vár` = castle / waits). What it must
// never do: introduce an analysis. The package therefore returns raw tokens
// and nothing more; the choosing — constrained to the pack's own candidate
// list — lives in morph.ApplyContextHints. See ADR-054.
//
// Offsets on the wire are UTF-8 BYTES, like every offset on the server, and
// Analyze re-checks them against the text it sent: a token that does not land
// on rune boundaries inside the text is dropped, never repaired.
package nlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"tanulom/internal/config"
)

var (
	// ErrOff: no sidecar configured. Callers fall back silently.
	ErrOff = errors.New("nlp: no analysis service is configured")
	// ErrTooLarge: the sidecar answered 413.
	ErrTooLarge = errors.New("nlp: text too large for the service")
	// ErrCircuitOpen: recent calls kept failing; the call was not made.
	ErrCircuitOpen = errors.New("nlp: service skipped after repeated failures")
)

// Circuit breaker: after breakerThreshold consecutive failed Analyze calls to
// an endpoint, further calls are skipped for breakerCooldown, so a loading or
// dead sidecar cannot add its timeout to every request. Package-level (keyed
// by endpoint) because clients are built per call. Health/Warmup bypass it.
const (
	breakerThreshold = 3
	breakerCooldown  = 30 * time.Second
)

type breakerState struct {
	fails     int
	openUntil time.Time
}

var (
	breakerMu sync.Mutex
	breakers  = map[string]*breakerState{}
	now       = time.Now // replaced in tests
)

func (c *Client) allow() bool {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	b := breakers[c.endpoint]
	return b == nil || !now().Before(b.openUntil)
}

func (c *Client) record(ok bool) {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	b := breakers[c.endpoint]
	if b == nil {
		b = &breakerState{}
		breakers[c.endpoint] = b
	}
	if ok {
		b.fails, b.openUntil = 0, time.Time{}
		return
	}
	b.fails++
	if b.fails >= breakerThreshold {
		b.openUntil = now().Add(breakerCooldown)
		b.fails = 0
	}
}

const (
	ConnectTimeout = 3 * time.Second
	RequestTimeout = 10 * time.Second
	// MaxBody bounds what we read back from the service.
	MaxBody = 8 << 20
	// MaxChars mirrors the sidecar's NLP_MAX_CHARS default. Longer texts are
	// not sent at all (no hint beats a 413 round trip).
	MaxChars = 20000
)

// Token is one sidecar token, offsets in bytes of the text that was sent.
type Token struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
	Lemma string `json:"lemma"`
	UPOS  string `json:"upos"`
	Feats string `json:"feats"`
	Sent  int    `json:"sent"`
}

// Health is the sidecar's own account of itself.
type Health struct {
	// Status is "downloading", "loading", "ready" or "error".
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Model    string  `json:"model"`
	Version  string  `json:"version"`
	Error    string  `json:"error,omitempty"`
}

type Client struct {
	endpoint string
	http     *http.Client
}

// New returns nil when the service is off. Provider "local" is the only
// enabled value (config validation refuses everything else).
func New(svc config.Service) *Client {
	return newClient(svc.Provider, svc.Endpoint)
}

func newClient(provider, endpoint string) *Client {
	if provider != config.ProviderLocal || endpoint == "" {
		return nil
	}
	tr := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: ConnectTimeout}).DialContext,
		ResponseHeaderTimeout: RequestTimeout,
		DisableKeepAlives:     true,
	}
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		http: &http.Client{
			Transport: tr, Timeout: RequestTimeout,
			// Never follow a redirect: the configured endpoint is the only host we talk to.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// Only the transport error: never a body from the remote host.
		return nil, fmt.Errorf("nlp: %s %s: %w", method, path, err)
	}
	return resp, nil
}

func decodeHealth(resp *http.Response) (Health, error) {
	defer resp.Body.Close()
	var h Health
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&h); err != nil || h.Status == "" {
		return Health{}, fmt.Errorf("nlp: bad status response (HTTP %d)", resp.StatusCode)
	}
	h.Progress = min(1, max(0, h.Progress))
	return h, nil
}

// Health asks for the state. The sidecar answers 503 while not ready; the
// body is the point, so a non-2xx with a valid body is NOT an error.
func (c *Client) Health(ctx context.Context) (Health, error) {
	if c == nil {
		return Health{}, ErrOff
	}
	resp, err := c.do(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return Health{}, err
	}
	return decodeHealth(resp)
}

// Warmup asks the sidecar to download/load its model.
func (c *Client) Warmup(ctx context.Context) (Health, error) {
	if c == nil {
		return Health{}, ErrOff
	}
	resp, err := c.do(ctx, http.MethodPost, "/warmup", nil)
	if err != nil {
		return Health{}, err
	}
	return decodeHealth(resp)
}

type analyzeReq struct {
	Text      string `json:"text"`
	Sentences bool   `json:"sentences"`
}

type analyzeResp struct {
	Model  string  `json:"model"`
	Tokens []Token `json:"tokens"`
}

// Result is one analysed text.
type Result struct {
	Model  string
	Tokens []Token
}

// Analyze sends the whole text once. Tokens with offsets outside the text, or
// not on rune boundaries, or whose text does not match the slice they claim,
// are dropped.
func (c *Client) Analyze(ctx context.Context, text string) (Result, error) {
	if c == nil {
		return Result{}, ErrOff
	}
	if utf8.RuneCountInString(text) > MaxChars {
		return Result{}, ErrTooLarge
	}
	if !c.allow() {
		return Result{}, ErrCircuitOpen
	}
	body, err := json.Marshal(analyzeReq{Text: text, Sentences: true})
	if err != nil {
		return Result{}, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/analyze", body)
	if err != nil {
		c.record(false)
		return Result{}, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusRequestEntityTooLarge:
		return Result{}, ErrTooLarge // about this text, not the service's health
	case resp.StatusCode != http.StatusOK:
		c.record(false)
		return Result{}, fmt.Errorf("nlp: analyze returned HTTP %d", resp.StatusCode)
	}
	var out analyzeResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxBody)).Decode(&out); err != nil {
		c.record(false)
		return Result{}, fmt.Errorf("nlp: bad analyze response: %w", err)
	}
	c.record(true)
	return Result{Model: out.Model, Tokens: ValidTokens(text, out.Tokens)}, nil
}

// ValidTokens keeps only tokens whose byte span is inside text, on rune
// boundaries, and spells the same string the sidecar says it does.
func ValidTokens(text string, in []Token) []Token {
	out := make([]Token, 0, len(in))
	for _, t := range in {
		if t.Start < 0 || t.End <= t.Start || t.End > len(text) {
			continue
		}
		if !utf8.RuneStart(text[t.Start]) || (t.End < len(text) && !utf8.RuneStart(text[t.End])) {
			continue
		}
		if text[t.Start:t.End] != t.Text {
			continue
		}
		out = append(out, t)
	}
	return out
}
