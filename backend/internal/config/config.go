// Package config owns config.yaml: the hand-editable settings file. The learning
// domain lives in SQLite, not here, and secrets never appear in this file.
//
// Invariant: the running config is mutated only through the api layer's
// updateConfig (clone -> mutate -> validate -> atomic save -> swap). Never write
// to a *Config that is already installed as the active one.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Exported defaults. These live in exactly one place so that API fallbacks cannot
// drift from what a fresh config gets. Do not re-hardcode any of them at a use site.
const (
	DefaultAddr = ":8012"

	DefaultUILocale  = "ru"
	DefaultGlossLang = "ru"

	DefaultDesiredRetention = 0.90
	MinDesiredRetention     = 0.80
	MaxDesiredRetention     = 0.95
	DefaultNewPerDay        = 20
	DefaultDailyMinutes     = 90
	DefaultGoal             = GoalConversation

	// Free practice session sizes (ADR-019/ADR-024). Finite on purpose: an
	// endless stream has no moment of having finished. The bounds clamp the
	// query parameter, not a config field — round length is a per-session whim,
	// not a setting to store.
	DefaultPracticeSession = 20
	MinPracticeSession     = 10
	MaxPracticeSession     = 50

	// Mining weights: see docs/ARCHITECTURE.md 5.1 and [R6].
	DefaultKnownThreshold = 0.80
	DefaultFeatureWeight  = 0.70
	DefaultTargetUnknown  = 1.5
	DefaultUnknownSigma   = 0.8
	// DefaultMinCoverage is the MINING floor, not a reading threshold. A mined
	// sentence wants one or two unknowns and a lookup; the 95%/98% figures
	// describe comprehending a whole text unaided. Applying the reading number
	// here would penalise every short n+1 sentence. See internal/mining/score.go.
	DefaultMinCoverage = 0.75

	// Drill gate: blocked -> interleaved. See [R5] —
	// .claude/research/05-interleaving.md. These are deliberately tunable
	// because the underlying study population is not our learner.
	DefaultGateThreshold    = 0.80
	DefaultGateWindow       = 20
	DefaultMasteryThreshold = 0.90
	DefaultMasteryWindow    = 50
	DefaultHysteresis       = 0.10

	// Definiteness item distribution mirrors the documented HunLearner error
	// distribution: 55% definite-object, 25% demonstrative-object. See [R15].
	DefaultDefiniteObjectPct = 55
	DefaultDemonstrativePct  = 25
	DefaultHarmonyLatencyMs  = 2500

	// HVPT talker variability is the active ingredient, not a nicety. See [R7].
	//
	// EvidenceMinTalkers is what the literature asks for. DefaultMinTalkers is
	// what we ship, and they differ on purpose: the language pack carries three
	// synthetic Piper voices, because three is how many freely-licensed
	// Hungarian voices exist. Shipping the evidence number would mean the ear
	// trainer refuses every session out of the box — a screen nobody can ever
	// pass, which teaches nothing and reads as a broken feature.
	//
	// So the shipped floor is the number we can actually meet, and the Ear
	// screen states the shortfall permanently: three voices, the research wants
	// five, the effect is correspondingly weaker. That is the project's rule —
	// a known limitation is stated in the product, not only in the docs — and
	// it is recorded as a documented divergence in docs/PEDAGOGY.md. Adding
	// human recordings raises the count; rate variants deliberately do not.
	EvidenceMinTalkers      = 5
	DefaultMinTalkers       = 3
	DefaultTrialsPerSession = 40
	DefaultRateJitter       = 0.10

	DefaultFocusUnlockListeningHours = 40

	// The downloadable full pack is committed in this repository
	// (langpack/release/), served by raw.githubusercontent.com and pinned by
	// SHA-256 below, so a moved branch fails loudly instead of installing
	// something else. It is NOT a release asset: `releases/latest` follows whatever app
	// release is newest and would 404 the moment one is cut (ADR-049).
	// DefaultLangPackSHA256 is the SHA-256 of the COMPRESSED asset, pinned here
	// because the installer never downloads without a checksum (it answers
	// errors.packNotPublished when this is empty). The asset is the pack with
	// only Tatoeba sentences (`make langpack-derive`), 35,311 words and 60,000
	// sentences. These live in the binary and are resolved at point of use by
	// EffectiveLangPack; they are never written into config.yaml.
	DefaultLangPackURL    = "https://raw.githubusercontent.com/antlko/learn-hungarian-ai/main/langpack/release/langpack-hu-v1.sqlite.gz"
	DefaultLangPackSHA256 = "584c4ca9838c2ad98713e6019f2ee20014efb6a6378afa943a7ee6d0274c4acd"
	// DefaultLangPackSizeBytes is the compressed download size, for the button
	// label only. 0 = unknown.
	DefaultLangPackSizeBytes int64 = 65535311

	// legacyLangPackURL was the default before the data release; configs that
	// normalize()d it onto disk are treated as "unset".
	legacyLangPackURL = "https://github.com/antlko/learn-hungarian-ai/releases/latest/download/langpack-hu-v1.sqlite.gz"
)

