package drill

import "strconv"

// Formatting helpers for Decision.Args.
//
// Numbers are formatted HERE, not in the frontend, so that both locales render
// the same figure. The reason text itself is an i18n key — the backend never
// picks a language. See ADR-004.

func itoa(n int) string     { return strconv.Itoa(n) }
func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// pct renders a 0..1 fraction as a whole-number percentage: 0.784 -> "78".
func pct(f float64) string {
	return strconv.Itoa(int(f*100 + 0.5))
}

func oneDecimal(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}
