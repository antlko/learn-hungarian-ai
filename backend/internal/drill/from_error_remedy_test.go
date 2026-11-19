package drill

import (
	"testing"

	"tanulom/internal/morph"
)

// ADR-084: vowel-length and vowel-quality errors are perception problems with
// no drill behind them; they must not claim one.
func TestPerceptionErrorsHaveNoDrill(t *testing.T) {
	for _, c := range []morph.ErrorClass{morph.ErrVowelLength, morph.ErrVowelQuality} {
		r, kinds := remedyFor(c)
		if r != RemedyPerception || kinds != nil || r.Actionable() {
			t.Errorf("%v: remedy=%q kinds=%v actionable=%v", c, r, kinds, r.Actionable())
		}
	}
}
