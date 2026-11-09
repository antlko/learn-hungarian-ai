package lang

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Installing the full pack from inside the app — ADR-049, amending ADR-010.
//
// The binary already carries a starter pack, so this is an UPGRADE the admin
// asks for, never a prerequisite. Four things make it safe to offer:
//
//   - nothing is downloaded without a pinned SHA-256 of the compressed file
//     (config langpack.sha256); empty means "nothing is published" and the job
//     refuses before it opens a socket;
//   - the live pack is never written: bytes land in two temp files beside it
//     and only a fully verified pack is renamed over it, atomically;
//   - the learner database's pointers into the pack (lemma and sentence ids)
//     are checked to still mean the same word in the new pack — a pack built
//     from a different Wiktionary dump numbers its lemmas differently, and
//     every note would silently point at a different word;
//   - the LIVE pack is never replaced inside a running process: database/sql
//     opens new pool connections by path, so a rename over it would make the
//     running app silently read the new file. The verified pack is parked as
//     langpack-hu-v1.sqlite.new and ApplyPending swaps it in at the next boot,
//     before anything opens the pack, re-running the id check against the
//     learner data as it is THEN. The job ends in "ready, restart required".

// Install states, in the order a job passes through them.
const (
	InstallIdle        = "idle"
	InstallDownloading = "downloading"
	InstallVerifying   = "verifying"
	InstallReady       = "ready"
	InstallFailed      = "failed"
)

// Error keys. They are i18n keys in both locales (web-ui/src/locales), never
// prose: the status travels to the screen as a key.
const (
	ErrKeyNotPublished  = "errors.packNotPublished"
	ErrKeyIncompatible  = "errors.packIncompatible"
	ErrKeyDiskFull      = "errors.diskFull"
	ErrKeyDownload      = "errors.packDownloadFailed"
	ErrKeyChecksum      = "errors.packChecksum"
	ErrKeyCorrupt       = "errors.packCorrupt"
	ErrKeyOlderInstall  = "errors.packOlder"
	installFileVersion  = StarterVersion // the release asset is langpack-hu-v1
	DefaultMaxUnpacked  = int64(2) << 30 // 2 GiB; the full pack is ~260 MB
	maxRedirects        = 5
	defaultStall        = 30 * time.Second
	headerTimeout       = 30 * time.Second
	temporaryFileSuffix = ".part"
)

var (
	// ErrBusy: a job is already running.
	ErrBusy = errors.New("a pack install is already running")
	// ErrNotPublished: no pinned checksum, so nothing may be downloaded.
	ErrNotPublished = errors.New("no checksum is pinned for the downloadable pack")
)

// InstallStatus is the job's state, as served to the admin screen.
type InstallStatus struct {
	State           string `json:"state"`
	Bytes           int64  `json:"bytes"`
	Total           int64  `json:"total"`
	ErrorKey        string `json:"errorKey"`
	RestartRequired bool   `json:"restartRequired"`
}

// InstallSpec is one job. URL and SHA256 come from config, never a request.
type InstallSpec struct {
	URL    string
	SHA256 string
	// Compat is the id-compatibility check against the learner database. It
	// receives the opened candidate. Nil means "no learner data to protect".
	Compat func(ctx context.Context, next *Pack) error
}

// Installer runs at most one install at a time.
type Installer struct {
	dataDir     string
	maxUnpacked int64
	stall       time.Duration
	client      *http.Client

	mu     sync.Mutex
	status InstallStatus
	wg     sync.WaitGroup
}

// NewInstaller builds an installer for a data directory.
func NewInstaller(dataDir string) *Installer {
	return &Installer{
		dataDir:     dataDir,
		maxUnpacked: DefaultMaxUnpacked,
		stall:       defaultStall,
		status:      InstallStatus{State: InstallIdle},
	}
}

// SetMaxUnpacked overrides the decompressed-size cap (tests).
func (in *Installer) SetMaxUnpacked(n int64) { in.maxUnpacked = n }

// SetStallTimeout overrides the download idle timeout (tests).
func (in *Installer) SetStallTimeout(d time.Duration) { in.stall = d }

// SetBootResult surfaces what ApplyPending did at this boot: a refused pack
// shows up as a failed job with its error key, so the admin learns why the
// restart did not bring the new pack.
func (in *Installer) SetBootResult(r PendingResult) {
	if r.ErrorKey == "" {
		return
	}
	in.set(func(s *InstallStatus) {
		*s = InstallStatus{State: InstallFailed, ErrorKey: r.ErrorKey}
	})
}

