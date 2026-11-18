package ttsfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Limits. Vars only so a test can lower them.
var (
	maxFileBytes  int64 = 256 << 20 // the biggest real file is 63 MB
	maxTotalBytes int64 = 1 << 30   // the whole fetch is ~0.4 GB
	deadlineSlack       = 60 * time.Second
)

// Config configures a Fetcher. The zero value is production: https to
// raw.githubusercontent.com, three attempts, a 60 s stall limit.
type Config struct {
	// BaseURL is the operator's mirror (flag/env only): https, or http on
	// loopback. Empty means https://raw.githubusercontent.com.
	BaseURL string
	Repo    string // default Repo (tests only)
	Ref     string // default "main"
	// Client is a test seam (a counting transport). Its redirect policy is replaced.
	Client      *http.Client
	Attempts    int                             // per file, default 3
	Backoff     func(attempt int) time.Duration // default 1 s, 3 s
	IdleTimeout time.Duration                   // no bytes for this long = network failure, default 60 s
	// MinBytesPerSecond is the slowest acceptable average: a file may take at most
	// size/this + 60 s overall, so a server dripping bytes is cut off. Default 100 KB/s.
	MinBytesPerSecond int64
}

// Fetcher downloads manifest-pinned files. Safe for one Fetch at a time.
type Fetcher struct {
	base     *url.URL
	repo     string
	ref      string
	client   *http.Client
	attempts int
	backoff  func(int) time.Duration
	idle     time.Duration
	minBPS   int64
}

