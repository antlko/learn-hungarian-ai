package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// seedPairAudio registers fake clips for the fixture pack's pairs so sessions
// can be planned without real WAVs. `talkers` controls the floor test.
func seedPairAudio(t *testing.T, dataDir string, talkers []string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO minimal_pair VALUES
		(1,'kor','kór','o/ó',1,'возраст','болезнь','age','disease','',''),
		(2,'öt','őt','ö/ő',1,'пять','его','five','him','','')`); err != nil {
		t.Fatal(err)
	}
	for pair := 1; pair <= 2; pair++ {
		for _, m := range []string{"a", "b"} {
			for _, talker := range talkers {
				if _, err := db.Exec(`INSERT INTO pair_audio VALUES (?,?,?,?,1.0,?,?)`,
					pair, m, talker, "piper", "hvpt/x.wav", "CC0"); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func newEarServer(t *testing.T, talkers []string) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	seedPairAudio(t, dir, talkers)
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := &client{t: t, app: s}
	resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}
	return s, c
}

// [R7] end to end: below the configured floor the session is REFUSED with the
// numbers, and the refusal is a designed screen rather than an error status.
//
// The shipped floor is 3, because three is how many freely-licensed Hungarian
// voices exist and a floor nobody can meet is a feature nobody can use — see
// config.EvidenceMinTalkers for the number the literature actually asks for and
// the divergence recorded in docs/PEDAGOGY.md. Here we raise the floor above
// what the fixture supplies to exercise the refusal itself.
func TestEarSessionRefusedBelowFloorOverHTTP(t *testing.T) {
	_, c := newEarServer(t, []string{"anna", "berta"})

	if resp, body := c.do(http.MethodPut, "/api/settings", map[string]any{
		"hvpt": map[string]any{"minTalkers": 5, "trialsPerSession": 10, "rateJitter": 0.1},
	}); resp.StatusCode != 200 {
		t.Fatalf("raise the floor: %d %s", resp.StatusCode, body)
	}

	var res HVPTSessionResponse
	if resp := c.json(http.MethodGet, "/api/hvpt/session", nil, &res); resp.StatusCode != 200 {
		t.Fatalf("session: %d", resp.StatusCode)
	}
	if res.Refused == nil {
		t.Fatal("2 talkers must refuse a floor-5 session")
	}
	if res.Refused.Have != 2 || res.Refused.Need != 5 {
		t.Errorf("refusal numbers: %+v", res.Refused)
	}
	if len(res.Trials) != 0 {
		t.Error("a refused session must serve no trials")
	}
}

// And the shipped default trains out of the box: the pack's three synthetic
// voices meet the shipped floor, so a fresh install can actually use the Ear
// screen instead of meeting a wall on first open.
func TestEarSessionRunsAtTheShippedDefault(t *testing.T) {
	_, c := newEarServer(t, []string{"anna", "berta", "imre"})

	var res HVPTSessionResponse
	c.json(http.MethodGet, "/api/hvpt/session", nil, &res)
	if res.Refused != nil {
		t.Fatalf("the shipped default must not refuse three voices: %+v", res.Refused)
	}
	if len(res.Trials) == 0 {
		t.Fatal("no trials served at the shipped default")
	}
}

// With the floor met, the full loop runs: trials carry no answer, the answer
// reveals the played member, state accumulates, replay of an expired id is 410.
func TestEarTrainingLoop(t *testing.T) {
	_, c := newEarServer(t, []string{"a1", "a2", "a3", "a4", "a5"})

	var res HVPTSessionResponse
	c.json(http.MethodGet, "/api/hvpt/session", nil, &res)
	if res.Refused != nil || len(res.Trials) == 0 {
		t.Fatalf("session refused or empty: %+v", res.Refused)
	}
	if res.Talkers != 5 {
		t.Errorf("talkers = %d", res.Talkers)
	}

	// The wire payload must not contain the played member anywhere.
	raw, _ := json.Marshal(res.Trials[0])
	var asMap map[string]any
	json.Unmarshal(raw, &asMap)
	if _, leaked := asMap["played"]; leaked {
		t.Fatal("the played member leaked into the trial payload")
	}
	if asMap["audioUrl"] == "" {
		t.Fatal("a trial must carry its audio URL")
	}

	// Answer every trial with "a": some right, some wrong, all recorded.
	correct := 0
	for _, tr := range res.Trials {
		var ans HVPTAnswerResponse
		resp := c.json(http.MethodPost, "/api/hvpt/answer", HVPTAnswerRequest{
			TrialID: tr.ID, Chosen: "a", RtMs: 900,
		}, &ans)
		if resp.StatusCode != 200 {
			t.Fatalf("answer: %d", resp.StatusCode)
		}
		if ans.Played != "a" && ans.Played != "b" {
			t.Fatalf("the answer must reveal the played member, got %q", ans.Played)
		}
		if ans.Correct != (ans.Played == "a") {
			t.Error("correctness must equal chosen==played")
		}
		if ans.Correct {
			correct++
		}
	}

	// Replay is 410, not a second grade.
	resp, _ := c.do(http.MethodPost, "/api/hvpt/answer", HVPTAnswerRequest{
		TrialID: res.Trials[0].ID, Chosen: "b",
	})
	if resp.StatusCode != http.StatusGone {
		t.Errorf("replayed trial: want 410, got %d", resp.StatusCode)
	}

	// The contrast list now carries state — and NOTHING is ever complete.
	var contrasts struct {
		Contrasts []HVPTContrastDTO `json:"contrasts"`
		Talkers   int               `json:"talkers"`
	}
	c.json(http.MethodGet, "/api/hvpt/contrasts", nil, &contrasts)
	seen := 0
	for _, ct := range contrasts.Contrasts {
		if ct.Seen {
			seen++
			if ct.Total == 0 {
				t.Errorf("%s: seen with zero trials", ct.Contrast)
			}
		}
	}
	if seen == 0 {
		t.Error("answered contrasts must show state")
	}

	// Today's stats survive a session refetch.
	var again HVPTSessionResponse
	c.json(http.MethodGet, "/api/hvpt/session", nil, &again)
	if again.Today.Trials != len(res.Trials) {
		t.Errorf("today's trials = %d, want %d", again.Today.Trials, len(res.Trials))
	}
	if again.Today.Talkers < 2 {
		t.Errorf("distinct talkers today = %d — variety must be auditable", again.Today.Talkers)
	}
}

// The listening loop feeds the word-order gate: hours accumulate from reports
// and the curriculum reads them.
func TestListeningHoursFeedTheCurriculumGate(t *testing.T) {
	_, c := newEarServer(t, []string{"a1"})

	// Upload a material: multipart with audio + transcript.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("title", "Hírek")
	mw.WriteField("transcript", "A házban van a könyv. Látom a házat.")
	fw, _ := mw.CreateFormFile("audio", "clip.mp3")
	fw.Write(bytes.Repeat([]byte{0x55}, 2048))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/listen/material", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	resp, err := c.app.App().Test(req, fiber.TestConfig{Timeout: 90 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: %d", resp.StatusCode)
	}
	var created struct {
		ID        int64  `json:"id"`
		AudioPath string `json:"audioPath"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.AudioPath == "" {
		t.Fatal("no audio path")
	}

	// Report progress: 3 chunks of 60 s = 0.05 h.
	for i := 0; i < 3; i++ {
		var out struct {
			TotalHours float64 `json:"totalHours"`
		}
		c.json(http.MethodPost, "/api/listen/"+itoa64s(created.ID)+"/progress",
			ListenProgressRequest{Pass: 1, PositionS: float64(i * 60), SecondsHeard: 60}, &out)
		if i == 2 && out.TotalHours < 0.049 {
			t.Errorf("totalHours = %v, want ~0.05", out.TotalHours)
		}
	}

	// A giant fabricated report is clamped to one minute.
	var clamped struct {
		TotalHours float64 `json:"totalHours"`
	}
	c.json(http.MethodPost, "/api/listen/"+itoa64s(created.ID)+"/progress",
		ListenProgressRequest{Pass: 1, SecondsHeard: 999999}, &clamped)
	if clamped.TotalHours > 0.07 {
		t.Errorf("hours can be accumulated, not claimed: %v", clamped.TotalHours)
	}

	// The pass state persists.
	c.json(http.MethodPost, "/api/listen/"+itoa64s(created.ID)+"/progress",
		ListenProgressRequest{Pass: 2, PositionS: 42}, nil)
	var st ListenStateDTO
	c.json(http.MethodGet, "/api/listen/"+itoa64s(created.ID)+"/state", nil, &st)
	if st.Pass != 2 || st.PositionS != 42 {
		t.Errorf("state = %+v", st)
	}
	if st.ListenedS < 200 {
		t.Errorf("listened seconds = %v", st.ListenedS)
	}

	// And the curriculum sees the hours.
	var curr CurriculumResponse
	c.json(http.MethodGet, "/api/curriculum", nil, &curr)
	for _, topic := range curr.Topics {
		if topic.Code != "focus-word-order" {
			continue
		}
		if topic.Decision.Reason == "gate.locked.listening" {
			if topic.Decision.Args["heard"] == "0.0" {
				t.Error("the listening gate must see the accumulated hours")
			}
		}
	}
}

func itoa64s(n int64) string { return fmt.Sprintf("%d", n) }
