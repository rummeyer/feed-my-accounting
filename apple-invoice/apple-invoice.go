// Package apple fetches Apple invoice emails via IMAP — from the inbox and where they were filed
// away — converts their HTML to PDF
// using headless Chrome, and sends all PDFs as attachments via SMTP.
package apple

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"feed-my-accounting/email"

	"github.com/PuerkitoBio/goquery"
)

var logger = log.New(os.Stderr, "[apple-invoice] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// FilterConfig defines which Apple invoice emails to look for.
type FilterConfig struct {
	Count   int      `yaml:"count"`   // how many recent emails to scan, in each folder
	Subject string   `yaml:"subject"` // exact subject to match
	From    string   `yaml:"from"`    // sender domain filter
	Folders []string `yaml:"folders"` // where they are, in this order: the inbox first
}

type Config struct {
	Mail   email.MailConfig
	Filter FilterConfig
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

// Run fetches this month's Apple invoice emails, converts them to PDFs, and sends them via email.
func Run(cfg Config) error {
	attachments, err := Collect(cfg, time.Time{})
	if err != nil {
		return err
	}
	return Send(cfg, attachments)
}

// Send mails the PDFs, if there are any.
func Send(cfg Config, attachments []email.Attachment) error {
	if len(attachments) == 0 {
		logger.Println("No Apple PDFs to send")
		return nil
	}
	logger.Printf("Sending email with %d Apple PDF attachment(s)...", len(attachments))
	return email.Send(cfg.Mail, "Deine PDF-Rechnungen von Apple", attachments...)
}

// Collect is the month's Apple invoices as PDFs — the current month's for a zero month — without
// sending them.
func Collect(cfg Config, month time.Time) ([]email.Attachment, error) {
	if month.IsZero() {
		month = time.Now()
	}
	filter := email.IMAPFilter{
		Count:      cfg.Filter.Count,
		Subject:    cfg.Filter.Subject,
		FromDomain: cfg.Filter.From,
		Folders:    cfg.Filter.Folders,
		Month:      month,
	}

	messages, err := email.FetchHTMLEmails(cfg.Mail, filter)
	if err != nil {
		return nil, fmt.Errorf("fetching invoices: %w", err)
	}
	if len(messages) == 0 {
		logger.Printf("No Apple invoices for %s", month.Format("01/2006"))
		return nil, nil
	}

	logger.Printf("Processing %d Apple invoice(s)...", len(messages))
	var attachments []email.Attachment
	seen := map[string]bool{} // the same invoice found in two folders: the first, the inbox's
	for i, msg := range messages {
		if order := extractOrderNumber(msg.HTMLBody); order != "" {
			if seen[order] {
				continue
			}
			seen[order] = true
		}
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

	return attachments, nil
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
