package icons

import "testing"

// A 32-bit DIB with alpha keeps it; one whose alpha is all zero (an
// old icon with a mask only) takes its alpha from the mask, where a 0
// bit means opaque.
func TestRGBAFromDIB(t *testing.T) {
	colour := []byte{
		0x10, 0x20, 0x30, 0xFF, 0x00, 0x00, 0x00, 0x00,
	}
	img := rgbaFromDIB(2, 1, colour, nil)

	first := img.RGBAAt(0, 0)
	if first.R != 0x30 || first.G != 0x20 || first.B != 0x10 ||
		first.A != 0xFF {
		t.Errorf("BGRA not swapped: %+v", first)
	}
	if img.RGBAAt(1, 0).A != 0 {
		t.Error("alpha of the second pixel")
	}

	noAlpha := []byte{0x10, 0x20, 0x30, 0x00, 0x40, 0x50, 0x60, 0x00}
	// Mask rows are padded to 32 bits: one row of two pixels is 4 bytes;
	// bit 7 is pixel 0 (0 = opaque), bit 6 pixel 1 (1 = transparent).
	mask := []byte{0x40, 0, 0, 0}
	img = rgbaFromDIB(2, 1, noAlpha, mask)
	if img.RGBAAt(0, 0).A != 0xFF || img.RGBAAt(1, 0).A != 0 {
		t.Errorf("mask alpha: %+v %+v",
			img.RGBAAt(0, 0), img.RGBAAt(1, 0))
	}
}
