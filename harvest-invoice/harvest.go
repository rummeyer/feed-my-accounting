package harvest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"feed-my-accounting/browser"
	"feed-my-accounting/pdfdoc"
)

var logger = log.New(os.Stderr, "[harvest-invoice] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// Config holds all configuration for the harvest-invoice module.
type Config struct {
	SkipExisting bool
	Harvest      HarvestAPI    `yaml:"harvest"`
	SevDesk      SevDeskConfig `yaml:"sevdesk"`
	Month        time.Time     // the month to invoice; zero is the current one
}

// HarvestAPI is the access to the Harvest API v2: a personal access token from
// id.getharvest.com/developers — on every plan, the free one too.
type HarvestAPI struct {
	Token     string `yaml:"token"`
	AccountID string `yaml:"accountId"` // empty: the token's Harvest account
	Client    string `yaml:"client"`    // client in Harvest; empty: the sevDesk client's name
	Person    string `yaml:"-"`         // whose hours, as Harvest names them; empty: everyone's
}

// Run reads the month's time entries of the client from the Harvest API, makes their work log,
// and creates a sevDesk draft invoice for their hours via browser automation — unless sevDesk has
// one for the month already. The work log comes back either way, once made.
func Run(cfg Config) (*pdfdoc.Document, error) {
	report, log, err := monthReport(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := browser.NewContext(browser.WithGermanLocale())
	defer cancel()

	// Guard: check if sevDesk already has an invoice for this client+period
	existing, sevdeskLoggedIn := alreadyInvoiced(ctx, cfg, report)
	if existing != nil {
		return withNumber(log, report.PeriodFrom, existing), nil
	}

	logger.Println("Creating sevDesk invoice...")
	if err := createInvoice(ctx, cfg.SevDesk, report, sevdeskLoggedIn); err != nil {
		return log, fmt.Errorf("creating sevDesk invoice: %w", err)
	}
	// The new draft's number, from the list, as the guard reads it.
	created, err := findInvoice(ctx, report.ClientName, report.PeriodFrom, report.PeriodTo)
	if err != nil {
		logger.Printf("The new draft's number not found: %v", err)
	}
	return withNumber(log, report.PeriodFrom, created), nil
}

// withNumber is the work log named after the invoice it goes with: "11_2026_Rechnung_0203_WorkLog.pdf".
// Without an invoice, or a number on it, it keeps its name.
func withNumber(log *pdfdoc.Document, from time.Time, invoice *invoiceRow) *pdfdoc.Document {
	number := ""
	if invoice != nil {
		number = invoice.number()
	}
	if number == "" {
		logger.Printf("Work log stays %s: no invoice number found in sevDesk's list", log.Filename)
		return log
	}
	named := *log
	named.Filename = WorkLogName(from, number)
	logger.Printf("Work log named after invoice %s: %s", number, named.Filename)
	return &named
}

// WorkLogOnly makes the month's work log, and nothing in sevDesk.
func WorkLogOnly(cfg Config) (*pdfdoc.Document, error) {
	_, log, err := monthReport(cfg)
	return log, err
}

// monthReport is the month's report from the Harvest API — the client's hours in it — and its
// work log. Before the sevDesk check, which is slower, so that an empty month stops early.
func monthReport(cfg Config) (*ReportData, *pdfdoc.Document, error) {
	if cfg.Harvest.Token == "" {
		return nil, nil, fmt.Errorf("no Harvest API token: set harvest.token")
	}
	month := cfg.Month
	if month.IsZero() {
		month = time.Now()
	}
	from := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)
	report := &ReportData{PeriodFrom: from, PeriodTo: from.AddDate(0, 1, -1), ClientName: cfg.SevDesk.ClientName}
	client := cfg.Harvest.Client
	if client == "" {
		client = cfg.SevDesk.ClientName
	}
	if client == "" {
		return nil, nil, fmt.Errorf("no client: set harvest.client or sevdesk.clientName")
	}
	if report.ClientName == "" {
		report.ClientName = client
	}
	who := ""
	if cfg.Harvest.Person != "" {
		who = ", " + cfg.Harvest.Person + "'s"
	}
	logger.Printf("Reading the time entries of %s%s for %s from the Harvest API...", client, who, from.Format("01/2006"))
	entries, err := APIEntries(cfg.Harvest, client, cfg.Harvest.Person, report.PeriodFrom, report.PeriodTo)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		report.TotalHours += e.Hours
	}
	logger.Printf("Period: %s – %s, %d time entries, %.2f hours", report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"), len(entries), report.TotalHours)

	company, err := APICompany(cfg.Harvest)
	if err != nil {
		return nil, nil, err
	}
	data, err := WorkLog(company, report.PeriodFrom, report.PeriodTo, entries)
	if err != nil {
		return nil, nil, fmt.Errorf("making the work log: %w", err)
	}
	log := &pdfdoc.Document{Filename: WorkLogName(from, ""), Data: data}
	logger.Printf("Work log: %s, %d KB", log.Filename, (len(data)+1023)/1024)
	return report, log, nil
}

// ErrAlreadyInvoiced is a report sevDesk already has an invoice for: same client, a Rechnungsdatum
// within its period.
var ErrAlreadyInvoiced = errors.New("sevDesk already has an invoice for this client and period")

// CreateDraft creates a sevDesk draft invoice from a report given directly — an uploaded Harvest
// PDF, its hours and period checked by hand — rather than found in the inbox. With SkipExisting
// it refuses a client and period sevDesk already has an invoice for.
func CreateDraft(cfg Config, report *ReportData) error {
	if cfg.SevDesk.ClientName != "" {
		report.ClientName = cfg.SevDesk.ClientName
	}
	if report.ClientName == "" {
		return errors.New("no client: set sevdesk.clientName in the settings")
	}
	if report.TotalHours <= 0 || report.PeriodFrom.IsZero() || report.PeriodTo.Before(report.PeriodFrom) {
		return fmt.Errorf("need hours and a period: %.2f h, %s – %s", report.TotalHours,
			report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"))
	}
	logger.Printf("Draft for %s: %.2f h, %s – %s", report.ClientName, report.TotalHours,
		report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"))

	ctx, cancel := browser.NewContext(browser.WithGermanLocale())
	defer cancel()
	existing, loggedIn := alreadyInvoiced(ctx, cfg, report)
	if existing != nil {
		return ErrAlreadyInvoiced
	}
	logger.Println("Creating sevDesk invoice...")
	if err := createInvoice(ctx, cfg.SevDesk, report, loggedIn); err != nil {
		return fmt.Errorf("creating sevDesk invoice: %w", err)
	}
	return nil
}

// alreadyInvoiced checks sevDesk for an invoice of the report's client and period, if
// SkipExisting asks for it: the one there is, if any, and whether that left the browser logged in.
// A failed check does not stop the run.
func alreadyInvoiced(ctx context.Context, cfg Config, report *ReportData) (existing *invoiceRow, loggedIn bool) {
	if !cfg.SkipExisting {
		return nil, false
	}
	logger.Println("Checking sevDesk for existing invoice...")
	existing, err := checkInvoiceExists(ctx, cfg.SevDesk, report.ClientName, report.PeriodFrom, report.PeriodTo)
	if err != nil {
		logger.Printf("WARNING: duplicate check failed: %v (continuing anyway)", err)
		return nil, true
	}
	if existing != nil {
		logger.Printf("Skipping: sevDesk already has an invoice for %s (%s – %s)",
			report.ClientName, report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"))
	}
	return existing, true
}
