//go:build !darwin && !windows

package platform

// SystemLanguage has no source to read on other systems.
func SystemLanguage() string {
	return LanguageEnglish
}
