// Package speech renders Hungarian text to audio with the managed Piper
// component and keeps the result on disk (ADR-079).
//
// What this is for. The reader can speak a whole text; browser voices for
// Hungarian are absent or uneven, so a local Piper child (services/tts) renders
// a sentence at a time and this package caches the WAV. Nothing here is a card,
// a gloss or a score: it is audio of text the learner is already looking at.
//
// What it must never do:
//
//   - depend on the child. Every failure is a typed error the API turns into
//     503/504, and the client falls back to the device voice silently.
//   - take a path, a URL or a command from a request. The cache file name is
//     the sha256 of (voice, model hash, style, text); Path accepts only that
//     shape, so a request can never name a file.
//   - render the same sentence twice at once, or hold the child with more than
//     one synthesis: a single-flight per key and a semaphore of one.
package speech

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Styles. A single word renders badly in Piper (it was trained on sentences),
// so "word" renders "<word>." slowly; a sentence is rendered as it is.
const (
	StyleSentence = "sentence"
	StyleWord     = "word"

	lengthSentence = 1.0
	lengthWord     = 1.4
)

// MaxTextBytes bounds one request: a sentence, not a chapter.
const MaxTextBytes = 1000

// Cache limits: past MaxCacheBytes the oldest files are deleted down to
// TargetCacheBytes.
const (
	MaxCacheBytes    = 512 << 20
	TargetCacheBytes = 400 << 20
	// DefaultTimeout covers waiting for the child and the synthesis itself.
	DefaultTimeout = 30 * time.Second
)

// Persona is one voice the app names. The file stems live in the child.
type Persona struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"` // female | male
}

// Personas in display order. Gender is chrome (the picker groups by it); it is
// not read from the model.
var Personas = []Persona{
	{ID: "anna", Name: "Anna", Gender: "female"},
	{ID: "berta", Name: "Berta", Gender: "female"},
	{ID: "imre", Name: "Imre", Gender: "male"},
}

func personaKnown(id string) bool {
	for _, p := range Personas {
		if p.ID == id {
			return true
		}
	}
	return false
}

// Errors. The API maps them to 400 / 503 / 504.
var (
	ErrBadText     = errors.New("speech: text must be 1..1000 bytes")
	ErrBadVoice    = errors.New("speech: unknown voice")
	ErrBadStyle    = errors.New("speech: style must be sentence or word")
	ErrUnavailable = errors.New("speech: the voice service is not reachable")
	ErrTimeout     = errors.New("speech: the voice service timed out")
)

// Request is one synthesis.
type Request struct {
	Text  string
	Voice string
	Style string
}

// Result is a cached rendering.
type Result struct {
	// File is "<64 hex>.wav", the name GET /api/speech/audio/:file serves.
	File      string
	DurationS float64
}

// Service renders and caches. Safe for concurrent use.
type Service struct {
	dir    string
	client *http.Client

	// Timeout bounds one Synthesize call; MaxBytes/TargetBytes the cache.
	Timeout     time.Duration
	MaxBytes    int64
	TargetBytes int64

	sem chan struct{} // one synthesis at a time against the child

	mu      sync.Mutex
	flights map[string]*flight
	hashes  map[string]hashEntry // endpoint -> voice model hashes
	approx  int64                // bytes in the cache, -1 = unknown
}

type flight struct {
	done chan struct{}
	res  Result
	err  error
}

type hashEntry struct {
	at time.Time
	m  map[string]string
}

// New builds a Service caching under dir (`<data>/cache/tts`). Nothing touches
// the disk until the first synthesis.
func New(dir string) *Service {
	return &Service{
		dir: dir, client: &http.Client{},
		Timeout: DefaultTimeout, MaxBytes: MaxCacheBytes, TargetBytes: TargetCacheBytes,
		sem: make(chan struct{}, 1), flights: map[string]*flight{},
		hashes: map[string]hashEntry{}, approx: -1,
	}
}

// Normalize is the text as it is keyed and rendered: NFC, whitespace runs
// collapsed to one space, trimmed. 1..MaxTextBytes bytes or ErrBadText.
func Normalize(text string) (string, error) {
	if !utf8.ValidString(text) {
		return "", ErrBadText
	}
	t := strings.Join(strings.Fields(norm.NFC.String(text)), " ")
	if t == "" || len(t) > MaxTextBytes {
		return "", ErrBadText
	}
	return t, nil
}

