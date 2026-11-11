package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// cronSpec is a parsed five-field cron expression: minute, hour, day of month, month, day of
// week. As in cron, a day matches if day of month or day of week does, when both are
// restricted — "0 12 25-31 * 1-5" runs on the 25th to 31st and on every weekday.
type cronSpec struct {
	minute, hour, dom, month, dow [64]bool
	domAny, dowAny                bool
}

var cronFields = []struct {
	name     string
	min, max int
}{{"minute", 0, 59}, {"hour", 0, 23}, {"day of month", 1, 31}, {"month", 1, 12}, {"day of week", 0, 7}}

// parseCron reads "m h dom mon dow" with *, numbers, ranges a-b, lists a,b and steps */n or a-b/n.
func parseCron(expr string) (*cronSpec, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron %q: want 5 fields: minute hour day month weekday", expr)
	}
	spec := &cronSpec{domAny: fields[2] == "*", dowAny: fields[4] == "*"}
	sets := []*[64]bool{&spec.minute, &spec.hour, &spec.dom, &spec.month, &spec.dow}
	for i, field := range fields {
		if err := parseCronField(field, cronFields[i].min, cronFields[i].max, sets[i]); err != nil {
			return nil, fmt.Errorf("cron %q, %s: %w", expr, cronFields[i].name, err)
		}
	}
	spec.dow[0] = spec.dow[0] || spec.dow[7] // Sunday is 0 or 7
	return spec, nil
}

func parseCronField(field string, min, max int, set *[64]bool) error {
	for _, part := range strings.Split(field, ",") {
		rng, step := part, 1
		if i := strings.Index(part, "/"); i >= 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n < 1 {
				return fmt.Errorf("bad step in %q", part)
			}
			rng, step = part[:i], n
		}
		from, to := min, max
		if rng != "*" {
			a, b, isRange := strings.Cut(rng, "-")
			var err error
			if from, err = strconv.Atoi(a); err != nil {
				return fmt.Errorf("bad value %q", part)
			}
			to = from
			if isRange {
				if to, err = strconv.Atoi(b); err != nil {
					return fmt.Errorf("bad range %q", part)
				}
			} else if step > 1 {
				to = max // "5/15": from 5 on, every 15
			}
		}
		if from < min || to > max || from > to {
			return fmt.Errorf("%q out of %d-%d", part, min, max)
		}
		for v := from; v <= to; v += step {
			set[v] = true
		}
	}
	return nil
}

// matches says whether the spec fires in t's minute.
func (c *cronSpec) matches(t time.Time) bool {
	if !c.minute[t.Minute()] || !c.hour[t.Hour()] || !c.month[int(t.Month())] {
		return false
	}
	dom, dow := c.dom[t.Day()], c.dow[int(t.Weekday())]
	switch {
	case c.domAny && c.dowAny:
		return true
	case c.domAny:
		return dow
	case c.dowAny:
		return dom
	default:
		return dom || dow
	}
}

// next is the first minute after t the spec fires in, within a year and a day; zero if none.
func (c *cronSpec) next(t time.Time) time.Time {
	t = t.Truncate(time.Minute).Add(time.Minute)
	for end := t.AddDate(1, 0, 1); t.Before(end); t = t.Add(time.Minute) {
		if !c.month[int(t.Month())] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).Add(-time.Minute)
			continue
		}
		if c.matches(t) {
			return t
		}
	}
	return time.Time{}
}

// ---------------------------------------------------------------------------
// The jobs a schedule can run
// ---------------------------------------------------------------------------

// scheduledJob is a module as cron runs it: with the configuration as it is then, for the
// current month.
type scheduledJob struct {
	title string
	run   func(*Config) error
}

var scheduledJobs = map[string]scheduledJob{
	"travel-expense": {"Reisekosten senden (laufender Monat)", func(cfg *Config) error {
		now := time.Now()
		return runTravelExpense(cfg, now.Year(), now.Month())
	}},
	"apple-invoice":    {"Apple-Rechnungen abholen", runAppleInvoice},
	"vodafone-invoice": {"Vodafone-Rechnungen abholen", runVodafoneInvoice},
	"harvest-invoice":  {"Kundenrechnung aus Harvest-Mail", runHarvestInvoice},
}

// nextRun is the schedule's next run after now: its job and when; nil if nothing is scheduled.
func nextRun(cfg *Config, now time.Time) map[string]string {
	var job string
	var at time.Time
	for _, e := range cfg.Schedule {
		spec, err := parseCron(e.Cron)
		if !e.Enabled || err != nil {
			continue
		}
		if t := spec.next(now); !t.IsZero() && (at.IsZero() || t.Before(at)) {
			job, at = e.Job, t
		}
	}
	if at.IsZero() {
		return nil
	}
	return map[string]string{"job": job, "at": at.Format(time.RFC3339)}
}

// scheduler starts each enabled schedule entry's job when its time comes. A job due while
// another runs is tried again every minute, for half an hour.
func (s *server) scheduler() {
	pending := map[string]time.Time{} // job → when it fell due
	for {
		now := time.Now()
		time.Sleep(now.Truncate(time.Minute).Add(time.Minute).Sub(now) + time.Second)
		now = time.Now().Truncate(time.Minute)
		cfg, err := s.config()
		if err != nil {
			log.Printf("schedule: %v", err)
			continue
		}
		for _, entry := range cfg.Schedule {
			if !entry.Enabled {
				continue
			}
			spec, err := parseCron(entry.Cron)
			if err != nil {
				log.Printf("schedule: %v", err)
				continue
			}
			if spec.matches(now) {
				pending[entry.Job] = now
			}
		}
		for job, due := range pending {
			if now.Sub(due) > 30*time.Minute {
				log.Printf("schedule: %s due at %s never got its turn", job, due.Format("15:04"))
				delete(pending, job)
				continue
			}
			sj, ok := scheduledJobs[job]
			if !ok {
				delete(pending, job)
				continue
			}
			if _, err := s.jobs.Start(job, sj.title+" · Cron", func() ([]File, error) { return nil, sj.run(cfg) }); err == nil {
				delete(pending, job)
			}
		}
	}
}