// Status returns a snapshot.
func (in *Installer) Status() InstallStatus {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.status
}

// Wait blocks until a running job ends (shutdown, tests).
func (in *Installer) Wait() { in.wg.Wait() }

func (in *Installer) set(f func(*InstallStatus)) {
	in.mu.Lock()
	f(&in.status)
	in.mu.Unlock()
}

// Start begins a job in the background and returns at once. ctx bounds the job
// (pass the server's background context, NOT the request's). Returns ErrBusy
// while one runs and ErrNotPublished when no checksum is pinned.
func (in *Installer) Start(ctx context.Context, spec InstallSpec) error {
	if strings.TrimSpace(spec.SHA256) == "" {
		return ErrNotPublished
	}
	in.mu.Lock()
	if in.status.State == InstallDownloading || in.status.State == InstallVerifying {
		in.mu.Unlock()
		return ErrBusy
	}
	in.status = InstallStatus{State: InstallDownloading}
	in.wg.Add(1)
	in.mu.Unlock()

	go func() {
		defer in.wg.Done()
		key, err := in.run(ctx, spec)
		if err != nil {
			slog.Warn("language pack install failed", "error_key", key, "error", redactErr(err))
			in.set(func(s *InstallStatus) {
				s.State, s.ErrorKey, s.RestartRequired = InstallFailed, key, false
			})
			return
		}
		slog.Info("language pack installed; restart to load it")
		in.set(func(s *InstallStatus) {
			s.State, s.ErrorKey, s.RestartRequired = InstallReady, "", true
		})
	}()
	return nil
}

func redactedURL(u *url.URL, parseErr error) string {
	if parseErr != nil || u == nil {
		return "(unparseable)"
	}
	return u.Redacted()
}

// redactErr strips credentials from the URL an *url.Error carries.
func redactErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		if u, perr := url.Parse(ue.URL); perr == nil {
			c := *ue
			c.URL = u.Redacted()
			return &c
		}
		c := *ue
		c.URL = "(unparseable)"
		return &c
	}
	return err
}

// loopbackHost mirrors config's rule: http is acceptable only to this machine.
func loopbackHost(h string) bool {
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

func (in *Installer) httpClient(start *url.URL) *http.Client {
	if in.client != nil {
		return in.client
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: headerTimeout,
		},
		// No overall Timeout: the file is hundreds of MB. A stall is caught by
		// the idle watchdog around the body instead.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			// A redirect may not downgrade to plain http (unless the whole
			// exchange is with this machine): the checksum protects the bytes,
			// but there is no reason to leak the request either.
			if req.URL.Scheme != "https" &&
				!(req.URL.Scheme == "http" && loopbackHost(req.URL.Hostname()) && loopbackHost(start.Hostname())) {
				return fmt.Errorf("redirect to a non-https URL refused: %s", req.URL.Redacted())
			}
			return nil
		},
	}
}

// idleReader cancels the request when no byte arrives for stallTimeout.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
	idle  time.Duration
}

func (i *idleReader) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	i.timer.Reset(i.idle)
	return n, err
}

