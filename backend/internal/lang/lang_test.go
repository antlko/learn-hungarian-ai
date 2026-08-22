package lang

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		present, v := Detect(t.TempDir())
		if present || v != "" {
			t.Errorf("empty dir: got (%v, %q), want (false, \"\")", present, v)
		}
	})

	t.Run("missing dir is not an error", func(t *testing.T) {
		if present, _ := Detect(filepath.Join(t.TempDir(), "nope")); present {
			t.Error("nonexistent dir reported a pack")
		}
	})

	t.Run("picks the highest version", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{
			"langpack-hu-v1.sqlite",
			"langpack-hu-v2.sqlite",
			"tanulom.db",            // must be ignored
			"config.yaml",           // must be ignored
			"langpack-de-v9.sqlite", // wrong language, must be ignored
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		present, v := Detect(dir)
		if !present || v != "2" {
			t.Errorf("got (%v, %q), want (true, \"2\")", present, v)
		}
	})
}

func TestPath(t *testing.T) {
	got := Path("/data", "1")
	want := filepath.Join("/data", "langpack-hu-v1.sqlite")
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	// Path and Detect must agree on the naming scheme.
	if !FilePattern.MatchString(filepath.Base(got)) {
		t.Errorf("Path produced %q which FilePattern does not match", filepath.Base(got))
	}
}
