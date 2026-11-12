package main

import (
	"testing"
	"time"
)

func TestParseMonthArg_ValidMonthYear(t *testing.T) {
	cases := []struct {
		input string
		year  int
		month time.Month
	}{
		{"1/2026", 2026, time.January},
		{"01/2026", 2026, time.January},
		{"3/2026", 2026, time.March},
		{"12/2025", 2025, time.December},
		{"9/2099", 2099, time.September},
	}
	for _, tc := range cases {
		year, month := parseMonthArg([]string{tc.input})
		if year != tc.year || month != tc.month {
			t.Errorf("parseMonthArg(%q) = %d/%s, want %d/%s", tc.input, year, month, tc.year, tc.month)
		}
	}
}

func TestParseMonthArg_InvalidFormats(t *testing.T) {
	cases := []string{
		"",
		"3",
		"2026/3",
		"13/2026",
		"0/2026",
		"3/26",
		"3/1999",
		"abc",
		"3-2026",
	}
	now := time.Now()
	wantYear, wantMonth, _ := now.Date()
	for _, input := range cases {
		year, month := parseMonthArg([]string{input})
		if year != wantYear || month != wantMonth {
			t.Errorf("parseMonthArg(%q) = %d/%s, want current month %d/%s", input, year, month, wantYear, wantMonth)
		}
	}
}

func TestParseMonthArg_NoArgs(t *testing.T) {
	now := time.Now()
	wantYear, wantMonth, _ := now.Date()
	year, month := parseMonthArg([]string{})
	if year != wantYear || month != wantMonth {
		t.Errorf("parseMonthArg([]) = %d/%s, want current %d/%s", year, month, wantYear, wantMonth)
	}
}

func TestParseMonthArg_FirstValidArgUsed(t *testing.T) {
	year, month := parseMonthArg([]string{"badarg", "5/2025", "6/2025"})
	if year != 2025 || month != time.May {
		t.Errorf("parseMonthArg() = %d/%s, want 2025/May", year, month)
	}
}

func TestParseMonthArg_ExtraArgs(t *testing.T) {
	year, month := parseMonthArg([]string{"--flag", "value", "7/2024"})
	if year != 2024 || month != time.July {
		t.Errorf("parseMonthArg() = %d/%s, want 2024/July", year, month)
	}
}

func TestClientDays(t *testing.T) {
	off := false
	cfg := &Config{Staff: StaffConfig{Names: []string{"Max Mustermann"}, Current: "Max Mustermann"}}
	cfg.TravelExpense = TravelExpenseConfig{
		ChristmasWeekOff: &off,
		Clients: []Client{
			{ID: "1", Name: "Acme GmbH", Province: "BW", Distance: 42},
			{ID: "2", Name: "Globex Inc.", Country: "US"},
		},
		Blocked:     []DateRange{{From: "2026-12-24"}, {From: "2026-12-31"}},
		Assignments: []Assignment{{Client: "2", From: "2026-12-03", To: "2026-12-09", Overnight: true}},
	}
	december := time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local)
	// The invoice's client: its days in the plan, not every workday — the overnight week takes five of them.
	cfg.HarvestInvoice.SevDesk.ClientName = "Acme GmbH"
	if n, which := clientDays(cfg)(december); n != 15 || which != "days at Acme GmbH" {
		t.Errorf("Acme in December: %d %s, want 15", n, which)
	}
	// No such client in the travel expenses: every workday — 23 weekdays, without the 25th, the
	// 24th and the 31st.
	cfg.HarvestInvoice.SevDesk.ClientName = "Someone Else"
	if n, which := clientDays(cfg)(december); n != 20 || which != "workdays" {
		t.Errorf("an unknown client in December: %d %s, want 20 workdays", n, which)
	}
}
