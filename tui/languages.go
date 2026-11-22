package tui

import (
	"strings"

	"github.com/hongnoul/mangap/source"
)

// language is a selectable MangaDex translation language.
type language struct{ code, name string }

// languages are common MangaDex translatedLanguage codes. Mangapill is
// English only, so this preference affects MangaDex.
var languages = []language{
	{"en", "English"},
	{"any", "Any language"},
	{"ja", "Japanese"},
	{"ko", "Korean"},
	{"zh", "Chinese (simplified)"},
	{"zh-hk", "Chinese (traditional)"},
	{"es", "Spanish"},
	{"es-la", "Spanish (Latin America)"},
	{"pt-br", "Portuguese (Brazil)"},
	{"pt", "Portuguese"},
	{"fr", "French"},
	{"de", "German"},
	{"it", "Italian"},
	{"ru", "Russian"},
	{"uk", "Ukrainian"},
	{"pl", "Polish"},
	{"tr", "Turkish"},
	{"ar", "Arabic"},
	{"id", "Indonesian"},
	{"vi", "Vietnamese"},
	{"th", "Thai"},
}

// languageIndex finds code in languages, appending unknown codes from the
// config so they are preserved.
func languageIndex(code string) int {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		code = "en"
	}
	for i, l := range languages {
		if l.code == code {
			return i
		}
	}
	languages = append(languages, language{code, code})
	return len(languages) - 1
}

// sourceNames is "Mangapill, Mangadex" for status lines.
func sourceNames(srcs []source.Source) string {
	var names []string
	for _, s := range srcs {
		names = append(names, s.Name())
	}
	return strings.Join(names, ", ")
}
