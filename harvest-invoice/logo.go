package harvest

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // logos come as PNG, too
	"os"
)

// logoPixels is the logo's width in the PDF: 57.6 mm at 300 dpi, enough for print. An uploaded
// logo is often four times that, and the work log would carry it whole — a megabyte for a page
// of text (12.11.2026).
const logoPixels = 680

// logoJPEG is the logo as the work log embeds it: no wider than logoPixels, on white — the page
// is white, so it looks the same without its transparency — and as a JPEG. The file uploaded
// is left as it is.
func logoJPEG(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("reading the logo: %w", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, shrinkOnWhite(src, logoPixels), &jpeg.Options{Quality: 88}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// shrinkOnWhite is src no wider than width, its proportions kept, each pixel the average of the
// ones it stands for, on white where src is transparent.
func shrinkOnWhite(src image.Image, width int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > width {
		w, h = width, max(1, b.Dy()*width/b.Dx())
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := b.Min.Y+y*b.Dy()/h, b.Min.Y+max((y+1)*b.Dy()/h, y*b.Dy()/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := b.Min.X+x*b.Dx()/w, b.Min.X+max((x+1)*b.Dx()/w, x*b.Dx()/w+1)
			var r, g, bl, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					// Premultiplied, so over white: the colour plus white where it lets it through.
					cr, cg, cb, ca := src.At(sx, sy).RGBA()
					r += uint64(cr + 0xffff - ca)
					g += uint64(cg + 0xffff - ca)
					bl += uint64(cb + 0xffff - ca)
					n++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), 0xff})
		}
	}
	return dst
}
