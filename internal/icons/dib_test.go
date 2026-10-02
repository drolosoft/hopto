package icons

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

// A 32-bit DIB with alpha keeps it; one whose alpha is all zero (an
// old icon with a mask only) takes its alpha from the mask, where a 0
// bit means opaque.
func TestNRGBAFromDIB(t *testing.T) {
	colour := []byte{
		0x10, 0x20, 0x30, 0xFF, 0x00, 0x00, 0x00, 0x00,
	}
	img := nrgbaFromDIB(2, 1, colour, nil)

	first := img.NRGBAAt(0, 0)
	if first.R != 0x30 || first.G != 0x20 || first.B != 0x10 ||
		first.A != 0xFF {
		t.Errorf("BGRA not swapped: %+v", first)
	}
	if img.NRGBAAt(1, 0).A != 0 {
		t.Error("alpha of the second pixel")
	}

	noAlpha := []byte{0x10, 0x20, 0x30, 0x00, 0x40, 0x50, 0x60, 0x00}
	// Mask rows are padded to 32 bits: one row of two pixels is 4 bytes;
	// bit 7 is pixel 0 (0 = opaque), bit 6 pixel 1 (1 = transparent).
	mask := []byte{0x40, 0, 0, 0}
	img = nrgbaFromDIB(2, 1, noAlpha, mask)
	if img.NRGBAAt(0, 0).A != 0xFF || img.NRGBAAt(1, 0).A != 0 {
		t.Errorf("mask alpha: %+v %+v",
			img.NRGBAAt(0, 0), img.NRGBAAt(1, 0))
	}
}

// Without alpha and without a mask there is nothing to say a pixel is
// transparent, so every pixel comes out opaque.
func TestNRGBAFromDIBNoMaskIsOpaque(t *testing.T) {
	noAlpha := []byte{0x10, 0x20, 0x30, 0x00, 0x40, 0x50, 0x60, 0x00}
	img := nrgbaFromDIB(2, 1, noAlpha, nil)

	for x := range 2 {
		if img.NRGBAAt(x, 0).A != 0xFF {
			t.Errorf("pixel %d is not opaque", x)
		}
	}
}

// A semi-transparent pixel whose colour is brighter than its alpha is
// valid straight alpha; it must survive PNG encoding unchanged, which
// it would not if the image were treated as premultiplied.
func TestNRGBAFromDIBSurvivesPNGRoundTrip(t *testing.T) {
	colour := []byte{200, 0, 0, 100}
	img := nrgbaFromDIB(1, 1, colour, nil)

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := png.Decode(&encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	got := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
	want := color.NRGBA{R: 0, G: 0, B: 200, A: 100}
	if got != want {
		t.Errorf("round trip: got %+v, want %+v", got, want)
	}
}

// With 33 pixels a mask row is 5 bytes of data padded to 8, so the
// second row only reads right when the stride is honoured.
func TestNRGBAFromDIBMaskStride(t *testing.T) {
	const width, height = 33, 2
	colour := make([]byte, width*height*4)

	// Row 0 is transparent from pixel 0; row 1 only at the last pixel.
	maskStride := 8
	mask := make([]byte, maskStride*height)
	mask[0] = 0x80
	mask[maskStride+4] = 0x80

	img := nrgbaFromDIB(width, height, colour, mask)

	if img.NRGBAAt(0, 0).A != 0 || img.NRGBAAt(1, 0).A != 0xFF {
		t.Errorf("row 0: %+v %+v", img.NRGBAAt(0, 0), img.NRGBAAt(1, 0))
	}
	if img.NRGBAAt(0, 1).A != 0xFF {
		t.Errorf("row 1 pixel 0 read from the wrong row")
	}
	if img.NRGBAAt(32, 1).A != 0 {
		t.Errorf("row 1 pixel 32 should be transparent")
	}
}
