package lang

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDetect(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		v, ok := Detect(t.TempDir())
		if ok || v != "" {
			t.Errorf("empty dir: got (%q, %v), want (\"\", false)", v, ok)
		}
	})

	t.Run("missing dir is not an error", func(t *testing.T) {
		if _, ok := Detect(filepath.Join(t.TempDir(), "nope")); ok {
			t.Error("nonexistent dir reported a pack")
		}
	})

	// The rule that matters: a NAME is not a pack. A truncated download leaves
	// a file Detect used to accept, after which the server booted with every
	// language feature dead behind a message telling the user to fetch a pack
	// that appeared to already be there.
	t.Run("a file that does not open is not a pack", func(t *testing.T) {
		dir := t.TempDir()
		for name, body := range map[string]string{
			"langpack-hu-v1.sqlite": "",           // zero bytes
			"langpack-hu-v2.sqlite": "not sqlite", // garbage
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if v, ok := Detect(dir); ok {
			t.Errorf("got (%q, true) for two unusable files, want false", v)
		}
	})

	t.Run("picks the highest USABLE version", func(t *testing.T) {
		dir := t.TempDir()
		// v2 is real, v1 is a truncated leftover: the newest usable one wins.
		writeTestPack(t, filepath.Join(dir, "langpack-hu-v2.sqlite"), "2")
		for _, name := range []string{
			"langpack-hu-v1.sqlite",
			"tanulom.db",            // must be ignored
			"config.yaml",           // must be ignored
			"langpack-de-v9.sqlite", // wrong language, must be ignored
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		v, ok := Detect(dir)
		if !ok || v != "2" {
			t.Errorf("got (%q, %v), want (\"2\", true)", v, ok)
		}
	})

	// A broken newest pack must fall back rather than leave the app with
	// nothing: an older pack is a worse experience, silence is a dead app.
	t.Run("falls back past a broken newer pack", func(t *testing.T) {
		dir := t.TempDir()
		writeTestPack(t, filepath.Join(dir, "langpack-hu-v1.sqlite"), "1")
		if err := os.WriteFile(filepath.Join(dir, "langpack-hu-v2.sqlite"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if v, ok := Detect(dir); !ok || v != "1" {
			t.Errorf("got (%q, %v), want (\"1\", true)", v, ok)
		}
	})
}

// writeTestPack lays down the minimum a pack needs to open: the metadata Open
// verifies. Anything more belongs in the packages that read those tables.
func writeTestPack(t *testing.T, path, version string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"version": version, "schema_version": "1", "gloss_langs": `["en"]`,
	} {
		if _, err := db.Exec(`INSERT INTO pack_meta VALUES (?,?)`, k, v); err != nil {
			t.Fatal(err)
		}
	}
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
