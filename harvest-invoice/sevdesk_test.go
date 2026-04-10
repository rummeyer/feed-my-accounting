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
