package config

import "testing"

// Speech is the one optional capability that needs nothing deployed, and the
// app is unusable without it: with TTS off, `srs.PlanFor` drops the listen and
// spell directions, the reader has nothing to play, and a fresh install shows
// four services reading "off" and makes no sound anywhere.
func TestFreshConfigSpeaksFromTheDevice(t *testing.T) {
	c := Default()
	if c.Services.TTS.Provider != ProviderDevice {
		t.Errorf("tts provider = %q, want %q", c.Services.TTS.Provider, ProviderDevice)
	}
	if c.Services.TTS.Endpoint != "" {
		t.Errorf("device speech has no endpoint, got %q", c.Services.TTS.Endpoint)
	}
	// The other three genuinely need a service someone has to run.
	for name, got := range map[string]string{
		"asr": c.Services.ASR.Provider,
		"nlp": c.Services.NLP.Provider,
		"llm": c.Services.LLM.Provider,
	} {
		if got != ProviderOff {
			t.Errorf("%s provider = %q, want %q", name, got, ProviderOff)
		}
	}
}

// normalize fills zero fields only: an install that deliberately turned speech
// off must stay off across an upgrade.
func TestStoredOffIsNotOverwritten(t *testing.T) {
	c := Config{}
	c.Services.TTS.Provider = ProviderOff
	c.normalize()
	if c.Services.TTS.Provider != ProviderOff {
		t.Errorf("stored value rewritten to %q", c.Services.TTS.Provider)
	}
}

func TestDeviceProviderIsSpeechOnly(t *testing.T) {
	c := Default()
	c.Services.ASR.Provider = ProviderDevice
	if _, err := c.Validate(); err == nil {
		t.Error("device is a speech provider; asr should have been rejected")
	}
}

func TestDeviceSpeechNeedsNoEndpoint(t *testing.T) {
	c := Default()
	if _, err := c.Validate(); err != nil {
		t.Errorf("a fresh config must validate: %v", err)
	}
}
