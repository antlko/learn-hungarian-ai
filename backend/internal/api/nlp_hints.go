package api

import (
	"context"
	"log/slog"
	"time"

	"tanulom/internal/morph"
	"tanulom/internal/nlp"
)

// Budgets for the whole sidecar round trip: ingest is a one-off write, while
// /api/analyze is interactive and must not wait long for an optional extra.
const (
	ingestHintsTimeout  = 10 * time.Second
	analyzeHintsTimeout = 3 * time.Second
)

// applyNLPHints is the opt-in step 4 of the lookup ladder (ADR-002, ADR-054).
// It runs only when the sidecar is configured AND the text has at least one
// word the pack found ambiguous, sends the whole text ONCE, and lets the
// sidecar choose among the pack's own candidates. Every failure — off,
// unreachable, loading, too large, bad offsets — returns the tokens untouched:
// the app is identical to one with no sidecar.
func (s *Server) applyNLPHints(ctx context.Context, budget time.Duration, text string, tokens []morph.AnalyzedToken) {
	client := nlp.New(s.service("nlp"))
	if client == nil {
		return
	}
	ambiguous := false
	for _, t := range tokens {
		if t.IsWord && t.Analysis.Certainty == morph.UncertainAnalysis {
			ambiguous = true
			break
		}
	}
	if !ambiguous {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	res, err := client.Analyze(ctx, text)
	if err != nil {
		slog.DebugContext(ctx, "nlp hints skipped", "err", err)
		return
	}
	hints := make([]morph.ContextHint, 0, len(res.Tokens))
	for _, t := range res.Tokens {
		hints = append(hints, morph.ContextHint{Start: t.Start, End: t.End, Lemma: t.Lemma, UPOS: t.UPOS})
	}
	if _, err := s.analyzer.ApplyContextHints(tokens, hints); err != nil {
		slog.DebugContext(ctx, "nlp hints failed", "err", err)
	}
}
