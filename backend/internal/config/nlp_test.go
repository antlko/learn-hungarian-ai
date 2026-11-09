package config

import "testing"

func TestNLPProviderValidation(t *testing.T) {
	ok := []Service{{}, {Provider: ProviderOff}, {Provider: ProviderLocal, Endpoint: "http://nlp:5002"}}
	for _, svc := range ok {
		c := Default()
		c.Services.NLP = svc
		if _, err := c.Validate(); err != nil {
			t.Errorf("%+v should validate: %v", svc, err)
		}
	}
	bad := []Service{
		{Provider: "huspacy", Endpoint: "http://x"},
		{Provider: ProviderOpenAI, Endpoint: "http://x"}, // speaks its own protocol
		{Provider: ProviderDevice},
		{Provider: ProviderLocal}, // no endpoint
	}
	for _, svc := range bad {
		c := Default()
		c.Services.NLP = svc
		if _, err := c.Validate(); err == nil {
			t.Errorf("%+v must be refused", svc)
		}
	}
}

// "managed" is a sidecar the binary runs itself (ADR-061): valid for mt and nlp
// only, with no endpoint stored.
func TestManagedProviderValidation(t *testing.T) {
	for _, name := range []string{"mt", "nlp"} {
		c := Default()
		svc := &c.Services.MT
		if name == "nlp" {
			svc = &c.Services.NLP
		}
		svc.Provider = ProviderManaged
		if _, err := c.Validate(); err != nil {
			t.Errorf("%s managed should validate: %v", name, err)
		}
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
	c := Default()
	c.Services.MT = Service{Provider: ProviderLocal, Endpoint: "http://mt:5003"}
	if _, err := c.Validate(); err != nil || c.Services.MT.Endpoint != "http://mt:5003" {
		t.Fatalf("local value changed: %v %+v", err, c.Services.MT)
	}
}
