// Package travelexpense generates monthly travel expense PDFs and sends
// them via email. It produces two documents per month:
//   - Kilometergelderstattung (mileage reimbursement)
//   - Verpflegungsmehraufwand (meal allowance)
package travelexpense

import (
	"fmt"
	"log"
	"time"

	"feed-my-accounting/email"

	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/de"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Customer struct {
	ID       string
	Name     string
	From     string
	To       string
	Reason   string
	Distance int    // one-way distance in km
	Province string // German state abbreviation (e.g., "BW", "BY")
}

type Config struct {
	SMTP             email.SMTPConfig
	Email            email.EmailConfig
	Mitarbeiter      string
	Customers        []Customer
	ChristmasWeekOff *bool
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
	c.Name = "Rummeyer Consulting GmbH"
	c.Description = "Default company calendar"
	holidays, ok := provinceHolidays[province]
	if !ok {
		holidays = de.HolidaysBW
	}
	c.AddHoliday(holidays...)
	return c
}

func getCustomerCalendars(customers []Customer) []*cal.BusinessCalendar {
	calendars := make([]*cal.BusinessCalendar, len(customers))
	for i, c := range customers {
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
// Run
// ---------------------------------------------------------------------------

// Run generates travel expense PDFs for the given month and sends them via email.
func Run(cfg Config, year int, month time.Month) error {
	if len(cfg.Customers) == 0 {
		return fmt.Errorf("no customers configured")
	}

	log.Printf("Generating travel expense PDFs for %02d/%d...", month, year)

	calendars := getCustomerCalendars(cfg.Customers)
	numDays := daysInMonth(year, month)
	customerDays := make(map[int][]string, len(cfg.Customers))
	customerIdx := 0
	var firstDateString, lastDateString string
	totalWorkdays := 0

	for day := 1; day <= numDays; day++ {
		date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		if isWorkday(calendars[customerIdx], date, cfg.christmasWeekOffEnabled()) {
			dateString := formatDate(year, month, day)
			customerDays[customerIdx] = append(customerDays[customerIdx], dateString)
			if firstDateString == "" {
				firstDateString = dateString
			}
			lastDateString = dateString
			totalWorkdays++
			customerIdx = (customerIdx + 1) % len(cfg.Customers)
		}
	}

	log.Printf("Calculated %d workday(s) for %02d/%d", totalWorkdays, month, year)

	kmBlocks := make([]string, 0, totalWorkdays+len(cfg.Customers))
	verpBlocks := make([]string, 0, totalWorkdays+len(cfg.Customers))
	var totalKmCost float64

	for i, customer := range cfg.Customers {
		days := customerDays[i]
		if len(days) == 0 {
			continue
		}
		log.Printf("Customer %q: %d day(s)", customer.Name, len(days))
		kmBlocks = append(kmBlocks, buildCustomerHeader(customer))
		verpBlocks = append(verpBlocks, buildCustomerHeader(customer))
		for _, dateString := range days {
			kmBlocks = append(kmBlocks, buildKilometerEntry(dateString, customer.Distance))
			verpBlocks = append(verpBlocks, buildMealAllowanceEntry(dateString))
		}
		totalKmCost += float64(len(days)) * float64(customer.Distance) * kmRatePerKm
	}

	kmHeader := buildDocumentHeader(year, month, lastDateString, firstDateString, lastDateString, "Kilometergelderstattung")
	verpHeader := buildDocumentHeader(year, month, lastDateString, firstDateString, lastDateString, "Verpflegungsmehraufwand")
	kmFooter := buildDocumentFooter(totalKmCost)
	verpFooter := buildDocumentFooter(verpflegungRate * float64(totalWorkdays))

	kmFilename := fmt.Sprintf("%02d_%d_Reisekosten_Kilometergelderstattung.pdf", month, year)
	verpFilename := fmt.Sprintf("%02d_%d_Reisekosten_Verpflegungsmehraufwand.pdf", month, year)

	log.Printf("Generating Kilometergelderstattung PDF...")
	kmData, err := createPDF(cfg.Mitarbeiter, kmHeader, kmBlocks, kmFooter)
	if err != nil {
		return fmt.Errorf("generating km PDF: %w", err)
	}
	log.Printf("Generating Verpflegungsmehraufwand PDF...")
	verpData, err := createPDF(cfg.Mitarbeiter, verpHeader, verpBlocks, verpFooter)
	if err != nil {
		return fmt.Errorf("generating verp PDF: %w", err)
	}

	log.Printf("Sending email with 2 travel expense PDF attachment(s)...")
	subject := fmt.Sprintf("Deine Reisekostenabrechnung %02d/%d", month, year)
	return email.Send(cfg.SMTP, cfg.Email, subject,
		email.Attachment{Filename: kmFilename, Data: kmData},
		email.Attachment{Filename: verpFilename, Data: verpData},
	)
}
