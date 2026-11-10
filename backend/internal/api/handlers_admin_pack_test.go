package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tanulom/internal/lang"
)

// gzPack returns the server's current test pack gzipped, after running
// mutate against an on-disk copy (nil = identical ids and words).
func gzPack(t *testing.T, s *Server, mutate string) ([]byte, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.dataDir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if mutate != "" {
		p := filepath.Join(t.TempDir(), "m.sqlite")
		if err := os.WriteFile(p, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(mutate); err != nil {
			t.Fatal(err)
		}
		db.Close()
		if raw, err = os.ReadFile(p); err != nil {
			t.Fatal(err)
		}
	}
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Write(raw)
	zw.Close()
	h := sha256.Sum256(b.Bytes())
	return b.Bytes(), hex.EncodeToString(h[:])
}

// pinPack points langpack.* at a test server through the real settings route.
func pinPack(t *testing.T, c *client, url, sha string) {
	t.Helper()
	resp, body := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{LangPack: &LangPackDTO{URL: url, SHA256: sha}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pin pack: %d %s", resp.StatusCode, body)
	}
}

func waitInstall(t *testing.T, c *client) PackInstallStatusDTO {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		var st PackInstallStatusDTO
		c.json(http.MethodGet, "/api/admin/pack/install", nil, &st)
		if st.State != "downloading" && st.State != "verifying" {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("install still %s", st.State)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAdminPackRoutesAreAdminOnly(t *testing.T) {
	s, admin := newAdmin(t)
	addUser(t, admin, "bea@example.com", "bea elég hosszú jelszó", "learner")
	learner := signIn(t, s, "bea@example.com", "bea elég hosszú jelszó")
	for _, rt := range [][2]string{
		{http.MethodPost, "/api/admin/pack/install"}, {http.MethodGet, "/api/admin/pack/install"},
		{http.MethodPost, "/api/admin/restart"}, {http.MethodGet, "/api/admin/components"},
	} {
		if resp, _ := learner.do(rt[0], rt[1], nil); resp.StatusCode != http.StatusForbidden {
			t.Errorf("learner %s %s = %d, want 403", rt[0], rt[1], resp.StatusCode)
		}
		anon := &client{t: t, app: s}
		if resp, _ := anon.do(rt[0], rt[1], nil); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s = %d, want 401", rt[0], rt[1], resp.StatusCode)
		}
	}
}

func TestPackInstallNotPublished(t *testing.T) {
	_, c := newPackedServer(t)
	var e struct{ Error string }
	resp := c.json(http.MethodPost, "/api/admin/pack/install", nil, &e)
	if resp.StatusCode != http.StatusUnprocessableEntity || e.Error != "errors.packNotPublished" {
		t.Errorf("default config: %d %q, want 422 errors.packNotPublished", resp.StatusCode, e.Error)
	}
}

func TestPackInstallHappyPathAndBusy(t *testing.T) {
	s, c := newPackedServer(t)
	body, sha := gzPack(t, s, "")
	origLive, _ := os.ReadFile(filepath.Join(s.dataDir, "langpack-hu-v1.sqlite"))
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Write(body)
	}))
	defer srv.Close()
	pinPack(t, c, srv.URL, sha)

	// The learner database points at lemma 1: it must still mean ház.
	if _, err := s.db.DB.Exec(`INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (1, 1, 'learning')`); err != nil {
		t.Fatal(err)
	}

	if resp, b := c.do(http.MethodPost, "/api/admin/pack/install", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("start: %d %s", resp.StatusCode, b)
	}
	if resp, _ := c.do(http.MethodPost, "/api/admin/pack/install", nil); resp.StatusCode != http.StatusConflict {
		t.Errorf("second start = %d, want 409", resp.StatusCode)
	}
	var comps []ComponentDTO
	c.json(http.MethodGet, "/api/admin/components", nil, &comps)
	if comps[0].ID != "pack" || comps[0].State != "installing" {
		t.Errorf("pack component while running = %+v", comps[0])
	}
	close(release)

	st := waitInstall(t, c)
	if st.State != "ready" || !st.RestartRequired || st.ErrorKey != "" {
		t.Fatalf("status = %+v", st)
	}
	c.json(http.MethodGet, "/api/admin/components", nil, &comps)
	if comps[0].State != "restartRequired" {
		t.Errorf("pack component after install = %+v", comps[0])
	}

	// The live file is untouched until the next boot applies the parked one.
	livePath := filepath.Join(s.dataDir, "langpack-hu-v1.sqlite")
	if _, err := os.Stat(livePath + ".new"); err != nil {
		t.Fatalf("no parked pack: %v", err)
	}
	if now, _ := os.ReadFile(livePath); !bytes.Equal(now, origLive) {
		t.Error("the live pack changed before the boot-apply step")
	}
	if res := lang.ApplyPending(context.Background(), s.dataDir, s.db.DB); !res.Applied {
		t.Fatalf("boot apply = %+v", res)
	}
	liveAfter, _ := os.ReadFile(livePath)
	if len(liveAfter) == 0 {
		t.Error("applied pack is empty")
	}
}

