// Package icons turns whatever an app bundle or a web site offers into the
// PNG the page shows: it reads .icns files, fetches favicons and
// apple-touch-icons, and re-encodes every image before it is served or
// saved, so nothing reaches the WKWebView that Go has not decoded first.
package icons

import (
	"bytes"
	"encoding/binary"
	"os"
)

// icnsHeaderBytes is the fixed file header ("icns" + total length); the
// smallest file that can hold one entry header on top of it is 16 bytes.
const icnsHeaderBytes = 8

// icnsTypes are the icon sizes inside an .icns, best first: 256 px, then
// 128 px, then the bigger ones. These entries hold plain PNG (or JPEG 2000,
// which is skipped) rather than the raw formats of the small sizes.
var icnsTypes = []string{"ic08", "ic07", "ic09", "ic13", "ic12"}

// PNG returns the PNG of the best-sized icon in an .icns file, exactly as
// stored. The container is a list of 4-byte type + 4-byte big-endian length
// (including the 8-byte entry header) + data, after an 8-byte file header.
func PNG(icnsPath string) ([]byte, bool) {
	data, err := os.ReadFile(icnsPath)
	if err != nil || len(data) < 2*icnsHeaderBytes ||
		string(data[:4]) != "icns" {
		return nil, false
	}

	found := map[string][]byte{}

	for offset := icnsHeaderBytes; offset+icnsHeaderBytes <= len(data); {
		kind := string(data[offset : offset+4])
		length := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))

		// A corrupt length would loop forever or run past the end.
		if length < icnsHeaderBytes || offset+length > len(data) {
			break
		}

		body := data[offset+icnsHeaderBytes : offset+length]
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
