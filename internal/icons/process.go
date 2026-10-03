package icons

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"

	// Registered so image.Decode understands the three formats a favicon
	// or an apple-touch-icon may come in; the output is always PNG.
	_ "image/gif"
	_ "image/jpeg"
)

// ErrNotAnImage is returned for bytes that are not a PNG, JPEG or GIF.
var ErrNotAnImage = errors.New("icons: not a PNG, JPEG or GIF image")

// ErrTooBig is returned for an image larger than maxSourceSide a side.
var ErrTooBig = errors.New("icons: image larger than 1024 pixels a side")

// maxSourceSide caps what gets decoded: 1024² is the largest icon macOS
// itself uses, and anything past it is a memory bomb, not an icon.
const maxSourceSide = 1024

// The sizes the page shows: link rows at 32 px on Retina need 64, but 128
// keeps a hand-picked icon crisp in the editor preview; app cards show
// icons at 64 px, and 256 is the size macOS bundles ship anyway.
const (
	LinkSide = 128
	AppSide  = 256
)

// Normalize decodes data, refuses anything oversized, shrinks it to fit in
// maxSide (never enlarging) and returns a freshly encoded PNG. The
// re-encoding is the point: only pixels Go has decoded reach the page or
// the disk.
func Normalize(data []byte, maxSide int) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotAnImage, err)
	}

	tooBig := config.Width > maxSourceSide || config.Height > maxSourceSide
	if config.Width <= 0 || config.Height <= 0 || tooBig {
		return nil, fmt.Errorf("%w: %dx%d", ErrTooBig, config.Width, config.Height)
	}

	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotAnImage, err)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, fit(source, maxSide)); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// fit returns the image as is when it already fits, or a copy scaled down
// so its longer side is maxSide.
func fit(source image.Image, maxSide int) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	if width <= maxSide && height <= maxSide {
		return source
	}

	scale := float64(maxSide) / float64(max(width, height))
	targetWidth := max(1, int(math.Round(float64(width)*scale)))
	targetHeight := max(1, int(math.Round(float64(height)*scale)))

	return downscale(source, targetWidth, targetHeight)
}

// downscale averages the source pixels that fall into each target pixel (a
// box filter). It works on premultiplied RGBA so transparent edges do not
// bleed their colour, and it only ever shrinks, which is all an icon needs.
// Written by hand to keep golang.org/x/image out of the dependencies.
func downscale(
	source image.Image, targetWidth, targetHeight int,
) *image.RGBA {
	bounds := source.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()

	premultiplied := image.NewRGBA(image.Rect(0, 0, sourceWidth, sourceHeight))
	draw.Draw(
		premultiplied, premultiplied.Bounds(), source, bounds.Min, draw.Src,
	)

	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	for y := range targetHeight {
		top := y * sourceHeight / targetHeight
		bottom := max(top+1, (y+1)*sourceHeight/targetHeight)

		for x := range targetWidth {
			left := x * sourceWidth / targetWidth
			right := max(left+1, (x+1)*sourceWidth/targetWidth)

			var red, green, blue, alpha uint64
			for row := top; row < bottom; row++ {
				start := row*premultiplied.Stride + left*4
				pixels := premultiplied.Pix[start : start+(right-left)*4]

				for index := 0; index < len(pixels); index += 4 {
					red += uint64(pixels[index])
					green += uint64(pixels[index+1])
					blue += uint64(pixels[index+2])
					alpha += uint64(pixels[index+3])
				}
			}

			count := uint64((bottom - top) * (right - left))
			offset := y*target.Stride + x*4
			target.Pix[offset] = uint8(red / count)
			target.Pix[offset+1] = uint8(green / count)
			target.Pix[offset+2] = uint8(blue / count)
			target.Pix[offset+3] = uint8(alpha / count)
		}
	}

	return target
}
