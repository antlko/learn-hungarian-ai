// Package sentfix is a small, human-curated list of corrections to sentence
// translations that ship in the language pack.
//
// The pack is read-only (ATTACH), and some of its Tatoeba translations are
// wrong in a way only a person catches: «Mindenkinek a magáét.» is the idiom
// «Каждому своё.», not «Кесарю кесарево.». This list sits between the learner's
// own saved override (which wins) and the pack (which it beats). A fix is human
// work, so it carries no model label. To add one, append an object to
// sentence_fixes.json; `ru` and `en` are each optional. ADR-078.
package sentfix

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed sentence_fixes.json
var raw []byte

type entry struct {
	HU string `json:"hu"`
	RU string `json:"ru"`
	EN string `json:"en"`
}

var (
	once  sync.Once
	byKey map[string]entry
)

func load() {
	byKey = map[string]entry{}
	var list []entry
	if err := json.Unmarshal(raw, &list); err != nil {
		panic("sentfix: sentence_fixes.json is not valid: " + err.Error())
	}
	for _, e := range list {
		byKey[Key(e.HU)] = e
	}
}

// Key normalises a Hungarian sentence for lookup: surrounding space and
// repeated inner space do not matter, case does not either.
func Key(hu string) string {
	return strings.ToLower(strings.Join(strings.Fields(hu), " "))
}

// Lookup returns the curated translation of hu into lang ("ru" or "en"), or "".
func Lookup(hu, lang string) string {
	once.Do(load)
	e, ok := byKey[Key(hu)]
	if !ok {
		return ""
	}
	switch lang {
	case "ru":
		return e.RU
	case "en":
		return e.EN
	}
	return ""
}
