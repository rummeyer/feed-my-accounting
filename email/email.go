// Package email provides shared SMTP sending and IMAP fetching utilities.
package email

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/go-gomail/gomail"
)

var logger = log.New(os.Stderr, "[email] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// MailConfig holds all mail-related settings: SMTP/IMAP servers, credentials,
// and envelope addresses. Mapped to the top-level "mail" YAML block.
type MailConfig struct {
	SMTPHost string `yaml:"smtpHost"`
	SMTPPort int    `yaml:"smtpPort"`
	IMAPHost string `yaml:"imapHost"`
	IMAPPort int    `yaml:"imapPort"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
	CC       string `yaml:"cc"`
	// Mailboxes are further accounts searched after this one's, for whatever mails are looked
	// for — receipts, the customers' invoices: by IMAP only, the mail goes out from this one.
	Mailboxes []Mailbox `yaml:"mailboxes,omitempty"`
	// Archive, if set, keeps the PDFs SendPDFs mailed somewhere else too — a cloud storage, say.
	// Not configuration: set by the caller.
	Archive func(vendor string, files []Attachment) error `yaml:"-" json:"-"`
	// Ledger, if set, keeps what SendPDFs mailed, and keeps it from mailing a receipt twice. Not
	// configuration: set by the caller.
	Ledger Ledger `yaml:"-" json:"-"`
}

// Ledger is the record of the receipts mailed: one mailed already — the same file, or the same
// receipt by its key — is not mailed again, sevDesk's Autobox would book it twice.
type Ledger interface {
	// Sent says whether a receipt went to the books before, and how — "sent on 12.11.2026
	// 11:37", "filed by hand as …" — or that it could not be told.
	Sent(vendor string, a Attachment) (how string, sent bool, err error)
	// Record notes receipts as mailed now.
	Record(vendor string, files []Attachment) error
}

// Mailbox is a further account to search, its server this one's unless it says otherwise.
type Mailbox struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	IMAPHost string `yaml:"imapHost,omitempty"`
	IMAPPort int    `yaml:"imapPort,omitempty"`
}

// accounts are the accounts to search: this one first, then the further mailboxes, each on this
// one's server where it names none.
func (c MailConfig) accounts() []MailConfig {
	main := c
	main.Mailboxes = nil
	all := []MailConfig{main}
	for _, m := range c.Mailboxes {
		if strings.TrimSpace(m.Username) == "" {
			continue
		}
		a := c
		a.Username, a.Password, a.Mailboxes = m.Username, m.Password, nil
		if m.IMAPHost != "" {
			a.IMAPHost = m.IMAPHost
		}
		if m.IMAPPort != 0 {
			a.IMAPPort = m.IMAPPort
		}
		all = append(all, a)
	}
	return all
}

// ---------------------------------------------------------------------------
// SMTP sending
// ---------------------------------------------------------------------------

// Attachment is an in-memory file to be attached to an email.
type Attachment struct {
	Filename string
	Data     []byte
	// Key is what the receipt is, whatever its file: an order's number, a mail's Message-ID —
	// a PDF made anew from an HTML mail differs from the last, its receipt is the same.
	Key string
}

// SendPDFs mails a module's PDFs under "Deine PDF-Rechnungen von <vendor>", if there are any —
// with a ledger, those not mailed before.
func SendPDFs(cfg MailConfig, vendor string, attachments []Attachment) error {
	if cfg.Ledger != nil {
		var err error
		if attachments, err = unsent(cfg.Ledger, vendor, attachments); err != nil {
			return fmt.Errorf("not sent, so that none goes twice: %w", err)
		}
	}
	if len(attachments) == 0 {
		logger.Printf("No %s PDFs to send", vendor)
		return nil
	}
	logger.Printf("Sending %d %s PDF(s)...", len(attachments), vendor)
	if err := Send(cfg, "Deine PDF-Rechnungen von "+vendor, attachments...); err != nil {
		return err
	}
	if cfg.Ledger != nil {
		if err := cfg.Ledger.Record(vendor, attachments); err != nil {
			logger.Printf("WARNING: sent, but not noted as sent: %v", err)
		}
	}
	// Mailed: kept in the archive too. Failing there, the mail went all the same — said so.
	if cfg.Archive != nil {
		if err := cfg.Archive(vendor, attachments); err != nil {
			return fmt.Errorf("sent by mail; rclone: %w", err)
		}
	}
	return nil
}

// Mail is one email to someone: the tax adviser told the month is booked, say.
type Mail struct {
	To, Bcc     string
	Subject     string
	Text        string // plain text
	Attachments []Attachment
}

// SendMail sends m from the account's sender, to m's recipients only: no copy to the Autobox.
func SendMail(cfg MailConfig, m Mail) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", cfg.From)
	msg.SetHeader("To", m.To)
	if m.Bcc != "" {
		msg.SetHeader("Bcc", m.Bcc)
	}
	msg.SetHeader("Subject", m.Subject)
	msg.SetBody("text/plain", m.Text)
	for _, a := range m.Attachments {
		data := a.Data
		msg.Attach(a.Filename, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		}))
	}
	logger.Printf("Sending %q to %s with %d attachment(s)", m.Subject, m.To, len(m.Attachments))
	if err := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.Username, cfg.Password).DialAndSend(msg); err != nil {
		return err
	}
	logger.Println("Email sent")
	return nil
}

// unsent are the attachments the ledger has not seen mailed; those it has, said so.
func unsent(ledger Ledger, vendor string, attachments []Attachment) ([]Attachment, error) {
	var fresh []Attachment
	for _, a := range attachments {
		how, sent, err := ledger.Sent(vendor, a)
		if err != nil {
			return nil, err
		}
		if sent {
			logger.Printf("%s: %s — not again", a.Filename, how)
			continue
		}
		fresh = append(fresh, a)
	}
	if skipped := len(attachments) - len(fresh); skipped > 0 && len(fresh) == 0 {
		logger.Printf("All %d %s PDF(s) went to the books before", skipped, vendor)
	}
	return fresh, nil
}

// Send sends an email with the given attachments via SMTP.
func Send(cfg MailConfig, subject string, attachments ...Attachment) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", cfg.From)
	msg.SetHeader("To", cfg.To)
	if cfg.CC != "" {
		msg.SetHeader("Cc", cfg.CC)
	}
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", "Dokumente anbei.<br>")

	for _, a := range attachments {
		data := a.Data
		msg.Attach(a.Filename, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		}))
	}

	logger.Printf("Sending %q to %s with %d attachment(s)", subject, cfg.To, len(attachments))
	dialer := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.Username, cfg.Password)
	if err := dialer.DialAndSend(msg); err != nil {
		return err
	}
	logger.Println("Email sent")
	return nil
}

// ---------------------------------------------------------------------------
// IMAP fetching
// ---------------------------------------------------------------------------

// IMAPFilter defines which emails to fetch from the inbox.
type IMAPFilter struct {
	// Count is how many messages to scan at the most, the latest: of the month asked for — the
	// server picks those — or, without one, of all.
	Count            int
	FromDomain       string    // sender hostname must contain this string, or one of them, comma-separated
	Subject          string    // exact subject match
	SubjectPrefix    string    // the subject starts with this, receipt numbers after it
	SubjectContains  string    // the subject has this in it, whatever the case
	CurrentMonthOnly bool      // only return emails received in the current month
	Month            time.Time // only those of this month, when set: its year and month count
	// Folders are searched one after the other, the latest Count of each: INBOX if none.
	Folders []string
}

// Message holds an email's subject, date, and HTML body — and with FetchEmails its attachments.
type Message struct {
	MessageID   string
	Subject     string
	Date        time.Time
	HTMLBody    string
	Attachments []Attachment
}

// FetchHTMLEmails connects to the IMAP server, scans the inbox for emails matching
// the filter, and returns their HTML bodies.
// Uses a two-pass approach: lightweight envelope fetch first, full body fetch only for matches.
func FetchHTMLEmails(cfg MailConfig, filter IMAPFilter) ([]Message, error) {
	return fetch(cfg, filter, false)
}

// FetchEmails is FetchHTMLEmails with the emails' attachments too, and their HTML where there
// is some: in every folder of the filter.
func FetchEmails(cfg MailConfig, filter IMAPFilter) ([]Message, error) {
	return fetch(cfg, filter, true)
}

func fetch(cfg MailConfig, filter IMAPFilter, attachments bool) ([]Message, error) {
	var all []Message
	for i, account := range cfg.accounts() {
		got, err := fetchAccount(account, filter, attachments)
		if err != nil {
			// The main account failing fails the run; a further mailbox only says so.
			if i == 0 {
				return nil, err
			}
			logger.Printf("WARNING: mailbox %s: %v", account.Username, err)
			continue
		}
		all = append(all, got...)
	}
	return all, nil
}

// fetchAccount is the matching emails of one account, in every folder of the filter.
func fetchAccount(cfg MailConfig, filter IMAPFilter, attachments bool) ([]Message, error) {
	addr := fmt.Sprintf("%s:%d", cfg.IMAPHost, cfg.IMAPPort)
	c, err := client.DialTLS(addr, &tls.Config{ServerName: cfg.IMAPHost})
	if err != nil {
		return nil, fmt.Errorf("connecting to IMAP server: %w", err)
	}
	defer c.Logout()

	if err := c.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("IMAP login: %w", err)
	}
	logger.Printf("Logged in to IMAP server as %s", cfg.Username)

	folders := filter.Folders
	if len(folders) == 0 {
		folders = []string{"INBOX"}
	}
	var all []Message
	for _, folder := range folders {
		got, err := fetchFolder(c, folder, filter, attachments)
		if err != nil {
			return nil, err
		}
		all = append(all, got...)
	}
	return all, nil
}

// fetchFolder is the matching emails of one folder; none if it is not there.
func fetchFolder(c *client.Client, folder string, filter IMAPFilter, attachments bool) ([]Message, error) {
	mbox, err := c.Select(folder, true)
	if err != nil {
		if folder == "INBOX" {
			return nil, fmt.Errorf("selecting INBOX: %w", err)
		}
		logger.Printf("No folder %q: %v", folder, err)
		return nil, nil
	}
	if mbox.Messages == 0 {
		return nil, nil
	}

	seqSet := new(imap.SeqSet)
	if month, ok := filter.month(); ok {
		// The month's mails, the server asked for them by their date — a day either side for the
		// time zones, the exact month checked by each envelope's: however full the folder, none
		// missed (17.11.2026: Anthropic's receipts behind more than fifty mails since).
		criteria := imap.NewSearchCriteria()
		criteria.SentSince = month.AddDate(0, 0, -1)
		criteria.SentBefore = month.AddDate(0, 1, 1)
		seqs, err := c.Search(criteria)
		if err != nil {
			return nil, fmt.Errorf("searching %s: %w", folder, err)
		}
		if len(seqs) == 0 {
			return nil, nil
		}
		sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
		if filter.Count > 0 && len(seqs) > filter.Count {
			logger.Printf("%s: %d mails of the month, the latest %d looked at", folder, len(seqs), filter.Count)
			seqs = seqs[len(seqs)-filter.Count:]
		}
		seqSet.AddNum(seqs...)
	} else {
		from := uint32(1)
		if filter.Count > 0 {
			count := uint32(filter.Count)
			if mbox.Messages > count {
				from = mbox.Messages - count + 1
			}
		}
		seqSet.AddRange(from, mbox.Messages)
	}

	matchUIDs := fetchMatchingUIDs(c, seqSet, filter)
	if len(matchUIDs) == 0 {
		return nil, nil
	}
	logger.Printf("Found %d matching email(s) in %s, fetching bodies...", len(matchUIDs), folder)
	return fetchBodies(c, matchUIDs, attachments)
}

// month is the first day of the month the filter asks for, if it asks for one.
func (f IMAPFilter) month() (time.Time, bool) {
	switch {
	case !f.Month.IsZero():
		return time.Date(f.Month.Year(), f.Month.Month(), 1, 0, 0, 0, 0, time.Local), true
	case f.CurrentMonthOnly:
		now := time.Now()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local), true
	}
	return time.Time{}, false
}

// fromDomain tells whether a sender's hostname contains one of the comma-separated domains.
func fromDomain(from []*imap.Address, domains string) bool {
	for _, d := range strings.Split(strings.ToLower(domains), ",") {
		if d = strings.TrimSpace(d); d == "" {
			continue
		}
		for _, addr := range from {
			if strings.Contains(strings.ToLower(addr.HostName), d) {
				return true
			}
		}
	}
	return false
}

func fetchMatchingUIDs(c *client.Client, seqSet *imap.SeqSet, filter IMAPFilter) []uint32 {
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid}
	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() { done <- c.Fetch(seqSet, items, messages) }()

	now := time.Now()
	var uids []uint32
	for msg := range messages {
		if msg.Envelope == nil {
			continue
		}
		env := msg.Envelope
		if filter.FromDomain != "" && !fromDomain(env.From, filter.FromDomain) {
			continue
		}
		if filter.CurrentMonthOnly && (env.Date.Year() != now.Year() || env.Date.Month() != now.Month()) {
			continue
		}
		if m, d := filter.Month, env.Date.Local(); !m.IsZero() && (d.Year() != m.Year() || d.Month() != m.Month()) {
			continue
		}
		if filter.Subject != "" && env.Subject != filter.Subject {
			continue
		}
		if filter.SubjectPrefix != "" && !strings.HasPrefix(env.Subject, filter.SubjectPrefix) {
			continue
		}
		if filter.SubjectContains != "" && !strings.Contains(strings.ToLower(env.Subject), strings.ToLower(filter.SubjectContains)) {
			continue
		}
		logger.Printf("Matched: %q (UID %d)", env.Subject, msg.Uid)
		uids = append(uids, msg.Uid)
	}
	if err := <-done; err != nil {
		logger.Printf("WARNING: fetching envelopes: %v", err)
	}
	return uids
}

func fetchBodies(c *client.Client, uids []uint32, attachments bool) ([]Message, error) {
	uidSet := new(imap.SeqSet)
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope}
	messages := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(uidSet, items, messages) }()

	var results []Message
	for msg := range messages {
		r := msg.GetBody(section)
		if r == nil {
			logger.Printf("WARNING: no body for UID %d", msg.Uid)
			continue
		}
		if attachments {
			m, err := readParts(r)
			if err != nil {
				logger.Printf("WARNING: reading UID %d: %v", msg.Uid, err)
				continue
			}
			m.Subject, m.Date, m.MessageID = msg.Envelope.Subject, msg.Envelope.Date, msg.Envelope.MessageId
			results = append(results, m)
			continue
		}
		htmlBody, err := extractHTMLBody(r)
		if err != nil {
			logger.Printf("WARNING: extracting HTML from UID %d: %v", msg.Uid, err)
			continue
		}
		results = append(results, Message{
			MessageID: msg.Envelope.MessageId,
			Subject:   msg.Envelope.Subject,
			Date:      msg.Envelope.Date,
			HTMLBody:  htmlBody,
		})
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("fetching bodies: %w", err)
	}
	return results, nil
}

// readParts is an email's HTML, if it has some, and its attachments.
func readParts(r io.Reader) (Message, error) {
	var m Message
	mr, err := mail.CreateReader(r)
	if err != nil {
		return m, fmt.Errorf("creating mail reader: %w", err)
	}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			return m, nil
		}
		if err != nil {
			return m, fmt.Errorf("reading mail part: %w", err)
		}
		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, params, _ := h.ContentType()
			if ct == "text/html" && m.HTMLBody == "" {
				body, err := io.ReadAll(p.Body)
				if err != nil {
					return m, fmt.Errorf("reading HTML body: %w", err)
				}
				m.HTMLBody = string(body)
				continue
			}
			// A file sent inline — a PDF with Content-Disposition: inline — is an attachment too.
			if strings.HasPrefix(ct, "text/") || strings.HasPrefix(ct, "multipart/") {
				continue
			}
			name := params["name"]
			if _, dp, err := h.ContentDisposition(); err == nil && dp["filename"] != "" {
				name = dp["filename"]
			}
			if name == "" {
				continue
			}
			data, err := io.ReadAll(p.Body)
			if err != nil {
				return m, fmt.Errorf("reading attachment %q: %w", name, err)
			}
			m.Attachments = append(m.Attachments, Attachment{Filename: name, Data: data})
		case *mail.AttachmentHeader:
			name, _ := h.Filename()
			data, err := io.ReadAll(p.Body)
			if err != nil {
				return m, fmt.Errorf("reading attachment %q: %w", name, err)
			}
			m.Attachments = append(m.Attachments, Attachment{Filename: name, Data: data})
		}
	}
}

func extractHTMLBody(r io.Reader) (string, error) {
	mr, err := mail.CreateReader(r)
	if err != nil {
		return "", fmt.Errorf("creating mail reader: %w", err)
	}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading mail part: %w", err)
		}
		if h, ok := p.Header.(*mail.InlineHeader); ok {
			if ct, _, _ := h.ContentType(); ct == "text/html" {
				body, err := io.ReadAll(p.Body)
				if err != nil {
					return "", fmt.Errorf("reading HTML body: %w", err)
				}
				return string(body), nil
			}
		}
	}
	return "", fmt.Errorf("no text/html part found")
}