// Goal re-weights which card directions are generated. See [R12] —
// .claude/research/12-retrieval-direction.md.
const (
	GoalReading      = "reading"
	GoalConversation = "conversation"
	GoalExam         = "exam"
)

// Provider selects where an optional capability is served from. See ADR-005.
const (
	ProviderOff    = "off"
	ProviderLocal  = "local"
	ProviderOpenAI = "openai"
	// ProviderDevice is TTS spoken by the LEARNER'S OWN BROWSER, through the
	// Web Speech API. It has no endpoint and nothing to deploy, which is the
	// whole point: every desktop OS ships a Hungarian voice, and requiring a
	// sidecar for the app to make any sound at all meant a fresh install had
	// no audio anywhere — no 🔊 in the reader, no listening cards, and a
	// settings screen reading "off" four times over.
	//
	// The server cannot verify it. `handleCapabilities` therefore reports it
	// as configured and NOT reachable, and the client decides: it asks
	// `speechSynthesis` for a Hungarian voice and says which one it found, or
	// says the device has none. Never asserted, always checked where it can
	// actually be checked.
	ProviderDevice = "device"
	// ProviderManaged is a sidecar the binary installs and runs ITSELF as a
	// child process (internal/components, ADR-061). Valid for mt and nlp only.
	// The endpoint is never stored: it is an ephemeral 127.0.0.1 port that only
	// the running manager knows, resolved at call time.
	ProviderManaged = "managed"
)

// Config is the whole of config.yaml.
type Config struct {
	Server     Server     `yaml:"server"`
	Locale     Locale     `yaml:"locale"`
	Review     Review     `yaml:"review"`
	Mining     Mining     `yaml:"mining"`
	Drill      Drill      `yaml:"drill"`
	HVPT       HVPT       `yaml:"hvpt"`
	Curriculum Curriculum `yaml:"curriculum"`
	Services   Services   `yaml:"services"`
	LangPack   LangPack   `yaml:"langpack"`
}

// LangPack says where the optional full language pack is downloaded from and
// what its compressed bytes must hash to. Admin-only, set through
// updateConfig like everything else; the URL is never taken from a request
// that starts an install. See ADR-049.
type LangPack struct {
	URL string `yaml:"url"`
	// SHA256 is the lowercase hex digest of the .gz file. Empty = nothing is
	// published and the install refuses.
	SHA256 string `yaml:"sha256"`
}

// EffectiveLangPack is the URL and checksum an install uses: the binary's
// pinned defaults unless the administrator stored their own. A stored URL equal
// to the legacy `releases/latest` default counts as unset.
func EffectiveLangPack(c *Config) (url, sha string) {
	return EffectiveLangPackFor(c, DefaultLangPackURL, DefaultLangPackSHA256)
}

// EffectiveLangPackFor is EffectiveLangPack with the defaults as parameters,
// so a test can exercise a published pin while the repository's is empty.
func EffectiveLangPackFor(c *Config, defURL, defSHA string) (url, sha string) {
	url, sha = c.LangPack.URL, strings.ToLower(strings.TrimSpace(c.LangPack.SHA256))
	if strings.TrimSpace(url) == "" || url == legacyLangPackURL || url == DefaultLangPackURL {
		url = defURL
	}
	if sha == "" {
		sha = defSHA
	}
	return url, sha
}

// validLangPackURL accepts https, and plain http only for a loopback host so a
// test (or a LAN mirror on this machine) can serve a pack without TLS.
func validLangPackURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil // empty = use the binary's default
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("langpack.url: not a URL: %q", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		switch u.Hostname() {
		case "localhost", "127.0.0.1", "::1":
			return nil
		}
	}
	return fmt.Errorf("langpack.url: want https (http only for localhost), got %q", raw)
}

