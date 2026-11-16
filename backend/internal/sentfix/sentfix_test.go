package sentfix

import "testing"

func TestMindenkinekAMagaet(t *testing.T) {
	t.Parallel()
	for _, hu := range []string{"Mindenkinek a magáét.", "  mindenkinek  a magáét. "} {
		if got := Lookup(hu, "ru"); got != "Каждому своё." {
			t.Errorf("ru(%q) = %q", hu, got)
		}
		if got := Lookup(hu, "en"); got != "To each their own." {
			t.Errorf("en(%q) = %q", hu, got)
		}
	}
	if Lookup("Jó napot.", "ru") != "" || Lookup("Mindenkinek a magáét.", "de") != "" {
		t.Error("an unlisted sentence or language must give nothing")
	}
}
