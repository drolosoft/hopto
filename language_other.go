//go:build !darwin

package main

// systemLanguage has no source to read on other systems.
func systemLanguage() string {
	return languageEnglish
}
