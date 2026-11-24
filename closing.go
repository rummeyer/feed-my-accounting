package main

import (
	"slices"
	"time"
)

// closingRow is one source of a month's books, as the closing shows it: the receipts sent, when
// it last looked, and whether the month still waits for it — a source the schedule runs that
// has neither sent nor looked.
type closingRow struct {
	Source   string    `json:"source"`
	Job      string    `json:"job"`
	Rule     bool      `json:"rule,omitempty"`   // one of the further senders
	Expected bool      `json:"expected"`         // the schedule runs it
	Docs     []SentDoc `json:"docs"`             // what it sent for the month
	Checked  time.Time `json:"checked,omitzero"` // when it last looked at the month
	Open     bool      `json:"open"`             // expected, and nothing yet
	Next     time.Time `json:"next,omitzero"`    // the schedule's next run of it
	Failed   string    `json:"failed,omitempty"` // its last run's error, if that failed
	FailedAt time.Time `json:"failedAt,omitzero"`
}

// closingSources are the sources in the order the closing lists them, with their jobs: the fixed
// ones, then a further sender per rule switched on.
func closingSources(cfg *Config) [][2]string {
	out := [][2]string{}
	for _, job := range []string{"travel-expense", "harvest-invoice", "amazon-invoice", "anthropic-invoice", "apple-invoice", "vodafone-invoice"} {
		out = append(out, [2]string{jobSources[job], job})
	}
	for _, r := range cfg.MailReceipts.Rules {
		if !r.Off && r.Name != "" {
			out = append(out, [2]string{r.Name, "mail-receipts"})
		}
	}
	return out
}

// closing is a month's books, source by source; last are the latest runs by job.
func closing(cfg *Config, d ledgerData, month time.Time, last map[string]Job, now time.Time) []closingRow {
	key := month.Format("2006-01")
	scheduled := map[string]bool{}
	next := map[string]time.Time{}
	for _, e := range cfg.Schedule {
		if !e.Enabled {
			continue
		}
		scheduled[e.Job] = true
		if spec, err := scheduleSpec(cfg, e.Cron); err == nil {
			if times := runTimes(cfg, e.Job, spec, now, 1); len(times) > 0 && (next[e.Job].IsZero() || times[0].Before(next[e.Job])) {
				next[e.Job] = times[0]
			}
		}
	}
	var rows []closingRow
	listed := map[string]bool{}
	add := func(source, job string, rule bool) {
		listed[source] = true
		r := closingRow{Source: source, Job: job, Rule: rule, Expected: scheduled[job], Docs: []SentDoc{}, Next: next[job]}
		for _, doc := range d.Docs {
			if doc.Source == source && doc.Month == key {
				r.Docs = append(r.Docs, doc)
			}
		}
		r.Checked = d.Checked[source][key]
		r.Open = r.Expected && len(r.Docs) == 0 && r.Checked.IsZero()
		if j, ok := last[job]; ok && j.Status == "failed" {
			r.Failed, r.FailedAt = j.Error, j.Ended
		}
		rows = append(rows, r)
	}
	for _, s := range closingSources(cfg) {
		add(s[0], s[1], s[1] == "mail-receipts")
	}
	// What was sent by a rule since removed: shown all the same.
	var others []string
	for _, doc := range d.Docs {
		if doc.Month == key && !listed[doc.Source] && !slices.Contains(others, doc.Source) {
			others = append(others, doc.Source)
		}
	}
	for _, source := range others {
		add(source, "", true)
	}
	return rows
}
