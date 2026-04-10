// Package apple fetches Apple invoice emails via IMAP, converts their HTML to PDF
// using headless Chrome, and sends all PDFs as attachments via SMTP.
package apple

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"feed-my-accounting/email"

	"github.com/PuerkitoBio/goquery"
)

var logger = log.New(os.Stderr, "[apple] ", log.LstdFlags)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// FilterConfig defines which Apple invoice emails to look for.
type FilterConfig struct {
	Count   int    `yaml:"count"`   // how many recent emails to scan
	Subject string `yaml:"subject"` // exact subject to match
	From    string `yaml:"from"`    // sender domain filter
}

type Config struct {
	Mail   email.MailConfig
	Filter FilterConfig
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

// Run fetches Apple invoice emails, converts them to PDFs, and sends them via email.
func Run(cfg Config) error {
	filter := email.IMAPFilter{
		Count:            cfg.Filter.Count,
		Subject:          cfg.Filter.Subject,
		FromDomain:       cfg.Filter.From,
		CurrentMonthOnly: true,
	}

	messages, err := email.FetchHTMLEmails(cfg.Mail, filter)
	if err != nil {
		return fmt.Errorf("fetching invoices: %w", err)
	}
	if len(messages) == 0 {
		logger.Println("No Apple invoices to process")
		return nil
	}

	logger.Printf("Processing %d Apple invoice(s)...", len(messages))
	var attachments []email.Attachment
	for i, msg := range messages {
		logger.Printf("[%d/%d] Converting %q to PDF...", i+1, len(messages), msg.Subject)

		cleaned, err := cleanHTML(msg.HTMLBody)
		if err != nil {
			logger.Printf("ERROR cleaning HTML: %v", err)
			continue
		}
		pdf, err := convertHTMLToPDF(cleaned)
		if err != nil {
			logger.Printf("ERROR converting to PDF: %v", err)
			continue
		}
		logger.Printf("[%d/%d] PDF generated (%d bytes)", i+1, len(messages), len(pdf))

		orderNum := extractOrderNumber(msg.HTMLBody)
		var filename string
		if orderNum != "" {
			filename = fmt.Sprintf("%02d_%04d_Rechnung_Apple_%s",
				msg.Date.Month(), msg.Date.Year(), sanitizeFilename(orderNum))
		} else {
			filename = sanitizeFilename(msg.Subject)
			if len(messages) > 1 {
				filename = fmt.Sprintf("%s_%d", filename, i+1)
			}
		}
		attachments = append(attachments, email.Attachment{Filename: filename + ".pdf", Data: pdf})
	}

	if len(attachments) == 0 {
		logger.Println("No Apple PDFs generated")
		return nil
	}

	logger.Printf("Sending email with %d Apple PDF attachment(s)...", len(attachments))
	return email.Send(cfg.Mail, "Deine PDF-Rechnungen von Apple", attachments...)
}

// ---------------------------------------------------------------------------
// Invoice parsing
// ---------------------------------------------------------------------------

// extractOrderNumber parses the invoice HTML for the order number following
// the "Bestellnummer:" label. Returns empty string if not found.
func extractOrderNumber(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}
	var orderNum string
	doc.Find("*").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		if strings.HasPrefix(text, "Bestellnummer:") {
			orderNum = strings.TrimSpace(strings.TrimPrefix(text, "Bestellnummer:"))
			if idx := strings.IndexAny(orderNum, "\n\r\t"); idx >= 0 {
				orderNum = strings.TrimSpace(orderNum[:idx])
			}
			return false
		}
		return true
	})
	return orderNum
}

func sanitizeFilename(s string) string {
	s = regexp.MustCompile(`[^a-zA-Z0-9äöüÄÖÜß\-_ ]+`).ReplaceAllString(s, "_")
	if s = strings.TrimSpace(s); s == "" {
		s = "invoice"
	}
	return s
}
