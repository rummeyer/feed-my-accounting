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
