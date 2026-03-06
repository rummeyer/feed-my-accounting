package travelexpense

import (
	"bytes"
	"strings"

	"github.com/go-pdf/fpdf"
)

// createPDF generates a PDF document with smart page breaks and returns it as bytes.
// Blocks are never split across pages — if a block doesn't fit, a new page is added.
// The lieferant name is rendered in bold larger font at the top for sevDesk OCR recognition.
func createPDF(lieferant, header string, blocks []string, footer string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	_, pageHeight := pdf.GetPageSize()
	_, _, _, marginBottom := pdf.GetMargins()
	maxY := pageHeight - marginBottom

	const cellWidth = 300

	pdf.SetFont("Courier", "", 14)
	pdf.MultiCell(cellWidth, pdfLineHeight+1, lieferant, "", "", false)
	pdf.SetFont("Courier", "", pdfFontSize)
	pdf.MultiCell(cellWidth, pdfLineHeight, "Lieferant / Absender\n\n", "", "", false)

	pdf.MultiCell(cellWidth, pdfLineHeight, header, "", "", false)

	for _, block := range blocks {
		blockHeight := float64(strings.Count(block, "\n")+1) * pdfLineHeight
		if pdf.GetY()+blockHeight > maxY {
			pdf.AddPage()
		}
		pdf.MultiCell(cellWidth, pdfLineHeight, block, "", "", false)
	}

	footerHeight := float64(strings.Count(footer, "\n")+1) * pdfLineHeight
	if pdf.GetY()+footerHeight > maxY {
		pdf.AddPage()
	}
	pdf.MultiCell(cellWidth, pdfLineHeight, footer, "", "", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