// run does the whole job and returns the error key to show on failure.
func (in *Installer) run(parent context.Context, spec InstallSpec) (key string, err error) {
	want := strings.ToLower(strings.TrimSpace(spec.SHA256))
	u, err := url.Parse(spec.URL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && loopbackHost(u.Hostname()))) {
		return ErrKeyDownload, fmt.Errorf("refusing URL %q", redactedURL(u, err))
	}

	dest := Path(in.dataDir, installFileVersion)
	part := dest + temporaryFileSuffix
	unpacked := dest + ".unpacked"
	cleanup := func() { _ = os.Remove(part); _ = os.Remove(unpacked) }
	cleanup() // stale leftovers from a killed process
	defer func() {
		cleanup()
	}()

	// ── download ─────────────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return ErrKeyDownload, err
	}
	resp, err := in.httpClient(u).Do(req)
	if err != nil {
		return ErrKeyDownload, redactErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrKeyDownload, fmt.Errorf("download answered %s", resp.Status)
	}
	total := resp.ContentLength // -1 when unknown
	if total < 0 {
		total = 0
	}
	in.set(func(s *InstallStatus) { s.Total = total })

	out, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return keyForFS(err), err
	}
	sum := sha256.New()
	// The watchdog guards the DOWNLOAD only: it is stopped the moment the body
	// ends, so a slow disk during verification cannot cancel a good pack.
	watchdog := time.AfterFunc(in.stall, cancel)
	defer watchdog.Stop()
	body := &idleReader{r: resp.Body, timer: watchdog, idle: in.stall}

	buf := make([]byte, 128<<10)
	var got int64
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			sum.Write(buf[:n])
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return keyForFS(werr), werr
			}
			got += int64(n)
			g := got
			in.set(func(s *InstallStatus) { s.Bytes = g })
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			return ErrKeyDownload, rerr
		}
	}
	watchdog.Stop()
	if err := out.Close(); err != nil {
		return keyForFS(err), err
	}
	if total > 0 && got != total {
		return ErrKeyDownload, fmt.Errorf("download ended after %d of %d bytes", got, total)
	}

	// ── verify ───────────────────────────────────────────────────────────
	in.set(func(s *InstallStatus) { s.State = InstallVerifying })
	if hex.EncodeToString(sum.Sum(nil)) != want {
		return ErrKeyChecksum, errors.New("sha256 of the download does not match the pinned value")
	}

	if err := gunzipFile(part, unpacked, in.maxUnpacked); err != nil {
		if errors.Is(err, errTooLarge) {
			return ErrKeyCorrupt, err
		}
		if k := keyForFS(err); k == ErrKeyDiskFull {
			return k, err
		}
		return ErrKeyCorrupt, err
	}
	if err := parent.Err(); err != nil {
		return ErrKeyDownload, err
	}

	next, err := openQuiet(unpacked)
	if err != nil {
		return ErrKeyIncompatible, err
	}
	closeNext := func() { _ = next.Close() }
	if _, err := next.Counts(); err != nil {
		closeNext()
		return ErrKeyIncompatible, err
	}
	if spec.Compat != nil {
		if err := spec.Compat(ctx, next); err != nil {
			closeNext()
			return ErrKeyIncompatible, err
		}
	}
	closeNext()

	// An already-installed HIGHER version would keep winning in Detect, so the
	// install would "succeed" and change nothing. Say so instead.
	if v, ok := Detect(in.dataDir); ok && v > installFileVersion {
		return ErrKeyOlderInstall, fmt.Errorf("pack v%s is installed and newer than v%s", v, installFileVersion)
	}

	// ── park ─────────────────────────────────────────────────────────────
	// NOT over the live file (see the header): beside it, for ApplyPending.
	// Atomic on one filesystem: the temp file lives in the same directory.
	if err := os.Rename(unpacked, dest+pendingSuffix); err != nil {
		return keyForFS(err), err
	}
	marker, _ := json.Marshal(map[string]string{
		"sha256": want, "at": time.Now().UTC().Format(time.RFC3339)})
	if err := os.WriteFile(dest+pendingSuffix+".json", marker, 0o644); err != nil {
		_ = os.Remove(dest + pendingSuffix)
		return keyForFS(err), err
	}
	return "", nil
}

const pendingSuffix = ".new"

// PendingResult is what ApplyPending did.
type PendingResult struct {
	Applied  bool
	ErrorKey string // set when a parked pack was refused and deleted
}

// ApplyPending runs at boot, BEFORE the pack is opened. It always removes the
// download's temp files (a killed process leaves them). If a verified pack was
// parked by an install it is re-opened, checked again — the learner may have
// added notes since the install ran — and renamed over the live pack. If the
// check fails the parked pack is deleted and the live one stays. userDB may be
// nil only when there is no learner data.
func ApplyPending(ctx context.Context, dataDir string, userDB *sql.DB) PendingResult {
	dest := Path(dataDir, installFileVersion)
	_ = os.Remove(dest + temporaryFileSuffix)
	_ = os.Remove(dest + ".unpacked")

	parked := dest + pendingSuffix
	if _, err := os.Stat(parked); err != nil {
		return PendingResult{}
	}
	defer os.Remove(dest + pendingSuffix + ".json")

	fail := func(key string, err error) PendingResult {
		slog.Warn("parked language pack refused; keeping the current one", "error_key", key, "error", err)
		_ = os.Remove(parked)
		return PendingResult{ErrorKey: key}
	}
	next, err := openQuiet(parked)
	if err != nil {
		return fail(ErrKeyIncompatible, err)
	}
	var cur *Pack
	if c, err := openQuiet(dest); err == nil {
		cur = c
	}
	// Close both handles before renaming (required on Windows, tidy elsewhere).
	closeBoth := func() {
		_ = next.Close()
		if cur != nil {
			_ = cur.Close()
		}
	}
	if _, err := next.Counts(); err != nil {
		closeBoth()
		return fail(ErrKeyIncompatible, err)
	}
	if userDB != nil {
		if err := CheckIDCompat(ctx, userDB, cur, next); err != nil {
			closeBoth()
			return fail(ErrKeyIncompatible, err)
		}
	}
	closeBoth()
	if err := os.Rename(parked, dest); err != nil {
		return fail(keyForFS(err), err)
	}
	slog.Info("full language pack applied", "path", dest)
	return PendingResult{Applied: true}
}

