package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"feed-my-accounting/pdfdoc"
)

// cronSpec is a parsed five-field cron expression: minute, hour, day of month, month, day of
// week. As in cron, a day matches if day of month or day of week does, when both are
// restricted — "0 12 25-31 * 1-5" runs on the 25th to 31st and on every weekday.
//
// Beyond standard cron, what Quartz has for days, in the same five fields: in day of month L
// (the last day), L-3 (three days before it), 15W (the workday nearest the 15th, within the
// month) and LW (the last workday); in day of week 5L (the last Friday) and 1#2 (the second
// Monday). Names for months and weekdays, JAN-DEC and SUN-SAT; ? for *; and the shorthands
// @yearly, @monthly, @weekly, @daily and @hourly.
type cronSpec struct {
	minute, hour, dom, month, dow [64]bool
	domAny, dowAny                bool

	lastDays    []int // L-n: n days before the month's last day; L itself is 0
	nearest     []int // 15W: the workday nearest the day
	lastWorkday bool  // LW
	nthDays     []nthDay
	lastOfDays  []int // 5L: the weekday's last in the month

	// workday says which days W and LW count, a date at midnight UTC: Monday to Friday if nil.
	workday func(time.Time) bool
}

// nthDay is a weekday's nth time in the month: 1#2, the second Monday.
type nthDay struct{ weekday, n int }

var cronFields = []struct {
	name     string
	min, max int
	names    []string // the values' names from min on, for months and weekdays
}{
	{"minute", 0, 59, nil}, {"hour", 0, 23, nil}, {"day of month", 1, 31, nil},
	{"month", 1, 12, strings.Fields("JAN FEB MAR APR MAY JUN JUL AUG SEP OCT NOV DEC")},
	{"day of week", 0, 7, strings.Fields("SUN MON TUE WED THU FRI SAT")},
}

// cronShorthands are what the @ forms stand for.
var cronShorthands = map[string]string{
	"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *",
	"@weekly": "0 0 * * 0", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *",
}

