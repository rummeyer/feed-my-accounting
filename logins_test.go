package main

import (
	"errors"
	"testing"
	"time"
)

func TestLoginGate(t *testing.T) {
	cfg := &Config{dir: t.TempDir()}
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if err := loginGate(cfg, "vodafone", at("2026-11-17 08:30")); err != nil {
		t.Fatalf("no login yet, held back: %v", err)
	}
	recordLogin(cfg, "vodafone", at("2026-11-17 08:30"))
	for _, c := range []struct {
		now  string
		held bool
	}{
		{"2026-11-17 09:00", true},  // the same day
		{"2026-12-14 07:00", true},  // 27 days on
		{"2026-12-15 08:30", false}, // 28 days on
		{"2026-12-15 06:00", false}, // 28 days on, earlier in the day
		{"2026-12-21 07:00", false}, // the next month's run
	} {
		err := loginGate(cfg, "vodafone", at(c.now))
		if held := errors.Is(err, errGated); held != c.held {
			t.Errorf("%s: held back %v, want %v (%v)", c.now, held, c.held, err)
		}
	}
	// February's run: 28 days after January's.
	feb := &Config{dir: t.TempDir()}
	recordLogin(feb, "vodafone", at("2027-02-21 07:00"))
	if err := loginGate(feb, "vodafone", at("2027-03-21 07:00")); err != nil {
		t.Errorf("28 days on, held back: %v", err)
	}
	if next := nextLogin(cfg, "vodafone", at("2026-11-20 10:00")); !next.Equal(at("2026-12-15 00:00")) {
		t.Errorf("next login %v, want 15.12. 00:00", next)
	}
	if err := loginGate(cfg, "amazon", at("2026-11-17 09:00")); err != nil {
		t.Errorf("another site held back: %v", err)
	}
	for i := 0; i < 20; i++ {
		recordLogin(cfg, "vodafone", at("2026-11-17 08:30").AddDate(0, i+1, 0))
	}
	if n := len(readLogins(cfg)["vodafone"]); n != 12 {
		t.Errorf("%d logins kept, want 12", n)
	}
}

func TestRunTimesSkipGated(t *testing.T) {
	cfg := &Config{dir: t.TempDir()}
	at := func(s string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		return v
	}
	spec, err := parseCron("0 7 * * *") // daily: only every 28th day may log in
	if err != nil {
		t.Fatal(err)
	}
	recordLogin(cfg, "vodafone", at("2026-11-17 08:30"))
	got := runTimes(cfg, "vodafone-invoice", spec, at("2026-11-18 00:00"), 2)
	if len(got) != 2 || !got[0].Equal(at("2026-12-15 07:00")) || !got[1].Equal(at("2027-01-12 07:00")) {
		t.Errorf("got %v", got)
	}
	monthly, _ := parseCron("0 7 21 * *")
	got = runTimes(cfg, "vodafone-invoice", monthly, at("2026-11-18 00:00"), 1)
	if len(got) != 1 || !got[0].Equal(at("2026-12-21 07:00")) {
		t.Errorf("monthly: got %v", got)
	}
	if got := runTimes(cfg, "apple-invoice", spec, at("2026-11-18 00:00"), 1); !got[0].Equal(at("2026-11-18 07:00")) {
		t.Errorf("an ungated job skipped: %v", got)
	}
}

func TestManualGateLeavesTheScheduleItsRun(t *testing.T) {
	at := func(s string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		return v
	}
	cfg := &Config{dir: t.TempDir(), Schedule: []ScheduleEntry{{Job: "vodafone-invoice", Cron: "0 7 21 * *", Enabled: true}}}
	recordLogin(cfg, "vodafone", at("2026-10-21 07:00"))
	// The 19th: 28 days after the last, but two before the schedule's run.
	if err := manualGate(cfg, "vodafone", at("2026-11-19 10:00")); !errors.Is(err, errGated) {
		t.Errorf("by hand before the schedule's run: not held back")
	}
	if got := scheduledLoginRun(cfg, "vodafone", at("2026-11-19 10:00")); !got.Equal(at("2026-11-21 07:00")) {
		t.Errorf("schedule's run %v, want 21.11.", got)
	}
	cfg.Schedule[0].Enabled = false
	if err := manualGate(cfg, "vodafone", at("2026-11-19 10:00")); err != nil {
		t.Errorf("schedule off, held back: %v", err)
	}
}

func TestManualGateWithoutSchedule(t *testing.T) {
	at := func(s string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		return v
	}
	cfg := &Config{dir: t.TempDir(), Schedule: []ScheduleEntry{{Job: "vodafone-invoice", Cron: "0 7 21 * *", Enabled: false}}}
	recordLogin(cfg, "vodafone", at("2026-11-17 08:30"))
	if err := manualGate(cfg, "vodafone", at("2026-11-20 10:00")); !errors.Is(err, errGated) {
		t.Errorf("3 days after the last login, no schedule: not held back")
	}
	if got := scheduledLoginRun(cfg, "vodafone", at("2026-11-20 10:00")); !got.IsZero() {
		t.Errorf("schedule off, a scheduled run %v", got)
	}
	if next := nextLogin(cfg, "vodafone", at("2026-11-20 10:00")); !next.Equal(at("2026-12-15 00:00")) {
		t.Errorf("next login %v, want 15.12. 00:00", next)
	}
	if err := manualGate(cfg, "vodafone", at("2026-12-15 00:00")); err != nil {
		t.Errorf("28 days on, no schedule: held back: %v", err)
	}
}

func TestGateInDays(t *testing.T) {
	at := func(s string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		return v
	}
	cfg := &Config{dir: t.TempDir()}
	recordLogin(cfg, "vodafone", at("2027-01-31 18:00"))
	if err := loginGate(cfg, "vodafone", at("2027-02-28 09:00")); err != nil {
		t.Errorf("February's last day, in the morning: held back: %v", err)
	}
	if err := loginGate(cfg, "vodafone", at("2027-02-27 23:59")); !errors.Is(err, errGated) {
		t.Errorf("a day early: not held back")
	}
}
