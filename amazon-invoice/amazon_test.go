package amazon

import (
	"reflect"
	"testing"
	"time"
)

func TestParseCard(t *testing.T) {
	c := card{Text: "BESTELLUNG AUFGEGEBEN\n3. November 2026\nSUMME\n42,99 €\nBESTELLNR. 302-1234567-7654321\nRechnung\nZugestellt am 5. November 2026", Popover: "/gp/x/invoice.html?orderId=302-1234567-7654321"}
	o, ok := parseCard(c)
	if !ok {
		t.Fatal("not parsed")
	}
	if o.ID != "302-1234567-7654321" || !o.Date.Equal(time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local)) || o.Popover != c.Popover {
		t.Errorf("got %+v", o)
	}
	if _, ok := parseCard(card{Text: "Kaufen Sie es noch einmal"}); ok {
		t.Error("a card with no order parsed")
	}
}

func TestPDFLinks(t *testing.T) {
	html := `<a href="/gp/css/summary/print.html?orderID=302-1">Bestellübersicht drucken</a>
<a href="/documents/download/abc-123/invoice.pdf">Rechnung 1</a>
<a href="https://www.amazon.de/gp/help/contact/contact.html?sellerID=X&amp;orderId=1">Rechnung anfordern</a>
<a href="/documents/download/abc-123/invoice.pdf">Rechnung 1</a>
<a href="/gp/shared-cs/generated_invoices/def.pdf?a=1&amp;b=2">Rechnung 2</a>`
	got := pdfLinks(html)
	want := []string{"/documents/download/abc-123/invoice.pdf", "/gp/shared-cs/generated_invoices/def.pdf?a=1&b=2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSignInPage(t *testing.T) {
	for _, c := range []struct{ url, text, field, want string }{
		{"https://www.amazon.de/your-orders/orders?timeFilter=year-2026", "", "", "in"},
		{"https://www.amazon.de/ap/signin?openid", "Anmelden E-Mail-Adresse", "email", "email"},
		{"https://www.amazon.de/ap/signin", "Anmelden Passwort", "password", "password"},
		{"https://www.amazon.de/ap/mfa?arb=1", "Zweistufige Überprüfung", "", "otp"},
		{"https://www.amazon.de/ap/signin", "Code eingeben", "otp", "otp"},
		{"https://www.amazon.de/ap/cvf/request", "Lösen Sie dieses Rätsel", "", "captcha"},
		{"https://www.amazon.de/ap/signin", "Es ist ein Problem aufgetreten. Ihr Passwort ist falsch", "password", "refused"},
		{"https://www.amazon.de/ap/signin", "Etwas Neues", "", ""},
		{"https://www.amazon.de/ax/claim/validate?arb=1", "Passwort", "", ""},
		{"https://www.amazon.de/ax/claim/validate?arb=1", "Passwort", "password", "password"},
		{"https://www.amazon.de/your-orders/orders", "Anmelden", "email", "email"},
	} {
		if got := signInPage(c.url, c.text, c.field); got != c.want {
			t.Errorf("signInPage(%q, %q, %q) = %q, want %q", c.url, c.text, c.field, got, c.want)
		}
	}
}

func TestFileName(t *testing.T) {
	used := map[string]bool{}
	o := order{ID: "302-1234567-7654321", Date: time.Date(2026, 11, 17, 0, 0, 0, 0, time.Local)}
	for _, want := range []string{"11_2026_Rechnung_Amazon_17112026.pdf", "11_2026_Rechnung_Amazon_17112026_2.pdf", "11_2026_Rechnung_Amazon_17112026_3.pdf"} {
		if got := fileName(o, used); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	o.Date = time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local)
	if got := fileName(o, used); got != "11_2026_Rechnung_Amazon_03112026.pdf" {
		t.Errorf("got %q", got)
	}
	for _, c := range []struct {
		now  time.Time
		want string
	}{
		{time.Date(2027, 1, 5, 7, 0, 0, 0, time.Local), "12/2026"},    // mid-month: the one before
		{time.Date(2026, 11, 30, 18, 0, 0, 0, time.Local), "11/2026"}, // its last day: its own
		{time.Date(2027, 2, 28, 9, 0, 0, 0, time.Local), "02/2027"},   // February's last day
		{time.Date(2028, 2, 28, 9, 0, 0, 0, time.Local), "01/2028"},   // not in a leap year's
		{time.Date(2026, 12, 1, 0, 30, 0, 0, time.Local), "11/2026"},  // the morning after
	} {
		if got := FetchedMonth(c.now).Format("01/2006"); got != c.want {
			t.Errorf("on %s: %s, want %s", c.now.Format("02.01.2006"), got, c.want)
		}
	}
}

func TestMonthBounds(t *testing.T) {
	first, last := monthBounds(time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local))
	if first.Day() != 1 || first.Month() != 2 || last.Day() != 28 || last.Month() != 2 {
		t.Errorf("got %v – %v", first, last)
	}
}

func TestHandedOn(t *testing.T) {
	for _, c := range []struct {
		url, field string
		want       bool
	}{
		{"https://www.amazon.de/your-orders/orders", "", true},
		{"https://www.amazon.de/ap/accountfixup?x=1", "", true}, // a notice after the code
		{"https://www.amazon.de/ap/mfa?x=1", "", false},
		{"https://www.amazon.de/ap/signin", "", false},
		{"https://www.amazon.de/ax/claim/validate", "", false},
		{"https://www.amazon.de/ap/accountfixup", "otp", false},
		{"chrome-error://chromewebdata/", "", false},
		{"about:blank", "", false},
	} {
		if got := handedOn(c.url, c.field); got != c.want {
			t.Errorf("handedOn(%q, %q) = %v", c.url, c.field, got)
		}
	}
}

func TestCancelled(t *testing.T) {
	open := card{Text: "Bestellung aufgegeben 18. Oktober 2026 Bestellnr. 304-0274617-9004348 Zugestellt Artikel stornieren"}
	gone := card{Text: "Bestellung aufgegeben 18. Oktober 2026 Bestellnr. 304-1298290-9009108 Storniert"}
	if o, _ := parseCard(open); o.Cancelled {
		t.Error("an open order with its cancel button taken for cancelled")
	}
	if o, _ := parseCard(gone); !o.Cancelled {
		t.Error("a cancelled order not taken for one")
	}
}
