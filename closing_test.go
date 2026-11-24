package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"feed-my-accounting/email"
	mailreceipts "feed-my-accounting/mail-receipts"
)

func TestClosingSaysWhatAMonthWaitsFor(t *testing.T) {
	cfg := &Config{dir: t.TempDir()}
	cfg.Schedule = []ScheduleEntry{
		{Job: "anthropic-invoice", Cron: "0 7 L * *", Enabled: true},
		{Job: "apple-invoice", Cron: "0 7 3 * *", Enabled: true},
		{Job: "vodafone-invoice", Cron: "0 7 21 * *", Enabled: true},
		{Job: "mail-receipts", Cron: "0 7 L * *", Enabled: true},
	}
	cfg.MailReceipts.Rules = []mailreceipts.Rule{{Name: "Strato", From: "strato.de"}, {Name: "Alt", From: "x.de", Off: true}}
	l := ledgerOf(cfg)
	_ = l.Record("Anthropic", []email.Attachment{{Filename: "11_2026_Rechnung_Anthropic_12112026.pdf", Data: []byte("a")}})
	_ = l.Checked("Apple", time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local)) // looked, nothing there
	d, _ := l.Data()
	last := map[string]Job{"vodafone-invoice": {Kind: "vodafone-invoice", Status: "failed", Error: "login failed"}}
	rows := closing(cfg, d, time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local), last, time.Date(2026, 12, 5, 9, 0, 0, 0, time.Local))

	by := map[string]closingRow{}
	var names []string
	for _, r := range rows {
		by[r.Source] = r
		names = append(names, r.Source)
	}
	if got := strings.Join(names, ","); got != "Reisekosten,Kundenrechnung,Amazon,Anthropic,Apple,Vodafone,Strato" {
		t.Errorf("sources: %s", got)
	}
	if r := by["Anthropic"]; r.Open || len(r.Docs) != 1 {
		t.Errorf("Anthropic sent one: %+v", r)
	}
	if r := by["Apple"]; r.Open || r.Checked.IsZero() {
		t.Errorf("Apple looked and found nothing: done %+v", r)
	}
	if r := by["Vodafone"]; !r.Open || r.Failed != "login failed" {
		t.Errorf("Vodafone still open, its last run failed: %+v", r)
	}
	if r := by["Strato"]; !r.Open || !r.Rule {
		t.Errorf("Strato, a rule the schedule runs, still open: %+v", r)
	}
	if r := by["Amazon"]; r.Open || r.Expected {
		t.Errorf("Amazon not scheduled: not waited for: %+v", r)
	}
	if by["Anthropic"].Next.IsZero() {
		t.Error("the next run of a scheduled source")
	}
}

func TestReminderNamesWhatIsOpen(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = append(got, r.Form.Get("title")+": "+r.Form.Get("message"))
		w.Write([]byte(`{"status":1}`))
	}))
	defer srv.Close()
	old := pushoverURL
	pushoverURL = srv.URL
	defer func() { pushoverURL = old }()

	cfg := &Config{}
	cfg.Notify.Pushover.Enabled, cfg.Notify.Pushover.Token, cfg.Notify.Pushover.User = true, "t", "u"
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	remindClosing(cfg, []closingRow{{Source: "Apple"}, {Source: "Vodafone", Open: true}, {Source: "Strato", Open: true}}, month)
	remindClosing(cfg, []closingRow{{Source: "Apple"}}, month)
	if len(got) != 1 || !strings.Contains(got[0], "November 2026") || !strings.Contains(got[0], "Vodafone, Strato") {
		t.Errorf("one notification of what is open, none when complete: %q", got)
	}
	cfg.Notify.Pushover.Enabled = false
	if err := push(cfg, "x", "y"); err != nil || len(got) != 1 {
		t.Error("switched off, nothing is sent")
	}
}
