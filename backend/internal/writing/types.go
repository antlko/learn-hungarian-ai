package writing

import "strings"

// Error types: the closed list a teacher may use. "other" is the landing place
// for anything unrecognised and is not offered to the model.
const (
	TypeSpelling     = "spelling"
	TypeVowelLength  = "vowel_length"
	TypeHarmony      = "harmony"
	TypeCase         = "case"
	TypePossessive   = "possessive"
	TypeDefiniteness = "definiteness"
	TypeAgreement    = "agreement"
	TypeVerbForm     = "verb_form"
	TypePreverb      = "preverb"
	TypeWordOrder    = "word_order"
	TypeArticle      = "article"
	TypeWordChoice   = "word_choice"
	TypeMissingWord  = "missing_word"
	TypeExtraWord    = "extra_word"
	TypePunctuation  = "punctuation"
	TypeStyle        = "style"
	TypeOther        = "other"
)

// Types is the closed TYPE list in prompt order, ending with "other".
var Types = []string{
	TypeSpelling, TypeVowelLength, TypeHarmony, TypeCase, TypePossessive,
	TypeDefiniteness, TypeAgreement, TypeVerbForm, TypePreverb, TypeWordOrder,
	TypeArticle, TypeWordChoice, TypeMissingWord, TypeExtraWord,
	TypePunctuation, TypeStyle, TypeOther,
}

// GrammarTopicHint maps an error type to the code of the pack grammar topic that
// best explains it. It is advice only: the API layer must check that the code
// exists in the installed pack and omit the link otherwise. Types with no
// single fitting lesson are absent.
var grammarTopicHint = map[string]string{
	TypeSpelling:     "orthography",
	TypeVowelLength:  "orthography",
	TypeHarmony:      "vowel-harmony",
	TypeCase:         "locative-grid",
	TypePossessive:   "possessive",
	TypeDefiniteness: "definite-conjugation",
	TypeVerbForm:     "present-indefinite",
	TypePreverb:      "preverbs",
	TypeWordOrder:    "focus-word-order",
	TypeStyle:        "register",
}

// GrammarTopicHint returns the suggested pack grammar-topic code for an error
// type, or "" when there is none.
func GrammarTopicHint(errType string) string { return grammarTopicHint[errType] }

// normalizeType maps free model output ("Vowel length", "case (illative)") onto
// the closed list, falling back to "other".
func normalizeType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	end := 0
	for end < len(s) {
		c := s[end]
		if (c >= 'a' && c <= 'z') || c == '_' || c == ' ' || c == '-' {
			end++
			continue
		}
		break
	}
	s = strings.TrimSpace(s[:end])
	s = strings.NewReplacer(" ", "_", "-", "_").Replace(s)
	if contains(Types, s) {
		return s
	}
	return TypeOther
}