func TestPackInstallRefusesIncompatibleIDs(t *testing.T) {
	s, c := newPackedServer(t)
	// Same ids, but id 1 is now a different word: every note on it would lie.
	body, sha := gzPack(t, s, `UPDATE lemma SET lemma = 'macska' WHERE id = 1`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer srv.Close()
	pinPack(t, c, srv.URL, sha)
	if _, err := s.db.DB.Exec(`INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (1, 1, 'learning')`); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(s.dataDir, "langpack-hu-v1.sqlite"))

	if resp, b := c.do(http.MethodPost, "/api/admin/pack/install", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("start: %d %s", resp.StatusCode, b)
	}
	st := waitInstall(t, c)
	if st.State != "failed" || st.ErrorKey != "errors.packIncompatible" || st.RestartRequired {
		t.Errorf("status = %+v", st)
	}
	after, _ := os.ReadFile(filepath.Join(s.dataDir, "langpack-hu-v1.sqlite"))
	if !bytes.Equal(before, after) {
		t.Error("the live pack changed")
	}
}

func TestPackInstallWrongChecksum(t *testing.T) {
	s, c := newPackedServer(t)
	body, _ := gzPack(t, s, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer srv.Close()
	pinPack(t, c, srv.URL, strings.Repeat("ab", 32))
	c.do(http.MethodPost, "/api/admin/pack/install", nil)
	if st := waitInstall(t, c); st.State != "failed" || st.ErrorKey != "errors.packChecksum" {
		t.Errorf("status = %+v", st)
	}
}

func TestLangPackSettingsValidation(t *testing.T) {
	_, c := newPackedServer(t)
	for _, bad := range []LangPackDTO{
		{URL: "http://example.com/p.gz"}, {URL: "ftp://x/p.gz"}, {URL: "https://x/p.gz", SHA256: "xyz"},
	} {
		resp, _ := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{LangPack: &bad})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%+v accepted: %d", bad, resp.StatusCode)
		}
	}
	var st SettingsDTO
	c.json(http.MethodGet, "/api/settings", nil, &st)
	if !strings.HasPrefix(st.LangPack.URL, "https://") || st.LangPack.SHA256 != "" {
		t.Errorf("defaults = %+v", st.LangPack)
	}
}

func TestAdminRestart(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want bool
	}{{"1", true}, {"", false}} {
		t.Setenv("TANULOM_SUPERVISED", tc.env)
		s, c := newPackedServer(t)
		stopped := make(chan struct{}, 1)
		s.stopProcess = func() { stopped <- struct{}{} }
		var out RestartResponse
		if resp := c.json(http.MethodPost, "/api/admin/restart", nil, &out); resp.StatusCode != http.StatusAccepted {
			t.Fatalf("restart = %d", resp.StatusCode)
		}
		if out.Supervised != tc.want {
			t.Errorf("env %q: supervised = %v", tc.env, out.Supervised)
		}
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Errorf("env %q: the process was never asked to stop", tc.env)
		}
	}
}

