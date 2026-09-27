package main

import "strings"

// The languages the page speaks; anything else falls back to English.
const (
	languageSpanish = "es"
	languageEnglish = "en"
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

		return languageFor("auto", strings.ToLower(tag))
	}

	return languageEnglish
}

// languageFor picks the page language: an explicit setting wins, "auto"
// (or nothing) follows the system, and only es and en exist.
func languageFor(setting, system string) string {
	if setting == languageSpanish || setting == languageEnglish {
		return setting
	}

	if setting != "auto" && setting != "" {
		return languageEnglish
	}

	if system == languageSpanish || strings.HasPrefix(system, languageSpanish+"-") {
		return languageSpanish
	}

	return languageEnglish
}
