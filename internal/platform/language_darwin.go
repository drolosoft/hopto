package platform

import "os/exec"

// SystemLanguage asks macOS for the user's preferred languages. An app
// opened with `open` has no LANG in its environment, so the defaults
// database is the only reliable source. Any failure means English.
func SystemLanguage() string {
	output, err := exec.Command(
		"/usr/bin/defaults", "read", "-g", "AppleLanguages",
	).Output()
	if err != nil {
		return LanguageEnglish
	}

	return parseAppleLanguages(string(output))
}
