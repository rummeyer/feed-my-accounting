package travelexpense

import (
	"strings"

	"feed-my-accounting/pdfdoc"
)

// createPDF generates a PDF document with smart page breaks and returns it as bytes.
// Blocks are never split across pages — if a block doesn't fit, a new page is added.
// The lieferant name is rendered in bold larger font at the top for sevDesk OCR recognition.
func createPDF(lieferant, header string, blocks []string, footer string) ([]byte, error) {
	pdf := pdfdoc.New()
	// The PDF's own encoding, cp1252: "Übernachtung", a client "Müller", as written.
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	lieferant, header, footer = tr(lieferant), tr(header), tr(footer)

	const cellWidth = 300

	pdf.SetFont("Courier", "", 14)
	pdf.MultiCell(cellWidth, pdfLineHeight+1, lieferant, "", "", false)
	pdf.SetFont("Courier", "", pdfFontSize)
	pdf.MultiCell(cellWidth, pdfLineHeight, "Lieferant / Absender\n\n", "", "", false)

	pdf.MultiCell(cellWidth, pdfLineHeight, header, "", "", false)

	for _, block := range append(blocks, footer) {
		pdf.Keep(float64(strings.Count(block, "\n")+1) * pdfLineHeight)
		pdf.MultiCell(cellWidth, pdfLineHeight, tr(block), "", "", false)
	}
	return pdf.Bytes()
}
