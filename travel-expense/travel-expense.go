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
	"strings"
	"time"

	"feed-my-accounting/email"
	"feed-my-accounting/pdfdoc"

	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/ar"
	"github.com/rickar/cal/v2/at"
	"github.com/rickar/cal/v2/be"
	"github.com/rickar/cal/v2/bg"
	"github.com/rickar/cal/v2/br"
	"github.com/rickar/cal/v2/ca"
	"github.com/rickar/cal/v2/ch"
	"github.com/rickar/cal/v2/cz"
	"github.com/rickar/cal/v2/de"
	"github.com/rickar/cal/v2/dk"
	"github.com/rickar/cal/v2/es"
	"github.com/rickar/cal/v2/fi"
	"github.com/rickar/cal/v2/fr"
	"github.com/rickar/cal/v2/gb"
	"github.com/rickar/cal/v2/gr"
	"github.com/rickar/cal/v2/hr"
	"github.com/rickar/cal/v2/ie"
	"github.com/rickar/cal/v2/is"
	"github.com/rickar/cal/v2/it"
	"github.com/rickar/cal/v2/jp"
	"github.com/rickar/cal/v2/lt"
	"github.com/rickar/cal/v2/lu"
	"github.com/rickar/cal/v2/lv"
	"github.com/rickar/cal/v2/mx"
	"github.com/rickar/cal/v2/nl"
	"github.com/rickar/cal/v2/no"
	"github.com/rickar/cal/v2/nz"
	"github.com/rickar/cal/v2/pl"
	"github.com/rickar/cal/v2/ro"
	"github.com/rickar/cal/v2/se"
	"github.com/rickar/cal/v2/si"
	"github.com/rickar/cal/v2/sk"
	"github.com/rickar/cal/v2/th"
	"github.com/rickar/cal/v2/ua"
	"github.com/rickar/cal/v2/us"
	"github.com/rickar/cal/v2/za"
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
	Country  string // ISO 3166 code, "DE" if empty: whose public holidays count
	Province string // German state abbreviation (e.g., "BW", "BY"), for Germany only

	// Meal allowance in EUR, zero for the German rates: a day trip of more than 8 hours, a full
	// day away, and the day of arrival or departure of an overnight trip. Abroad, the BMF's
	// Auslandspauschalen for the country.
	MealDay       float64
	MealFullDay   float64
	MealTravelDay float64
}

// MealRates are meal allowance rates in EUR (Verpflegungspauschalen).
type MealRates struct {
	Day       float64 // a day trip, more than 8 hours away
	FullDay   float64 // a full day, 24 hours away
	TravelDay float64 // the day of arrival or departure of an overnight trip
}

// GermanRates are the German rates by law, as of 2026: what the inland rates are unless the
// configuration says otherwise.
var GermanRates = MealRates{Day: 14, FullDay: 28, TravelDay: 14}

// inlandRates are the configured inland rates, the legal ones where left out.
func (cfg *Config) inlandRates() MealRates {
	return MealRates{
		Day:       orDefault(cfg.InlandRates.Day, GermanRates.Day),
		FullDay:   orDefault(cfg.InlandRates.FullDay, GermanRates.FullDay),
		TravelDay: orDefault(cfg.InlandRates.TravelDay, GermanRates.TravelDay),
	}
}

// The client's rates: its own where it has them, else the inland ones. Abroad, the BMF's table
// has one rate for the day of arrival or departure and for a day of more than 8 hours away: a
// client's day trip rate and travel day rate stand in for each other, before the inland one.
func (c Client) dayRate() float64 {
	if c.MealDay == 0 && c.Abroad() {
		return c.MealTravelDay
	}
	return c.MealDay
}

func (c Client) travelDayRate() float64 {
	if c.MealTravelDay == 0 && c.Abroad() {
		return c.MealDay
	}
	return c.MealTravelDay
}

