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
}

// Run reads the month's hours of the client from the Harvest API and creates a sevDesk draft
// invoice for them via browser automation — unless sevDesk has one for the month already.
func Run(cfg Config) error {
	if cfg.Harvest.Token == "" {
		return fmt.Errorf("no Harvest API token: set harvest.token")
	}
	report, err := apiReport(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := browser.NewContext(browser.WithGermanLocale())
	defer cancel()

	// Guard: check if sevDesk already has an invoice for this client+period
	exists, sevdeskLoggedIn := alreadyInvoiced(ctx, cfg, report)
	if exists {
		return nil
	}

	logger.Println("Creating sevDesk invoice...")
	if err := createInvoice(ctx, cfg.SevDesk, report, sevdeskLoggedIn); err != nil {
		return fmt.Errorf("creating sevDesk invoice: %w", err)
	}
	return nil
}

// apiReport is the month's report from the Harvest API: the client's hours in it. Before the
// sevDesk check — which is slower — so an empty month stops early.
func apiReport(cfg Config) (*ReportData, error) {
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
		return nil, fmt.Errorf("no client: set harvest.client or sevdesk.clientName")
	}
	if report.ClientName == "" {
		report.ClientName = client
	}
	logger.Printf("Reading the hours of %s for %s from the Harvest API...", client, from.Format("01/2006"))
	hours, entries, err := APIHours(cfg.Harvest, client, report.PeriodFrom, report.PeriodTo)
	if err != nil {
		return nil, err
	}
	report.TotalHours = hours
	logger.Printf("Period: %s – %s, %d time entries, %.2f hours", report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"), entries, hours)
	return report, nil
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
	exists, loggedIn := alreadyInvoiced(ctx, cfg, report)
	if exists {
		return ErrAlreadyInvoiced
	}
	logger.Println("Creating sevDesk invoice...")
	if err := createInvoice(ctx, cfg.SevDesk, report, loggedIn); err != nil {
		return fmt.Errorf("creating sevDesk invoice: %w", err)
	}
	return nil
}

// alreadyInvoiced checks sevDesk for an invoice of the report's client and period, if
// SkipExisting asks for it: whether there is one, and whether that left the browser logged in.
// A failed check does not stop the run.
func alreadyInvoiced(ctx context.Context, cfg Config, report *ReportData) (exists, loggedIn bool) {
	if !cfg.SkipExisting {
		return false, false
	}
	logger.Println("Checking sevDesk for existing invoice...")
	exists, err := checkInvoiceExists(ctx, cfg.SevDesk, report.ClientName, report.PeriodFrom, report.PeriodTo)
	if err != nil {
		logger.Printf("WARNING: duplicate check failed: %v (continuing anyway)", err)
		return false, true
	}
	if exists {
		logger.Printf("Skipping: sevDesk already has an invoice for %s (%s – %s)",
			report.ClientName, report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"))
	}
	return exists, true
}
