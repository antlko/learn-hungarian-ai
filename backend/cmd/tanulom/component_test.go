package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"tanulom/internal/components"
	"tanulom/internal/components/bundlefix"
)

func TestComponentArgs(t *testing.T) {
	rest, dir, _, _, ok := componentArgs([]string{"component", "run", "mt"})
	if !ok || dir != "" || !reflect.DeepEqual(rest, []string{"run", "mt"}) {
		t.Fatalf("%v %q %v", rest, dir, ok)
	}
	rest, dir, _, _, ok = componentArgs([]string{"--data-dir", "/x", "component", "run", "mt"})
	if !ok || dir != "/x" || len(rest) != 2 {
		t.Fatalf("%v %q %v", rest, dir, ok)
	}
	rest, dir, voices, rt, ok := componentArgs([]string{"--voices-dir", "/v", "--data-dir=/x", "--tts-runtime-dir", "/r", "component", "run", "tts"})
	if !ok || dir != "/x" || voices != "/v" || rt != "/r" || len(rest) != 2 {
		t.Fatalf("%v %q %q %q %v", rest, dir, voices, rt, ok)
	}
	// The server's own invocations are never taken for this mode.
	for _, a := range [][]string{nil, {"--addr", ":1"}, {"reset-password"}, {"--version"}} {
		if _, _, _, _, ok := componentArgs(a); ok {
			t.Fatalf("%v taken as component mode", a)
		}
	}
}

func TestComponentRunUsageErrorsExitTwo(t *testing.T) {
	for _, args := range [][]string{
		nil, {"stop", "mt"}, {"run"}, {"run", "asr"}, {"run", "mt", "--port", "80"},
		{"run", "mt", "--host", "example.com"}, {"run", "mt", "extra"}, {"run", "mt", "--url", "http://x"},
	} {
		var o, e bytes.Buffer
		if code := runComponentTo(args, t.TempDir(), &o, &e, nil); code != 2 {
			t.Errorf("%v: exit %d, want 2 (%s)", args, code, e.String())
		}
	}
}

func TestComponentRunTTSWithoutVoicesIsRefusedBeforeAnyInstall(t *testing.T) {
	voices := t.TempDir() // empty: no hu_HU-*-medium pair
	bundle := t.TempDir()
	if err := bundlefix.Write(bundle, "linux-amd64", bundlefix.Options{}); err != nil {
		t.Fatal(err)
	}
	var o, e bytes.Buffer
	mk := func(dir string) *components.Manager {
		return components.New(components.Options{DataDir: dir, VoicesDir: voices, TTSRuntimeDir: bundle, GOOS: "linux", GOARCH: "amd64"})
	}
	code := runComponentTo([]string{"run", "tts", "--port", "5999"}, t.TempDir(), &o, &e, mk)
	if code != 1 || !strings.Contains(e.String(), "No Piper voice files") {
		t.Fatalf("exit %d, stderr %q", code, e.String())
	}
}

func TestComponentRunTTSWithoutARuntimeSaysSoAndNamesTheFlag(t *testing.T) {
	var o, e bytes.Buffer
	mk := func(dir string) *components.Manager {
		return components.New(components.Options{DataDir: dir, GOOS: "windows", GOARCH: "amd64"})
	}
	code := runComponentTo([]string{"run", "tts", "--port", "5999"}, t.TempDir(), &o, &e, mk)
	if code != 1 || !strings.Contains(e.String(), "--tts-runtime-dir") {
		t.Fatalf("exit %d, stderr %q", code, e.String())
	}
}
