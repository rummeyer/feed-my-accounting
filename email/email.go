// Package email provides shared SMTP sending and IMAP fetching utilities.
package email

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
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
}

// ---------------------------------------------------------------------------
// SMTP sending
// ---------------------------------------------------------------------------

// Attachment is an in-memory file to be attached to an email.
type Attachment struct {
	Filename string
	Data     []byte
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
	Count            int    // how many recent messages to scan (0 = all)
	FromDomain       string // sender hostname must contain this string
	Subject          string // exact subject match
	SubjectPrefix    string // the subject starts with this, receipt numbers after it
	CurrentMonthOnly bool   // only return emails received in the current month
	// Folders are searched one after the other, the latest Count of each: INBOX if none.
	Folders []string
}

// Message holds an email's subject, date, and HTML body — and with FetchEmails its attachments.
type Message struct {
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
	addr := fmt.Sprintf("%s:%d", cfg.IMAPHost, cfg.IMAPPort)
	c, err := client.DialTLS(addr, &tls.Config{ServerName: cfg.IMAPHost})
	if err != nil {
		return nil, fmt.Errorf("connecting to IMAP server: %w", err)
	}
	defer c.Logout()

	if err := c.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("IMAP login: %w", err)
	}
	logger.Println("Logged in to IMAP server")

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

	from := uint32(1)
	if filter.Count > 0 {
		count := uint32(filter.Count)
		if mbox.Messages > count {
			from = mbox.Messages - count + 1
		}
	}
	seqSet := new(imap.SeqSet)
	seqSet.AddRange(from, mbox.Messages)

	matchUIDs := fetchMatchingUIDs(c, seqSet, filter)
	if len(matchUIDs) == 0 {
		return nil, nil
	}
	logger.Printf("Found %d matching email(s) in %s, fetching bodies...", len(matchUIDs), folder)
	return fetchBodies(c, matchUIDs, attachments)
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
		if filter.FromDomain != "" {
			matched := false
			for _, addr := range env.From {
				if strings.Contains(strings.ToLower(addr.HostName), strings.ToLower(filter.FromDomain)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if filter.CurrentMonthOnly && (env.Date.Year() != now.Year() || env.Date.Month() != now.Month()) {
			continue
		}
		if filter.Subject != "" && env.Subject != filter.Subject {
			continue
		}
		if filter.SubjectPrefix != "" && !strings.HasPrefix(env.Subject, filter.SubjectPrefix) {
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
			m.Subject, m.Date = msg.Envelope.Subject, msg.Envelope.Date
			results = append(results, m)
			continue
		}
		htmlBody, err := extractHTMLBody(r)
		if err != nil {
			logger.Printf("WARNING: extracting HTML from UID %d: %v", msg.Uid, err)
			continue
		}
		results = append(results, Message{
			Subject:  msg.Envelope.Subject,
			Date:     msg.Envelope.Date,
			HTMLBody: htmlBody,
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
			if ct, _, _ := h.ContentType(); ct == "text/html" && m.HTMLBody == "" {
				body, err := io.ReadAll(p.Body)
				if err != nil {
					return m, fmt.Errorf("reading HTML body: %w", err)
				}
				m.HTMLBody = string(body)
			}
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