type Server struct {
	Addr string `yaml:"addr"`
}

type Locale struct {
	UI            string `yaml:"ui"`    // ru | en
	Gloss         string `yaml:"gloss"` // ru | en | both
	GlossFallback bool   `yaml:"glossFallback"`
}

type Review struct {
	DesiredRetention float64              `yaml:"desiredRetention"`
	NewPerDay        int                  `yaml:"newPerDay"`
	MaxReviewsPerDay int                  `yaml:"maxReviewsPerDay"` // 0 = unlimited
	DailyMinutes     int                  `yaml:"dailyMinutes"`
	Goal             string               `yaml:"goal"`
	Directions       map[string]Direction `yaml:"directions"`
	// BurySiblings is a pointer because its default is TRUE.
	//
	// normalize() fills only zero-valued fields, and for a plain bool "unset"
	// and "explicitly false" are the same value — so a true default would
	// silently re-enable itself every time a user turned it off. A pointer makes
	// absence distinguishable from a deliberate false, which is the only correct
	// way to give a bool a non-zero default here.
	BurySiblings *bool `yaml:"burySiblings"`
	// AccentTolerance accepts a typed answer that differs from the target only
	// in diacritics — what a plain English keyboard cannot produce. A pointer
	// because its default is TRUE.
	AccentTolerance *bool `yaml:"accentTolerance"`
}

// BurySiblingsEnabled resolves the pointer, defaulting to true.
//
// Showing `recognize` and `produce` of the same note in one session turns the
// second into a copying exercise: the answer is still in the learner's short-term
// memory, so it tests nothing.
func (r Review) BurySiblingsEnabled() bool {
	return r.BurySiblings == nil || *r.BurySiblings
}

// AccentToleranceEnabled resolves the pointer, defaulting to true.
//
// Vowel length and rounding are phonemic in Hungarian — hat and hát are
// different words — so an accepted accentless answer is never a silent pass:
// it rates Hard, the diff shows exactly which mark was dropped, and the error
// class still routes to the ear trainer. What tolerance buys is that a learner
// typing on a plain keyboard is not failed on hardware. The `spell` direction
// is exempt: its whole purpose is the diacritics.
func (r Review) AccentToleranceEnabled() bool {
	return r.AccentTolerance == nil || *r.AccentTolerance
}

// Direction is one review direction. Five exist: recognize, produce, listen,
// cloze, spell. Each carries independent FSRS state per note.
type Direction struct {
	Enabled bool    `yaml:"enabled"`
	Weight  float64 `yaml:"weight"`
}

type Mining struct {
	KnownThreshold float64       `yaml:"knownThreshold"`
	FeatureWeight  float64       `yaml:"featureWeight"`
	TargetUnknown  float64       `yaml:"targetUnknown"`
	UnknownSigma   float64       `yaml:"unknownSigma"`
	MinCoverage    float64       `yaml:"minCoverage"`
	TargetLength   [2]int        `yaml:"targetLength"`
	Weights        MiningWeights `yaml:"weights"`
}

type MiningWeights struct {
	SweetSpot float64 `yaml:"sweetSpot"`
	Coverage  float64 `yaml:"coverage"`
	Utility   float64 `yaml:"utility"`
	Relevance float64 `yaml:"relevance"`
	Length    float64 `yaml:"length"`
	Duplicate float64 `yaml:"duplicate"`
}

type Drill struct {
	GateThreshold    float64      `yaml:"gateThreshold"`
	GateWindow       int          `yaml:"gateWindow"`
	MasteryThreshold float64      `yaml:"masteryThreshold"`
	MasteryWindow    int          `yaml:"masteryWindow"`
	Hysteresis       float64      `yaml:"hysteresis"`
	Definiteness     Definiteness `yaml:"definiteness"`
	HarmonyLatencyMs int          `yaml:"harmonyLatencyMs"`
}

type Definiteness struct {
	DefiniteObjectPct int `yaml:"definiteObjectPct"`
	DemonstrativePct  int `yaml:"demonstrativePct"`
}

type HVPT struct {
	MinTalkers       int     `yaml:"minTalkers"`
	TrialsPerSession int     `yaml:"trialsPerSession"`
	RateJitter       float64 `yaml:"rateJitter"`
}

type Curriculum struct {
	FocusUnlockListeningHours int `yaml:"focusUnlockListeningHours"`
	// ExamDate is the learner's booked exam, ISO date. [R14]: a pre-booked,
	// pre-paid exam is the best-known defence against the month-4–7
	// motivational trough, so the app treats it as a first-class object and
	// counts down to it. Empty = none booked.
	ExamDate string `yaml:"examDate,omitempty"`
}

