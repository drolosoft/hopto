package platform

import "strings"

// The languages the page speaks; anything else falls back to English.
const (
	LanguageSpanish = "es"
	LanguageEnglish = "en"
)

// parseAppleLanguages reads the first tag out of what `defaults read -g
// AppleLanguages` prints, a parenthesised list such as ("es-ES", "en-US"),
// and reduces it to a language hopto speaks.
func parseAppleLanguages(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		tag := strings.Trim(strings.TrimSpace(line), "\",")
		if tag == "" || tag == "(" || tag == ")" {
			continue
		}

		return LanguageFor("auto", strings.ToLower(tag))
	}

	return LanguageEnglish
}

// LanguageFor picks the page language: an explicit setting wins, "auto"
// (or nothing) follows the system, and only es and en exist.
func LanguageFor(setting, system string) string {
	if setting == LanguageSpanish || setting == LanguageEnglish {
		return setting
	}

	if setting != "auto" && setting != "" {
		return LanguageEnglish
	}

	spanish := system == LanguageSpanish ||
		strings.HasPrefix(system, LanguageSpanish+"-")
	if spanish {
		return LanguageSpanish
	}

	return LanguageEnglish
}
