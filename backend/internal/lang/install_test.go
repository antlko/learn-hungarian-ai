package lang

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildPackGz writes a minimal valid pack (the tables Counts reads) with one
// lemma per given name, ids 1..n, and returns it gzipped.
func buildPackGz(t *testing.T, lemmas ...string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "p.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`INSERT INTO pack_meta VALUES ('version','hu-v1'),('schema_version','1'),('gloss_langs','["en"]'),('edition','full')`,
		`CREATE TABLE lemma (id INTEGER PRIMARY KEY, lemma TEXT, upos TEXT)`,
		`CREATE TABLE wordform (form TEXT)`,
		`CREATE TABLE sentence (id INTEGER PRIMARY KEY, hu TEXT)`,
		`CREATE TABLE morph_feature (id TEXT)`,
		`CREATE TABLE grammar_topic (id INTEGER)`,
		`CREATE TABLE minimal_pair (id INTEGER)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i, l := range lemmas {
		if _, err := db.Exec(`INSERT INTO lemma VALUES (?,?,'NOUN')`, i+1, l); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return gz(t, raw)
}

func gz(t *testing.T, raw []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Write(raw)
	zw.Close()
	return b.Bytes()
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func serve(t *testing.T, body []byte) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	t.Cleanup(srv.Close)
	return srv
}

func finish(t *testing.T, in *Installer) InstallStatus {
	t.Helper()
	done := make(chan struct{})
	go func() { in.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("install did not finish")
	}
	return in.Status()
}

func noTemps(t *testing.T, dir string) {
	t.Helper()
	for _, suffix := range []string{".part", ".unpacked"} {
		if _, err := os.Stat(Path(dir, "1") + suffix); !os.IsNotExist(err) {
			t.Errorf("temp file %s left behind", suffix)
		}
	}
}

// liveFile writes a marker "current pack" so a test can prove it survives.
func liveFile(t *testing.T, dir string) []byte {
	t.Helper()
	b := []byte("the live pack")
	if err := os.WriteFile(Path(dir, "1"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

func assertLive(t *testing.T, dir string, want []byte) {
	t.Helper()
	got, _ := os.ReadFile(Path(dir, "1"))
	if !bytes.Equal(got, want) {
		t.Error("the live pack was touched by a failed install")
	}
}

func TestInstallEmptyChecksumRefusesBeforeAnyRequest(t *testing.T) {
	dir := t.TempDir()
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()
	err := NewInstaller(dir).Start(context.Background(), InstallSpec{URL: srv.URL})
	if !errors.Is(err, ErrNotPublished) || hit {
		t.Fatalf("err=%v requested=%v, want ErrNotPublished and no request", err, hit)
	}
}

func TestInstallWrongChecksum(t *testing.T) {
	dir := t.TempDir()
	live := liveFile(t, dir)
	body := buildPackGz(t, "ház")
	in := NewInstaller(dir)
	if err := in.Start(context.Background(), InstallSpec{URL: serve(t, body).URL, SHA256: sum([]byte("other"))}); err != nil {
		t.Fatal(err)
	}
	st := finish(t, in)
	if st.State != InstallFailed || st.ErrorKey != ErrKeyChecksum || st.RestartRequired {
		t.Errorf("status = %+v", st)
	}
	assertLive(t, dir, live)
	noTemps(t, dir)
}

func TestInstallTruncatedStream(t *testing.T) {
	dir := t.TempDir()
	live := liveFile(t, dir)
	body := buildPackGz(t, "ház")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.Write(body[:len(body)/2])
	}))
	defer srv.Close()
	in := NewInstaller(dir)
	if err := in.Start(context.Background(), InstallSpec{URL: srv.URL, SHA256: sum(body)}); err != nil {
		t.Fatal(err)
	}
	st := finish(t, in)
	if st.State != InstallFailed || st.ErrorKey != ErrKeyDownload {
		t.Errorf("status = %+v", st)
	}
	assertLive(t, dir, live)
	noTemps(t, dir)
}

func TestInstallIDMismatchRefused(t *testing.T) {
	dir := t.TempDir()
	live := liveFile(t, dir)
	body := buildPackGz(t, "ház")
	in := NewInstaller(dir)
	err := in.Start(context.Background(), InstallSpec{
		URL: serve(t, body).URL, SHA256: sum(body),
		Compat: func(context.Context, *Pack) error { return errors.New("lemma id 1 means something else") },
	})
	if err != nil {
		t.Fatal(err)
	}
	st := finish(t, in)
	if st.State != InstallFailed || st.ErrorKey != ErrKeyIncompatible {
		t.Errorf("status = %+v", st)
	}
	assertLive(t, dir, live)
	noTemps(t, dir)
}

func emptyUserDB(t *testing.T) *sql.DB {
	t.Helper()
	user, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	user.SetMaxOpenConns(1)
	t.Cleanup(func() { user.Close() })
	for _, q := range []string{
		`CREATE TABLE lemma_state (lemma_id INTEGER)`,
		`CREATE TABLE note (lemma_id INTEGER, sentence_id INTEGER)`,
		`CREATE TABLE text_token (lemma_id INTEGER)`,
	} {
		if _, err := user.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return user
}

func TestInstallHappyPathParksThenBootApplies(t *testing.T) {
	dir := t.TempDir()
	live := liveFile(t, dir)
	body := buildPackGz(t, "ház")
	in := NewInstaller(dir)
	if err := in.Start(context.Background(), InstallSpec{URL: serve(t, body).URL, SHA256: sum(body)}); err != nil {
		t.Fatal(err)
	}
	st := finish(t, in)
	if st.State != InstallReady || !st.RestartRequired || st.ErrorKey != "" {
		t.Fatalf("status = %+v", st)
	}
	if st.Bytes != int64(len(body)) || st.Total != int64(len(body)) {
		t.Errorf("progress bytes=%d total=%d, want %d", st.Bytes, st.Total, len(body))
	}
	// The live pack is NOT replaced in a running process.
	assertLive(t, dir, live)
	noTemps(t, dir)
	parked, err := openQuiet(Path(dir, "1") + ".new")
	if err != nil {
		t.Fatalf("parked pack does not open: %v", err)
	}
	parked.Close()

	// The next boot applies it.
	res := ApplyPending(context.Background(), dir, emptyUserDB(t))
	if !res.Applied || res.ErrorKey != "" {
		t.Fatalf("ApplyPending = %+v", res)
	}
	p, err := Open(Path(dir, "1"))
	if err != nil {
		t.Fatalf("applied pack does not open: %v", err)
	}
	defer p.Close()
	if p.Meta.Edition != EditionFull {
		t.Errorf("edition = %q", p.Meta.Edition)
	}
	for _, f := range []string{".new", ".new.json"} {
		if _, err := os.Stat(Path(dir, "1") + f); !os.IsNotExist(err) {
			t.Errorf("%s left after apply", f)
		}
	}
}

func TestApplyPendingRechecksIDsAgainstCurrentData(t *testing.T) {
	dir := t.TempDir()
	// Live pack: lemma 1 = ház. Parked pack: lemma 1 = könyv. The learner added
	// a note on lemma 1 between install and restart.
	write := func(path string, lemmas ...string) {
		zr, _ := gzip.NewReader(bytes.NewReader(buildPackGz(t, lemmas...)))
		var b bytes.Buffer
		b.ReadFrom(zr)
		os.WriteFile(path, b.Bytes(), 0o644)
	}
	write(Path(dir, "1"), "ház")
	write(Path(dir, "1")+".new", "könyv")
	before, _ := os.ReadFile(Path(dir, "1"))
	user := emptyUserDB(t)
	user.Exec(`INSERT INTO note VALUES (1, NULL)`)

	in := NewInstaller(dir)
	res := ApplyPending(context.Background(), dir, user)
	in.SetBootResult(res)
	if res.Applied || res.ErrorKey != ErrKeyIncompatible {
		t.Fatalf("ApplyPending = %+v", res)
	}
	after, _ := os.ReadFile(Path(dir, "1"))
	if !bytes.Equal(before, after) {
		t.Error("the live pack changed")
	}
	if _, err := os.Stat(Path(dir, "1") + ".new"); !os.IsNotExist(err) {
		t.Error("refused pack was not deleted")
	}
	if st := in.Status(); st.State != InstallFailed || st.ErrorKey != ErrKeyIncompatible {
		t.Errorf("admin sees %+v", st)
	}
}

func TestApplyPendingRemovesStaleTemps(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{".part", ".unpacked"} {
		os.WriteFile(Path(dir, "1")+f, []byte("x"), 0o644)
	}
	ApplyPending(context.Background(), dir, nil)
	noTemps(t, dir)
}

// Verification may legitimately take longer than the download's idle timeout
// (a slow disk): the watchdog must not outlive the download.
func TestInstallSlowVerificationIsNotAStall(t *testing.T) {
	dir := t.TempDir()
	body := buildPackGz(t, "ház")
	in := NewInstaller(dir)
	in.SetStallTimeout(100 * time.Millisecond)
	err := in.Start(context.Background(), InstallSpec{
		URL: serve(t, body).URL, SHA256: sum(body),
		Compat: func(ctx context.Context, _ *Pack) error { time.Sleep(500 * time.Millisecond); return ctx.Err() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if st := finish(t, in); st.State != InstallReady {
		t.Errorf("status = %+v", st)
	}
}

func TestInstallSecondStartWhileRunningIsBusy(t *testing.T) {
	dir := t.TempDir()
	body := buildPackGz(t, "ház")
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Write(body)
	}))
	defer srv.Close()
	in := NewInstaller(dir)
	spec := InstallSpec{URL: srv.URL, SHA256: sum(body)}
	if err := in.Start(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if err := in.Start(context.Background(), spec); !errors.Is(err, ErrBusy) {
		t.Errorf("second start: %v, want ErrBusy", err)
	}
	close(release)
	if st := finish(t, in); st.State != InstallReady {
		t.Errorf("status = %+v", st)
	}
}

func TestInstallDecompressionCap(t *testing.T) {
	dir := t.TempDir()
	live := liveFile(t, dir)
	body := gz(t, bytes.Repeat([]byte{0}, 1<<20)) // 1 MiB of zeros, tiny on the wire
	in := NewInstaller(dir)
	in.SetMaxUnpacked(64 << 10)
	if err := in.Start(context.Background(), InstallSpec{URL: serve(t, body).URL, SHA256: sum(body)}); err != nil {
		t.Fatal(err)
	}
	st := finish(t, in)
	if st.State != InstallFailed || st.ErrorKey != ErrKeyCorrupt {
		t.Errorf("status = %+v", st)
	}
	assertLive(t, dir, live)
	noTemps(t, dir)
}

func TestInstallRefusesPlainHTTPToARemoteHost(t *testing.T) {
	dir := t.TempDir()
	in := NewInstaller(dir)
	if err := in.Start(context.Background(), InstallSpec{URL: "http://example.com/p.gz", SHA256: sum(nil)}); err != nil {
		t.Fatal(err)
	}
	if st := finish(t, in); st.State != InstallFailed {
		t.Errorf("status = %+v", st)
	}
}

func TestInstallRefusesRedirectToPlainHTTP(t *testing.T) {
	dir := t.TempDir()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	// A TLS front that redirects to the plain-http target: the downgrade must
	// be refused even though both ends are local.
	front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/x", http.StatusFound)
	}))
	defer front.Close()
	in := NewInstaller(dir)
	in.client = front.Client()
	in.client.CheckRedirect = nil
	// Use the production redirect policy on a client that trusts the test CA.
	prod := NewInstaller(dir).httpClient(mustParse(t, front.URL))
	in.client.CheckRedirect = prod.CheckRedirect
	if err := in.Start(context.Background(), InstallSpec{URL: front.URL, SHA256: sum(nil)}); err != nil {
		t.Fatal(err)
	}
	st := finish(t, in)
	if st.State != InstallFailed || st.ErrorKey != ErrKeyDownload {
		t.Errorf("status = %+v", st)
	}
}

func TestCheckIDCompat(t *testing.T) {
	open := func(lemmas ...string) *Pack {
		dir := t.TempDir()
		raw := buildPackGz(t, lemmas...)
		zr, _ := gzip.NewReader(bytes.NewReader(raw))
		var b bytes.Buffer
		b.ReadFrom(zr)
		p := filepath.Join(dir, "x.sqlite")
		os.WriteFile(p, b.Bytes(), 0o644)
		pk, err := openQuiet(p)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pk.Close() })
		return pk
	}
	user, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	user.SetMaxOpenConns(1)
	defer user.Close()
	for _, q := range []string{
		`CREATE TABLE lemma_state (lemma_id INTEGER)`,
		`CREATE TABLE note (lemma_id INTEGER, sentence_id INTEGER)`,
		`CREATE TABLE text_token (lemma_id INTEGER)`,
		`INSERT INTO lemma_state VALUES (1)`, `INSERT INTO note VALUES (2, NULL)`, `INSERT INTO text_token VALUES (NULL)`,
	} {
		if _, err := user.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	cur := open("ház", "könyv")
	ctx := context.Background()
	if err := CheckIDCompat(ctx, user, cur, open("ház", "könyv", "extra")); err != nil {
		t.Errorf("same ids, extra words: %v", err)
	}
	if err := CheckIDCompat(ctx, user, cur, open("könyv", "ház")); err == nil {
		t.Error("renumbered lemmas must be refused")
	}
	if err := CheckIDCompat(ctx, user, cur, open("ház")); err == nil {
		t.Error("a referenced id missing from the new pack must be refused")
	}
	if err := CheckIDCompat(ctx, user, nil, open("ház", "könyv")); err == nil {
		t.Error("references with no current pack to compare must be refused")
	}
}

func mustParse(t *testing.T, raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
