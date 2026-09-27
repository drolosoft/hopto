package icons

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

// square encodes a solid image of the given size in the given format.
func square(t *testing.T, width, height int, format string) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.NRGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}

	var out bytes.Buffer
	var err error

	switch format {
	case "png":
		err = png.Encode(&out, img)
	case "jpeg":
		err = jpeg.Encode(&out, img, nil)
	case "gif":
		err = gif.Encode(&out, img, nil)
	}

	if err != nil {
		t.Fatal(err)
	}

	return out.Bytes()
}

// size decodes the output and returns its dimensions, failing unless it is
// a PNG.
func size(t *testing.T, data []byte) (int, int) {
	t.Helper()

	if !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Fatal("output is not a PNG")
	}

	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	return config.Width, config.Height
}

// Every accepted format comes out as a PNG no bigger than the side asked
// for, with the aspect ratio kept.
func TestNormalizeFitsAndReencodes(t *testing.T) {
	cases := []struct {
		name         string
		data         []byte
		side         int
		wantW, wantH int
	}{
		{"large png shrinks", square(t, 512, 512, "png"), 128, 128, 128},
		{"small png stays", square(t, 64, 64, "png"), 128, 64, 64},
		{"wide keeps ratio", square(t, 300, 100, "png"), 128, 128, 43},
		{"tall keeps ratio", square(t, 100, 300, "png"), 128, 43, 128},
		{"jpeg becomes png", square(t, 256, 256, "jpeg"), 128, 128, 128},
		{"gif becomes png", square(t, 256, 256, "gif"), 256, 256, 256},
		{"exact side untouched", square(t, 1024, 1024, "png"), 1024, 1024, 1024},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Normalize(tc.data, tc.side)
			if err != nil {
				t.Fatal(err)
			}

			w, h := size(t, out)
			if w != tc.wantW || h != tc.wantH {
				t.Errorf("got %dx%d, want %dx%d", w, h, tc.wantW, tc.wantH)
			}
		})
	}
}

// What must never become an icon: text, SVG, a PNG that declares a huge
// canvas, an empty payload, a header with nothing behind it.
func TestNormalizeRejects(t *testing.T) {
	hugeHeader := square(t, 1, 1, "png")
	// Patch the IHDR width to 20000: DecodeConfig reads it before any
	// pixel, and a 20000x20000 canvas is 1.6 GB of memory if decoded.
	hugeHeader = append([]byte{}, hugeHeader...)
	hugeHeader[16] = 0
	hugeHeader[17] = 0
	hugeHeader[18] = 0x4e
	hugeHeader[19] = 0x20
	// The decoder checks the IHDR checksum before looking at the size, so
	// the patched chunk needs a matching CRC (type + data, bytes 12 to 28).
	crc := crc32.ChecksumIEEE(hugeHeader[12:29])
	binary.BigEndian.PutUint32(hugeHeader[29:33], crc)

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"html as png", []byte("<!DOCTYPE html><html></html>"), ErrNotAnImage},
		{"svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), ErrNotAnImage},
		{"empty", []byte{}, ErrNotAnImage},
		{"huge canvas", hugeHeader, ErrTooBig},
		{"too wide", square(t, 1025, 10, "png"), ErrTooBig},
		{"png header only", []byte("\x89PNG\r\n\x1a\n"), ErrNotAnImage},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Normalize(tc.data, 128)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// The average of a solid colour is that colour: the box filter must not
// darken or lighten the image.
func TestDownscaleKeepsSolidColour(t *testing.T) {
	out, err := Normalize(square(t, 500, 500, "png"), 100)
	if err != nil {
		t.Fatal(err)
	}

	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(50, 50).RGBA()
	if r>>8 != 200 || g>>8 != 40 || b>>8 != 40 || a>>8 != 255 {
		t.Errorf("pixel = %d %d %d %d", r>>8, g>>8, b>>8, a>>8)
	}
}
