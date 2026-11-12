// Package anthropic fetches Anthropic's receipt emails via IMAP — from Rechnungen as well as
// the inbox — takes the receipt PDF each brings, and sends them all as attachments via SMTP.
package anthropic

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"feed-my-accounting/email"
)

var logger = log.New(os.Stderr, "[anthropic-invoice] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// FilterConfig defines which Anthropic receipt emails to look for.
type FilterConfig struct {
	Count   int      `yaml:"count"`   // how many recent emails to scan, in each folder
	Subject string   `yaml:"subject"` // the subject's start: the receipt's number follows
	From    string   `yaml:"from"`    // sender domain filter
	Folders []string `yaml:"folders"` // where they are: the inbox, or filed away already
}

type Config struct {
	Mail   email.MailConfig
	Filter FilterConfig
}

// Run fetches this month's Anthropic receipts and sends their PDFs via email.
func Run(cfg Config) error {
	messages, err := email.FetchEmails(cfg.Mail, email.IMAPFilter{
		Count:            cfg.Filter.Count,
		SubjectPrefix:    cfg.Filter.Subject,
		FromDomain:       cfg.Filter.From,
		Folders:          cfg.Filter.Folders,
		CurrentMonthOnly: true,
	})
	if err != nil {
		return fmt.Errorf("fetching receipts: %w", err)
	}
	attachments := Receipts(messages)
	if len(attachments) == 0 {
		logger.Println("No Anthropic receipts to send")
		return nil
	}
	logger.Printf("Sending email with %d Anthropic PDF attachment(s)...", len(attachments))
	return email.Send(cfg.Mail, "Deine PDF-Rechnungen von Anthropic", attachments...)
}

// Receipts are the receipt PDFs of the emails — Receipt-2999-5691-3003.pdf beside the invoice —
// named as the other receipts are: 11_2026_Rechnung_Anthropic_12112026.pdf, by the email's date;
// a second of the same day after it, _2. The same email found twice, in two folders, counts once.
func Receipts(messages []email.Message) []email.Attachment {
	var out []email.Attachment
	seen := map[string]bool{}
	named := map[string]int{}
	for _, msg := range messages {
		receipt := receiptOf(msg)
		if receipt == nil {
			logger.Printf("No receipt PDF in %q", msg.Subject)
			continue
		}
		if seen[receipt.Filename] {
			continue
		}
		seen[receipt.Filename] = true
		name := FileName(msg.Date)
		if named[name]++; named[name] > 1 {
			name = strings.TrimSuffix(name, ".pdf") + fmt.Sprintf("_%d.pdf", named[name])
		}
		logger.Printf("%s → %s (%d bytes)", receipt.Filename, name, len(receipt.Data))
		out = append(out, email.Attachment{Filename: name, Data: receipt.Data})
	}
	return out
}

// receiptOf is the email's receipt PDF: the attachment named Receipt-…, not the invoice.
func receiptOf(msg email.Message) *email.Attachment {
	for i, a := range msg.Attachments {
		name := strings.ToLower(a.Filename)
		if strings.HasPrefix(name, "receipt") && strings.HasSuffix(name, ".pdf") {
			return &msg.Attachments[i]
		}
	}
	return nil
}

// FileName is a receipt's name by its email's date: 11_2026_Rechnung_Anthropic_12112026.pdf.
func FileName(date time.Time) string {
	date = date.Local()
	return fmt.Sprintf("%02d_%04d_Rechnung_Anthropic_%s.pdf", date.Month(), date.Year(), date.Format("02012006"))
}