// parseCron reads "m h dom mon dow" with *, numbers, ranges a-b, lists a,b and steps */n or a-b/n,
// and the Quartz forms above.
func parseCron(expr string) (*cronSpec, error) {
	if long, ok := cronShorthands[strings.ToLower(strings.TrimSpace(expr))]; ok {
		expr = long
	}
	fields := strings.Fields(strings.ToUpper(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron %q: want 5 fields: minute hour day month weekday", expr)
	}
	for i, f := range fields {
		if f == "?" {
			fields[i] = "*"
		}
	}
	spec := &cronSpec{domAny: fields[2] == "*", dowAny: fields[4] == "*"}
	sets := []*[64]bool{&spec.minute, &spec.hour, &spec.dom, &spec.month, &spec.dow}
	for i, field := range fields {
		var err error
		switch i {
		case 2:
			field, err = spec.domSpecials(field)
		case 4:
			field, err = spec.dowSpecials(field)
		}
		if err == nil && field != "" {
			err = parseCronField(field, cronFields[i].min, cronFields[i].max, cronFields[i].names, sets[i])
		}
		if err != nil {
			return nil, fmt.Errorf("cron %q, %s: %w", expr, cronFields[i].name, err)
		}
	}
	spec.dow[0] = spec.dow[0] || spec.dow[7] // Sunday is 0 or 7
	return spec, nil
}

// domSpecials takes L, L-n, nW and LW out of a day of month field; what is left is plain cron.
func (c *cronSpec) domSpecials(field string) (string, error) {
	var rest []string
	for _, part := range strings.Split(field, ",") {
		switch {
		case part == "L":
			c.lastDays = append(c.lastDays, 0)
		case part == "LW":
			c.lastWorkday = true
		case strings.HasPrefix(part, "L-"):
			n, err := strconv.Atoi(part[2:])
			if err != nil || n < 0 || n > 30 {
				return "", fmt.Errorf("bad %q: L-0 to L-30", part)
			}
			c.lastDays = append(c.lastDays, n)
		case strings.HasSuffix(part, "W"):
			n, err := strconv.Atoi(strings.TrimSuffix(part, "W"))
			if err != nil || n < 1 || n > 31 {
				return "", fmt.Errorf("bad %q: 1W to 31W", part)
			}
			c.nearest = append(c.nearest, n)
		default:
			rest = append(rest, part)
		}
	}
	return strings.Join(rest, ","), nil
}

// dowSpecials takes nL and n#k out of a day of week field; what is left is plain cron.
func (c *cronSpec) dowSpecials(field string) (string, error) {
	var rest []string
	for _, part := range strings.Split(field, ",") {
		switch {
		case strings.Contains(part, "#"):
			day, nth, _ := strings.Cut(part, "#")
			wd, err1 := cronValue(day, 0, 7, cronFields[4].names)
			n, err2 := strconv.Atoi(nth)
			if err1 != nil || err2 != nil || wd < 0 || wd > 7 || n < 1 || n > 5 {
				return "", fmt.Errorf("bad %q: weekday#1 to #5", part)
			}
			c.nthDays = append(c.nthDays, nthDay{wd % 7, n})
		case len(part) > 1 && strings.HasSuffix(part, "L"):
			wd, err := cronValue(strings.TrimSuffix(part, "L"), 0, 7, cronFields[4].names)
			if err != nil || wd < 0 || wd > 7 {
				return "", fmt.Errorf("bad %q: weekday L", part)
			}
			c.lastOfDays = append(c.lastOfDays, wd%7)
		default:
			rest = append(rest, part)
		}
	}
	return strings.Join(rest, ","), nil
}

// cronValue is a number, or a name of the field's: JAN, MON.
func cronValue(s string, min, max int, names []string) (int, error) {
	for i, name := range names {
		if s == name {
			return min + i, nil
		}
	}
	return strconv.Atoi(s)
}

func parseCronField(field string, min, max int, names []string, set *[64]bool) error {
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
			if from, err = cronValue(a, min, max, names); err != nil {
				return fmt.Errorf("bad value %q", part)
			}
			to = from
			if isRange {
				if to, err = cronValue(b, min, max, names); err != nil {
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
	dom, dow := c.domMatches(t), c.dowMatches(t)
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

// domMatches says whether t's day is one of the day of month field's: a number, or L, L-n, nW, LW.
func (c *cronSpec) domMatches(t time.Time) bool {
	if c.dom[t.Day()] {
		return true
	}
	last := daysIn(t)
	for _, n := range c.lastDays {
		if t.Day() == last-n {
			return true
		}
	}
	for _, n := range c.nearest {
		if t.Day() == c.nearestWorkday(t, min(n, last)) {
			return true
		}
	}
	return c.lastWorkday && t.Day() == c.nearestWorkday(t, last)
}

// dowMatches says whether t's weekday is one of the day of week field's: a day, nL, or n#k.
func (c *cronSpec) dowMatches(t time.Time) bool {
	wd := int(t.Weekday())
	if c.dow[wd] {
		return true
	}
	for _, n := range c.nthDays {
		if wd == n.weekday && (t.Day()-1)/7+1 == n.n {
			return true
		}
	}
	for _, d := range c.lastOfDays {
		if wd == d && t.Day()+7 > daysIn(t) {
			return true
		}
	}
	return false
}

// daysIn is the number of days in t's month.
func daysIn(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// nearestWorkday is the workday nearest the day of t's month, as Quartz has W: a Saturday's the
// Friday before, a Sunday's the Monday after — the day before first at the same distance — and
// never into the month before or after: the 1st a Saturday is Monday the 3rd, the last a Sunday
// is the Friday before. 0 if the month has none.
func (c *cronSpec) nearestWorkday(t time.Time, day int) int {
	last := daysIn(t)
	for d := 0; d < last; d++ {
		for _, n := range []int{day - d, day + d} {
			if n >= 1 && n <= last && c.isWorkday(time.Date(t.Year(), t.Month(), n, 0, 0, 0, 0, time.UTC)) {
				return n
			}
		}
	}
	return 0
}

// isWorkday says whether the date counts for W and LW.
func (c *cronSpec) isWorkday(date time.Time) bool {
	if c.workday != nil {
		return c.workday(date)
	}
	return date.Weekday() != time.Saturday && date.Weekday() != time.Sunday
}

// scheduleSpec is the expression as the schedule runs it: its W and LW skipping public holidays,
// the blocked days and, when off, the Christmas week, as the Reisekosten have them.
func scheduleSpec(cfg *Config, expr string) (*cronSpec, error) {
	spec, err := parseCron(expr)
	// The holiday calendar only for the expressions that ask for it: it is built anew each time.
	if err == nil && (len(spec.nearest) > 0 || spec.lastWorkday) {
		spec.workday = workdays(cfg)
	}
	return spec, err
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
	run   func(*Config) ([]File, error)
}

var scheduledJobs = map[string]scheduledJob{
	"travel-expense": {"Reisekosten senden (laufender Monat)", noFiles(func(cfg *Config) error {
		now := time.Now()
		return runTravelExpense(cfg, now.Year(), now.Month())
	})},
	"anthropic-invoice": {"Anthropic-Belege abholen", noFiles(runAnthropicInvoice)},
	"apple-invoice":     {"Apple-Rechnungen abholen", noFiles(runAppleInvoice)},
	"vodafone-invoice":  {"Vodafone-Rechnungen abholen", noFiles(runVodafoneInvoice)},
	"harvest-invoice": {"Kundenrechnung aus Harvest (laufender Monat)", func(cfg *Config) ([]File, error) {
		now := time.Now()
		return withFile(runHarvestInvoice(cfg, now.Year(), now.Month(), false))
	}},
}

// withFile is a run's document as the file it offers for download, if it made one.
func withFile(doc *pdfdoc.Document, err error) ([]File, error) {
	if doc == nil {
		return nil, err
	}
	return []File{{Name: doc.Filename, Size: len(doc.Data), data: doc.Data}}, err
}

// noFiles is a module run that makes no files to download.
func noFiles(run func(*Config) error) func(*Config) ([]File, error) {
	return func(cfg *Config) ([]File, error) { return nil, run(cfg) }
}

// nextRun is the schedule's next run after now: its job and when; nil if nothing is scheduled.
func nextRun(cfg *Config, now time.Time) map[string]string {
	var job string
	var at time.Time
	for _, e := range cfg.Schedule {
		spec, err := scheduleSpec(cfg, e.Cron)
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
			spec, err := scheduleSpec(cfg, entry.Cron)
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
			if _, err := s.jobs.Start(job, sj.title+" · Zeitplan", func() ([]File, error) { return sj.run(cfg) }); err == nil {
				delete(pending, job)
			}
		}
	}
}
