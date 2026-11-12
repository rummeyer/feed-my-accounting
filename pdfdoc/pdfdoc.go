// Package pdfdoc is what the PDFs feed-my-accounting writes have in common: A4 pages, blocks
// that are never split across two of them, and the file they become.
package pdfdoc

import (
	"bytes"

	"github.com/go-pdf/fpdf"
)

// Document is a generated PDF and the name it is saved or sent under.
type Document struct {
	Filename string
	Data     []byte
}

// Doc is an A4 page in portrait, in millimetres, the first page added.
type Doc struct {
	*fpdf.Fpdf
}

// New is a document with its first page; header and footer, set before, are drawn on it too.
func New(setup ...func(*fpdf.Fpdf)) *Doc {
	pdf := fpdf.New("P", "mm", "A4", "")
	for _, s := range setup {
		s(pdf)
	}
	pdf.AddPage()
	return &Doc{pdf}
}

// Room is the height left on the page above its bottom margin.
func (d *Doc) Room() float64 {
	_, height := d.GetPageSize()
	_, _, _, bottom := d.GetMargins()
	return height - bottom - d.GetY()
}

// Keep starts a new page unless a block of this height fits on this one: a block is never split.
func (d *Doc) Keep(height float64) {
	if height > d.Room() {
		d.AddPage()
	}
}

// Bytes is the finished PDF.
func (d *Doc) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
