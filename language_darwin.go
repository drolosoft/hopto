package main

import "os/exec"

// systemLanguage asks macOS for the user's preferred languages. An app
// opened with `open` has no LANG in its environment, so the defaults
// database is the only reliable source. Any failure means English.
func systemLanguage() string {
	output, err := exec.Command("/usr/bin/defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return languageEnglish
	}

	return parseAppleLanguages(string(output))
}
