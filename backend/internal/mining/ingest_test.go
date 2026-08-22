package mining

import (
	"strings"
	"testing"
)

func TestFromPaste(t *testing.T) {
	got, err := FromPaste("", "  A ház   nagy.\n\n\n\nA kert kicsi.  ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "A ház nagy.\n\nA kert kicsi." {
		t.Errorf("whitespace not normalised: %q", got.Body)
	}
	if got.Title == "" {
		t.Error("a title should be derived when none is given")
	}
}

func TestFromPasteRejectsEmptyAndOversized(t *testing.T) {
	if _, err := FromPaste("", "   \n\n  "); err != ErrEmpty {
		t.Errorf("want ErrEmpty, got %v", err)
	}
	if _, err := FromPaste("", strings.Repeat("a", MaxTextBytes+1)); err == nil {
		t.Error("want an error for an oversized text")
	}
}

func TestFromSRTStripsTimecodesAndRejoinsCaptions(t *testing.T) {
	// A caption split across two lines must be rejoined, or every sentence ends
	// up truncated mid-clause and the analyser sees fragments.
	raw := "1\n" +
		"00:00:01,000 --> 00:00:03,500\n" +
		"<i>Jó reggelt!</i>\n" +
		"\n" +
		"2\n" +
		"00:00:04,000 --> 00:00:07,000\n" +
		"A házaimban sok\n" +
		"könyv van.\n"

	got, err := FromSRT("Film", raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Body, "-->") || strings.Contains(got.Body, "<i>") {
		t.Errorf("timecodes or tags survived: %q", got.Body)
	}
	if !strings.Contains(got.Body, "A házaimban sok könyv van.") {
		t.Errorf("split caption was not rejoined: %q", got.Body)
	}
	if strings.Contains(got.Body, "\n1\n") || strings.Contains(got.Body, "\n2\n") {
		t.Errorf("index lines survived: %q", got.Body)
	}
	if got.Kind != KindSRT {
		t.Errorf("kind: got %q", got.Kind)
	}
}

func TestFromHTMLRemovesScriptContentNotJustTags(t *testing.T) {
	// Untagging a <script> would drop the tag and keep the JavaScript, which
	// would then be handed to the morphological analyser as Hungarian.
	raw := `<html><head><style>.a{color:red}</style>
	<script>var x = "ház";</script></head>
	<body><h1>Cím</h1><p>A ház nagy.</p><p>A kert &amp; a fa.</p></body></html>`

	got, err := FromHTML("", raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Body, "var x") || strings.Contains(got.Body, "color:red") {
		t.Errorf("script or style content survived: %q", got.Body)
	}
	if !strings.Contains(got.Body, "A ház nagy.") {
		t.Errorf("body text lost: %q", got.Body)
	}
	if !strings.Contains(got.Body, "A kert & a fa.") {
		t.Errorf("entities not decoded: %q", got.Body)
	}
	// Block tags must become breaks, or headings run into paragraphs.
	if strings.Contains(got.Body, "CímA ház") {
		t.Errorf("block boundaries lost: %q", got.Body)
	}
}

func TestTitleDerivation(t *testing.T) {
	got, _ := FromPaste("", "Négy új metrókocsi áll forgalomba. A többi később jön.")
	if got.Title != "Négy új metrókocsi áll forgalomba" {
		t.Errorf("title: got %q", got.Title)
	}
	explicit, _ := FromPaste("Telex", "bármi")
	if explicit.Title != "Telex" {
		t.Errorf("an explicit title should win, got %q", explicit.Title)
	}
}

func TestNonBreakingSpacesAreNormalised(t *testing.T) {
	// Copied web text is full of them, and they break tokenisation silently.
	got, err := FromPaste("", "A ház nagy.")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Body, " ") {
		t.Errorf("non-breaking spaces survived: %q", got.Body)
	}
	if got.Body != "A ház nagy." {
		t.Errorf("got %q", got.Body)
	}
}