// Services holds endpoints only. API keys live in SQLite — never in YAML.
type Services struct {
	TTS Service `yaml:"tts"`
	ASR Service `yaml:"asr"`
	NLP Service `yaml:"nlp"`
	LLM Service `yaml:"llm"`
	// MT is sentence translation for the reader — a dedicated translation
	// model, not a chat model. It is deliberately NOT the `llm` slot: what it
	// produces is a crib for a sentence the learner is looking at, and it is
	// labelled as machine output wherever it appears. See ADR-014.
	MT Service `yaml:"mt"`
}

type Service struct {
	Provider string   `yaml:"provider"`
	Endpoint string   `yaml:"endpoint"`
	Model    string   `yaml:"model,omitempty"`
	Voices   []string `yaml:"voices,omitempty"`
}

// Default returns a fresh config with every field populated.
func Default() Config {
	c := Config{}
	c.normalize()
	return c
}

// normalize fills only zero-valued fields. Changing a default therefore affects
// new installs and unset fields; it never silently rewrites a user's stored value.
func (c *Config) normalize() {
	if c.Server.Addr == "" {
		c.Server.Addr = DefaultAddr
	}
	if c.Locale.UI == "" {
		c.Locale.UI = DefaultUILocale
	}
	if c.Locale.Gloss == "" {
		c.Locale.Gloss = DefaultGlossLang
	}
	if c.Review.DesiredRetention == 0 {
		c.Review.DesiredRetention = DefaultDesiredRetention
	}
	if c.Review.NewPerDay == 0 {
		c.Review.NewPerDay = DefaultNewPerDay
	}
	if c.Review.DailyMinutes == 0 {
		c.Review.DailyMinutes = DefaultDailyMinutes
	}
	if c.Review.Goal == "" {
		c.Review.Goal = DefaultGoal
	}
	if c.Review.Directions == nil {
		c.Review.Directions = DefaultDirections()
	}
	if c.Review.AccentTolerance == nil {
		t := true
		c.Review.AccentTolerance = &t
	}
	if c.Review.BurySiblings == nil {
		t := true
		c.Review.BurySiblings = &t
	}
	if c.Mining.KnownThreshold == 0 {
		c.Mining.KnownThreshold = DefaultKnownThreshold
	}
	if c.Mining.FeatureWeight == 0 {
		c.Mining.FeatureWeight = DefaultFeatureWeight
	}
	if c.Mining.TargetUnknown == 0 {
		c.Mining.TargetUnknown = DefaultTargetUnknown
	}
	if c.Mining.UnknownSigma == 0 {
		c.Mining.UnknownSigma = DefaultUnknownSigma
	}
	if c.Mining.MinCoverage == 0 {
		c.Mining.MinCoverage = DefaultMinCoverage
	}
	if c.Mining.TargetLength == [2]int{} {
		c.Mining.TargetLength = [2]int{6, 14}
	}
	if c.Mining.Weights == (MiningWeights{}) {
		c.Mining.Weights = MiningWeights{
			SweetSpot: 1.0, Coverage: 0.8, Utility: 0.6,
			Relevance: 0.5, Length: 0.3, Duplicate: 1.0,
		}
	}
	if c.Drill.GateThreshold == 0 {
		c.Drill.GateThreshold = DefaultGateThreshold
	}
	if c.Drill.GateWindow == 0 {
		c.Drill.GateWindow = DefaultGateWindow
	}
	if c.Drill.MasteryThreshold == 0 {
		c.Drill.MasteryThreshold = DefaultMasteryThreshold
	}
	if c.Drill.MasteryWindow == 0 {
		c.Drill.MasteryWindow = DefaultMasteryWindow
	}
	if c.Drill.Hysteresis == 0 {
		c.Drill.Hysteresis = DefaultHysteresis
	}
	if c.Drill.Definiteness.DefiniteObjectPct == 0 {
		c.Drill.Definiteness.DefiniteObjectPct = DefaultDefiniteObjectPct
	}
	if c.Drill.Definiteness.DemonstrativePct == 0 {
		c.Drill.Definiteness.DemonstrativePct = DefaultDemonstrativePct
	}
	if c.Drill.HarmonyLatencyMs == 0 {
		c.Drill.HarmonyLatencyMs = DefaultHarmonyLatencyMs
	}
	if c.HVPT.MinTalkers == 0 {
		c.HVPT.MinTalkers = DefaultMinTalkers
	}
	if c.HVPT.TrialsPerSession == 0 {
		c.HVPT.TrialsPerSession = DefaultTrialsPerSession
	}
	if c.HVPT.RateJitter == 0 {
		c.HVPT.RateJitter = DefaultRateJitter
	}
	if c.Curriculum.FocusUnlockListeningHours == 0 {
		c.Curriculum.FocusUnlockListeningHours = DefaultFocusUnlockListeningHours
	}
	// LangPack.URL is deliberately NOT filled: empty means "the binary's pin"
	// (EffectiveLangPack). Writing the default to YAML would freeze it and
	// override a future binary's default.
	// SHA256 has no non-empty default on purpose: an unpublished pack must
	// refuse, not download unverified bytes.
	// ExamDate has no default on purpose: an exam the learner did not book is
	// not an exam.
	for _, s := range []*Service{&c.Services.ASR, &c.Services.NLP, &c.Services.LLM, &c.Services.MT} {
		if s.Provider == "" {
			s.Provider = ProviderOff
		}
	}
	// TTS defaults to the device, not to off. It is the one optional service
	// that needs no deployment, and speech is load-bearing for a language the
	// learner has to hear: with it off, `srs.PlanFor` drops the listen and
	// spell directions and the reader has nothing to play.
	if c.Services.TTS.Provider == "" {
		c.Services.TTS.Provider = ProviderDevice
	}
}

