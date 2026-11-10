package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	c := Default()
	clamped, err := c.Validate()
	if err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if len(clamped) != 0 {
		t.Fatalf("default config should need no clamping, got %v", clamped)
	}
	if got := len(c.Review.Directions); got != 5 {
		t.Errorf("want 5 review directions, got %d", got)
	}
}

func TestNormalizeFillsOnlyZeroFields(t *testing.T) {
	// A user who set an unusual-but-valid value must keep it.
	c := Config{}
	c.Review.NewPerDay = 7
	c.Review.DesiredRetention = 0.83
	c.Drill.GateThreshold = 0.65
	c.normalize()

	if c.Review.NewPerDay != 7 {
		t.Errorf("newPerDay overwritten: got %d, want 7", c.Review.NewPerDay)
	}
	if c.Review.DesiredRetention != 0.83 {
		t.Errorf("desiredRetention overwritten: got %v, want 0.83", c.Review.DesiredRetention)
	}
	if c.Drill.GateThreshold != 0.65 {
		t.Errorf("gateThreshold overwritten: got %v, want 0.65", c.Drill.GateThreshold)
	}
	if c.Review.DailyMinutes != DefaultDailyMinutes {
		t.Errorf("unset field not filled: dailyMinutes = %d", c.Review.DailyMinutes)
	}
}

