package main

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"

	"feed-my-accounting/email"
)

func TestClosingState(t *testing.T) {
	sent := closingRow{Source: "Apple", Docs: []SentDoc{{Name: "a.pdf"}}}
	looked := closingRow{Source: "Strato", Checked: time.Now()}
	open := closingRow{Source: "Vodafone", Expected: true, Open: true}
	draft := closingRow{Source: "Kundenrechnung", Job: "harvest-invoice", Docs: []SentDoc{{Name: "11_2026_Rechnung_Entwurf"}}}
	noDraft := closingRow{Source: "Kundenrechnung", Job: "harvest-invoice", Expected: true, Open: true}
	cases := []struct {
		rows []closingRow
		want string
	}{
		{[]closingRow{noDraft, {Source: "Apple"}, open}, "none"},
		{[]closingRow{noDraft, sent, open}, "progress"},
		{[]closingRow{noDraft, sent, looked}, "progress"}, // all receipts in, the invoice not yet
		{[]closingRow{draft, sent, looked}, "done"},
		{[]closingRow{draft, sent, open}, "progress"},
	}
	for i, c := range cases {
		if got := closingState(c.rows); got != c.want {
			t.Errorf("%d: %s, want %s", i, got, c.want)
		}
	}
}

func TestAdvisorMail(t *testing.T) {
	cfg := &Config{}
	cfg.Mail.From, cfg.Mail.CC = "me@example.com", "autobox@sevdesk.email"
	cfg.Advisor.To, cfg.Advisor.Bcc = "berater@example.com", "me@example.com"
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
