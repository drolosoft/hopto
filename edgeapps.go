package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"howett.net/plist"
)

// edgeAppsDir, under the home folder, is where Edge (and Chrome) put the
// bundles of "installed" web apps. Every .app in it is a launcher entry,
// found on each showing so a newly installed app appears without a rebuild.
const edgeAppsDir = "Applications/Edge Apps.localized"

// edgeCategory is the chip these apps sit under.
const edgeCategory = "edge"

// edgeInfo is the part of a web app bundle's Info.plist the launcher reads:
// the name Edge gave it and the URL it opens.
type edgeInfo struct {
	Name string `plist:"CrAppModeShortcutName"`
	URL  string `plist:"CrAppModeShortcutURL"`
}

// scanEdgeApps lists the web app bundles in dir as launcher entries, with
// the icon extracted from the bundle so the page needs no file of its own.
// A missing folder is not an error: the tab just has no Edge apps.
func scanEdgeApps(dir string) []Entry {
	bundles, err := filepath.Glob(filepath.Join(dir, "*.app"))
	if err != nil {
		return nil
	}

	apps := make([]Entry, 0, len(bundles))
	for _, bundle := range bundles {
		apps = append(apps, edgeEntry(bundle))
	}

	return apps
}

// edgeEntry builds the entry of one bundle. The folder name is the name
// (it is what the user sees in the Finder); the plist adds the URL, shown
// as the description so two apps on the same site can be told apart.
func edgeEntry(bundle string) Entry {
	name := strings.TrimSuffix(filepath.Base(bundle), ".app")

	entry := Entry{
		ID:          "edge-" + slug(name),
		Name:        name,
		Description: "App de Edge",
		Bundle:      filepath.Base(bundle),
		Category:    edgeCategory,
		Path:        bundle,
	}

	// The host is enough as description: a full URL overflows the card.
	if info, err := readEdgeInfo(filepath.Join(bundle, "Contents", "Info.plist")); err == nil {
		if host, err := hostOf(info.URL); err == nil {
			entry.Description = host
		}
	}

	if png, ok := pngFromICNS(filepath.Join(bundle, "Contents", "Resources", "app.icns")); ok {
		entry.Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}

	return entry
}

// readEdgeInfo decodes the two keys of interest from a (binary or XML)
// Info.plist.
func readEdgeInfo(path string) (edgeInfo, error) {
	var info edgeInfo

	data, err := os.ReadFile(path)
	if err != nil {
		return info, err
	}

	_, err = plist.Unmarshal(data, &info)

	return info, err
}

// Icon sizes inside an .icns, best first: 256 px, then 128 px, then the
// bigger ones. These entries hold plain PNG data, so no decoding is needed.
var icnsTypes = []string{"ic08", "ic07", "ic09", "ic13", "ic12"}

// pngFromICNS returns the PNG of the best-sized icon in an .icns file. The
// container is a list of 4-byte type + 4-byte big-endian length (including
// the 8-byte header) + data, after an 8-byte file header.
func pngFromICNS(path string) ([]byte, bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 8 || string(data[:4]) != "icns" {
		return nil, false
	}

	found := map[string][]byte{}

	for offset := 8; offset+8 <= len(data); {
		kind := string(data[offset : offset+4])
		length := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))

		// A corrupt length would loop forever or run past the end.
		if length < 8 || offset+length > len(data) {
			break
		}

		body := data[offset+8 : offset+length]
		if bytes.HasPrefix(body, []byte("\x89PNG")) {
			found[kind] = body
		}

		offset += length
	}

	for _, kind := range icnsTypes {
		if png, ok := found[kind]; ok {
			return png, true
		}
	}

	return nil, false
}

// slug turns "Outlook (PWA)" into "outlook-pwa": lowercase letters and
// digits, single dashes between them, for the usage keys and DOM ids.
func slug(name string) string {
	var out strings.Builder
	dash := false

	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			dash = false
			continue
		}

		if !dash && out.Len() > 0 {
			out.WriteByte('-')
			dash = true
		}
	}

	return strings.TrimSuffix(out.String(), "-")
}
