package anthropic

import (
	"testing"
	"time"

	"feed-my-accounting/email"
)

func TestReceipts(t *testing.T) {
	day := time.Date(2026, 11, 12, 9, 30, 0, 0, time.Local)
	mail := func(receipt string, at time.Time) email.Message {
		return email.Message{Subject: "Your receipt from Anthropic, PBC #" + receipt, Date: at, Attachments: []email.Attachment{
			{Filename: "Invoice-S8RLHB0Y-0003.pdf", Data: []byte("invoice")},
			{Filename: "Receipt-" + receipt + ".pdf", Data: []byte("receipt " + receipt)},
		}}
	}
	got := Receipts([]email.Message{
		mail("2999-5691-3003", day),
		mail("2185-6196-3156", day.Add(time.Hour)), // a second the same day
		mail("2999-5691-3003", day),                // the first again, from another folder
		{Subject: "Your receipt from Anthropic, PBC #0000", Date: day, Attachments: []email.Attachment{{Filename: "Invoice-1.pdf"}}},
	})
	want := []struct{ name, data string }{
		{"11_2026_Rechnung_Anthropic_12112026.pdf", "receipt 2999-5691-3003"},
		{"11_2026_Rechnung_Anthropic_12112026_2.pdf", "receipt 2185-6196-3156"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d receipts, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Filename != w.name || string(got[i].Data) != w.data {
			t.Errorf("%d: %s %q, want %s %q", i, got[i].Filename, got[i].Data, w.name, w.data)
		}
	}
}