// DefaultDirections returns the five review directions with their default
// weights. cloze is weighted highest because it is the only direction that
// grades morphological features; spell is on by default because vowel length is
// phonemic in Hungarian and never gets trained otherwise. See [R12], [R15].
func DefaultDirections() map[string]Direction {
	return map[string]Direction{
		"recognize": {Enabled: true, Weight: 1.0},
		"produce":   {Enabled: true, Weight: 1.4},
		"listen":    {Enabled: true, Weight: 1.0},
		"cloze":     {Enabled: true, Weight: 1.6},
		"spell":     {Enabled: true, Weight: 1.2},
	}
}

// ErrClamped reports fields that Validate adjusted rather than rejected. The api
// layer returns these to the UI so a silent clamp is never invisible to the user.
type ErrClamped struct {
	Field string
	From  float64
	To    float64
}

func (e ErrClamped) Error() string {
	return fmt.Sprintf("%s clamped from %g to %g", e.Field, e.From, e.To)
}

// Validate rejects invalid values. It clamps only where a clamp is documented,
// and reports every clamp it made so the caller can surface it.
func (c *Config) Validate() (clamped []ErrClamped, err error) {
	c.normalize()

	switch c.Locale.UI {
	case "ru", "en":
	default:
		return nil, fmt.Errorf("locale.ui: want ru or en, got %q", c.Locale.UI)
	}
	switch c.Locale.Gloss {
	case "ru", "en", "both":
	default:
		return nil, fmt.Errorf("locale.gloss: want ru, en or both, got %q", c.Locale.Gloss)
	}
	switch c.Review.Goal {
	case GoalReading, GoalConversation, GoalExam:
	default:
		return nil, fmt.Errorf("review.goal: want reading, conversation or exam, got %q", c.Review.Goal)
	}

	if r := c.Review.DesiredRetention; r < MinDesiredRetention || r > MaxDesiredRetention {
		to := min(max(r, MinDesiredRetention), MaxDesiredRetention)
		clamped = append(clamped, ErrClamped{"review.desiredRetention", r, to})
		c.Review.DesiredRetention = to
	}
	if c.Review.NewPerDay < 0 {
		return nil, errors.New("review.newPerDay must be >= 0")
	}
	if c.Review.DailyMinutes <= 0 {
		return nil, errors.New("review.dailyMinutes must be > 0")
	}
	for name := range c.Review.Directions {
		switch name {
		case "recognize", "produce", "listen", "cloze", "spell":
		default:
			return nil, fmt.Errorf("review.directions: unknown direction %q", name)
		}
	}

	if c.Mining.KnownThreshold <= 0 || c.Mining.KnownThreshold >= 1 {
		return nil, errors.New("mining.knownThreshold must be in (0,1)")
	}
	if c.Mining.MinCoverage <= 0 || c.Mining.MinCoverage > 1 {
		return nil, errors.New("mining.minCoverage must be in (0,1]")
	}
	if c.Mining.TargetLength[0] < 1 || c.Mining.TargetLength[1] < c.Mining.TargetLength[0] {
		return nil, errors.New("mining.targetLength must be [min,max] with 1 <= min <= max")
	}

	if c.Drill.GateThreshold <= 0 || c.Drill.GateThreshold > 1 {
		return nil, errors.New("drill.gateThreshold must be in (0,1]")
	}
	if c.Drill.MasteryThreshold < c.Drill.GateThreshold {
		return nil, errors.New("drill.masteryThreshold must be >= drill.gateThreshold")
	}
	if c.Drill.GateWindow < 5 {
		return nil, errors.New("drill.gateWindow must be >= 5")
	}
	if c.Drill.Hysteresis < 0 || c.Drill.Hysteresis >= c.Drill.GateThreshold {
		return nil, errors.New("drill.hysteresis must be in [0, gateThreshold)")
	}
	if p := c.Drill.Definiteness; p.DefiniteObjectPct+p.DemonstrativePct > 100 {
		return nil, errors.New("drill.definiteness percentages must sum to <= 100")
	}

	if c.HVPT.MinTalkers < 2 {
		return nil, errors.New("hvpt.minTalkers must be >= 2")
	}
	if d := c.Curriculum.ExamDate; d != "" {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return nil, fmt.Errorf("curriculum.examDate: want YYYY-MM-DD, got %q", d)
		}
	}
	if c.HVPT.TrialsPerSession < 1 {
		return nil, errors.New("hvpt.trialsPerSession must be >= 1")
	}

	for name, s := range map[string]Service{
		"tts": c.Services.TTS, "asr": c.Services.ASR,
		"nlp": c.Services.NLP, "llm": c.Services.LLM,
		"mt": c.Services.MT,
	} {
		switch s.Provider {
		case ProviderOff:
		case ProviderDevice:
			// Only speech can come from the device, and it never has an
			// endpoint — there is no server involved at all.
			if name != "tts" {
				return nil, fmt.Errorf("services.%s.provider: %q is only valid for tts", name, s.Provider)
			}
		case ProviderOpenAI:
			// The translator speaks its own `/translate`, not the chat API, so
			// accepting `openai` here would promise a protocol we do not talk.
			// Point `mt` at services/mt; point `llm` at the OpenAI-compatible box.
			if name == "mt" || name == "nlp" {
				return nil, fmt.Errorf(
					"services.%s.provider: want off, local or managed, got %q — %s speaks its own protocol, not the OpenAI one",
					name, s.Provider, name)
			}
			if s.Endpoint == "" {
				return nil, fmt.Errorf("services.%s: endpoint required when provider is %q", name, s.Provider)
			}
		case ProviderManaged:
			if name != "mt" && name != "nlp" {
				return nil, fmt.Errorf("services.%s.provider: %q is only valid for mt and nlp", name, s.Provider)
			}
		case ProviderLocal:
			if s.Endpoint == "" {
				return nil, fmt.Errorf("services.%s: endpoint required when provider is %q", name, s.Provider)
			}
		default:
			return nil, fmt.Errorf(
				"services.%s.provider: want off, local or openai%s, got %q",
				name, map[bool]string{true: ", device"}[name == "tts"], s.Provider)
		}
	}
	if err := validLangPackURL(c.LangPack.URL); err != nil {
		return nil, err
	}
	c.LangPack.SHA256 = strings.ToLower(strings.TrimSpace(c.LangPack.SHA256))
	if h := c.LangPack.SHA256; h != "" {
		if _, err := hex.DecodeString(h); err != nil || len(h) != 64 {
			return nil, errors.New("langpack.sha256: want 64 hex characters or empty")
		}
	}
	return clamped, nil
}

// Path returns the config file location inside a data directory.
func Path(dataDir string) string { return filepath.Join(dataDir, "config.yaml") }

// Load reads config.yaml, creating it with defaults if absent.
func Load(dataDir string) (Config, error) {
	p := Path(dataDir)
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		c := Default()
		if err := Save(dataDir, c); err != nil {
			return Config{}, err
		}
		return c, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if _, err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return c, nil
}

// Save writes config.yaml atomically: temp file in the same directory, fsync,
// then rename. A crash mid-write leaves the previous file intact.
func Save(dataDir string, c Config) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	p := Path(dataDir)
	tmp, err := os.CreateTemp(dataDir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