func (c Client) mealDay(inland MealRates) float64 { return orDefault(c.dayRate(), inland.Day) }
func (c Client) mealFullDay(inland MealRates) float64 {
	return orDefault(c.MealFullDay, inland.FullDay)
}
func (c Client) mealTravelDay(inland MealRates) float64 {
	return orDefault(c.travelDayRate(), inland.TravelDay)
}

// missingRates names the rates the days need that the client leaves to the inland ones.
func (c Client) missingRates(days []Day) string {
	var missing []string
	need := map[string]bool{}
	for _, d := range days {
		need[d.MealText] = true
	}
	if need["8h - 24h"] && c.dayRate() == 0 {
		missing = append(missing, "mealDay")
	}
	if need["24h"] && c.MealFullDay == 0 {
		missing = append(missing, "mealFullDay")
	}
	if need["An-/Abreisetag"] && c.travelDayRate() == 0 {
		missing = append(missing, "mealTravelDay")
	}
	return strings.Join(missing, ", ")
}

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
	// InlandRates are the German meal rates for every client without its own; zero for the
	// legal ones (GermanRates).
	InlandRates MealRates
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

// Countries are the countries whose public holidays are known, by ISO 3166 code, with their
// German names. Germany's depend on the state, the others' are the national ones.
var Countries = map[string]string{
	"DE": "Deutschland", "AR": "Argentinien", "AT": "Österreich", "BE": "Belgien", "BG": "Bulgarien",
	"BR": "Brasilien", "CA": "Kanada", "CH": "Schweiz", "CZ": "Tschechien", "DK": "Dänemark",
	"ES": "Spanien", "FI": "Finnland", "FR": "Frankreich", "GB": "Vereinigtes Königreich",
	"GR": "Griechenland", "HR": "Kroatien", "IE": "Irland", "IS": "Island", "IT": "Italien",
	"JP": "Japan", "LT": "Litauen", "LU": "Luxemburg", "LV": "Lettland", "MX": "Mexiko",
	"NL": "Niederlande", "NO": "Norwegen", "NZ": "Neuseeland", "PL": "Polen", "RO": "Rumänien",
	"SE": "Schweden", "SI": "Slowenien", "SK": "Slowakei", "TH": "Thailand", "UA": "Ukraine",
	"US": "USA", "ZA": "Südafrika",
}

var countryHolidays = map[string][]*cal.Holiday{
	"AR": ar.Holidays, "AT": at.Holidays, "BE": be.Holidays, "BG": bg.Holidays, "BR": br.Holidays,
	"CA": ca.Holidays, "CH": ch.Holidays, "CZ": cz.Holidays, "DK": dk.Holidays, "ES": es.Holidays,
	"FI": fi.Holidays, "FR": fr.Holidays, "GB": gb.Holidays, "GR": gr.Holidays, "HR": hr.Holidays,
	"IE": ie.Holidays, "IS": is.Holidays, "IT": it.Holidays, "JP": jp.Holidays, "LT": lt.Holidays,
	"LU": lu.Holidays, "LV": lv.Holidays, "MX": mx.Holidays, "NL": nl.Holidays, "NO": no.Holidays,
	"NZ": nz.Holidays, "PL": pl.Holidays, "RO": ro.Holidays, "SE": se.Holidays, "SI": si.Holidays,
	"SK": sk.Holidays, "TH": th.Holidays, "UA": ua.Holidays, "US": us.Holidays, "ZA": za.Holidays,
}

// newBusinessCalendar is a client's working days: weekends and the public holidays of its
// country — in Germany of its state, Baden-Württemberg if none is given.
func newBusinessCalendar(country, province string) *cal.BusinessCalendar {
	c := cal.NewBusinessCalendar()
	c.Name = "Company"
	c.Description = "Business calendar"
	if country = normalCountry(country); country != "DE" {
		c.AddHoliday(countryHolidays[country]...)
		return c
	}
	holidays, ok := provinceHolidays[province]
	if !ok {
		holidays = de.HolidaysBW
	}
	c.AddHoliday(holidays...)
	return c
}

