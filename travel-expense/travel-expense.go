// Package travelexpense generates monthly travel expense PDFs and sends
// them via email. It produces two documents per month:
//   - Kilometergelderstattung (mileage reimbursement)
//   - Verpflegungsmehraufwand (meal allowance)
package travelexpense

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"feed-my-accounting/email"

	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/de"
)

var logger = log.New(os.Stderr, "[travel-expense] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Client struct {
	ID       string
	Name     string
	From     string
	To       string
	Reason   string
	Distance int    // one-way distance in km
	Province string // German state abbreviation (e.g., "BW", "BY")

	// Meal allowance in EUR, zero for the German rates: a day trip of more than 8 hours, a full
	// day away, and the day of arrival or departure of an overnight trip. Abroad, the BMF's
	// Auslandspauschalen for the country.
	MealDay       float64
	MealFullDay   float64
	MealTravelDay float64
	// OnlyAssigned keeps the client out of the equal distribution: it gets the days assigned to
	// it, and no others.
	OnlyAssigned bool
}

// German meal allowance rates (Verpflegungspauschalen).
const (
	defaultMealDay       = 14.0 // more than 8 hours away
	defaultMealFullDay   = 28.0 // 24 hours away
	defaultMealTravelDay = 14.0 // day of arrival or departure of an overnight trip
)

func (c Client) mealDay() float64       { return orDefault(c.MealDay, defaultMealDay) }
func (c Client) mealFullDay() float64   { return orDefault(c.MealFullDay, defaultMealFullDay) }
func (c Client) mealTravelDay() float64 { return orDefault(c.MealTravelDay, defaultMealTravelDay) }

