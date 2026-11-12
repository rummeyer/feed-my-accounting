package harvest

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // logos come as PNG, too
	"os"

	"github.com/go-pdf/fpdf"
)

// logoPixels is the logo's width in the PDF: 57.6 mm at 300 dpi, enough for print. An uploaded
// logo is often four times that, and the work log would carry it whole — a megabyte for a page
// of text (12.11.2026).
const logoPixels = 680

// logoBytes is the largest logo the work log embeds as uploaded, when it is no wider than
// logoPixels too: compressed already, as one made for sevDesk is (42 KB, 12.11.2026).
const logoBytes = 150 << 10

// logoForPDF is the logo as the work log embeds it, and its type for fpdf: as uploaded, when it
// is small already and fpdf reads it — a PNG keeps its transparency then — else shrunk to a JPEG.
func logoForPDF(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if config, format, err := image.DecodeConfig(bytes.NewReader(data)); err == nil &&
		len(data) <= logoBytes && config.Width <= logoPixels && !paletteAlpha(config.ColorModel) {
		kind := map[string]string{"png": "PNG", "jpeg": "JPG"}[format]
		// fpdf reads PNGs its own way, not every kind of them: tried on a page of its own.
		trial := fpdf.New("P", "mm", "A4", "")
		trial.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: kind}, bytes.NewReader(data))
		if kind != "" && trial.Error() == nil {
			return data, kind, nil
		}
	}
	shrunk, err := logoJPEG(path)
	return shrunk, "JPG", err
}

// paletteAlpha says whether a PNG's colours are a palette with transparent ones in it. fpdf
// draws those opaque — the half transparent pixels at a drawing's edges dark specks (12.11.2026,
// a logo of 256 colours) — so such a logo is shrunk to a JPEG on white instead.
func paletteAlpha(model color.Model) bool {
	palette, ok := model.(color.Palette)
	if !ok {
		return false
	}
	for _, c := range palette {
		if _, _, _, a := c.RGBA(); a != 0xffff {
			return true
		}
	}
	return false
}

// logoJPEG is the logo as the work log embeds it when it is not small already: no wider than logoPixels, on white — the page
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