// Key is the cache key: it changes with the voice, the model file, the style
// and the text, and with nothing else (speed is the browser's playbackRate).
func Key(voice, modelHash, style, text string) string {
	h := sha256.New()
	for _, p := range []string{voice, modelHash, style, text} {
		fmt.Fprintf(h, "%d:%s\n", len(p), p) // length-prefixed: no field can bleed into the next
	}
	return hex.EncodeToString(h.Sum(nil))
}

var reFile = regexp.MustCompile(`^[0-9a-f]{64}\.wav$`)

// ValidFile reports whether name has the one shape the cache produces.
func ValidFile(name string) bool { return reFile.MatchString(name) }

// Path returns the on-disk path of a cache file, or "" for any name that is not
// exactly "<64 hex>.wav" (so "../x", separators and case tricks all fail).
func (s *Service) Path(name string) string {
	if !ValidFile(name) {
		return ""
	}
	return filepath.Join(s.dir, name[:2], name)
}

// Voice is one entry of the child's GET /voices.
type Voice struct {
	ID         string `json:"id"`
	SampleRate int    `json:"sampleRate"`
	ModelHash  string `json:"modelHash"`
}

// ChildVoices asks the child which voices it can render.
func (s *Service) ChildVoices(ctx context.Context, endpoint string) ([]Voice, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/voices", nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, mapNetErr(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrUnavailable
	}
	var vs []Voice
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&vs); err != nil {
		return nil, ErrUnavailable
	}
	return vs, nil
}

// modelHash resolves a voice's model hash, from a 30 s cache of /voices.
func (s *Service) modelHash(ctx context.Context, endpoint, voice string) (string, error) {
	s.mu.Lock()
	e, ok := s.hashes[endpoint]
	s.mu.Unlock()
	if !ok || time.Since(e.at) > 30*time.Second || e.m[voice] == "" {
		vs, err := s.ChildVoices(ctx, endpoint)
		if err != nil {
			return "", err
		}
		e = hashEntry{at: time.Now(), m: map[string]string{}}
		for _, v := range vs {
			e.m[v.ID] = v.ModelHash
		}
		s.mu.Lock()
		s.hashes[endpoint] = e
		s.mu.Unlock()
	}
	if e.m[voice] == "" {
		return "", ErrUnavailable // the child has no model for this persona
	}
	return e.m[voice], nil
}

// Synthesize returns the cached rendering, making it first when needed. The
// endpoint is the child's resolved `http://127.0.0.1:<port>`; "" is
// ErrUnavailable (checked after validation, so a bad request is a 400 either way).
func (s *Service) Synthesize(ctx context.Context, endpoint string, r Request) (Result, error) {
	text, err := Normalize(r.Text)
	if err != nil {
		return Result{}, err
	}
	if !personaKnown(r.Voice) {
		return Result{}, ErrBadVoice
	}
	if r.Style != StyleSentence && r.Style != StyleWord {
		return Result{}, ErrBadStyle
	}
	if endpoint == "" {
		return Result{}, ErrUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	hash, err := s.modelHash(ctx, endpoint, r.Voice)
	if err != nil {
		return Result{}, err
	}
	key := Key(r.Voice, hash, r.Style, text)

	// Cache hit: no child involved at all.
	if res, ok := s.cached(key); ok {
		return res, nil
	}

	// A follower that sees the leader fail because of the LEADER's context
	// (its client left, its deadline ran out) while its own context is live
	// retries once, becoming the leader, instead of inheriting that failure.
	for attempt := 0; ; attempt++ {
		s.mu.Lock()
		f, ok := s.flights[key]
		if !ok {
			f = &flight{done: make(chan struct{})}
			s.flights[key] = f
			s.mu.Unlock()
			// The leader's context bounds the work; the cleanup is deferred so
			// even a panic in render cannot leave the key in flight forever.
			func() {
				defer func() {
					s.mu.Lock()
					delete(s.flights, key)
					s.mu.Unlock()
					close(f.done)
				}()
				f.res, f.err = s.render(ctx, endpoint, key, text, r)
			}()
			return f.res, f.err
		}
		s.mu.Unlock()
		select {
		case <-f.done:
		case <-ctx.Done():
			return Result{}, ctxErr(ctx)
		}
		leaderCtxErr := errors.Is(f.err, ErrTimeout) || errors.Is(f.err, context.Canceled)
		if f.err == nil || !leaderCtxErr || ctx.Err() != nil || attempt >= 1 {
			return f.res, f.err
		}
	}
}

func (s *Service) cached(key string) (Result, bool) {
	name := key + ".wav"
	p := s.Path(name)
	d, err := wavFileDuration(p)
	if err != nil {
		return Result{}, false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now) // pruning drops the least recently used first
	return Result{File: name, DurationS: d}, true
}

func (s *Service) render(ctx context.Context, endpoint, key, text string, r Request) (Result, error) {
	// A second caller that lost the race to a finished flight finds the file.
	if res, ok := s.cached(key); ok {
		return res, nil
	}
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return Result{}, ctxErr(ctx)
	}

	body := text
	scale := lengthSentence
	if r.Style == StyleWord {
		scale = lengthWord
		if !strings.HasSuffix(body, ".") && !strings.HasSuffix(body, "!") && !strings.HasSuffix(body, "?") {
			body += "." // a bare word renders badly; a one-word sentence does not
		}
	}
	payload, _ := json.Marshal(map[string]any{"text": body, "voice": r.Voice, "lengthScale": scale})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/synthesize", bytes.NewReader(payload))
	if err != nil {
		return Result{}, ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return Result{}, mapNetErr(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return Result{}, ErrUnavailable
	}
	wav, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Result{}, mapNetErr(ctx, err)
	}
	dur, err := wavDuration(wav)
	if err != nil {
		return Result{}, ErrUnavailable // the child answered with something that is not audio
	}

	name := key + ".wav"
	p := s.Path(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return Result{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".w-*")
	if err != nil {
		return Result{}, err
	}
	_, werr := tmp.Write(wav)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		return Result{}, errors.Join(werr, cerr)
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		_ = os.Remove(tmp.Name())
		return Result{}, err
	}
	s.afterWrite(int64(len(wav)))
	return Result{File: name, DurationS: dur}, nil
}

