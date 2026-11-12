package harvest

import (
	"testing"
	"time"
)

func TestParseDatum(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
	}{
		// DD.MM.YYYY (4-digit year)
		{"01.04.2026", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
		{"30.04.2026", time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
		{"1.1.2026", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		// DD.MM.YY (2-digit year, as shown in sevDesk invoice list)
		{"10.04.26", time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)},
		{"26.03.26", time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)},
		{"05.12.25", time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC)},
		{"31.10.25", time.Date(2025, 10, 31, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got := parseDatum(tt.input)
		if !got.Equal(tt.want) {
			t.Errorf("parseDatum(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseDatum_Invalid(t *testing.T) {
	for _, input := range []string{"", "not-a-date", "2026-04-01", "abc"} {
		got := parseDatum(input)
		if !got.IsZero() {
			t.Errorf("parseDatum(%q) = %v, want zero", input, got)
		}
	}
}

func TestTruncateToDay(t *testing.T) {
	tests := []struct {
		input time.Time
		want  time.Time
	}{
		{
			time.Date(2026, 4, 15, 13, 45, 30, 123, time.UTC),
			time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC),
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		got := truncateToDay(tt.input)
		if !got.Equal(tt.want) {
			t.Errorf("truncateToDay(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDuplicateMatchLogic(t *testing.T) {
	// Simulate the matching logic used in checkInvoiceExists:
	// an invoice row matches if its Datum is within fromDay–toDay.
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	fromDay := truncateToDay(from)
	toDay := truncateToDay(to)

	tests := []struct {
		name      string
		datum     string
		wantMatch bool
	}{
		{"within period", "10.04.26", true},
		{"first day", "01.04.26", true},
		{"last day", "30.04.26", true},
		{"before period", "31.03.26", false},
		{"after period", "01.05.26", false},
		{"different year", "10.04.25", false},
		{"invalid date", "abc", false},
	}
	for _, tt := range tests {
		datum := parseDatum(tt.datum)
		match := !datum.IsZero() && !datum.Before(fromDay) && !datum.After(toDay)
		if match != tt.wantMatch {
			t.Errorf("%s: datum=%q match=%v, want %v", tt.name, tt.datum, match, tt.wantMatch)
		}
	}
}

func TestInvoiceNumber(t *testing.T) {
	for _, c := range []struct {
		cells []string
		want  string
	}{
		// As sevDesk's list showed a new draft on 12.11.2026: the number beneath the client.
		{[]string{"Entwurf", "-", "-", "Acme GmbH\nRechnung Nr. 0203", "12.11.26", "10.275,00\u00a0€", "12.227,25\u00a0€", ""}, "0203"},
		{[]string{"Offen", "-", "-", "Acme GmbH\nRechnungs-Nr.: RE-0204", "12.11.26"}, "0204"},
		{[]string{"Entwurf", "0203", "Acme GmbH", "", "12.11.26", "10.275,00 €"}, "0203"},
		{[]string{"Offen", "RE-0202", "Acme GmbH", "", "03.11.26"}, "0202"},
		{[]string{"Bezahlt", "RE 1001", "Acme GmbH", "", "01.10.26"}, "1001"},
		// The status and the date are never the number, nor are amounts or names.
		{[]string{"Entwurf", "", "Acme GmbH", "", "12.11.26", "10.275,00 €"}, ""},
		{[]string{"1234", "", "", "", "5678"}, ""},
		{nil, ""},
	} {
		if got := (invoiceRow{Cells: c.cells}).number(); got != c.want {
			t.Errorf("number of %q = %q, want %q", c.cells, got, c.want)
		}
	}
}

func TestNextNumber(t *testing.T) {
	row := func(status, number string) invoiceRow {
		return invoiceRow{Status: status, Cells: []string{status, number, "-", "Acme GmbH", "01.10.26"}}
	}
	for _, c := range []struct {
		name string
		rows []invoiceRow
		want string
	}{
		{"after the highest", []invoiceRow{row("Bezahlt", "0200"), row("Offen", "0202"), row("Bezahlt", "0201")}, "0203"},
		{"drafts left out", []invoiceRow{row("Offen", "0202"), row("Entwurf", "-"), row("Entwurf", "0207")}, "0203"},
		{"with a prefix", []invoiceRow{row("Offen", "RE-0999")}, "1000"},
		{"none", []invoiceRow{row("Entwurf", "-")}, ""},
	} {
		if got := nextNumber(c.rows); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
