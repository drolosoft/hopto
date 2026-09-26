package library

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// fallbackID stands in for a name with no usable characters, so an id is
// never empty (usage keys and icon names depend on it).
const fallbackID = "item"

// maxIDLength matches idPattern: one character plus up to 63.
const maxIDLength = 64

// Slug derives an id from a name: lowercase, accents stripped, every run of
// other characters collapsed into one dash, trimmed, capped at 64. A name
// that would start with a reserved prefix gets "item-" in front.
func Slug(name string) string {
	var builder strings.Builder
	lastWasDash := true

	// NFD splits "ó" into "o" plus a combining accent, which is then dropped.
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}

		isWordChar := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isWordChar {
			builder.WriteRune(r)
			lastWasDash = false
			continue
		}

		if !lastWasDash {
			builder.WriteByte('-')
			lastWasDash = true
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return fallbackID
	}

	for _, prefix := range reservedPrefixes {
		if strings.HasPrefix(slug, prefix) {
			slug = fallbackID + "-" + slug
			break
		}
	}

	if len(slug) > maxIDLength {
		slug = strings.TrimRight(slug[:maxIDLength], "-")
	}

	return slug
}

// UniqueID returns base when it is free, otherwise base-2, base-3 and so
// on until taken says no. The suffix is trimmed to keep the 64 limit.
func UniqueID(base string, taken func(string) bool) string {
	if !taken(base) {
		return base
	}

	for counter := 2; ; counter++ {
		suffix := "-" + strconv.Itoa(counter)
		room := maxIDLength - len(suffix)
		candidate := base

		if len(candidate) > room {
			candidate = strings.TrimRight(candidate[:room], "-")
		}

		if !taken(candidate + suffix) {
			return candidate + suffix
		}
	}
}
