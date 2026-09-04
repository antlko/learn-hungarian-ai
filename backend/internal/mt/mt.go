// Package mt talks to the optional translation sidecar.
//
// What this is for: a learner reading a Hungarian text wants, occasionally, to
// know what a sentence says — and the alternative to answering that here is
// them pasting the sentence into somebody else's chat window, which is worse
// for their privacy and breaks the reading loop [R11].
//
// What it is NOT for. This produces a CRIB, not content. Nothing it returns
// becomes a card, a gloss, or an answer, and every string that reaches the
// screen is labelled as machine output with the model named — see ADR-014.
// The distinction matters because the guardrail the project actually enforces
// (`internal/corpus/attest.go`) is about generated HUNGARIAN; this generates
// Russian and English, where the failure mode is not "a sentence nobody has
// written" but "a translation that is quietly wrong". Labelling is the only
// defence available, so it is not optional.
//
// The service is a dedicated translation model (NLLB-200), not a chat model,
// and it speaks its own protocol rather than the OpenAI chat API. That is why
// `mt` is a separate config slot from `llm` and why `provider: openai` is
// rejected for it.
package mt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tanulom/internal/config"
)

// ErrOff reports that no translator is configured. Callers turn this into a
// 503 with a reason, never into an empty result that looks like a translation.
var ErrOff = errors.New("mt: no translation service is configured")

// Timeout is generous by API standards because it is not an API: a CPU running
// a 600M-parameter model answers a long sentence in a second or two, and the
// alternative to waiting is the learner going to a different tab.
const Timeout = 20 * time.Second

// Languages the sidecar is asked for, keyed by the app's own locale codes. The
// values are NLLB's FLORES-200 codes; the mapping lives here rather than in the
// sidecar so the wire protocol stays about languages, not about our UI.
var flores = map[string]string{
	"ru": "rus_Cyrl",
	"en": "eng_Latn",
}

// Supported reports whether a target locale can be asked for at all.
func Supported(lang string) bool { _, ok := flores[lang]; return ok }

// Client is a translation sidecar. A nil Client is a valid "switched off"
// value: every method reports ErrOff rather than panicking, so the caller does
// not need a second flag beside it.
type Client struct {
	endpoint string
	http     *http.Client
}

// New returns a client, or nil when the service is switched off. `openai` is
// rejected in config validation, so only `local` reaches here with an endpoint.
func New(svc config.Service) *Client {
	if svc.Provider != config.ProviderLocal || svc.Endpoint == "" {
		return nil
	}
	return &Client{
		endpoint: strings.TrimRight(svc.Endpoint, "/"),
		http:     &http.Client{Timeout: Timeout},
	}
}

// Status is what the sidecar says about itself.
//
// It exists because the capability probe cannot carry this. That probe maps any
// 5xx to "unreachable", and a sidecar downloading 2.4 GB of weights answers
// /health with 503 — so "the model is coming" and "nothing is deployed" reached
// Settings as the same grey «нет связи». Everything a learner needs to tell
// those apart is in the body, and this is the type that carries it up.
type Status struct {
	// State is "loading", "ready" or "failed", straight from the service.
	State string `json:"state"`
	// Detail is the service's own reason when it failed. Passed through
	// verbatim rather than rewritten: a Python traceback tail is more use to
	// whoever has to fix it than a sentence we invent about it.
	Detail string `json:"detail"`
	Model  string `json:"model"`
	// DownloadedBytes/TotalBytes are the first-run download, and are zero when
	// the service cannot measure it. Zero means "draw an indeterminate bar",
	// never "0%": the difference between not knowing and knowing it is nothing
	// is the whole reason this project draws honest progress or none.
	DownloadedBytes int64 `json:"downloadedBytes"`
	TotalBytes      int64 `json:"totalBytes"`
}

type wireStatus struct {
	Status          string `json:"status"`
	Detail          string `json:"detail"`
	Model           string `json:"model"`
	DownloadedBytes int64  `json:"downloadedBytes"`
	TotalBytes      int64  `json:"totalBytes"`
}

func (w wireStatus) toStatus() Status {
	return Status{
		State: w.Status, Detail: w.Detail, Model: w.Model,
		DownloadedBytes: w.DownloadedBytes, TotalBytes: w.TotalBytes,
	}
}

// statusTimeout is short on purpose: this is polled while a model downloads,
// and a status call that hangs for the translation timeout would stack up
// behind itself and tell the learner nothing for twenty seconds at a time.
const statusTimeout = 3 * time.Second

// Status asks the sidecar what it is doing. `/health` answers 503 while
// loading, which is correct and is NOT an error here — the body is the point.
func (c *Client) Status(ctx context.Context) (Status, error) {
	if c == nil {
		return Status{}, ErrOff
	}
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/health", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("mt: %w", err)
	}
	defer resp.Body.Close()

	var out wireStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&out); err != nil {
		return Status{}, fmt.Errorf("mt: bad status response: %w", err)
	}
	if out.Status == "" {
		return Status{}, fmt.Errorf("mt: service returned %d with no status", resp.StatusCode)
	}
	return out.toStatus(), nil
}

// Warmup asks the sidecar to fetch the model, or to try again after a failure.
//
// The point of this endpoint is that a download that died halfway — no disk, no
// network, a rate limit — can be retried from Settings. Before it, the only
// remedy was restarting the container, which on a deployed instance means SSH.
//
// It takes the translation timeout rather than the status one: the service
// answers as soon as it has STARTED, but a cold start still has to reach the
// handler.
func (c *Client) Warmup(ctx context.Context) (Status, error) {
	if c == nil {
		return Status{}, ErrOff
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/warmup", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("mt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Status{}, fmt.Errorf("mt: warmup returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var out wireStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&out); err != nil {
		return Status{}, fmt.Errorf("mt: bad warmup response: %w", err)
	}
	return out.toStatus(), nil
}

// Result is one batch of translations, in the order the texts were given.
type Result struct {
	Texts []string
	Model string
}

type wireRequest struct {
	Texts  []string `json:"texts"`
	Target string   `json:"target"`
}

type wireResponse struct {
	Texts []string `json:"texts"`
	Model string   `json:"model"`
}

// Translate renders texts into lang ("ru" or "en").
//
// The whole batch fails or the whole batch succeeds. Returning a partial result
// would mean deciding, here, which sentence to silently drop — and a reader who
// asked for a translation and got a blank line has been told nothing.
func (c *Client) Translate(ctx context.Context, texts []string, lang string) (Result, error) {
	if c == nil {
		return Result{}, ErrOff
	}
	target, ok := flores[lang]
	if !ok {
		return Result{}, fmt.Errorf("mt: no translation target for locale %q", lang)
	}
	if len(texts) == 0 {
		return Result{}, nil
	}

	body, err := json.Marshal(wireRequest{Texts: texts, Target: target})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/translate", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("mt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Result{}, fmt.Errorf("mt: service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var out wireResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Result{}, fmt.Errorf("mt: bad response: %w", err)
	}
	// A short batch would silently pair sentence 3's translation with sentence
	// 2 — the one failure of this service that would look like a working one.
	if len(out.Texts) != len(texts) {
		return Result{}, fmt.Errorf("mt: asked for %d translations, got %d", len(texts), len(out.Texts))
	}
	return Result{Texts: out.Texts, Model: out.Model}, nil
}