// normalCountry is the country's code in capitals, Germany if empty.
func normalCountry(country string) string {
	if country = strings.ToUpper(strings.TrimSpace(country)); country == "" {
		return "DE"
	}
	return country
}

// Abroad says whether the client is outside Germany, where the German meal rates do not apply.
func (c Client) Abroad() bool { return normalCountry(c.Country) != "DE" }

// europe are the known countries in Europe: a day trip there is a long drive, but one.
var europe = map[string]bool{
	"DE": true, "AT": true, "BE": true, "BG": true, "CH": true, "CZ": true, "DK": true, "ES": true,
	"FI": true, "FR": true, "GB": true, "GR": true, "HR": true, "IE": true, "IS": true, "IT": true,
	"LT": true, "LU": true, "LV": true, "NL": true, "NO": true, "PL": true, "RO": true, "SE": true,
	"SI": true, "SK": true, "UA": true,
}

// Overseas says whether the client is outside Europe: no day trips there, only assigned days —
// an overnight trip — never the equally distributed ones.
func (c Client) Overseas() bool { return !europe[normalCountry(c.Country)] }

func getClientCalendars(clients []Client) []*cal.BusinessCalendar {
	calendars := make([]*cal.BusinessCalendar, len(clients))
	for i, c := range clients {
		calendars[i] = newBusinessCalendar(c.Country, c.Province)
	}
	return calendars
}

func isWorkday(c *cal.BusinessCalendar, date time.Time, christmasWeekOff bool) bool {
	return c.IsWorkday(date) && !(christmasWeekOff && christmasWeek(date))
}

// christmasWeek says whether date is Dec 24 or Dec 27–31, the days off with christmasWeekOff;
// Dec 25 and 26 are holidays anyway.
func christmasWeek(date time.Time) bool {
	day := date.Day()
	return date.Month() == 12 && (day == 24 || (day >= 27 && day <= 31))
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
	// Missing names the client's own rate the day needed but did not find, abroad, where the
	// inland rate was taken instead: "mealDay", "mealFullDay" or "mealTravelDay".
	Missing string
}

// Plan works out each day of the month. An overnight assignment counts every day within it.
// Otherwise weekends, the Christmas week and blocked ranges are off; a workday assigned to a
// client goes to it, checked against its own holidays; and the remaining workdays go to the
// clients in Europe without assignments of their own, in one consecutive block each, of equal
// size, in the order the clients are listed.
func Plan(cfg Config, year int, month time.Month) []Day {
	if len(cfg.Clients) == 0 {
		return nil
	}
	calendars := getClientCalendars(cfg.Clients)
	christmas := cfg.christmasWeekOffEnabled()
	inland := cfg.inlandRates()
	// The clients sharing the unassigned workdays: those without assignments of their own. A
	// client with assignments gets those days, and no others.
	assignedTo := map[int]bool{}
	for _, a := range cfg.Assignments {
		assignedTo[a.Client] = true
	}
	// Nor do clients outside Europe: going there and back every day is no way to travel.
	var shared []int
	for i, c := range cfg.Clients {
		if !assignedTo[i] && !c.Overseas() {
			shared = append(shared, i)
		}
	}
	sharedCalendars := []*cal.BusinessCalendar{newBusinessCalendar("", "")}
	if len(shared) > 0 {
		sharedCalendars = sharedCalendars[:0]
		for _, i := range shared {
			sharedCalendars = append(sharedCalendars, calendars[i])
		}
	}

	days := make([]Day, 0, 31)
	var open []int // the days left for the shared clients, by index into days
	for d := 1; d <= daysInMonth(year, month); d++ {
		date := time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
		day := Day{Date: date, Client: -1}
		assigned := assignmentOn(cfg.Assignments, date)
		if assigned != nil && !cfg.validClient(assigned.Client) {
			assigned = nil
		}
		if assigned != nil && assigned.Overnight {
			days = append(days, awayDay(cfg.Clients[assigned.Client], *assigned, day, inland))
			continue
		}
		// Whose holidays count: the assigned client's; for the shared days, any of the shared
		// clients' — a day off for all of them is off, the rest is settled with the blocks.
		cals := sharedCalendars
		if assigned != nil {
			cals = []*cal.BusinessCalendar{calendars[assigned.Client]}
		}
		switch {
		case date.Weekday() == time.Saturday || date.Weekday() == time.Sunday:
			day.Kind = DayWeekend
		case holidayForAll(cals, date, &day):
			day.Kind = DayHoliday
		case christmas && christmasWeek(date):
			day.Kind = DayChristmas
		case blockedNote(cfg.Blocked, date) != nil:
			day.Kind = DayBlocked
			day.Note = *blockedNote(cfg.Blocked, date)
		case assigned != nil:
			day = travelDay(day, cfg.Clients[assigned.Client], assigned.Client, inland)
			day.Assigned, day.Note = true, assigned.Note
		case len(shared) == 0:
			day.Kind = DayHome
		default:
			open = append(open, len(days))
		}
		days = append(days, day)
	}

	// The open days in consecutive blocks, one per shared client, as equal as they come: the
	// first clients get one more where they do not divide evenly.
	for k, from := 0, 0; k < len(shared); k++ {
		size := len(open) / len(shared)
		if k < len(open)%len(shared) {
			size++
		}
		client := shared[k]
		for _, i := range open[from : from+size] {
			day := days[i]
			if c := calendars[client]; !c.IsWorkday(day.Date) {
				day.Kind = DayHoliday // a holiday of this client's, another's workday
				if _, _, h := c.IsHoliday(day.Date); h != nil {
					day.Note = h.Name
				}
			} else {
				day = travelDay(day, cfg.Clients[client], client, inland)
			}
			days[i] = day
		}
		from += size
	}
	return days
}

