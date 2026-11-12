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

func TestCronQuartzDays(t *testing.T) {
	for _, tt := range []struct {
		expr, from, want string
	}{
		// The last day, and days before it.
		{"0 7 L * *", "2026-11-12 10:00", "2026-11-30 07:00"},
		{"0 7 L * *", "2026-11-30 07:00", "2026-12-31 07:00"},
		{"0 7 L * *", "2027-01-31 08:00", "2027-02-28 07:00"}, // February's last
		{"0 7 L-2 * *", "2026-11-12 10:00", "2026-11-28 07:00"},
		// The workday nearest a day: the 15th of November 2026 a Sunday, so Monday the 16th.
		{"0 7 15W * *", "2026-11-12 10:00", "2026-11-16 07:00"},
		// Never into another month: the 1st of August 2026 a Saturday, so Monday the 3rd.
		{"0 7 1W * *", "2026-07-31 10:00", "2026-08-03 07:00"},
		// The last workday: the 31st of January 2027 a Sunday, so Friday the 29th.
		{"0 7 LW * *", "2026-12-31 08:00", "2027-01-29 07:00"},
		{"0 7 LW * *", "2026-11-12 10:00", "2026-11-30 07:00"}, // a Monday: itself
		// The last Friday, the second Monday.
		{"0 7 * * 5L", "2026-11-12 10:00", "2026-11-27 07:00"},
		{"0 7 * * FRIL", "2026-11-12 10:00", "2026-11-27 07:00"},
		{"0 7 * * 1#2", "2026-12-01 10:00", "2026-12-14 07:00"},
		{"0 7 * * MON#1", "2026-12-01 10:00", "2026-12-07 07:00"},
		// Names, ? and shorthands.
		{"0 7 * JAN MON", "2026-11-12 10:00", "2027-01-04 07:00"},
		{"0 7 ? * MON-FRI", "2026-11-13 08:00", "2026-11-16 07:00"},
		{"0 7 * * sat,sun", "2026-11-12 10:00", "2026-11-14 07:00"},
		{"@monthly", "2026-11-12 10:00", "2026-12-01 00:00"},
		{"@weekly", "2026-11-12 10:00", "2026-11-15 00:00"},
		{"@daily", "2026-11-12 10:00", "2026-11-13 00:00"},
	} {
		spec, err := parseCron(tt.expr)
		if err != nil {
			t.Fatalf("%s: %v", tt.expr, err)
		}
		if got := spec.next(at(tt.from)).Format("2006-01-02 15:04"); got != tt.want {
			t.Errorf("%s after %s: %s, want %s", tt.expr, tt.from, got, tt.want)
		}
	}
	for _, bad := range []string{"0 7 L-40 * *", "0 7 0W * *", "0 7 32W * *", "0 7 * * 1#6", "0 7 * * 8L", "0 7 * * XYZ", "0 7 * FOO *", "@sometimes"} {
		if _, err := parseCron(bad); err == nil {
			t.Errorf("%q taken", bad)
		}
	}
}

func TestScheduleWorkdays(t *testing.T) {
	off := false
	cfg := &Config{TravelExpense: TravelExpenseConfig{
		ChristmasWeekOff: &off,
		Clients:          []Client{{ID: "1", Province: "BW"}, {ID: "2", Country: "US"}},
		Blocked:          []DateRange{{From: "2026-12-24"}, {From: "2026-12-31"}},
	}}
	for _, tt := range []struct{ expr, from, want string }{
		// The 31st blocked: the 30th.
		{"0 7 LW * *", "2026-12-01 10:00", "2026-12-30 07:00"},
		// The 6th of January 2027 Epiphany in Baden-Württemberg: the day before, the 5th.
		{"0 7 6W * *", "2026-12-31 10:00", "2027-01-05 07:00"},
		// November without blocked days: still the 30th.
		{"0 7 LW * *", "2026-11-12 10:00", "2026-11-30 07:00"},
	} {
		spec, err := scheduleSpec(cfg, tt.expr)
		if err != nil {
			t.Fatalf("%s: %v", tt.expr, err)
		}
		if got := spec.next(at(tt.from)).Format("2006-01-02 15:04"); got != tt.want {
			t.Errorf("%s after %s: %s, want %s", tt.expr, tt.from, got, tt.want)
		}
	}
	on := true
	cfg.TravelExpense.ChristmasWeekOff = &on // the 24th and 27th to 31st off: the 23rd
	if spec, _ := scheduleSpec(cfg, "0 7 LW * *"); spec.next(at("2026-12-01 10:00")).Day() != 23 {
		t.Errorf("LW with the Christmas week off: %v", spec.next(at("2026-12-01 10:00")))
	}
}
