// Package llm is a minimal client for an OpenAI-compatible chat endpoint — the
// consumer of the `services.llm` slot.
//
// Two calls, no retries, no tool use: one blocking completion (a text-level
// estimate needs a few dozen tokens) and one streamed completion (the grammar
// tutor, where a local model on a CPU takes seconds per sentence and an answer
// that appears all at once after twenty of them reads as a hang).
//
// Anything this package returns is MACHINE OUTPUT and reaches the UI only
// labelled as such (ADR-023; the labelling precedent is ADR-014). It never
// generates Hungarian learning material, so internal/corpus attestation is not
// implicated — ADR-027 keeps the tutor on the explaining side of that line.
package llm

import (
	"bufio"
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

// Client speaks POST {endpoint}/v1/chat/completions.
type Client struct {
	endpoint string
	model    string
	key      string
	// local is `provider: local` — Ollama, llama.cpp, vLLM on the learner's
	// own box — as opposed to a hosted API. See request.
	local bool
	http  *http.Client
}

// ErrOff is returned when the llm service is not configured.
var ErrOff = errors.New("llm service is not configured")

// ErrNoModel is returned when the service is on but names no model. Every
// request carries the model's name and every answer is labelled with it, and
// Ollama and the hosted APIs refuse a blank one ("model is required") — so a
// client built without one could only ever fail, and it failed as «модель не
// ответила пригодным образом» on a settings row that said «на связи». The
// first learner to plug in an Ollama hit exactly that. Reported separately
// from ErrOff so the panel can say which dial to turn.
var ErrNoModel = errors.New("llm service names no model")

// New builds a client from the service config and the stored key. The key may
// be empty — a local Ollama needs none.
func New(svc config.Service, key string) (*Client, error) {
	if svc.Provider == "" || svc.Provider == config.ProviderOff || svc.Endpoint == "" {
		return nil, ErrOff
	}
	if strings.TrimSpace(svc.Model) == "" {
		return nil, ErrNoModel
	}
	return &Client{
		endpoint: strings.TrimRight(svc.Endpoint, "/"),
		model:    svc.Model,
		key:      key,
		local:    svc.Provider == config.ProviderLocal,
		// No client-side timeout: each call bounds itself through its context,
		// and a streamed answer legitimately runs longer than a level estimate.
		http: &http.Client{},
	}, nil
}

// ErrTruncated: the model stopped because max_tokens ran out, not because it
// had finished. Everything it wrote has already been delivered; the caller
// marks the answer as cut rather than passing it off as complete.
var ErrTruncated = errors.New("llm: answer cut off by max_tokens")

// Message is one turn of a chat.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	// MaxTokens bounds the answer; a level estimate is a few dozen tokens.
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature"`
	Stream      bool    `json:"stream,omitempty"`
	// ReasoningEffort is "none" for a local server. A thinking model (qwen3
	// is Ollama's default suggestion) spends its hidden reasoning out of the
	// same max_tokens as the answer: measured on qwen3:1.7b, six hundred
	// characters of English deliberation before the first Russian word, and
	// with the tutor's full prompt that is the whole budget — the stream ends
	// on `length` with nothing said. Ollama's OpenAI endpoint maps this value
	// to thinking off (its `think` field is ignored there); llama.cpp and
	// vLLM accept or ignore it. A hosted API is left at the vendor's default,
	// because a non-reasoning hosted model rejects the field outright.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// StreamOptions asks for the usage figures on the final chunk — OpenAI's
	// own field, honoured by Ollama, vLLM and llama.cpp and ignored by the
	// rest. The context meter (ADR-031) is built on it.
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// Usage is what the server reported spending on one answer, in its own
// tokens: the prompt as the model actually saw it and the answer as written.
// Zero when the server did not say — the caller prints "not reported", never
// a guess.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// modelList is GET {endpoint}/v1/models.
type modelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// streamChunk is one server-sent event of a streamed completion.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		// FinishReason arrives on the last chunk: "stop" when the model
		// finished, "length" when max_tokens ran out first.
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	// Usage rides on a final chunk with no choices when stream_options asked
	// for it.
	Usage *Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Model names what will answer, for the labels the honesty rules require.
func (c *Client) Model() string {
	if c.model == "" {
		return "llm"
	}
	return c.model
}

func (c *Client) request(ctx context.Context, body chatRequest) (*http.Response, error) {
	if c.local {
		body.ReasoningEffort = "none"
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.endpoint+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}
	return res, nil
}

// ListModels asks the endpoint which models it serves — GET /v1/models, the
// one OpenAI-compatible route Ollama, llama.cpp, vLLM and the vendors all
// answer. The settings probe uses it to say «укажите модель» with the names
// to choose from, and to notice a name the server does not carry, instead of
// reporting «на связи» for an endpoint that will refuse every request.
//
// It needs no model, so it is callable on a config New would refuse.
func ListModels(ctx context.Context, svc config.Service, key string) ([]string, error) {
	if svc.Provider == "" || svc.Provider == config.ProviderOff || svc.Endpoint == "" {
		return nil, ErrOff
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(svc.Endpoint, "/")+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm models: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("llm models: status %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("llm models: %w", err)
	}
	var out modelList
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("llm models: non-JSON body")
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// Chat sends one system+user exchange and returns the assistant's text.
func (c *Client) Chat(ctx context.Context, system, user string, maxTokens int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	res, err := c.request(ctx, chatRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("llm response: %w", err)
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("llm answered status %d with a non-JSON body", res.StatusCode)
	}
	if out.Error != nil {
		return "", fmt.Errorf("llm error: %s", out.Error.Message)
	}
	if res.StatusCode >= 400 || len(out.Choices) == 0 {
		return "", fmt.Errorf("llm answered status %d with no choices", res.StatusCode)
	}
	return out.Choices[0].Message.Content, nil
}

// Stream sends a whole conversation and hands each delta of the answer to
// onDelta as it arrives. It returns when the model finishes, the context ends,
// or onDelta returns an error (the client went away) — and ErrTruncated,
// after delivering everything, when the model stopped for max_tokens rather
// than because it was done. The Usage is whatever the server reported, zero
// when it reported nothing; it is valid alongside ErrTruncated.
//
// Speaks the OpenAI streaming wire format — `data: {json}` events terminated
// by `data: [DONE]` — which Ollama, llama.cpp, vLLM and the vendors all share.
func (c *Client) Stream(ctx context.Context, messages []Message, maxTokens int,
	temperature float64, onDelta func(string) error) (Usage, error) {
	var usage Usage
	res, err := c.request(ctx, chatRequest{
		Model: c.model, MaxTokens: maxTokens, Temperature: temperature,
		Messages: messages, Stream: true, StreamOptions: &streamOptions{IncludeUsage: true},
	})
	if err != nil {
		return usage, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		// An error body is plain JSON, not an event stream.
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		var out chatResponse
		if json.Unmarshal(raw, &out) == nil && out.Error != nil {
			return usage, fmt.Errorf("llm error: %s", out.Error.Message)
		}
		return usage, fmt.Errorf("llm answered status %d", res.StatusCode)
	}

	rd := bufio.NewReaderSize(res.Body, 64<<10)
	truncated := false
	done := func() error {
		if truncated {
			return ErrTruncated
		}
		return nil
	}
	for {
		line, err := rd.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if data, ok := strings.CutPrefix(line, "data:"); ok {
				data = strings.TrimSpace(data)
				if data == "[DONE]" {
					return usage, done()
				}
				var chunk streamChunk
				if json.Unmarshal([]byte(data), &chunk) != nil {
					// A malformed event is skipped rather than fatal: the
					// stream may still carry the rest of the answer.
					continue
				}
				if chunk.Error != nil {
					return usage, fmt.Errorf("llm error: %s", chunk.Error.Message)
				}
				if chunk.Usage != nil {
					usage = *chunk.Usage
				}
				for _, ch := range chunk.Choices {
					if ch.FinishReason == "length" {
						truncated = true
					}
					if ch.Delta.Content == "" {
						continue
					}
					if err := onDelta(ch.Delta.Content); err != nil {
						return usage, err
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return usage, done()
			}
			return usage, fmt.Errorf("llm stream: %w", err)
		}
	}
}

// psList is GET {endpoint}/api/ps — Ollama's list of running models.
type psList struct {
	Models []struct {
		Name          string `json:"name"`
		Model         string `json:"model"`
		ContextLength int    `json:"context_length"`
	} `json:"models"`
}

// ContextLength asks an Ollama server what window the model is running with
// — GET {endpoint}/api/ps, the one place the figure exists. Ollama picks it
// from the machine's memory or the desktop app's slider, the OpenAI-compatible
// routes never mention it, and a prompt longer than it is cut from the HEAD
// without a word (docs/TUTOR.md §9); the context meter (ADR-031) prints it so
// that cut is visible. Meaningful right after an answer, while the model is
// loaded. Returns 0 — "not reported" — for a hosted provider, a server that is
// not Ollama, a model that is not running, or any error; the caller says so
// rather than guessing.
func ContextLength(ctx context.Context, svc config.Service, model string) int {
	if svc.Provider != config.ProviderLocal || svc.Endpoint == "" || model == "" {
		return 0
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(svc.Endpoint, "/")+"/api/ps", nil)
	if err != nil {
		return 0
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0
	}
	var out psList
	if json.Unmarshal(raw, &out) != nil {
		return 0
	}
	for _, m := range out.Models {
		if m.Name == model || m.Model == model {
			return m.ContextLength
		}
	}
	return 0
}