// holidayForAll says whether date is a public holiday for every one of the calendars, and names
// it in day if so.
func holidayForAll(cals []*cal.BusinessCalendar, date time.Time, day *Day) bool {
	for _, c := range cals {
		if c.IsWorkday(date) {
			return false
		}
	}
	if _, _, h := cals[0].IsHoliday(date); h != nil {
		day.Note = h.Name
	}
	return true
}

// travelDay is a day trip to the client: the day rate, the hours away, and the drive there and
// back.
func travelDay(day Day, c Client, client int, inland MealRates) Day {
	day.Kind, day.Client = DayTravel, client
	day.Times, day.Meal, day.MealText = "07:00 - 17:00", c.mealDay(inland), "8h - 24h"
	day.Mileage = float64(c.Distance) * kmRatePerKm
	if c.Abroad() && c.dayRate() == 0 {
		day.Missing = "mealDay"
	}
	return day
}

// awayDay is a day of an overnight trip: the travel day rate on its first and last day, with the
// drive there or back; the full day rate between. A trip of one day is a day trip.
func awayDay(c Client, a Assignment, day Day, inland MealRates) Day {
	to := a.To
	if to.IsZero() {
		to = a.From
	}
	first, last := day.Date.Equal(a.From), day.Date.Equal(to)
	day.Kind, day.Client, day.Assigned, day.Note = DayAway, a.Client, true, a.Note
	own := 0.0
	switch {
	case first && last:
		day.Times, day.Meal, day.MealText, own = "07:00 - 17:00", c.mealDay(inland), "8h - 24h", c.dayRate()
		day.Missing = "mealDay"
	case first || last:
		day.Meal, day.MealText, own = c.mealTravelDay(inland), "An-/Abreisetag", c.travelDayRate()
		day.Missing = "mealTravelDay"
	default:
		day.Meal, day.MealText, own = c.mealFullDay(inland), "24h", c.MealFullDay
		day.Missing = "mealFullDay"
	}
	if !c.Abroad() || own > 0 {
		day.Missing = ""
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
type Document = pdfdoc.Document

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
		if missing := client.missingRates(days); client.Abroad() && missing != "" {
			logger.Printf("WARNING: %q is abroad (%s) but has no %s: the inland rate is used", client.Name, normalCountry(client.Country), missing)
		}
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
