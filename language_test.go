package main

import "testing"

// The output of `defaults read -g AppleLanguages` is a parenthesised list
// of quoted tags; the first one is the user's language.
func TestParseAppleLanguages(t *testing.T) {
	cases := map[string]string{
		"(\n    \"es-ES\",\n    \"en-US\"\n)\n": "es",
		"(\n    \"en-GB\",\n    \"es-ES\"\n)\n": "en",
		"(\n    \"ca-ES\",\n    \"es-ES\"\n)\n": "en",
		"(\n    es\n)\n":                        "es",
		"":                                      "en",
		"garbage":                               "en",
	}

	for output, want := range cases {
		if got := parseAppleLanguages(output); got != want {
			t.Errorf("parseAppleLanguages(%q) = %q, want %q", output, got, want)
		}
	}
}

// The setting wins over the system; "auto" follows the system; anything
// else falls back to English.
func TestLanguageFor(t *testing.T) {
	cases := []struct{ setting, system, want string }{
		{"auto", "es", "es"},
		{"auto", "en", "en"},
		{"es", "en", "es"},
		{"en", "es", "en"},
		{"", "es", "es"},
		{"fr", "es", "en"},
	}

	for _, tc := range cases {
		if got := languageFor(tc.setting, tc.system); got != tc.want {
			t.Errorf("languageFor(%q, %q) = %q, want %q", tc.setting, tc.system, got, tc.want)
		}
	}
}
