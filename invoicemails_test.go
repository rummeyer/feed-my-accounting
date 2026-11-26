package main

import (
	"net/http/httptest"
	"testing"
	"time"

	"feed-my-accounting/email"
	mailreceipts "feed-my-accounting/mail-receipts"
)

func TestInvoicesIn(t *testing.T) {
	rule := mailreceipts.Rule{Name: "Acme GmbH", From: "sevdesk.de", Subject: "Kopie von Rechnung"}
	sent := time.Date(2026, 11, 30, 11, 25, 0, 0, time.Local)
	copy := email.Message{
		MessageID: "a",
		Subject:   "Kopie von Rechnung Nr. 0203 von Beispiel GmbH zur Bestellung Nr. 4500012345",
		Date:      sent,
		Attachments: []email.Attachment{
			{Filename: "0203.pdf", Data: []byte("%PDF invoice")},
			{Filename: "11_2026_Rechnung_0203_WorkLog.pdf", Data: []byte("%PDF log")},
		},
	}
	// The month is the work log's: sent on 1 December for November, the name says November.
	early := copy
	early.Date = time.Date(2026, 12, 1, 9, 0, 0, 0, time.Local)
	// No work log, the PDF named otherwise: the mail's month, the subject's number.
	other := email.Message{MessageID: "b", Subject: "Kopie von Rechnung Nr. RE-0204", Date: sent,
		Attachments: []email.Attachment{{Filename: "Rechnung.pdf", Data: []byte("%PDF c")}}}

	got := invoicesIn(rule, []email.Message{copy, early, other})
	want := []struct {
		name    string
		workLog bool
	}{
		{"11_2026_Rechnung_0203.pdf", false},
		{"11_2026_Rechnung_0203_WorkLog.pdf", true},
		{"11_2026_Rechnung_RE-0204.pdf", false},
	}
	if len(got) != len(want) {
		t.Fatalf("%d invoices, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].File.Filename != w.name || got[i].WorkLog != w.workLog || got[i].Customer != rule.Name {
			t.Errorf("%d: %s (work log %v, %s), want %s", i, got[i].File.Filename, got[i].WorkLog, got[i].Customer, w.name)
		}
	}
	// The PDF's name filter leaves the rest out; the work log is no invoice, whatever it says.
	rule.Attachment = "nothing"
	if got := invoicesIn(rule, []email.Message{copy}); len(got) != 1 || !got[0].WorkLog {
		t.Errorf("with the filter: %+v", got)
	}
}

func TestInvoiceNumber(t *testing.T) {
	for _, c := range []struct{ file, subject, want string }{
		{"0203.pdf", "", "0203"},
		{"RE-0203.pdf", "", "0203"},
		{"Rechnung.pdf", "Kopie von Rechnung Nr. 0205 von …", "0205"},
		{"Rechnung.pdf", "Ihre Rechnung", ""},
	} {
		if got := invoiceNumber(c.file, c.subject); got != c.want {
			t.Errorf("invoiceNumber(%q, %q) = %q, want %q", c.file, c.subject, got, c.want)
		}
	}
}

func TestStoreInvoiceToPaperless(t *testing.T) {
	fake := &fakePaperless{tags: map[string]int{"Firma": 1, "Rechnungen": 2}, corrs: map[string]int{}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	dir := t.TempDir()
	cfg := &Config{dir: dir, Paperless: PaperlessConfig{Enabled: true, URL: srv.URL, Token: "secret", Tags: []string{"Firma"}},
		InvoiceMails: InvoiceMailsConfig{Tags: []string{"Rechnungen"}}}
	day := time.Date(2026, 11, 30, 11, 25, 0, 0, time.Local)
	invoice := customerInvoice{Customer: "Acme GmbH", Number: "0203", Date: day,
		File: email.Attachment{Filename: "11_2026_Rechnung_0203.pdf", Data: []byte("%PDF invoice")}}
	workLog := customerInvoice{Customer: "Acme GmbH", Number: "0203", Date: day, WorkLog: true,
		File: email.Attachment{Filename: "11_2026_Rechnung_0203_WorkLog.pdf", Data: []byte("%PDF log")}}
	for _, inv := range []customerInvoice{invoice, workLog} {
		if err := storeInvoice(cfg, inv); err != nil { // the storage off: nothing asked of rclone
			t.Fatal(err)
		}
	}
	flushPaperless(cfg)
	if len(fake.documents) != 1 {
		t.Fatalf("%d uploaded, want the invoice only", len(fake.documents))
	}
	d := fake.documents[0]
	if d["title"][0] != "Rechnung 0203 Acme GmbH" || d["created"][0] != "2026-11-30" ||
		len(d["tags"]) != 1 || d["tags"][0] != "2" || d["correspondent"] == nil {
		t.Errorf("uploaded: %v", d)
	}
	// Taken once: the next run's the same file is not queued again.
	if err := storeInvoice(cfg, invoice); err != nil {
		t.Fatal(err)
	}
	if n := waiting(t, dir); n != 0 {
		t.Fatalf("%d waiting again", n)
	}
}
