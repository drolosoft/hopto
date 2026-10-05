package icons

import "image"

// nrgbaFromDIB turns the pixels GetDIBits hands back (top-down, 32 bits,
// BGRA) into an image. GDI keeps the alpha straight (not multiplied into
// the colour), which is what image.NRGBA stores; image.RGBA would be read
// as premultiplied and the soft edges of the icon would come out wrong.
// An icon drawn before alpha existed has every alpha byte at zero and a
// 1-bit mask instead; when the whole alpha channel is zero the mask
// decides, with a set bit meaning transparent.
func nrgbaFromDIB(
	width, height int, colour, mask []byte,
) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
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

// The sizes Windows draws icons at, smallest first. A corner icon is cut
// to the first one that holds its drawing, so it keeps its own margin.
var iconSizes = []int{16, 24, 32, 48, 64, 128}

// drawnPart is the part of a drawn icon that holds the icon. The jumbo
// image list has no 256 px picture for a program that ships small icons
// only: it draws the 48 px one in the top-left corner of an empty 256 px
// canvas, which in a grid tile looks like no icon at all. When everything
// drawn fits in the top-left quarter, the result is that corner, cut to
// the icon size that holds it. It reports false for a canvas with nothing
// on it, so the caller can try another source.
func drawnPart(img *image.NRGBA) (*image.NRGBA, bool) {
	bounds := img.Bounds()

	// The furthest column and row with a pixel that is not transparent.
	reach := -1
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.NRGBAAt(x, y).A == 0 {
				continue
			}

			reach = max(reach, x-bounds.Min.X, y-bounds.Min.Y)
		}
	}

	if reach < 0 {
		return nil, false
	}

	if reach >= bounds.Dx()/2 {
		return img, true
	}

	for _, size := range iconSizes {
		if reach < size && size < bounds.Dx() {
			corner := image.Rect(0, 0, size, size).Add(bounds.Min)

			return img.SubImage(corner).(*image.NRGBA), true
		}
	}

	return img, true
}
