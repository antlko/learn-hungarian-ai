// Package config owns config.yaml: the hand-editable settings file. The learning
// domain lives in SQLite, not here, and secrets never appear in this file.
//
// Invariant: the running config is mutated only through the api layer's
// updateConfig (clone -> mutate -> validate -> atomic save -> swap). Never write
// to a *Config that is already installed as the active one.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Exported defaults. These live in exactly one place so that API fallbacks cannot
// drift from what a fresh config gets. Do not re-hardcode any of them at a use site.
const (
	DefaultAddr = ":8080"

	DefaultUILocale  = "ru"
	DefaultGlossLang = "ru"

	DefaultDesiredRetention = 0.90
	MinDesiredRetention     = 0.80
	MaxDesiredRetention     = 0.95
	DefaultNewPerDay        = 20
	DefaultDailyMinutes     = 90
	DefaultGoal             = GoalConversation

	// Mining weights: see docs/ARCHITECTURE.md 5.1 and [R6].
	DefaultKnownThreshold = 0.80
	DefaultFeatureWeight  = 0.70
	DefaultTargetUnknown  = 1.5
	DefaultUnknownSigma   = 0.8
	DefaultMinCoverage    = 0.90

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
	DefaultMinTalkers       = 5
	DefaultTrialsPerSession = 40
	DefaultRateJitter       = 0.10

	DefaultFocusUnlockListeningHours = 40
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
	BurySiblings     bool                 `yaml:"burySiblings"`
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
}

// Services holds endpoints only. API keys live in SQLite — never in YAML.
type Services struct {
	TTS Service `yaml:"tts"`
	ASR Service `yaml:"asr"`
	NLP Service `yaml:"nlp"`
	LLM Service `yaml:"llm"`
}

type Service struct {
	Provider string   `yaml:"provider"`
	Endpoint string   `yaml:"endpoint"`
	Model    string   `yaml:"model,omitempty"`
	Voices   []string `yaml:"voices,omitempty"`
}

type LangPack struct {
	AutoDownload bool   `yaml:"autoDownload"`
	ReleaseURL   string `yaml:"releaseUrl"`
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
	for _, s := range []*Service{&c.Services.TTS, &c.Services.ASR, &c.Services.NLP, &c.Services.LLM} {
		if s.Provider == "" {
			s.Provider = ProviderOff
		}
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
	if c.HVPT.TrialsPerSession < 1 {
		return nil, errors.New("hvpt.trialsPerSession must be >= 1")
	}

	for name, s := range map[string]Service{
		"tts": c.Services.TTS, "asr": c.Services.ASR,
		"nlp": c.Services.NLP, "llm": c.Services.LLM,
	} {
		switch s.Provider {
		case ProviderOff:
		case ProviderLocal, ProviderOpenAI:
			if s.Endpoint == "" {
				return nil, fmt.Errorf("services.%s: endpoint required when provider is %q", name, s.Provider)
			}
		default:
			return nil, fmt.Errorf("services.%s.provider: want off, local or openai, got %q", name, s.Provider)
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
