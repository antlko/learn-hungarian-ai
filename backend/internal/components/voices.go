package components

import (
	"os"
	"path/filepath"
)

// Personas are the three Piper voices the app names, in display order. The file
// for persona p is hu_HU-<p>-medium.onnx plus hu_HU-<p>-medium.onnx.json.
var Personas = []string{"anna", "berta", "imre"}

// VoiceFile is the model file name of a persona.
func VoiceFile(persona string) string { return "hu_HU-" + persona + "-medium.onnx" }

// VoicesDir is the directory the tts child is told to read: the operator's
// --voices-dir when it holds a voice, else the voices downloaded at install
// (`<data>/components/tts/voices`), else `<data>/voices`. Read from Options only:
// no route, no config field.
func (m *Manager) VoicesDir() string {
	dir, _ := m.voiceSource()
	return dir
}

// Voices lists the personas whose model AND config files are present in that
// source, in Personas order. It reads on every call (two stats per persona), so
// adding a file needs no restart.
func (m *Manager) Voices() []string {
	_, v := m.voiceSource()
	return v
}

// FindVoices is Voices for an explicit directory.
func FindVoices(dir string) []string {
	var out []string
	for _, p := range Personas {
		f := filepath.Join(dir, VoiceFile(p))
		if fileExists(f) && fileExists(f+".json") {
			out = append(out, p)
		}
	}
	return out
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
