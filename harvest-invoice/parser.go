// Package harvest reads a month's hours from the Harvest API — or an uploaded Harvest report
// PDF — and creates a matching draft invoice in sevDesk via browser automation.
package harvest

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// ReportData holds the extracted information from a Harvest report.
type ReportData struct {
	TotalHours float64
	PeriodFrom time.Time // first day of reported period
	PeriodTo   time.Time // last day of reported period
	ClientName string    // client in sevDesk
}

// date pattern dd.mm.yyyy
var datePattern = regexp.MustCompile(`(\d{2})\.(\d{2})\.(\d{4})`)

// totalHoursPattern matches "157,00 Hours" or "157.00 Hours", and "157,00 Stunden" as the work
// log feed-my-accounting makes has it.
var totalHoursPattern = regexp.MustCompile(`([\d.,]+)\s+(?:Hours|Stunden)`)

// ParsePDFHours extracts the total hours from a Harvest PDF report.
// Looks for the pattern "157,00 Hours" near the top of the document.
func ParsePDFHours(pdfData []byte) (float64, error) {
	text, err := pdfText(pdfData)
	if err != nil {
		return 0, err
	}
	return hoursIn(text)
}

// ParseReportPDF reads an uploaded Harvest report: its total hours and, where it states them as
// dd.mm.yyyy, the first and last day of its period. The period is left
// zero if the PDF does not state it; the caller then has to give it.
func ParseReportPDF(pdfData []byte) (*ReportData, error) {
	text, err := pdfText(pdfData)
	if err != nil {
		return nil, err
	}
	hours, err := hoursIn(text)
	if err != nil {
		return nil, err
	}
	report := &ReportData{TotalHours: hours}
	if dates := datePattern.FindAllStringSubmatch(text, 2); len(dates) == 2 {
		report.PeriodFrom, _ = parseDate(dates[0])
		report.PeriodTo, _ = parseDate(dates[1])
	}
	return report, nil
}

// pdfText is the plain text of a report's first two pages, where the total and period are.
func pdfText(pdfData []byte) (string, error) {
	pdfReader, err := pdf.NewReader(bytes.NewReader(pdfData), int64(len(pdfData)))
	if err != nil {
		return "", fmt.Errorf("opening PDF: %w", err)
	}
	var text strings.Builder
	for i := 1; i <= min(pdfReader.NumPage(), 2); i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		if content, err := page.GetPlainText(nil); err == nil {
			text.WriteString(content)
		}
	}
	return text.String(), nil
}

func hoursIn(pdfText string) (float64, error) {
	if matches := totalHoursPattern.FindStringSubmatch(pdfText); len(matches) >= 2 {
		return parseFloat(matches[1])
	}
	return 0, fmt.Errorf("could not find total hours in PDF text")
}

func parseDate(match []string) (time.Time, error) {
	if len(match) < 4 {
		return time.Time{}, fmt.Errorf("invalid date match")
	}
	day, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	year, _ := strconv.Atoi(match[3])
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}

// parseFloat handles both "160.5" and "160,5" formats.
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	return strconv.ParseFloat(s, 64)
}
