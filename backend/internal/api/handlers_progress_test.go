package api

import (
	"net/http"
	"testing"

	"tanulom/internal/drill"
	"tanulom/internal/stats"
)

// The whole Phase-7 spine: interactions fill the ledger, the ledger drives the
// streak, checkpoints read the evidence, a failure blocks the curriculum, a
// retake unblocks it.
func TestProgressLedgerStreakAndCheckpoints(t *testing.T) {
	_, c := newPackedServer(t)

	// A fresh account: overview exists, streak zero, next checkpoint is m1.
	var ov OverviewResponse
	if resp := c.json(http.MethodGet, "/api/stats/overview", nil, &ov); resp.StatusCode != 200 {
		t.Fatalf("overview: %d", resp.StatusCode)
	}
	if ov.Streak.Current != 0 || ov.Streak.TodayMet {
		t.Errorf("fresh streak = %+v", ov.Streak)
	}
	if ov.NextCheckpoint == nil || ov.NextCheckpoint.Spec.Code != "m1" {
		t.Fatalf("next checkpoint = %+v, want m1", ov.NextCheckpoint)
	}

	// Answer a review: the ledger records it.
	c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
	}, nil)
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	if len(queue.Items) == 0 {
		t.Fatal("no queue")
	}
	c.json(http.MethodPost, "/api/review/answer", AnswerRequest{
		CardID: queue.Items[0].CardID, Rating: 3, DurationMs: 5000,
	}, nil)

	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if ov.Today.Reviews != 1 || ov.Today.ActiveMs < 5000 {
		t.Errorf("ledger after one review: %+v", ov.Today)
	}

	// Loop 4 attestation lands in the ledger and does NOT change the streak.
	c.json(http.MethodPost, "/api/today/human", HumanAttestRequest{Happened: true}, nil)
	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if !ov.Today.Human {
		t.Error("attestation not recorded")
	}
	if ov.Streak.TodayMet {
		t.Error("attestation must not satisfy the obligations")
	}

	// Fail m1 (zero lemmas known, nothing attested) → diagnostic appears and
	// nothing is blocked (m1 gates no level).
	var cp CheckpointResponse
	c.json(http.MethodPost, "/api/checkpoints", CheckpointRequest{Code: "m1"}, &cp)
	if cp.Checkpoint.Status != "failed" {
		t.Fatalf("m1 with no lemmas must fail, got %s", cp.Checkpoint.Status)
	}
	if cp.Diagnostic == nil || len(cp.Diagnostic.Findings) == 0 {
		t.Fatal("a failure must open the diagnostic")
	}
	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if len(ov.BlockedLevels) != 0 {
		t.Errorf("m1 gates nothing, got %v", ov.BlockedLevels)
	}

	// Fail m3 → A2 and above lock in the curriculum with the checkpoint reason.
	c.json(http.MethodPost, "/api/checkpoints", CheckpointRequest{Code: "m3"}, &cp)
	if cp.Checkpoint.Status != "failed" {
		t.Fatal("m3 must fail too")
	}
	var curr CurriculumResponse
	c.json(http.MethodGet, "/api/curriculum", nil, &curr)
	blockedSeen := false
	for _, topic := range curr.Topics {
		if topic.Level == "A2" || topic.Level == "B1" || topic.Level == "B2" {
			if topic.Phase != string(drill.Locked) {
				t.Errorf("%s (%s): phase %s — a failed m3 must lock it", topic.Code, topic.Level, topic.Phase)
			}
			if topic.Decision.Reason == string(drill.ReasonCheckpoint) {
				blockedSeen = true
			}
		}
		if topic.Level == "A1" && topic.Decision.Reason == string(drill.ReasonCheckpoint) {
			t.Errorf("%s: A1 must stay open for repair", topic.Code)
		}
	}
	if !blockedSeen {
		t.Error("no topic carries the checkpoint reason")
	}

	// The evidence stored with the verdict names the three failure modes' data.
	if cp.Checkpoint.Evidence.LemmaTarget != 900 {
		t.Errorf("m3 target = %d", cp.Checkpoint.Evidence.LemmaTarget)
	}

	// A retake that passes lifts the block. Give the account the lemmas and the
	// attestation. 900 lemmas via direct state writes would be slow over HTTP;
	// use the store's own contract instead: mark known lemmas.
	seedLemmas(t, c, 900)
	var cp2 CheckpointResponse
	c.json(http.MethodPost, "/api/checkpoints", CheckpointRequest{
		Code: "m3", Attested: []string{"checkpoint.m3.monologue"},
	}, &cp2)
	if cp2.Checkpoint.Status != "passed" {
		t.Fatalf("retake with lemmas+attestation must pass, got %+v", cp2.Checkpoint)
	}
	c.json(http.MethodGet, "/api/curriculum", nil, &curr)
	for _, topic := range curr.Topics {
		if topic.Decision.Reason == string(drill.ReasonCheckpoint) {
			t.Errorf("%s still checkpoint-blocked after the pass", topic.Code)
		}
	}

	// History keeps every attempt.
	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if len(ov.Checkpoints) != 3 {
		t.Errorf("history = %d entries, want 3", len(ov.Checkpoints))
	}
	_ = stats.Specs
}

// seedLemmas marks n pack lemmas as learning via the knowledge endpoint's
// bulk behaviour — direct SQL, because the fixture pack has only 7 lemmas.
func seedLemmas(t *testing.T, c *client, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := c.app.db.Exec(`
			INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (1, ?, 'learning')
			ON CONFLICT(user_id, lemma_id) DO UPDATE SET status='learning'`, 100000+i); err != nil {
			t.Fatal(err)
		}
	}
}
