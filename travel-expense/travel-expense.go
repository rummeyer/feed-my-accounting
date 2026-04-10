// Package travelexpense generates monthly travel expense PDFs and sends
// them via email. It produces two documents per month:
//   - Kilometergelderstattung (mileage reimbursement)
//   - Verpflegungsmehraufwand (meal allowance)
package travelexpense

import (
	"fmt"
	"log"
	"os"
	"time"

	"feed-my-accounting/email"

	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/de"
)

var logger = log.New(os.Stderr, "[travel] ", log.LstdFlags)

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
}

type Config struct {
	Mail             email.MailConfig
	Employee      string
	Clients        []Client
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
// Run
// ---------------------------------------------------------------------------

// Run generates travel expense PDFs for the given month and sends them via email.
func Run(cfg Config, year int, month time.Month) error {
	if len(cfg.Clients) == 0 {
		return fmt.Errorf("no clients configured")
	}

	logger.Printf("Generating travel expense PDFs for %02d/%d...", month, year)

	calendars := getClientCalendars(cfg.Clients)
	numDays := daysInMonth(year, month)
	clientDays := make(map[int][]string, len(cfg.Clients))
	clientIdx := 0
	var firstDateString, lastDateString string
	totalWorkdays := 0

	for day := 1; day <= numDays; day++ {
		date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		if isWorkday(calendars[clientIdx], date, cfg.christmasWeekOffEnabled()) {
			dateString := formatDate(year, month, day)
			clientDays[clientIdx] = append(clientDays[clientIdx], dateString)
			if firstDateString == "" {
				firstDateString = dateString
			}
			lastDateString = dateString
			totalWorkdays++
			clientIdx = (clientIdx + 1) % len(cfg.Clients)
		}
	}

	logger.Printf("Calculated %d workday(s) for %02d/%d", totalWorkdays, month, year)

	kmBlocks := make([]string, 0, totalWorkdays+len(cfg.Clients))
	verpBlocks := make([]string, 0, totalWorkdays+len(cfg.Clients))
	var totalKmCost float64

	for i, client := range cfg.Clients {
		days := clientDays[i]
		if len(days) == 0 {
			continue
		}
		logger.Printf("Client %q: %d day(s)", client.Name, len(days))
		kmBlocks = append(kmBlocks, buildClientHeader(client))
		verpBlocks = append(verpBlocks, buildClientHeader(client))
		for _, dateString := range days {
			kmBlocks = append(kmBlocks, buildKilometerEntry(dateString, client.Distance))
			verpBlocks = append(verpBlocks, buildMealAllowanceEntry(dateString))
		}
		totalKmCost += float64(len(days)) * float64(client.Distance) * kmRatePerKm
	}

	kmHeader := buildDocumentHeader(year, month, lastDateString, firstDateString, lastDateString, "Kilometergelderstattung")
	verpHeader := buildDocumentHeader(year, month, lastDateString, firstDateString, lastDateString, "Verpflegungsmehraufwand")
	kmFooter := buildDocumentFooter(totalKmCost)
	verpFooter := buildDocumentFooter(verpflegungRate * float64(totalWorkdays))

	kmFilename := fmt.Sprintf("%02d_%d_Reisekosten_Kilometergelderstattung.pdf", month, year)
	verpFilename := fmt.Sprintf("%02d_%d_Reisekosten_Verpflegungsmehraufwand.pdf", month, year)

	logger.Printf("Generating Kilometergelderstattung PDF...")
	kmData, err := createPDF(cfg.Employee, kmHeader, kmBlocks, kmFooter)
	if err != nil {
		return fmt.Errorf("generating km PDF: %w", err)
	}
	logger.Printf("Generating Verpflegungsmehraufwand PDF...")
	verpData, err := createPDF(cfg.Employee, verpHeader, verpBlocks, verpFooter)
	if err != nil {
		return fmt.Errorf("generating verp PDF: %w", err)
	}

	logger.Printf("Sending email with 2 travel expense PDF attachment(s)...")
	subject := fmt.Sprintf("Deine Reisekostenabrechnung %02d/%d", month, year)
	return email.Send(cfg.Mail, subject,
		email.Attachment{Filename: kmFilename, Data: kmData},
		email.Attachment{Filename: verpFilename, Data: verpData},
	)
}
