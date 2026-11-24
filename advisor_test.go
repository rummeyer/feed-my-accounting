package main

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"

	"feed-my-accounting/email"
)

func TestClosingLight(t *testing.T) {
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	early := time.Date(2026, 12, 3, 12, 0, 0, 0, time.Local)
	late := time.Date(2026, 12, 5, 9, 1, 0, 0, time.Local)
	done := closingRow{Source: "Apple", Docs: []SentDoc{{Name: "a.pdf"}}}
	open := closingRow{Source: "Vodafone", Expected: true, Open: true}
	failedBefore := closingRow{Source: "Amazon", Docs: []SentDoc{{Name: "b.pdf"}}, Failed: "login"} // sent, a later run failed: done
	cases := []struct {
		rows []closingRow
		now  time.Time
		want string
	}{
		{[]closingRow{done, failedBefore}, late, "green"},
		{[]closingRow{done, open}, early, "yellow"},
		{[]closingRow{done, open}, late, "red"},
		{[]closingRow{done, {Source: "Strato", Expected: true, Open: true, Failed: "imap", FailedAt: early}}, early, "red"},
		// Failed in the month before: not this one's.
		{[]closingRow{done, {Source: "Strato", Failed: "imap", FailedAt: nov.AddDate(0, 0, -3)}}, early, "green"},
	}
	for i, c := range cases {
		if got := closingLight(c.rows, nov, 5, c.now); got != c.want {
			t.Errorf("%d: %s, want %s", i, got, c.want)
		}
	}
}

func TestAdvisorMail(t *testing.T) {
	cfg := &Config{}
	cfg.Mail.From, cfg.Mail.CC = "me@example.com", "autobox@sevdesk.email"
	cfg.Advisor.To = "berater@example.com"
	cfg.Advisor.Text = "Hallo,\n\nalle Belege für {MM}/{YYYY} ({MONAT}) gebucht."
	m := advisorMail(cfg, time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local))
	if m.To != "berater@example.com" || m.Bcc != "me@example.com" || m.Subject != "Belege 09/2026" || m.Text != "Hallo,\n\nalle Belege für 09/2026 (September) gebucht." {
		t.Errorf("mail: %+v", m)
	}
	if _, err := informAdvisor(&Config{}, time.Now(), false); err == nil {
		t.Error("no address: not sent")
	}
}

func TestZipped(t *testing.T) {
	z, err := zipped([]email.Attachment{{Filename: "11_2026_Rechnung_A.pdf", Data: []byte("a")}, {Filename: "11_2026_Rechnung_B.pdf", Data: []byte("bb")}}, time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(z.Data), int64(len(z.Data)))
	if err != nil || z.Filename != "Belege_11_2026.zip" || len(r.File) != 2 || r.File[1].Name != "11_2026_Rechnung_B.pdf" {
		t.Errorf("zip %s: %v %v", z.Filename, err, r)
	}
}