func orDefault(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

// Assignment gives a stretch of days to one client explicitly, instead of the equal distribution.
// As day trips, the workdays within it go to the client. Overnight, every day within it counts,
// weekends and holidays too: the first and last at the travel day rate, those between at the
// full day rate, with mileage only for the drive there and back.
type Assignment struct {
	Client    int // index into Config.Clients
	From      time.Time
	To        time.Time
	Overnight bool
	Note      string
}

func (a Assignment) contains(date time.Time) bool {
	return BlockedRange{From: a.From, To: a.To}.Contains(date)
}

// BlockedRange is a stretch of days without travel — vacation, illness, a conference — from
// From to To inclusive. To may be zero for a single day.
type BlockedRange struct {
	From time.Time
	To   time.Time
	Note string
}

// Contains says whether date falls within the range.
func (b BlockedRange) Contains(date time.Time) bool {
	to := b.To
	if to.IsZero() {
		to = b.From
	}
	return !date.Before(b.From) && !date.After(to)
}

type Config struct {
	Mail             email.MailConfig
	Employee         string
	Clients          []Client
	ChristmasWeekOff *bool
	Blocked          []BlockedRange
	Assignments      []Assignment
}

func (c *Config) christmasWeekOffEnabled() bool {
	return c.ChristmasWeekOff == nil || *c.ChristmasWeekOff
}

// ---------------------------------------------------------------------------
// Business Calendar
// ---------------------------------------------------------------------------

var provinceHolidays = map[string][]*cal.Holiday{
	"BW": de.HolidaysBW, "BY": de.HolidaysBY, "BE": de.HolidaysBE,
	"BB": de.HolidaysBB, "HB": de.HolidaysHB, "HH": de.HolidaysHH,
	"HE": de.HolidaysHE, "MV": de.HolidaysMV, "NI": de.HolidaysNI,
	"NW": de.HolidaysNW, "RP": de.HolidaysRP, "SL": de.HolidaysSL,
	"SN": de.HolidaysSN, "ST": de.HolidaysST, "SH": de.HolidaysSH,
	"TH": de.HolidaysTH,
}

func newBusinessCalendar(province string) *cal.BusinessCalendar {
	c := cal.NewBusinessCalendar()
	c.Name = "Company"
	c.Description = "Business calendar"
	holidays, ok := provinceHolidays[province]
	if !ok {
		holidays = de.HolidaysBW
	}
	c.AddHoliday(holidays...)
	return c
}

func getClientCalendars(clients []Client) []*cal.BusinessCalendar {
	calendars := make([]*cal.BusinessCalendar, len(clients))
	for i, c := range clients {
		calendars[i] = newBusinessCalendar(c.Province)
	}
	return calendars
}

func isWorkday(c *cal.BusinessCalendar, date time.Time, christmasWeekOff bool) bool {
	if !c.IsWorkday(date) {
		return false
	}
	if christmasWeekOff && date.Month() == 12 {
		day := date.Day()
		if day == 24 || (day >= 27 && day <= 31) {
			return false
		}
	}
	return true
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// ---------------------------------------------------------------------------
// Plan
// ---------------------------------------------------------------------------

// Day kinds in a Plan.
const (
	DayTravel    = "travel"    // a day trip to a client
	DayAway      = "away"      // a day of an overnight trip
	DayHome      = "home"      // a workday no client gets: all of them only take assigned days
	DayWeekend   = "weekend"   // Saturday or Sunday
	DayHoliday   = "holiday"   // a public holiday in the client's province
	DayChristmas = "christmas" // Dec 24 or Dec 27–31, with christmasWeekOff
	DayBlocked   = "blocked"   // a configured blocked range: vacation and the like
)

// Day is one day of a month's plan: whether it is travelled, to which client, and what it is
// worth.
type Day struct {
	Date     time.Time
	Kind     string
	Client   int    // index into Config.Clients for travel and away days, else -1
	Assigned bool   // given to the client explicitly, not by the equal distribution
	Note     string // the holiday's, blocked range's or assignment's name
	Times    string // the hours away, for a day trip
	Meal     float64
	MealText string // what the meal allowance is for: "8h - 24h", "24h", "An-/Abreisetag"
	Mileage  float64
}

// Plan works out each day of the month. An overnight assignment counts every day within it.
// Otherwise weekends, holidays, the Christmas week and blocked ranges are off, and a workday goes
// to the client it is assigned to, or else round-robin to the clients not only taking assigned
// days, each checked against its own province's holidays.
func Plan(cfg Config, year int, month time.Month) []Day {
	if len(cfg.Clients) == 0 {
		return nil
	}
	calendars := getClientCalendars(cfg.Clients)
	christmas := cfg.christmasWeekOffEnabled()
	var shared []int // the clients sharing the unassigned workdays
	for i, c := range cfg.Clients {
		if !c.OnlyAssigned {
			shared = append(shared, i)
		}
	}
	days := make([]Day, 0, 31)
	next := 0 // the next client in shared to get a day
	for d := 1; d <= daysInMonth(year, month); d++ {
		date := time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
		day := Day{Date: date, Client: -1}
		assigned := assignmentOn(cfg.Assignments, date)
		if assigned != nil && assigned.Overnight && cfg.validClient(assigned.Client) {
			days = append(days, awayDay(cfg.Clients[assigned.Client], *assigned, day))
			continue
		}
		// Whose holidays count: the assigned client's, else the next one's in turn.
		client := -1
		switch {
		case assigned != nil && cfg.validClient(assigned.Client):
			client = assigned.Client
		case len(shared) > 0:
			client = shared[next]
		}
		c := newBusinessCalendar("")
		if client >= 0 {
			c = calendars[client]
		}
		switch {
		case date.Weekday() == time.Saturday || date.Weekday() == time.Sunday:
			day.Kind = DayWeekend
		case !c.IsWorkday(date):
			day.Kind = DayHoliday
			if _, _, h := c.IsHoliday(date); h != nil {
				day.Note = h.Name
			}
		case !isWorkday(c, date, christmas):
			day.Kind = DayChristmas
		case blockedNote(cfg.Blocked, date) != nil:
			day.Kind = DayBlocked
			day.Note = *blockedNote(cfg.Blocked, date)
		case client < 0:
			day.Kind = DayHome
		default:
			cl := cfg.Clients[client]
			day.Kind, day.Client = DayTravel, client
			day.Times, day.Meal, day.MealText = "07:00 - 17:00", cl.mealDay(), "8h - 24h"
			day.Mileage = float64(cl.Distance) * kmRatePerKm
			if assigned != nil {
				day.Assigned, day.Note = true, assigned.Note
			} else {
				next = (next + 1) % len(shared)
			}
		}
		days = append(days, day)
	}
	return days
}

// awayDay is a day of an overnight trip: the travel day rate on its first and last day, with the
// drive there or back; the full day rate between. A trip of one day is a day trip.
func awayDay(c Client, a Assignment, day Day) Day {
	to := a.To
	if to.IsZero() {
		to = a.From
	}
	first, last := day.Date.Equal(a.From), day.Date.Equal(to)
	day.Kind, day.Client, day.Assigned, day.Note = DayAway, a.Client, true, a.Note
	switch {
	case first && last:
		day.Times, day.Meal, day.MealText = "07:00 - 17:00", c.mealDay(), "8h - 24h"
	case first || last:
		day.Meal, day.MealText = c.mealTravelDay(), "An-/Abreisetag"
	default:
		day.Meal, day.MealText = c.mealFullDay(), "24h"
	}
	if first || last {
		day.Mileage = float64(c.Distance) * kmRatePerKm
	}
	return day
}

// assignmentOn is the assignment covering date, the first one listed if several do.
func assignmentOn(assignments []Assignment, date time.Time) *Assignment {
	for i := range assignments {
		if assignments[i].contains(date) {
			return &assignments[i]
		}
	}
	return nil
}

func (c *Config) validClient(i int) bool { return i >= 0 && i < len(c.Clients) }

// blockedNote is the note of the blocked range containing date, if any.
func blockedNote(blocked []BlockedRange, date time.Time) *string {
	for _, b := range blocked {
		if b.Contains(date) {
			note := b.Note
			return &note
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Generate & send
// ---------------------------------------------------------------------------

// ErrNoTravelDays is a month without a day to travel: all of it weekends, holidays or blocked.
var ErrNoTravelDays = errors.New("no travel days")

// Document is a generated PDF.
type Document struct {
	Filename string
	Data     []byte
}

// Result is a month's travel expense PDFs and their totals. Mileage is nil for a month without
// any driving, as one spent abroad.
type Result struct {
	Days       int // days with a meal allowance: day trips and days away
	MileageEUR float64
	MealsEUR   float64
	Mileage    *Document // Kilometergelderstattung
	Meals      *Document // Verpflegungsmehraufwand
}

// Documents are the PDFs to send.
func (r *Result) Documents() []Document {
	var docs []Document
	for _, d := range []*Document{r.Mileage, r.Meals} {
		if d != nil {
			docs = append(docs, *d)
		}
	}
	return docs
}

// Generate builds the month's PDFs from its plan without sending them.
func Generate(cfg Config, year int, month time.Month) (*Result, error) {
	if len(cfg.Clients) == 0 {
		return nil, fmt.Errorf("no clients configured")
	}

	logger.Printf("Generating travel expense PDFs for %02d/%d...", month, year)

	byClient := make(map[int][]Day, len(cfg.Clients))
	var first, last string
	r := &Result{}
	for _, day := range Plan(cfg, year, month) {
		if day.Kind != DayTravel && day.Kind != DayAway {
			continue
		}
		byClient[day.Client] = append(byClient[day.Client], day)
		if first == "" {
			first = formatDate(year, month, day.Date.Day())
		}
		last = formatDate(year, month, day.Date.Day())
		r.Days++
		r.MealsEUR += day.Meal
		r.MileageEUR += day.Mileage
	}

	logger.Printf("Calculated %d travel day(s) for %02d/%d", r.Days, month, year)
	if r.Days == 0 {
		return nil, fmt.Errorf("%02d/%d: %w", month, year, ErrNoTravelDays)
	}

	var kmBlocks, mealBlocks []string
	for i, client := range cfg.Clients {
		days := byClient[i]
		if len(days) == 0 {
			continue
		}
		logger.Printf("Client %q: %d day(s)", client.Name, len(days))
		mealBlocks = append(mealBlocks, buildClientHeader(client))
		drives := false
		for _, day := range days {
			date := formatDate(year, month, day.Date.Day())
			mealBlocks = append(mealBlocks, buildMealEntry(date, day.Times, day.MealText, day.Meal))
			if day.Mileage > 0 {
				if !drives {
					kmBlocks, drives = append(kmBlocks, buildClientHeader(client)), true
				}
				kmBlocks = append(kmBlocks, buildKilometerEntry(date, client.Distance))
			}
		}
	}

	if len(kmBlocks) > 0 {
		logger.Printf("Generating Kilometergelderstattung PDF...")
		header := buildDocumentHeader(year, month, last, first, last, "Kilometergelderstattung")
		data, err := createPDF(cfg.Employee, header, kmBlocks, buildDocumentFooter(r.MileageEUR))
		if err != nil {
			return nil, fmt.Errorf("generating km PDF: %w", err)
		}
		r.Mileage = &Document{Filename: fmt.Sprintf("%02d_%d_Reisekosten_Kilometergelderstattung.pdf", month, year), Data: data}
	}
	logger.Printf("Generating Verpflegungsmehraufwand PDF...")
	header := buildDocumentHeader(year, month, last, first, last, "Verpflegungsmehraufwand")
	data, err := createPDF(cfg.Employee, header, mealBlocks, buildDocumentFooter(r.MealsEUR))
	if err != nil {
		return nil, fmt.Errorf("generating verp PDF: %w", err)
	}
	r.Meals = &Document{Filename: fmt.Sprintf("%02d_%d_Reisekosten_Verpflegungsmehraufwand.pdf", month, year), Data: data}
	return r, nil
}

// Send mails a month's PDFs to the configured address.
func Send(cfg Config, year int, month time.Month, r *Result) error {
	var attachments []email.Attachment
	for _, d := range r.Documents() {
		attachments = append(attachments, email.Attachment{Filename: d.Filename, Data: d.Data})
	}
	logger.Printf("Sending email with %d travel expense PDF attachment(s)...", len(attachments))
	subject := fmt.Sprintf("Deine Reisekostenabrechnung %02d/%d", month, year)
	return email.Send(cfg.Mail, subject, attachments...)
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

// Run generates travel expense PDFs for the given month and sends them via email.
func Run(cfg Config, year int, month time.Month) error {
	r, err := Generate(cfg, year, month)
	if errors.Is(err, ErrNoTravelDays) {
		logger.Printf("Nothing to send: %v", err)
		return nil
	}
	if err != nil {
		return err
	}
	return Send(cfg, year, month, r)
}
