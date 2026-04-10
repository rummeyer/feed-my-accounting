// Package harvest fetches Harvest monthly report emails, downloads the PDF
// export, extracts total hours and service period, and creates a matching
// invoice in sevDesk via browser automation.
package harvest

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ledongthuc/pdf"
)

// ReportData holds the extracted information from a Harvest report.
type ReportData struct {
	TotalHours  float64
	PeriodFrom  time.Time // first day of reported period
	PeriodTo    time.Time // last day of reported period
	ExportURL   string    // download link from the email
	ClientName string // client mentioned in the email
	PDFData    []byte // raw PDF bytes for attaching to invoice
}

// date pattern dd.mm.yyyy
var datePattern = regexp.MustCompile(`(\d{2})\.(\d{2})\.(\d{4})`)

// totalHoursPattern matches "157,00 Hours" or "157.00 Hours"
var totalHoursPattern = regexp.MustCompile(`([\d.,]+)\s+Hours`)

// ParseEmail extracts the download link and period from a Harvest export notification email.
func ParseEmail(html string) (*ReportData, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	data := &ReportData{}

	// Extract export download URL
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			if strings.Contains(href, "/exports/") {
				text := strings.TrimSpace(s.Text())
				if text == "Download" || strings.Contains(strings.ToLower(text), "download") {
					data.ExportURL = href
				}
			}
		}
	})
	// Fallback: schema.org ViewAction link
	if data.ExportURL == "" {
		doc.Find(`link[itemprop="url"]`).Each(func(_ int, s *goquery.Selection) {
			if href, exists := s.Attr("href"); exists && strings.Contains(href, "/exports/") {
				data.ExportURL = href
			}
		})
	}
	if data.ExportURL == "" {
		return nil, fmt.Errorf("no export download URL found in email")
	}

	// Extract period from heading like "from 01.03.2026 to 31.03.2026"
	text := doc.Text()
	dates := datePattern.FindAllStringSubmatch(text, -1)
	if len(dates) >= 2 {
		from, err := parseDate(dates[0])
		if err != nil {
			return nil, fmt.Errorf("parsing from-date: %w", err)
		}
		to, err := parseDate(dates[1])
		if err != nil {
			return nil, fmt.Errorf("parsing to-date: %w", err)
		}
		data.PeriodFrom = from
		data.PeriodTo = to
	} else {
		return nil, fmt.Errorf("could not find date range in email")
	}

	// Extract client name from "to <ClientName>" list item
	doc.Find("li").Each(func(_ int, s *goquery.Selection) {
		t := strings.TrimSpace(s.Text())
		if strings.HasPrefix(t, "to ") && !strings.HasPrefix(t, "to the project") && !strings.HasPrefix(t, "to any") {
			data.ClientName = strings.TrimPrefix(t, "to ")
		}
	})

	return data, nil
}

// ParsePDFHours extracts the total hours from a Harvest PDF report.
// Looks for the pattern "157,00 Hours" near the top of the document.
func ParsePDFHours(pdfData []byte) (float64, error) {
	reader := bytes.NewReader(pdfData)
	pdfReader, err := pdf.NewReader(reader, int64(len(pdfData)))
	if err != nil {
		return 0, fmt.Errorf("opening PDF: %w", err)
	}

	var text strings.Builder
	numPages := pdfReader.NumPage()
	// Only need to read first page for the total
	maxPages := numPages
	if maxPages > 2 {
		maxPages = 2
	}
	for i := 1; i <= maxPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		text.WriteString(content)
	}

	pdfText := text.String()

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