func ctxErr(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return ctx.Err()
}

// mapNetErr: a deadline is a timeout; anything else is "not reachable".
func mapNetErr(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	return ErrUnavailable
}

// ---- cache size ----

func (s *Service) afterWrite(n int64) {
	s.mu.Lock()
	if s.approx >= 0 {
		s.approx += n
	}
	over := s.approx < 0 || s.approx > s.MaxBytes
	s.mu.Unlock()
	if over {
		s.Prune()
	}
}

var reCacheFile = regexp.MustCompile(`^[0-9a-f]{64}\.wav$`)

// Prune deletes the least recently used files until the cache is at most
// TargetBytes, when it is over MaxBytes. Returns the bytes removed.
func (s *Service) Prune() int64 {
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	var files []file
	var total int64
	_ = filepath.WalkDir(s.dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !reCacheFile.MatchString(d.Name()) {
			return nil // in-flight temp files (.w-*) and strangers are never touched
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, file{p, info.Size(), info.ModTime()})
		total += info.Size()
		return nil
	})
	var removed int64
	if total > s.MaxBytes {
		sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
		for _, f := range files {
			if total <= s.TargetBytes {
				break
			}
			if os.Remove(f.path) == nil {
				total -= f.size
				removed += f.size
			}
		}
	}
	s.mu.Lock()
	s.approx = total
	s.mu.Unlock()
	return removed
}

// ---- WAV ----

// wavDuration reads the duration from a PCM WAV's header and data chunk size.
func wavDuration(b []byte) (float64, error) { return wavDurationWithSize(b, int64(len(b))) }

// wavFileDuration reads only the header of a cached file.
func wavFileDuration(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	b := head[:n]
	// The data chunk header is within the first 512 bytes of Piper's output.
	return wavDurationWithSize(b, st.Size())
}

// wavDurationWithSize is wavDuration for a header prefix of a file of total bytes.
func wavDurationWithSize(prefix []byte, total int64) (float64, error) {
	if len(prefix) < 12 || string(prefix[0:4]) != "RIFF" || string(prefix[8:12]) != "WAVE" {
		return 0, errors.New("not a WAV")
	}
	var byteRate uint32
	for i := 12; i+8 <= len(prefix); {
		id, size := string(prefix[i:i+4]), binary.LittleEndian.Uint32(prefix[i+4:i+8])
		body := i + 8
		switch id {
		case "fmt ":
			if body+12 > len(prefix) {
				return 0, errors.New("short fmt chunk")
			}
			byteRate = binary.LittleEndian.Uint32(prefix[body+8 : body+12])
		case "data":
			if byteRate == 0 {
				return 0, errors.New("data before fmt")
			}
			n := int64(size)
			if rest := total - int64(body); n > rest {
				n = rest
			}
			return float64(n) / float64(byteRate), nil
		}
		i = body + int(size) + int(size&1)
	}
	return 0, errors.New("no data chunk")
}
