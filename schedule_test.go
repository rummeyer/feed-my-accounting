package main

import (
	"fmt"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCronLikeTheCrontab(t *testing.T) {
	for _, tt := range []struct {
		expr, from, want string
	}{
		{"0 6 5 * *", "2026-11-11 12:00", "2026-12-05 06:00"},
		{"0 7 3 * *", "2026-11-03 06:59", "2026-11-03 07:00"},
		{"30 7 21 * *", "2026-11-21 07:30", "2026-12-21 07:30"},
		// Day of month or weekday, as cron has it: Thursday the 12th is a weekday.
		{"0 12 25-31 * 1-5", "2026-11-11 12:00", "2026-11-12 12:00"},
		// Saturday the 28th is not a weekday, but the 28th.
		{"0 12 25-31 * 1-5", "2026-11-27 12:00", "2026-11-28 12:00"},
		{"*/15 * * * *", "2026-11-11 10:07", "2026-11-11 10:15"},
		{"0 9 * * 7", "2026-11-11 10:00", "2026-11-15 09:00"},  // Sunday as 7
		{"0 0 29 2 *", "2026-11-11 10:00", "2027-02-28 00:00"}, // none within a year: zero
	} {
		spec, err := parseCron(tt.expr)
		if err != nil {
			t.Fatalf("%s: %v", tt.expr, err)
		}
		got := spec.next(at(tt.from))
		if tt.expr == "0 0 29 2 *" {
			if !got.IsZero() {
				t.Errorf("%s: want none, got %s", tt.expr, got)
			}
			continue
		}
		if !got.Equal(at(tt.want)) {
			t.Errorf("%s from %s: got %s, want %s", tt.expr, tt.from, got.Format("2006-01-02 15:04"), tt.want)
		}
	}
}

func TestCronRejectsNonsense(t *testing.T) {
	for _, expr := range []string{"", "0 6 5 *", "60 * * * *", "0 24 * * *", "0 0 0 * *", "0 0 * 13 *", "x * * * *", "*/0 * * * *", "5-1 * * * *"} {
		if _, err := parseCron(expr); err == nil {
			t.Errorf("%q should not parse", expr)
		}
	}
}

func TestNextRunIsTheSoonestEnabledEntry(t *testing.T) {
	cfg := &Config{Schedule: []ScheduleEntry{
		{Job: "travel-expense", Cron: "0 6 5 * *", Enabled: true},
		{Job: "vodafone-invoice", Cron: "30 7 21 * *", Enabled: true},
		{Job: "apple-invoice", Cron: "0 7 12 * *", Enabled: false}, // sooner, but off
	}}
	next := nextRun(cfg, at("2026-11-11 13:00"))
	if next["job"] != "vodafone-invoice" || next["at"] != at("2026-11-21 07:30").Format(time.RFC3339) {
		t.Errorf("next: %v", next)
	}
	if nextRun(&Config{}, at("2026-11-11 13:00")) != nil {
		t.Error("nothing scheduled: nil")
	}
}

func TestSlashMonth(t *testing.T) {
	for in, want := range map[string]string{"11/2026": "2026-11", "1/2027": "2027-01", "01/2027": "2027-01", " 3/2026 ": "2026-03"} {
		y, m, err := slashMonth(in)
		if err != nil || fmt.Sprintf("%d-%02d", y, m) != want {
			t.Errorf("slashMonth(%q) = %d-%02d, %v; want %s", in, y, m, err, want)
		}
	}
	for _, bad := range []string{"2026-11", "13/2026", "11/26", "nov"} {
		if _, _, err := slashMonth(bad); err == nil {
			t.Errorf("slashMonth(%q): no error", bad)
		}
	}
	if y, m, _ := slashMonth(""); y != time.Now().Year() || m != time.Now().Month() {
		t.Errorf("slashMonth(\"\") = %d/%d, want the current month", m, y)
	}
}
