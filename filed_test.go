package main

import (
	"testing"

	"feed-my-accounting/email"
)

func TestFiledByHandIsNotSent(t *testing.T) {
	cfg := &Config{dir: t.TempDir()}
	l := ledgerOf(cfg)
	f := &filedLedger{Ledger: l, months: map[string][]email.Attachment{"2026-11": {
		{Filename: "11_2026_Rechnung_Amazon_22112026.pdf", Data: []byte("amazon pdf")},
		{Filename: "11_2026_Rechnung_Apple_MX1 (1).pdf", Data: []byte("apple, as Mail printed it")},
		{Filename: "11_2026_Rechnung_Vodafone_Kabel.pdf", Data: []byte("vodafone")},
	}}}
	sent := func(source, name, data string) bool {
		_, ok, err := f.Sent(source, email.Attachment{Filename: name, Data: []byte(data)})
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	// The same file, under any name: filed.
	if !sent("Amazon", "11_2026_Rechnung_Amazon_22112026_2.pdf", "amazon pdf") {
		t.Error("Amazon's invoice, downloaded by hand: filed")
	}
	// Amazon by name only: another order of the day.
	if sent("Amazon", "11_2026_Rechnung_Amazon_22112026.pdf", "another order") {
		t.Error("Amazon by name is not enough")
	}
	// Apple and Vodafone by name, their files made anew.
	if !sent("Apple", "11_2026_Rechnung_Apple_MX1.pdf", "apple, rendered here") || !sent("Vodafone", "11_2026_Rechnung_Vodafone_Kabel.pdf", "downloaded anew") {
		t.Error("Apple and Vodafone go by name")
	}
	// Noted in the ledger as filed: the closing shows it.
	d, _ := l.Data()
	if len(d.Docs) != 3 || d.Docs[0].Filed != "11_2026_Rechnung_Amazon_22112026.pdf" {
		t.Errorf("noted as filed: %+v", d.Docs)
	}
	// Taken out to go again: past the storage.
	if _, err := l.Forget("Amazon", d.Docs[0].Hash); err != nil {
		t.Fatal(err)
	}
	if sent("Amazon", "11_2026_Rechnung_Amazon_22112026_2.pdf", "amazon pdf") {
		t.Error("taken out, it goes again, filed or not")
	}
	// Sent again: no longer waiting to.
	_ = l.Record("Amazon", []email.Attachment{{Filename: "11_2026_Rechnung_Amazon_22112026_2.pdf", Data: []byte("amazon pdf")}})
	if d, _ := l.Data(); len(d.Again) != 0 {
		t.Errorf("sent, out of Again: %+v", d.Again)
	}
}

func TestSameName(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"11_2026_Rechnung_Apple_MX1.pdf", "11_2026_Rechnung_Apple_MX1.pdf", true},
		{"11_2026_Rechnung_Apple_MX1 (1).pdf", "11_2026_Rechnung_Apple_MX1.pdf", true},
		{"11_2026_Rechnung_Amazon_17112026-2.pdf", "11_2026_Rechnung_Amazon_17112026_2.pdf", true},
		{"11_2026_Rechnung_Apple_MX2.pdf", "11_2026_Rechnung_Apple_MX1.pdf", false},
	} {
		if got := sameName(c.a, c.b); got != c.same {
			t.Errorf("%s ~ %s: %v", c.a, c.b, got)
		}
	}
}
