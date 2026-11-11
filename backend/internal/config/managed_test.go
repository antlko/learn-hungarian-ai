package config

import (
	"os"
	"strings"
	"testing"
)

// "managed" is a sidecar the binary runs itself (ADR-061): valid for mt only,
// with no endpoint stored.
func TestManagedProviderValidation(t *testing.T) {
	c := Default()
	c.Services.MT.Provider = ProviderManaged
	if _, err := c.Validate(); err != nil {
		t.Errorf("mt managed should validate: %v", err)
	}
	for name, set := range map[string]func(*Config){
		"tts": func(c *Config) { c.Services.TTS.Provider = ProviderManaged },
		"asr": func(c *Config) { c.Services.ASR.Provider = ProviderManaged },
		"llm": func(c *Config) { c.Services.LLM.Provider = ProviderManaged },
	} {
		c := Default()
		set(&c)
		if _, err := c.Validate(); err == nil {
			t.Errorf("%s must refuse managed", name)
		}
	}
	// Existing values are untouched by normalize.
	c = Default()
	c.Services.MT = Service{Provider: ProviderLocal, Endpoint: "http://mt:5003"}
	if _, err := c.Validate(); err != nil || c.Services.MT.Endpoint != "http://mt:5003" {
		t.Fatalf("local value changed: %v %+v", err, c.Services.MT)
	}
}

// ADR-069 removed the nlp service. A config.yaml written before that still has
// a services.nlp block: it must load (unknown keys are ignored) and the block
// is gone after the next write.
func TestLegacyNLPBlockLoadsAndIsDroppedOnSave(t *testing.T) {
	dir := t.TempDir()
	legacy := "services:\n  nlp:\n    provider: managed\n  mt:\n    provider: local\n    endpoint: http://mt:5003\n"
	if err := os.WriteFile(Path(dir), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("legacy config must load: %v", err)
	}
	if c.Services.MT.Endpoint != "http://mt:5003" {
		t.Errorf("mt lost: %+v", c.Services.MT)
	}
	if err := Save(dir, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(Path(dir))
	if strings.Contains(string(b), "nlp") {
		t.Errorf("nlp block survived a save:\n%s", b)
	}
}