func TestValidateClampsRetentionAndReportsIt(t *testing.T) {
	c := Default()
	c.Review.DesiredRetention = 0.99

	clamped, err := c.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(clamped) != 1 || clamped[0].Field != "review.desiredRetention" {
		t.Fatalf("want one clamp on desiredRetention, got %v", clamped)
	}
	if c.Review.DesiredRetention != MaxDesiredRetention {
		t.Errorf("not clamped: got %v, want %v", c.Review.DesiredRetention, MaxDesiredRetention)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"bad ui locale", func(c *Config) { c.Locale.UI = "hu" }},
		{"bad gloss lang", func(c *Config) { c.Locale.Gloss = "de" }},
		{"bad goal", func(c *Config) { c.Review.Goal = "fluency" }},
		{"unknown direction", func(c *Config) { c.Review.Directions["shout"] = Direction{} }},
		{"mastery below gate", func(c *Config) { c.Drill.MasteryThreshold = 0.5; c.Drill.GateThreshold = 0.8 }},
		{"hysteresis too large", func(c *Config) { c.Drill.Hysteresis = 0.9 }},
		{"definiteness over 100", func(c *Config) {
			c.Drill.Definiteness.DefiniteObjectPct = 80
			c.Drill.Definiteness.DemonstrativePct = 40
		}},
		{"too few talkers", func(c *Config) { c.HVPT.MinTalkers = 1 }},
		{"provider without endpoint", func(c *Config) {
			c.Services.TTS = Service{Provider: ProviderLocal}
		}},
		{"unknown provider", func(c *Config) {
			c.Services.LLM = Service{Provider: "magic", Endpoint: "http://x"}
		}},
		{"zero daily minutes", func(c *Config) { c.Review.DailyMinutes = -1 }},
		{"bad target length", func(c *Config) { c.Mining.TargetLength = [2]int{14, 6} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Default()
			tt.mutate(&c)
			if _, err := c.Validate(); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestCloneIsDeep(t *testing.T) {
	orig := Default()
	orig.Services.TTS = Service{Provider: ProviderLocal, Endpoint: "http://tts", Voices: []string{"anna", "berta"}}

	cp := orig.Clone()
	cp.Review.Directions["cloze"] = Direction{Enabled: false, Weight: 9}
	cp.Services.TTS.Voices[0] = "MUTATED"
	cp.Review.NewPerDay = 999

	if d := orig.Review.Directions["cloze"]; !d.Enabled || d.Weight == 9 {
		t.Error("mutating the clone's directions map changed the original")
	}
	if orig.Services.TTS.Voices[0] != "anna" {
		t.Error("mutating the clone's voices slice changed the original")
	}
	if orig.Review.NewPerDay == 999 {
		t.Error("mutating the clone's scalar changed the original")
	}
}

func TestSaveIsAtomicAndRoundTrips(t *testing.T) {
	dir := t.TempDir()

	c := Default()
	c.Review.NewPerDay = 13
	c.Locale.Gloss = "both"
	c.Services.TTS = Service{Provider: ProviderLocal, Endpoint: "http://tts:5000", Voices: []string{"anna"}}

	if err := Save(dir, c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Review.NewPerDay != 13 {
		t.Errorf("newPerDay: got %d, want 13", got.Review.NewPerDay)
	}
	if got.Locale.Gloss != "both" {
		t.Errorf("gloss: got %q, want both", got.Locale.Gloss)
	}
	if len(got.Services.TTS.Voices) != 1 || got.Services.TTS.Voices[0] != "anna" {
		t.Errorf("voices round-trip failed: %v", got.Services.TTS.Voices)
	}

	// No temp files may survive a successful save.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "config.yaml" {
			t.Errorf("stray file after save: %s", e.Name())
		}
	}
}

func TestLoadCreatesDefaultWhenAbsent(t *testing.T) {
	dir := t.TempDir()

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Review.DesiredRetention != DefaultDesiredRetention {
		t.Errorf("got %v, want default retention", c.Review.DesiredRetention)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Errorf("Load should have written config.yaml: %v", err)
	}
}

func TestLoadRejectsInvalidStoredConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("locale:\n  ui: klingon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("want error for invalid stored config, got nil")
	}
}

func TestBurySiblingsDefaultsTrueAndRespectsAnExplicitFalse(t *testing.T) {
	// A plain bool cannot carry a true default through normalize(), because
	// "unset" and "explicitly false" are the same value — the setting would
	// silently switch itself back on every time the user turned it off.
	if !Default().Review.BurySiblingsEnabled() {
		t.Error("burySiblings should default to true")
	}

	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("review:\n  burySiblings: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Review.BurySiblingsEnabled() {
		t.Error("an explicit false was overwritten by the default")
	}

	// And it must survive a save/load round trip rather than reverting.
	if err := Save(dir, got); err != nil {
		t.Fatal(err)
	}
	again, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.Review.BurySiblingsEnabled() {
		t.Error("explicit false lost on round trip")
	}
}

func TestEffectiveLangPack(t *testing.T) {
	const defURL, defSHA = "https://example.com/d.gz", "aa"
	legacy := "https://github.com/antlko/learn-hungarian-ai/releases/latest/download/langpack-hu-v1.sqlite.gz"
	cases := []struct{ name, url, sha, wantURL, wantSHA string }{
		{"empty takes both defaults", "", "", defURL, defSHA},
		{"legacy latest URL maps to default", legacy, "", defURL, defSHA},
		{"custom url and sha win", "https://m.example/p.gz", "BB", "https://m.example/p.gz", "bb"},
		{"custom sha only", "", "cc", defURL, "cc"},
	}
	for _, tc := range cases {
		c := &Config{LangPack: LangPack{URL: tc.url, SHA256: tc.sha}}
		u, s := EffectiveLangPackFor(c, defURL, defSHA)
		if u != tc.wantURL || s != tc.wantSHA {
			t.Errorf("%s: got %q %q", tc.name, u, s)
		}
		if c.LangPack.URL != tc.url || c.LangPack.SHA256 != tc.sha {
			t.Errorf("%s: config mutated", tc.name)
		}
	}
	if strings.Contains(DefaultLangPackURL, "/latest/") || !strings.Contains(DefaultLangPackURL, "/releases/download/langpack-hu-v1/") {
		t.Errorf("default URL = %s", DefaultLangPackURL)
	}
	if u, s := EffectiveLangPack(&Config{}); u != DefaultLangPackURL || s != DefaultLangPackSHA256 {
		t.Errorf("EffectiveLangPack(zero) = %q %q", u, s)
	}
}

func TestLangPackURLStaysEmptyOnDisk(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(dir) // fresh dir: writes the default config
	if err != nil {
		t.Fatal(err)
	}
	if c.LangPack.URL != "" {
		t.Errorf("loaded url = %q, want empty", c.LangPack.URL)
	}
	b, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "langpack-hu-v1") {
		t.Errorf("default URL written to YAML:\n%s", b)
	}
	if _, err := c.Validate(); err != nil {
		t.Errorf("empty langpack.url must validate: %v", err)
	}
	if u, _ := EffectiveLangPack(&Config{LangPack: LangPack{URL: DefaultLangPackURL}}); u != DefaultLangPackURL {
		t.Errorf("stored default URL = %q", u)
	}
}
