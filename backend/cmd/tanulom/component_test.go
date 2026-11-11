package main

import (
	"bytes"
	"reflect"
	"testing"
)

func TestComponentArgs(t *testing.T) {
	rest, dir, ok := componentArgs([]string{"component", "run", "mt"})
	if !ok || dir != "" || !reflect.DeepEqual(rest, []string{"run", "mt"}) {
		t.Fatalf("%v %q %v", rest, dir, ok)
	}
	rest, dir, ok = componentArgs([]string{"--data-dir", "/x", "component", "run", "mt"})
	if !ok || dir != "/x" || len(rest) != 2 {
		t.Fatalf("%v %q %v", rest, dir, ok)
	}
	// The server's own invocations are never taken for this mode.
	for _, a := range [][]string{nil, {"--addr", ":1"}, {"reset-password"}, {"--version"}} {
		if _, _, ok := componentArgs(a); ok {
			t.Fatalf("%v taken as component mode", a)
		}
	}
}

func TestComponentRunUsageErrorsExitTwo(t *testing.T) {
	for _, args := range [][]string{
		nil, {"stop", "mt"}, {"run"}, {"run", "tts"}, {"run", "mt", "--port", "80"},
		{"run", "mt", "--host", "example.com"}, {"run", "mt", "extra"}, {"run", "mt", "--url", "http://x"},
	} {
		var o, e bytes.Buffer
		if code := runComponentTo(args, t.TempDir(), &o, &e, nil); code != 2 {
			t.Errorf("%v: exit %d, want 2 (%s)", args, code, e.String())
		}
	}
}
