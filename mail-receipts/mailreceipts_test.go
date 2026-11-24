package mailreceipts

import (
	"testing"
	"time"

	"feed-my-accounting/email"
)

func TestPDFsOfARule(t *testing.T) {
	day := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	mail := email.Message{MessageID: "<a@strato>", Subject: "Ihre Rechnung", Date: day, Attachments: []email.Attachment{
		{Filename: "Rechnung_123.pdf", Data: []byte("r")},
		{Filename: "AGB.pdf", Data: []byte("agb")},
		{Filename: "logo.png", Data: []byte("png")},
	}}
	second := email.Message{MessageID: "<b@strato>", Subject: "Ihre Rechnung", Date: day, Attachments: []email.Attachment{{Filename: "Rechnung_124.pdf", Data: []byte("s")}}}

	// Every PDF; the same mail found in two folders once.
	all := PDFs(Rule{Name: "Strato AG"}, []email.Message{mail, mail, second})
	if len(all) != 3 {
		t.Fatalf("want 3 PDFs, got %+v", all)
	}
	if all[0].Filename != "11_2026_Rechnung_Strato-AG_03112026.pdf" || all[1].Filename != "11_2026_Rechnung_Strato-AG_03112026_2.pdf" {
		t.Errorf("names: %s, %s", all[0].Filename, all[1].Filename)
	}
	if all[0].Key != "<a@strato>/Rechnung_123.pdf" {
		t.Errorf("keyed by mail and attachment: %s", all[0].Key)
	}
	// Only those whose name says so.
	if got := PDFs(Rule{Name: "Strato", Attachment: "rechnung"}, []email.Message{mail}); len(got) != 1 || string(got[0].Data) != "r" {
		t.Errorf("only the invoice: %+v", got)
	}
}
