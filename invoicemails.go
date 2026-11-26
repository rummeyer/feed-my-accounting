package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"feed-my-accounting/email"
	mailreceipts "feed-my-accounting/mail-receipts"
)

// The customers' invoices come back from the mailboxes: sevDesk sends a copy of each one sent,
// "Kopie von Rechnung Nr. 0203 …", with the invoice, 0203.pdf, and the work log attached to the
// draft. Found by rules as the further senders' receipts are, a rule each — its name the customer
// — the invoice is stored in the storage's folder for invoices beside its work log, named as it
// is, 11_2026_Rechnung_0203.pdf, and given to Paperless with the customer as correspondent. The
// work log too goes to the folder, should it not be there yet. Nothing is mailed: the books have
// them from sevDesk.

// invoiceLog is the invoices' log, "[rechnungen] " first, as the modules' are.
var invoiceLog = log.New(os.Stderr, "[rechnungen] ", log.LstdFlags)

// workLogName is a work log named after its invoice: its month, and its number.
var workLogName = regexp.MustCompile(`(?i)^(\d{2}_\d{4})_Rechnung_([A-Za-z0-9-]+)_WorkLog\.pdf$`)

// subjectNumber is an invoice's number as the copy's subject says it: "Rechnung Nr. 0203".
var subjectNumber = regexp.MustCompile(`(?i)\bNr\.?\s*([A-Za-z0-9-]*\d[A-Za-z0-9-]*)`)

// lastNumber is the last run of digits in a name: 0203 of "RE-0203.pdf".
var lastNumber = regexp.MustCompile(`\d+`)

// customerInvoice is an invoice of a mail, or its work log, as it is stored.
type customerInvoice struct {
	Customer string
	Number   string
	Date     time.Time // the mail's: the day it was sent
	WorkLog  bool
	File     email.Attachment
}

// invoicesIn are the invoices of the mails a rule found, with their work logs, named after
// their numbers. The number is the PDF's — 0203.pdf — or else the subject's; one with neither
// is left out, said so. The month is the work log's, as the run made it, or else the mail's.
func invoicesIn(rule mailreceipts.Rule, messages []email.Message) []customerInvoice {
	var out []customerInvoice
	seen := map[string]bool{}
	want := strings.ToLower(strings.TrimSpace(rule.Attachment))
	for _, msg := range messages {
		months := map[string]string{} // work logs' months by number
		for _, a := range msg.Attachments {
			if m := workLogName.FindStringSubmatch(a.Filename); m != nil {
				months[m[2]] = m[1]
			}
		}
		found := false
		for _, a := range msg.Attachments {
			name := strings.ToLower(a.Filename)
			if !strings.HasSuffix(name, ".pdf") {
				continue
			}
			inv := customerInvoice{Customer: rule.Name, Date: msg.Date}
			if m := workLogName.FindStringSubmatch(a.Filename); m != nil {
				inv.Number, inv.WorkLog = m[2], true
				inv.File = email.Attachment{Filename: fmt.Sprintf("%s_Rechnung_%s_WorkLog.pdf", m[1], m[2]), Data: a.Data}
			} else {
				if strings.Contains(name, "worklog") || (want != "" && !strings.Contains(name, want)) {
					continue
				}
				inv.Number = invoiceNumber(a.Filename, msg.Subject)
				if inv.Number == "" {
					invoiceLog.Printf("%s: no invoice number in %s or %q: left out", rule.Name, a.Filename, msg.Subject)
					continue
				}
				month := months[inv.Number]
				if month == "" {
					month = msg.Date.Local().Format("01_2006")
				}
				inv.File = email.Attachment{Filename: fmt.Sprintf("%s_Rechnung_%s.pdf", month, inv.Number), Data: a.Data}
			}
			found = true
			// The same mail in two folders, or two mailboxes: once.
			if seen[inv.File.Filename] {
				continue
			}
			seen[inv.File.Filename] = true
			invoiceLog.Printf("%s: %q, %s → %s", rule.Name, msg.Subject, a.Filename, inv.File.Filename)
			out = append(out, inv)
		}
		if !found {
			invoiceLog.Printf("%s: no invoice PDF in %q", rule.Name, msg.Subject)
		}
	}
	return out
}

// invoiceNumber is an invoice's number: its PDF's name's — 0203.pdf, RE-0203.pdf — if that
// has one, else the subject's, "Rechnung Nr. 0203".
func invoiceNumber(file, subject string) string {
	if all := lastNumber.FindAllString(strings.TrimSuffix(file, ".pdf"), -1); len(all) > 0 {
		return all[len(all)-1]
	}
	if m := subjectNumber.FindStringSubmatch(subject); m != nil {
		return m[1]
	}
	return ""
}

// invoiceMails collects the month's invoices by the rules, and stores them if asked: in the
// storage's folder for invoices, and to Paperless. found, if given, gets them all.
func invoiceMails(cfg *Config, month time.Time, store bool, found *[]email.Attachment) error {
	im := cfg.InvoiceMails
	mail := ledgerMail(cfg)
	var errs []error
	for _, rule := range im.Rules {
		if rule.Off || strings.TrimSpace(rule.Name) == "" || strings.TrimSpace(rule.From) == "" {
			continue
		}
		messages, err := email.FetchEmails(mail, email.IMAPFilter{
			Count:           im.Count,
			FromDomain:      rule.From,
			SubjectContains: strings.TrimSpace(rule.Subject),
			Folders:         im.Folders,
			Month:           month,
		})
		if err != nil {
			return fmt.Errorf("%s: fetching mails: %w", rule.Name, err)
		}
		invoices := invoicesIn(rule, messages)
		invoiceLog.Printf("%s: %d PDF(s) in %d mail(s) of %s", rule.Name, len(invoices), len(messages), month.Format("01/2006"))
		for _, inv := range invoices {
			if found != nil {
				*found = append(*found, inv.File)
			}
			if store {
				if err := storeInvoice(cfg, inv); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", inv.File.Filename, err))
				}
			}
		}
	}
	if store && cfg.Paperless.Enabled {
		flushPaperless(cfg)
	}
	return errors.Join(errs...)
}

// storeInvoice puts an invoice in the storage's folder for invoices, and in Paperless's queue —
// the work log only in the folder.
func storeInvoice(cfg *Config, inv customerInvoice) error {
	if cfg.Paperless.Enabled && cfg.dir != "" && !inv.WorkLog {
		w := paperlessWaiting{
			Name:    inv.File.Filename,
			Source:  inv.Customer,
			Title:   "Rechnung " + inv.Number + " " + inv.Customer,
			Created: inv.Date.Local().Format("2006-01-02"),
			Tags:    cfg.InvoiceMails.Tags, // none: Paperless's own
		}
		if len(w.Tags) == 0 {
			w.Tags = nil
		}
		if err := queueOneForPaperless(cfg.dir, w, inv.File.Data); err != nil {
			paperlessLog.Printf("WARNING: not queued for Paperless: %v", err)
		}
	}
	return archiveInvoice(cfg.Archive, inv.File.Filename, inv.File.Data)
}

// runInvoiceMails stores the current month's invoices, and the month before's: one sent on the
// first is seen still.
func runInvoiceMails(cfg *Config) error {
	now := time.Now()
	this := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	return errors.Join(invoiceMails(cfg, this.AddDate(0, -1, 0), true, nil), invoiceMails(cfg, this, true, nil))
}
