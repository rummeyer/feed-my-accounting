package travelexpense

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	kmRatePerKm     = 0.30 // EUR per kilometer
	verpflegungRate = 14.0 // 8h < 24h meal allowance
	pdfLineHeight   = 5.0
	pdfFontSize     = 11

	lineWidth  = 75
	lineSingle = "---------------------------------------------------------------------------"
	lineDouble = "==========================================================================="
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func documentID(year int, month time.Month) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 4)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return fmt.Sprintf("RK-%d-%02d-%s", year, month, string(b))
}

func formatDate(year int, month time.Month, day int) string {
	return fmt.Sprintf("%02d.%02d.%d", day, month, year)
}

func formatAmount(amount float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", amount), ".", ",", 1)
}

func rightAlign(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// ---------------------------------------------------------------------------
// Document Content Builders
// ---------------------------------------------------------------------------

func buildDocumentHeader(year int, month time.Month, dateString, periodStart, periodEnd, title string) string {
	var b strings.Builder

	header := fmt.Sprintf("%s %02d/%d", strings.ToUpper(title), month, year)
	padding := (lineWidth - len(header)) / 2
	b.WriteString(lineDouble + "\n")
	b.WriteString(fmt.Sprintf("%s%s\n", strings.Repeat(" ", padding), header))
	b.WriteString(lineDouble + "\n\n")

	b.WriteString(fmt.Sprintf("Beleg-Nr.:            %s\n", documentID(year, month)))
	b.WriteString(fmt.Sprintf("Datum:                %s\n", dateString))
	b.WriteString(fmt.Sprintf("Rechnungsart:         Reisekosten - %s\n", title))
	b.WriteString(fmt.Sprintf("Abrechnungszeitraum:  %s - %s\n", periodStart, periodEnd))
	b.WriteString("\n")

	return b.String()
}

func buildCustomerHeader(c Customer) string {
	var b strings.Builder

	b.WriteString(lineSingle + "\n")
	b.WriteString(fmt.Sprintf("%s) %s\n", c.ID, c.Name))
	b.WriteString(lineSingle + "\n\n")

	b.WriteString(fmt.Sprintf("Von:    %s\n", c.From))
	b.WriteString(fmt.Sprintf("Nach:   %s\n", c.To))
	b.WriteString(fmt.Sprintf("Grund:  %s\n\n", c.Reason))

	return b.String()
}

func buildKilometerEntry(dateString string, distanceKm int) string {
	var b strings.Builder

	amount := float64(distanceKm) * kmRatePerKm
	amountStr := formatAmount(amount) + " EUR"

	b.WriteString(fmt.Sprintf("  %s\n", dateString))
	b.WriteString(fmt.Sprintf("    Fahrkosten (%d km x 0,30 EUR)%s\n\n",
		distanceKm, rightAlign(amountStr, 45-len(fmt.Sprintf("Fahrkosten (%d km x 0,30 EUR)", distanceKm)))))

	return b.String()
}

func buildMealAllowanceEntry(dateString string) string {
	var b strings.Builder

	amountStr := "14,00 EUR"

	b.WriteString(fmt.Sprintf("  %s  (07:00 - 17:00)\n", dateString))
	b.WriteString(fmt.Sprintf("    Verpflegungsmehraufwand (8h - 24h)%s\n\n",
		rightAlign(amountStr, 45-len("Verpflegungsmehraufwand (8h - 24h)"))))

	return b.String()
}

func buildDocumentFooter(totalAmount float64) string {
	var b strings.Builder

	amountStr := formatAmount(totalAmount) + " EUR"

	b.WriteString(lineDouble + "\n")
	b.WriteString(fmt.Sprintf("Rechnungsbetrag:%s\n", rightAlign(amountStr, 59)))
	b.WriteString(lineDouble + "\n")

	return b.String()
}
