package main

import (
	"testing"
	"time"

	"feed-my-accounting/email"
)

func TestLedgerKeepsReceiptsFromGoingTwice(t *testing.T) {
	cfg := &Config{dir: t.TempDir()}
	l := ledgerOf(cfg)
	invoice := email.Attachment{Filename: "11_2026_Rechnung_Amazon_17112026.pdf", Data: []byte("pdf 1"), Key: "302-1#1"}
	if _, ok, _ := l.Sent("Amazon", invoice); ok {
		t.Fatal("nothing sent yet")
	}
	if err := l.Record("Amazon", []email.Attachment{invoice}); err != nil {
		t.Fatal(err)
	}
	// The same file, by another name; the same order, downloaded anew: both sent already.
	if _, ok, _ := l.Sent("Amazon", email.Attachment{Filename: "other.pdf", Data: []byte("pdf 1")}); !ok {
		t.Error("the same file was sent")
	}
	if _, ok, _ := l.Sent("Amazon", email.Attachment{Filename: "11_2026_Rechnung_Amazon_17112026_2.pdf", Data: []byte("pdf 1b"), Key: "302-1#1"}); !ok {
		t.Error("the same order's invoice was sent")
	}
	// Another order the same day, under the name the first had: not sent.
	if _, ok, _ := l.Sent("Amazon", email.Attachment{Filename: invoice.Filename, Data: []byte("pdf 2"), Key: "302-2#1"}); ok {
		t.Error("another order's invoice is new")
	}
	// An entry from before keys goes by its name.
	_ = l.change(func(d *ledgerData) {
		d.Docs = append(d.Docs, SentDoc{Source: "Apple", Name: "11_2026_Rechnung_Apple_MX1.pdf", Month: "2026-11"})
	})
	if _, ok, _ := l.Sent("Apple", email.Attachment{Filename: "11_2026_Rechnung_Apple_MX1.pdf", Data: []byte("rendered anew"), Key: "MX1"}); !ok {
		t.Error("an entry without key goes by its name")
	}
	// Drafts have no file: never taken for one another.
	recordDraft(cfg, time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local), "")
	recordDraft(cfg, time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local), "11_2026_Rechnung_0203_WorkLog.pdf")
	d, _ := l.Data()
	months := map[string]bool{}
	for _, doc := range d.Docs {
		if doc.Source == "Kundenrechnung" {
			months[doc.Month] = true
		}
	}
	if !months["2026-10"] || !months["2026-11"] {
		t.Errorf("two drafts, October's and November's: %+v", d.Docs)
	}
	if d.Checked["Amazon"]["2026-11"].IsZero() {
		t.Error("a receipt sent is its month looked at")
	}
	// Forgotten, it may go again.
	if ok, err := l.Forget("Amazon", hashOf([]byte("pdf 1"))); !ok || err != nil {
		t.Fatalf("forget: %v %v", ok, err)
	}
	if _, ok, _ := l.Sent("Amazon", invoice); ok {
		t.Error("forgotten, it is new again")
	}
}

func TestSendPDFsLeavesOutWhatWasSent(t *testing.T) {
	cfg := &Config{dir: t.TempDir()}
	l := ledgerOf(cfg)
	old := email.Attachment{Filename: "a.pdf", Data: []byte("a")}
	_ = l.Record("Anthropic", []email.Attachment{old})
	mail := ledgerMail(cfg)
	mail.SMTPHost = "127.0.0.1"
	mail.SMTPPort = 1 // nothing listens: sending fails, but only if there is something to send
	if err := email.SendPDFs(mail, "Anthropic", []email.Attachment{old}); err != nil {
		t.Errorf("all sent before: nothing to send, no error: %v", err)
	}
	if err := email.SendPDFs(mail, "Anthropic", []email.Attachment{old, {Filename: "b.pdf", Data: []byte("b")}}); err == nil {
		t.Error("a new one is sent")
	}
	if _, ok, _ := l.Sent("Anthropic", email.Attachment{Filename: "b.pdf", Data: []byte("b")}); ok {
		t.Error("not sent, not recorded")
	}
}
