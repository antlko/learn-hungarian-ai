package config

// Clone returns a deep copy. This is the first step of the only sanctioned way to
// change the running config:
//
//	clone -> mutate -> validate -> atomic save -> swap under lock
//
// IMPORTANT: when adding a new field to Config that contains a map, slice or
// pointer, add it here too. A missed field means two "different" configs sharing
// backing memory, and a mutation on the clone silently corrupts the live one.
func (c Config) Clone() Config {
	out := c // copies every value field

	if c.Review.Directions != nil {
		out.Review.Directions = make(map[string]Direction, len(c.Review.Directions))
		for k, v := range c.Review.Directions {
			out.Review.Directions[k] = v
		}
	}
	if c.Services.TTS.Voices != nil {
		out.Services.TTS.Voices = append([]string(nil), c.Services.TTS.Voices...)
	}
	if c.Services.ASR.Voices != nil {
		out.Services.ASR.Voices = append([]string(nil), c.Services.ASR.Voices...)
	}
	if c.Services.LLM.Voices != nil {
		out.Services.LLM.Voices = append([]string(nil), c.Services.LLM.Voices...)
	}
	if c.Services.MT.Voices != nil {
		out.Services.MT.Voices = append([]string(nil), c.Services.MT.Voices...)
	}
	if c.Review.BurySiblings != nil {
		v := *c.Review.BurySiblings
		out.Review.BurySiblings = &v
	}
	if c.Review.AccentTolerance != nil {
		v := *c.Review.AccentTolerance
		out.Review.AccentTolerance = &v
	}
	// LangPack holds two strings only: copied by value above, nothing to deep-copy.
	// Mining.TargetLength is a [2]int array, not a slice: copied by value above.
	return out
}
