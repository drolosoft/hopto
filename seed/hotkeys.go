package seed

import (
	"bytes"
	"regexp"
)

// The two shortcut lines of the seed, matched whole so nothing else in
// the file can be touched. The optional \r lets them match a seed with
// CRLF line endings, which a Windows checkout may produce.
var (
	appsLine  = regexp.MustCompile(`(?m)^hotkey_apps = ".*"\r?$`)
	linksLine = regexp.MustCompile(`(?m)^hotkey_links = ".*"\r?$`)
)

// replaceLine rewrites every match of pattern as `key = "value"`, keeping
// the line ending the match had so the file's endings do not change.
func replaceLine(
	data []byte, pattern *regexp.Regexp, key, value string,
) []byte {
	return pattern.ReplaceAllFunc(data, func(match []byte) []byte {
		line := key + ` = "` + value + `"`
		if bytes.HasSuffix(match, []byte("\r")) {
			line += "\r"
		}

		return []byte(line)
	})
}

// withHotkeys returns the seed with its two shortcut lines replaced:
// the links and the categories are the same on every system, only the
// keys differ.
func withHotkeys(data []byte, apps, links string) []byte {
	out := replaceLine(data, appsLine, "hotkey_apps", apps)

	return replaceLine(out, linksLine, "hotkey_links", links)
}
