package icons

import "image"

// rgbaFromDIB turns the pixels GetDIBits hands back (top-down, 32 bits,
// BGRA) into an RGBA image. An icon drawn before alpha existed has every
// alpha byte at zero and a 1-bit mask instead; when the whole alpha
// channel is zero the mask decides, with a set bit meaning transparent.
func rgbaFromDIB(width, height int, colour, mask []byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if len(colour) < width*height*4 {
		return img
	}

	hasAlpha := false
	for index := 3; index < width*height*4; index += 4 {
		if colour[index] != 0 {
			hasAlpha = true
			break
		}
	}

	// A mask row is padded to a multiple of 4 bytes.
	maskStride := ((width + 31) / 32) * 4

	for y := range height {
		for x := range width {
			offset := (y*width + x) * 4
			pixel := img.Pix[offset:]
			pixel[0] = colour[offset+2]
			pixel[1] = colour[offset+1]
			pixel[2] = colour[offset]

			switch {
			case hasAlpha:
				pixel[3] = colour[offset+3]
			case len(mask) >= maskStride*height:
				bit := mask[y*maskStride+x/8] & (0x80 >> (x % 8))
				if bit == 0 {
					pixel[3] = 0xFF
				}
			default:
				pixel[3] = 0xFF
			}
		}
	}

	return img
}
