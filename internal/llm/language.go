package llm

import (
	"fmt"
	"strings"
)

// languageNames maps common ISO 639-1 codes to their English names, so the
// model gets a clear, unambiguous instruction ("Write your response in
// Spanish.") rather than a bare code. Unknown codes fall back to the code.
var languageNames = map[string]string{
	"en": "English",
	"es": "Spanish",
	"fr": "French",
	"de": "German",
	"it": "Italian",
	"pt": "Portuguese",
	"nl": "Dutch",
	"ru": "Russian",
	"zh": "Chinese",
	"ja": "Japanese",
	"ko": "Korean",
	"ar": "Arabic",
	"hi": "Hindi",
	"ca": "Catalan",
	"gl": "Galician",
	"eu": "Basque",
	"pl": "Polish",
	"tr": "Turkish",
	"sv": "Swedish",
	"no": "Norwegian",
	"da": "Danish",
	"fi": "Finnish",
	"cs": "Czech",
	"uk": "Ukrainian",
	"el": "Greek",
	"he": "Hebrew",
	"fa": "Persian",
}

// LanguageInstruction returns a prompt suffix asking the model to respond in a
// given ISO 639-1 language. An empty code (or "en") implies English and returns
// an empty string so the caller can keep its existing prompt unchanged.
func LanguageInstruction(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" || code == "en" {
		return ""
	}
	if name, ok := languageNames[code]; ok {
		return fmt.Sprintf("\n\nWrite your response in %s.", name)
	}
	return fmt.Sprintf("\n\nWrite your response in the language with ISO 639-1 code %q.", code)
}
