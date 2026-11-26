// Package mailreceipts takes the PDF receipts of further senders from the mailboxes, by rules
// set up on the web interface rather than a module each: mail from strato.de with "Rechnung" in
// its subject, say. Each rule's PDFs go out as the other modules' do, under its own name.
package mailreceipts

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"feed-my-accounting/email"
)

var logger = log.New(os.Stderr, "[mail-receipts] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// Rule finds one sender's receipts: mails from one of its domains, their subject containing its
// text, if it has one, and their PDFs — those whose name contains its attachment text, if any.
type Rule struct {
	Name       string `yaml:"name"`                 // the sender, in the file names and the mail's subject: Strato
	From       string `yaml:"from"`                 // sender domains, comma-separated
	Subject    string `yaml:"subject,omitempty"`    // the subject contains this, whatever the case
	Attachment string `yaml:"attachment,omitempty"` // the PDF's name contains this, whatever the case
	Off        bool   `yaml:"off,omitempty"`        // switched off: left out
}

// On says whether a rule is used: switched on, with a name and a sender.
func (r Rule) On() bool {
	return !r.Off && strings.TrimSpace(r.Name) != "" && strings.TrimSpace(r.From) != ""
}

// Takes says whether a file of the rule's mails is one of its PDFs: a PDF, its name containing
// the rule's attachment text, if it has one.
func (r Rule) Takes(filename string) bool {
	name := strings.ToLower(filename)
	want := strings.ToLower(strings.TrimSpace(r.Attachment))
	return strings.HasSuffix(name, ".pdf") && (want == "" || strings.Contains(name, want))
}

// Mails are the rule's mails of the month: from its domains, the subject containing its text —
// at most count in each folder, in folders' order, in every mailbox.
func Mails(mail email.MailConfig, r Rule, count int, folders []string, month time.Time) ([]email.Message, error) {
	return email.FetchEmails(mail, email.IMAPFilter{
		Count:           count,
		FromDomain:      r.From,
		SubjectContains: strings.TrimSpace(r.Subject),
		Folders:         folders,
		Month:           month,
	})
}

// Config is the rules, and where and how far to look for their mails.
type Config struct {
	Mail    email.MailConfig
	Rules   []Rule
	Count   int      // how many of the month's mails to look at, at most, in each folder
	Folders []string // where they are: the inbox, or filed away already
}

// Receipts are one rule's PDFs of a month.
type Receipts struct {
	Rule  Rule
	Files []email.Attachment
}

// Run sends the current month's receipts of every rule.
func Run(cfg Config) error {
	found, err := Collect(cfg, time.Now())
	if err != nil {
		return err
	}
	return Send(cfg, found)
}

// Collect is each rule's PDFs of the month, without sending them. A rule failing — its mailbox
// not there — fails the whole: the next run tries them all again, none sent twice.
func Collect(cfg Config, month time.Time) ([]Receipts, error) {
	var out []Receipts
	for _, rule := range cfg.Rules {
		if !rule.On() {
			continue
		}
		messages, err := Mails(cfg.Mail, rule, cfg.Count, cfg.Folders, month)
		if err != nil {
			return out, fmt.Errorf("%s: fetching mails: %w", rule.Name, err)
		}
		files := PDFs(rule, messages)
		logger.Printf("%s: %d PDF(s) in %d mail(s) of %s", rule.Name, len(files), len(messages), month.Format("01/2006"))
		out = append(out, Receipts{Rule: rule, Files: files})
	}
	return out, nil
}

// Send mails each rule's PDFs, under its name.
func Send(cfg Config, found []Receipts) error {
	for _, r := range found {
		if err := email.SendPDFs(cfg.Mail, r.Rule.Name, r.Files); err != nil {
			return fmt.Errorf("%s: %w", r.Rule.Name, err)
		}
	}
	return nil
}

// PDFs are the rule's PDFs in the mails — named as the other receipts are,
// 11_2026_Rechnung_Strato_03112026.pdf, a second of the same day _2 — each keyed by its mail and
// its own name: the same mail in two folders counts once.
func PDFs(rule Rule, messages []email.Message) []email.Attachment {
	var out []email.Attachment
	seen := map[string]bool{}
	named := map[string]int{}
	for _, msg := range messages {
		found := false
		for _, a := range msg.Attachments {
			if !rule.Takes(a.Filename) {
				continue
			}
			found = true
			key := msg.MessageID + "/" + a.Filename
			if msg.MessageID == "" {
				key = msg.Date.UTC().Format(time.RFC3339) + "/" + msg.Subject + "/" + a.Filename
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			file := FileName(rule.Name, msg.Date)
			if named[file]++; named[file] > 1 {
				file = strings.TrimSuffix(file, ".pdf") + fmt.Sprintf("_%d.pdf", named[file])
			}
			logger.Printf("%s: %q, %s → %s (%d bytes)", rule.Name, msg.Subject, a.Filename, file, len(a.Data))
			out = append(out, email.Attachment{Filename: file, Data: a.Data, Key: key})
		}
		if !found {
			logger.Printf("%s: no PDF in %q", rule.Name, msg.Subject)
		}
	}
	return out
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9ÄÖÜäöüß-]+`)

// FileName is a receipt's name by its sender and its mail's date:
// 11_2026_Rechnung_Strato_03112026.pdf.
func FileName(sender string, date time.Time) string {
	date = date.Local()
	name := strings.Trim(unsafeName.ReplaceAllString(sender, "-"), "-")
	return fmt.Sprintf("%02d_%04d_Rechnung_%s_%s.pdf", date.Month(), date.Year(), name, date.Format("02012006"))
}