var (
	reRepo    = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
	reRefName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// ValidateBaseURL checks an operator-supplied mirror: https, or http on
// loopback (127.0.0.1, localhost, ::1), no credentials, query or fragment.
// Empty is valid (the default host).
func ValidateBaseURL(raw string) error {
	_, err := parseBase(raw)
	return err
}

func parseBase(raw string) (*url.URL, error) {
	if raw == "" {
		return &url.URL{Scheme: "https", Host: Host}, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, fmt.Errorf("the mirror URL %q is not usable", raw)
	}
	switch u.Scheme {
	case "https":
	case "http":
		switch u.Hostname() {
		case "127.0.0.1", "localhost", "::1":
		default:
			return nil, fmt.Errorf("http is only allowed for localhost, not %q", u.Hostname())
		}
	default:
		return nil, fmt.Errorf("the mirror URL must be https (or http on localhost)")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

// New validates cfg and builds a Fetcher.
func New(cfg Config) (*Fetcher, error) {
	base, err := parseBase(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	repo, ref := cfg.Repo, cfg.Ref
	if repo == "" {
		repo = Repo
	}
	if ref == "" {
		ref = "main"
	}
	if !reRepo.MatchString(repo) || !reRefName.MatchString(ref) || ref == "." || ref == ".." {
		return nil, errors.New("invalid repository or ref")
	}
	f := &Fetcher{base: base, repo: repo, ref: ref, attempts: cfg.Attempts, backoff: cfg.Backoff, idle: cfg.IdleTimeout, minBPS: cfg.MinBytesPerSecond}
	if f.attempts <= 0 {
		f.attempts = 3
	}
	if f.backoff == nil {
		f.backoff = func(n int) time.Duration { return time.Duration(1+2*(n-1)) * time.Second }
	}
	if f.minBPS <= 0 {
		f.minBPS = 100 << 10
	}
	if f.idle <= 0 {
		f.idle = 60 * time.Second
	}
	origin := base.Host
	policy := func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != base.Scheme || req.URL.Host != origin {
			return fmt.Errorf("redirect to another host refused")
		}
		return nil
	}
	if cfg.Client != nil {
		cl := *cfg.Client
		cl.CheckRedirect = policy
		f.client = &cl
	} else {
		f.client = &http.Client{
			CheckRedirect: policy,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				DisableCompression:    true,
				MaxIdleConns:          2,
				IdleConnTimeout:       30 * time.Second,
			},
		}
	}
	return f, nil
}

// url builds the one URL of a file from fixed parts: base + repo + ref + the
// manifest's path, each segment escaped. Nothing else ever reaches it.
func (f *Fetcher) url(file File) (string, error) {
	segs := strings.Split(file.Repo, "/")
	for i, s := range segs {
		if s == "" || s == "." || s == ".." {
			return "", corrupt(file.Rel, "unsafe path")
		}
		segs[i] = url.PathEscape(s)
	}
	return f.base.Scheme + "://" + f.base.Host + f.base.Path + "/" + f.repo + "/" + f.ref + "/" + strings.Join(segs, "/"), nil
}

// Valid reports whether dst is exactly the pinned file (size, then sha256).
func Valid(dst string, size int64, sum string) bool {
	st, err := os.Stat(dst)
	if err != nil || !st.Mode().IsRegular() || st.Size() != size {
		return false
	}
	fh, err := os.Open(dst)
	if err != nil {
		return false
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == sum
}

// Fetch makes every file of the plan exist under dir (at File.Rel), verified.
// Files that are already valid are kept and cost no request, so a retry after a
// failure only fetches what is missing. progress(done, total) is monotonic and
// counts the kept files as done. Failures are *Error (KindDownload after the
// retries, KindCorrupt at once, the bad file deleted) or the context's error.
func (f *Fetcher) Fetch(ctx context.Context, dir string, files []File, progress func(done, total int64)) error {
	var total int64
	for _, fl := range files {
		if fl.Size <= 0 || fl.Size > maxFileBytes {
			return corrupt(fl.Rel, "size %d is out of range", fl.Size)
		}
		if total += fl.Size; total > maxTotalBytes {
			return corrupt("", "the plan is larger than %d bytes", maxTotalBytes)
		}
	}
	var done, high int64
	report := func(d int64) {
		if d > high {
			high = d
		}
		if progress != nil {
			progress(high, total)
		}
	}
	report(0)
	for _, fl := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(fl.Rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		_ = os.Remove(dst + ".part")
		if Valid(dst, fl.Size, fl.Sum) {
			done += fl.Size
			report(done)
			continue
		}
		_ = os.Remove(dst)
		if err := f.getWithRetry(ctx, fl, dst, func(n int64) { report(done + n) }); err != nil {
			return err
		}
		done += fl.Size
		report(done)
	}
	return nil
}

type retryable struct{ err error }

func (r retryable) Error() string { return r.err.Error() }
func (r retryable) Unwrap() error { return r.err }

func (f *Fetcher) getWithRetry(ctx context.Context, fl File, dst string, progress func(int64)) error {
	var last error
	for attempt := 1; attempt <= f.attempts; attempt++ {
		err := f.getOnce(ctx, fl, dst, progress)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var r retryable
		if !errors.As(err, &r) {
			return err
		}
		last = r.err
		if attempt == f.attempts {
			break
		}
		t := time.NewTimer(f.backoff(attempt))
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	return &Error{Kind: KindDownload, File: fl.Rel, Err: last}
}

// getOnce downloads one file to dst.part and renames it into place only when
// the size and sha256 are exactly the pinned ones.
func (f *Fetcher) getOnce(ctx context.Context, fl File, dst string, progress func(int64)) error {
	u, err := f.url(fl)
	if err != nil {
		return err
	}
	part := dst + ".part"
	defer os.Remove(part) // no-op after a successful rename

	// Overall per-file deadline: a slow drip never resets it. Expiry reads as a network failure.
	rctx, cancel := context.WithTimeout(ctx, time.Duration(fl.Size/f.minBPS)*time.Second+deadlineSlack)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, u, nil)
	if err != nil {
		return &Error{Kind: KindDownload, File: fl.Rel, Err: err}
	}
	req.Header.Set("User-Agent", "tanulom-component-fetch")
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := f.client.Do(req)
	if err != nil {
		return retryable{scrub(err)}
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return retryable{fmt.Errorf("HTTP %d", resp.StatusCode)}
	default:
		return &Error{Kind: KindDownload, File: fl.Rel, Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}
	if resp.ContentLength >= 0 && resp.ContentLength != fl.Size {
		return corrupt(fl.Rel, "the server announces %d bytes, expected %d", resp.ContentLength, fl.Size)
	}

	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	wd := newWatchdog(f.idle, cancel)
	defer wd.stop()
	h := sha256.New()
	var n int64
	buf := make([]byte, 64<<10)
	var readErr error
	for n <= fl.Size {
		k, rerr := resp.Body.Read(buf[:min(int64(len(buf)), fl.Size+1-n)])
		wd.reset()
		if k > 0 {
			if _, werr := out.Write(buf[:k]); werr != nil {
				_ = out.Close()
				return werr // disk full and the like: not a network failure
			}
			h.Write(buf[:k])
			n += int64(k)
			progress(min(n, fl.Size))
		}
		if rerr != nil {
			if rerr != io.EOF {
				readErr = rerr
			}
			break
		}
	}
	if cerr := out.Close(); cerr != nil {
		return cerr
	}
	if readErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return retryable{scrub(readErr)}
	}
	if n != fl.Size {
		return corrupt(fl.Rel, "got %s bytes, expected %d", sizeWord(n, fl.Size), fl.Size)
	}
	if hex.EncodeToString(h.Sum(nil)) != fl.Sum {
		return corrupt(fl.Rel, "sha256 differs")
	}
	if err := os.Rename(part, dst); err != nil {
		return err
	}
	return nil
}

func sizeWord(n, want int64) string {
	if n > want {
		return fmt.Sprintf("more than %d", want)
	}
	return fmt.Sprint(n)
}

// scrub keeps the cause of a transport error but drops the URL it carries.
func scrub(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s: %v", ue.Op, ue.Err)
	}
	return err
}

// watchdog cancels a request that has been silent for d.
type watchdog struct {
	mu sync.Mutex
	t  *time.Timer
	d  time.Duration
}

func newWatchdog(d time.Duration, cancel context.CancelFunc) *watchdog {
	return &watchdog{d: d, t: time.AfterFunc(d, cancel)}
}
func (w *watchdog) reset() { w.mu.Lock(); w.t.Reset(w.d); w.mu.Unlock() }
func (w *watchdog) stop()  { w.mu.Lock(); w.t.Stop(); w.mu.Unlock() }