func TestAdminComponentsShape(t *testing.T) {
	_, c := newPackedServer(t)
	var comps []ComponentDTO
	if resp := c.json(http.MethodGet, "/api/admin/components", nil, &comps); resp.StatusCode != 200 {
		t.Fatalf("components = %d", resp.StatusCode)
	}
	ids := []string{}
	byID := map[string]ComponentDTO{}
	for _, d := range comps {
		ids = append(ids, d.ID)
		byID[d.ID] = d
	}
	if strings.Join(ids, ",") != "pack,mt,nlp,tts,asr,llm" {
		t.Errorf("ids = %v", ids)
	}
	if p := byID["pack"]; !p.Reachable || (p.State != "starter" && p.State != "full") {
		t.Errorf("pack = %+v", p)
	}
	if !byID["nlp"].Built || byID["nlp"].State != "off" || byID["nlp"].RAMHint == "" {
		t.Errorf("nlp = %+v", byID["nlp"])
	}
	if byID["tts"].State != "device" {
		t.Errorf("tts = %+v", byID["tts"])
	}
	if !byID["mt"].Built || byID["mt"].ComposeProfile != "mt" {
		t.Errorf("mt = %+v", byID["mt"])
	}
}

// Every column of the learner database that points into the pack must be in
// lang's id-compatibility list, or an install could re-point a learner's data
// at different words without anyone noticing.
func TestIDRefsCoverTheLearnerSchema(t *testing.T) {
	s, _ := newTestServer(t)
	lemma, sentence := lang.IDRefs()
	listed := map[string]bool{}
	for _, r := range append(lemma, sentence...) {
		listed[r[0]+"."+r[1]] = true
	}
	rows, err := s.db.DB.Query(`SELECT m.name, p.name FROM sqlite_master m, pragma_table_info(m.name) p
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%' AND p.name IN ('lemma_id','sentence_id')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col string
		rows.Scan(&tbl, &col)
		if !listed[tbl+"."+col] {
			t.Errorf("%s.%s points into the pack but is not checked by lang.CheckIDCompat", tbl, col)
		}
	}
}

// A pin compiled into the binary is used when config.yaml stores none: the
// URL is the test server's, the checksum comes from the "binary".
func TestPackInstallUsesBinaryPinnedChecksum(t *testing.T) {
	s, c := newPackedServer(t)
	body, sha := gzPack(t, s, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer srv.Close()
	old := packDefaultSHA
	packDefaultSHA = sha
	defer func() { packDefaultSHA = old }()
	pinPack(t, c, srv.URL, "")

	var st PackInstallStatusDTO
	c.json(http.MethodGet, "/api/admin/pack/install", nil, &st)
	if !st.Published || st.URL != srv.URL {
		t.Errorf("status = %+v", st)
	}
	if resp, b := c.do(http.MethodPost, "/api/admin/pack/install", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("start: %d %s", resp.StatusCode, b)
	}
	if got := waitInstall(t, c); got.State != "ready" {
		t.Errorf("install = %+v", got)
	}
}

// An unpublished pack (empty pin, nothing stored) makes no network request.
func TestPackInstallUnpublishedMakesNoRequest(t *testing.T) {
	_, c := newPackedServer(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer srv.Close()
	pinPack(t, c, srv.URL, "")
	var st PackInstallStatusDTO
	c.json(http.MethodGet, "/api/admin/pack/install", nil, &st)
	if st.Published {
		t.Errorf("published = true with no pin: %+v", st)
	}
	var e struct{ Error string }
	resp := c.json(http.MethodPost, "/api/admin/pack/install", nil, &e)
	if resp.StatusCode != http.StatusUnprocessableEntity || e.Error != "errors.packNotPublished" || hits != 0 {
		t.Errorf("%d %q hits=%d", resp.StatusCode, e.Error, hits)
	}
}
