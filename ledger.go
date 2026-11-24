package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"feed-my-accounting/email"
)

// The ledger keeps what went to the books: every receipt mailed, and for each source and month
// when it was last looked at — found nothing is something too. It keeps a receipt from going to
// sevDesk's Autobox twice, which would book it twice, and it is what the month's closing shows.
// One file beside the configuration, read and written on each use: the command line and the
// web interface share it.

// ledgerFile is the ledger's name beside the configuration.
const ledgerFile = "feed-my-accounting-sent.json"

// SentDoc is one receipt mailed, or one invoice drafted in sevDesk.
type SentDoc struct {
	Source string    `json:"source"` // Amazon, Apple, Reisekosten, Kundenrechnung, a mail rule's name…
	Name   string    `json:"name"`
	Key    string    `json:"key,omitempty"`
	Hash   string    `json:"hash,omitempty"` // SHA-256 of the file
	Size   int       `json:"size,omitempty"`
	Month  string    `json:"month"` // 2026-11: the receipt's, as its name begins, else when it was sent
	At     time.Time `json:"at"`
}

type ledgerData struct {
	Docs []SentDoc `json:"docs"`
	// Checked is when each source last looked at a month, by source and month: 2026-11.
	Checked map[string]map[string]time.Time `json:"checked"`
}

// Ledger is the ledger in its file.
type Ledger struct {
	path string
	mu   sync.Mutex
}

// ledgers are the ledgers by file, one each: a run and the page share its lock.
var ledgers sync.Map

// ledgerOf is the ledger beside the configuration.
func ledgerOf(cfg *Config) *Ledger {
	path := filepath.Join(cfg.dir, ledgerFile)
	l, _ := ledgers.LoadOrStore(path, &Ledger{path: path})
	return l.(*Ledger)
}

func (l *Ledger) read() (ledgerData, error) {
	var d ledgerData
	raw, err := os.ReadFile(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(raw, &d)
}

func (l *Ledger) write(d ledgerData) error {
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, l.path)
}

// change reads the ledger, changes it and writes it back.
func (l *Ledger) change(f func(*ledgerData)) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	d, err := l.read()
	if err != nil {
		return err
	}
	f(&d)
	return l.write(d)
}

// Data is the ledger as it is.
func (l *Ledger) Data() (ledgerData, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.read()
}

// hashOf is a file's SHA-256; none of no file — a draft in sevDesk has none.
func hashOf(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// sentBefore is the receipt as the ledger has it, if it does: the same file, or the same receipt
// by its key. Entries from before keys, without one, go by their name.
func sentBefore(docs []SentDoc, source string, a email.Attachment) (SentDoc, bool) {
	hash := hashOf(a.Data)
	for _, d := range docs {
		if hash != "" && d.Hash == hash {
			return d, true
		}
		if d.Source != source {
			continue
		}
		if (a.Key != "" && d.Key == a.Key) || (d.Key == "" && d.Name == a.Filename) {
			return d, true
		}
	}
	return SentDoc{}, false
}

// Sent is when a receipt was mailed before, if it was.
func (l *Ledger) Sent(source string, a email.Attachment) (time.Time, bool) {
	d, err := l.Data()
	if err != nil {
		return time.Time{}, false
	}
	doc, ok := sentBefore(d.Docs, source, a)
	return doc.At, ok
}

// Record notes receipts as mailed now, and their months as looked at.
func (l *Ledger) Record(source string, files []email.Attachment) error {
	now := time.Now()
	return l.change(func(d *ledgerData) {
		for _, f := range files {
			if _, ok := sentBefore(d.Docs, source, f); ok {
				continue
			}
			doc := SentDoc{Source: source, Name: f.Filename, Key: f.Key, Hash: hashOf(f.Data), Size: len(f.Data), Month: docMonth(f.Filename, now), At: now}
			d.Docs = append(d.Docs, doc)
			markChecked(d, source, doc.Month, now)
		}
	})
}

// Checked notes that a source looked at a month now, whatever it found.
func (l *Ledger) Checked(source string, month time.Time) error {
	return l.change(func(d *ledgerData) { markChecked(d, source, month.Format("2006-01"), time.Now()) })
}

func markChecked(d *ledgerData, source, month string, at time.Time) {
	if d.Checked == nil {
		d.Checked = map[string]map[string]time.Time{}
	}
	if d.Checked[source] == nil {
		d.Checked[source] = map[string]time.Time{}
	}
	d.Checked[source][month] = at
}

// Forget takes a receipt out of the ledger, by its hash, or by name for one without: it may be
// mailed again.
func (l *Ledger) Forget(source, id string) (bool, error) {
	found := false
	err := l.change(func(d *ledgerData) {
		d.Docs = slices.DeleteFunc(d.Docs, func(doc SentDoc) bool {
			gone := doc.Source == source && (doc.Hash == id || doc.Hash == "" && doc.Name == id)
			found = found || gone
			return gone
		})
	})
	return found, err
}

// docMonth is a receipt's month as its name begins, 11_2026_…: 2026-11; else at's.
func docMonth(name string, at time.Time) string {
	if m := receiptMonth.FindStringSubmatch(filepath.Base(name)); m != nil {
		return m[2] + "-" + m[1]
	}
	return at.Format("2006-01")
}

// noteChecked notes in the ledger that a source looked at a month, saying so in the log should
// the ledger fail: the run went well all the same.
func noteChecked(cfg *Config, source string, month time.Time) {
	if cfg.dir == "" {
		return
	}
	if err := ledgerOf(cfg).Checked(source, month); err != nil {
		archiveLog.Printf("WARNING: ledger: %v", err)
	}
}

// ledgerMail is the mail configuration with the ledger: receipts not mailed twice. None without a
// configuration's folder, as in tests.
func ledgerMail(cfg *Config) email.MailConfig {
	mail := cfg.Mail
	if cfg.dir != "" {
		mail.Ledger = ledgerOf(cfg)
	}
	return mail
}

// recordDraft notes a draft invoice made in sevDesk for a month: the month's Kundenrechnung, with
// its work log's name if there is one.
func recordDraft(cfg *Config, month time.Time, name string) {
	if cfg.dir == "" {
		return
	}
	if name == "" {
		name = month.Format("01_2006") + "_Rechnung_Entwurf"
	}
	draft := email.Attachment{Filename: name, Key: "draft-" + month.Format("2006-01")}
	if err := ledgerOf(cfg).Record(jobSources["harvest-invoice"], []email.Attachment{draft}); err != nil {
		archiveLog.Printf("WARNING: ledger: %v", err)
	}
}

// sourceName is the ledger's name for a job's source: Apple for apple-invoice.
func sourceName(job string) string {
	if name, ok := jobSources[job]; ok {
		return name
	}
	return strings.TrimSuffix(job, "-invoice")
}

// jobSources are the sources by the jobs that fetch them.
var jobSources = map[string]string{
	"travel-expense":   "Reisekosten",
	"harvest-invoice":  "Kundenrechnung",
	"amazon-invoice":   "Amazon",
	"apple-invoice":    "Apple",
	"vodafone-invoice": "Vodafone",
}