func keyForFS(err error) string {
	if errors.Is(err, syscall.ENOSPC) {
		return ErrKeyDiskFull
	}
	return ErrKeyDownload
}

var errTooLarge = errors.New("decompressed pack exceeds the size cap")

// gunzipFile decompresses src into dst, refusing to write more than max bytes.
func gunzipFile(src, dst string, max int64) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	zr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer zr.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(zr, max+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > max {
		return errTooLarge
	}
	return nil
}

// ── id compatibility ─────────────────────────────────────────────────────

// idRefs lists every learner-database column holding a pointer into the pack,
// found by grepping the migrations (00002_srs.sql, 00003_texts.sql). Add a
// row here when a migration adds another — TestIDRefsCoverMigrations pins it.
var (
	lemmaRefs    = [][2]string{{"lemma_state", "lemma_id"}, {"note", "lemma_id"}, {"text_token", "lemma_id"}}
	sentenceRefs = [][2]string{{"note", "sentence_id"}}
)

// IDRefs exposes the lists for the migration-coverage test.
func IDRefs() (lemma, sentence [][2]string) { return lemmaRefs, sentenceRefs }

// CheckIDCompat verifies that every pack id the learner database references
// means the same thing in next as in cur: a lemma id must map to the same
// (lemma, upos), a sentence id to the same Hungarian text. A mismatch — or an
// id that next lacks — refuses the install. With no current pack there is
// nothing to compare against, so any reference refuses.
func CheckIDCompat(ctx context.Context, userDB *sql.DB, cur, next *Pack) error {
	lemmaIDs, err := distinctIDs(ctx, userDB, lemmaRefs)
	if err != nil {
		return err
	}
	sentIDs, err := distinctIDs(ctx, userDB, sentenceRefs)
	if err != nil {
		return err
	}
	if len(lemmaIDs)+len(sentIDs) == 0 {
		return nil
	}
	if cur == nil {
		return errors.New("learner data references the pack but no pack is loaded to compare with")
	}
	if err := sameRows(ctx, cur.db, next.db, lemmaIDs,
		`SELECT id, lemma || '|' || upos FROM lemma WHERE id IN `, "lemma"); err != nil {
		return err
	}
	return sameRows(ctx, cur.db, next.db, sentIDs,
		`SELECT id, hu FROM sentence WHERE id IN `, "sentence")
}

func distinctIDs(ctx context.Context, db *sql.DB, refs [][2]string) ([]int64, error) {
	seen := map[int64]struct{}{}
	for _, r := range refs {
		rows, err := db.QueryContext(ctx,
			fmt.Sprintf(`SELECT DISTINCT %s FROM %s WHERE %s IS NOT NULL AND %s > 0`, r[1], r[0], r[1], r[1]))
		if err != nil {
			return nil, fmt.Errorf("read %s.%s: %w", r[0], r[1], err)
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			seen[id] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	ids := make([]int64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids, nil
}

func sameRows(ctx context.Context, a, b *sql.DB, ids []int64, query, what string) error {
	const chunk = 500
	for lo := 0; lo < len(ids); lo += chunk {
		part := ids[lo:min(lo+chunk, len(ids))]
		ph := "(" + strings.TrimSuffix(strings.Repeat("?,", len(part)), ",") + ")"
		args := make([]any, len(part))
		for i, id := range part {
			args[i] = id
		}
		old, err := keyed(ctx, a, query+ph, args)
		if err != nil {
			return err
		}
		nu, err := keyed(ctx, b, query+ph, args)
		if err != nil {
			return err
		}
		for id, v := range old {
			if w, ok := nu[id]; !ok || w != v {
				return fmt.Errorf("%s id %d means %q in the loaded pack and %q in the new one", what, id, v, w)
			}
		}
		// An id the learner references but the CURRENT pack lacks cannot be
		// compared; if the new pack knows it, it can only be a different word.
		for id := range nu {
			if _, ok := old[id]; !ok {
				return fmt.Errorf("%s id %d is referenced by learner data but absent from the loaded pack", what, id)
			}
		}
	}
	return nil
}

func keyed(ctx context.Context, db *sql.DB, q string, args []any) (map[int64]string, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[int64]string{}
	for rows.Next() {
		var id int64
		var v string
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		m[id] = v
	}
	return m, rows.Err()
}
