package harvest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"feed-my-accounting/browser"
	"feed-my-accounting/email"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

var logger = log.New(os.Stderr, "[harvest-invoice] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// Config holds all configuration for the harvest-invoice module.
type Config struct {
	Mail             email.MailConfig
	CurrentMonthOnly bool
	SkipExisting     bool
	Harvest          HarvestLogin  `yaml:"harvest"`
	SevDesk          SevDeskConfig `yaml:"sevdesk"`
	Filter           FilterConfig  `yaml:"filter"`
}

// HarvestLogin holds credentials for Harvest: a personal access token for the API, which is
// used when set; else the web login, which downloads the report PDF.
type HarvestLogin struct {
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Token     string `yaml:"token"`
	AccountID string `yaml:"accountId"` // empty: the token's Harvest account
}

// FilterConfig defines which Harvest emails to look for.
type FilterConfig struct {
	Count   int    `yaml:"count"`
	Subject string `yaml:"subject"`
	From    string `yaml:"from"`
}

// Run fetches Harvest report emails, downloads the PDF, parses hours and period,
// and creates a sevDesk invoice via browser automation.
func Run(cfg Config) error {
	_, err := RunReport(cfg)
	return err
}

// RunReport is Run, handing back the report it worked with — with the downloaded PDF, once
// there is one, even if the invoice then fails: the web interface offers it for download. Nil
// if no report was found, or one was skipped before downloading.
func RunReport(cfg Config) (*ReportData, error) {
	logger.Println("Fetching Harvest report emails...")
	messages, err := email.FetchHTMLEmails(cfg.Mail, email.IMAPFilter{
		Count:      cfg.Filter.Count,
		Subject:    cfg.Filter.Subject,
		FromDomain: cfg.Filter.From,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching emails: %w", err)
	}

	if len(messages) == 0 {
		logger.Println("No Harvest report emails found")
		return nil, nil
	}

	// Find the most recent export email (skip password change etc.)
	var report *ReportData
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		logger.Printf("Checking: %q (%s)", msg.Subject, msg.Date.Format("2006-01-02"))
		r, err := ParseEmail(msg.HTMLBody)
		if err != nil {
			continue // not an export email
		}
		report = r
		logger.Printf("Found export: %q", msg.Subject)
		break
	}
	if report == nil {
		logger.Println("No Harvest export emails found (checked all matches)")
		return nil, nil
	}

	// Use fixed client name from config if set, otherwise use the one from the email
	harvestClient := report.ClientName
	if cfg.SevDesk.ClientName != "" {
		report.ClientName = cfg.SevDesk.ClientName
	}

	logger.Printf("Export URL: %s", report.ExportURL)
	logger.Printf("Period: %s – %s", report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"))
	if report.ClientName != "" {
		logger.Printf("Client: %s", report.ClientName)
	}

	// Guard: only process reports for the current month
	if cfg.CurrentMonthOnly {
		now := time.Now()
		if report.PeriodFrom.Year() != now.Year() || report.PeriodFrom.Month() != now.Month() {
			logger.Printf("Skipping: report period %s is not current month (%s)",
				report.PeriodFrom.Format("01/2006"), now.Format("01/2006"))
			return nil, nil
		}
	}

	// Create browser context (shared for sevDesk check, Harvest download, and invoice creation)
	ctx, cancel := browser.NewContext(browser.WithGermanLocale())
	defer cancel()

	// Guard: check if sevDesk already has an invoice for this client+period
	exists, sevdeskLoggedIn := alreadyInvoiced(ctx, cfg, report)
	if exists {
		return nil, nil
	}

	if cfg.Harvest.Token != "" {
		logger.Println("Reading the hours from the Harvest API...")
		hours, entries, err := APIHours(cfg.Harvest, harvestClient, report.PeriodFrom, report.PeriodTo)
		if err != nil {
			return nil, err
		}
		report.TotalHours = hours
		logger.Printf("%d time entries", entries)
	} else {
		logger.Println("Downloading the report PDF from Harvest...")
		pdfData, err := downloadExportPDF(cfg.Harvest, report.ExportURL)
		if err != nil {
			return nil, fmt.Errorf("downloading PDF: %w", err)
		}
		report.PDFData = pdfData
		logger.Printf("Downloaded PDF: %d bytes", len(pdfData))

		totalHours, err := ParsePDFHours(pdfData)
		if err != nil {
			return report, fmt.Errorf("parsing PDF hours: %w", err)
		}
		report.TotalHours = totalHours
	}

	logger.Printf("Total hours: %.2f", report.TotalHours)

	// Create sevDesk invoice via browser (reuse the same browser context)
	logger.Println("Creating sevDesk invoice...")
	if err := createInvoice(ctx, cfg.SevDesk, report, sevdeskLoggedIn); err != nil {
		return report, fmt.Errorf("creating sevDesk invoice: %w", err)
	}

	return report, nil
}

// harvestPageState says what the browser shows when the download does not come: still the sign
// in page, a captcha asked for — Harvest's sign in has an invisible hCaptcha since 2026, which
// may turn into a challenge for an automated browser — or an error Harvest shows. Never what was
// typed.
func harvestPageState(ctx context.Context) string {
	var state struct {
		Page    string `json:"page"`
		Captcha bool   `json:"captcha"`
		Message string `json:"message"`
	}
	err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const frames = [...document.querySelectorAll('iframe[src*="hcaptcha"], iframe[src*="recaptcha"]')];
		const challenge = frames.some((f) => { const r = f.getBoundingClientRect(); return r.width > 100 && r.height > 100; });
		const alert = document.querySelector('[role="alert"], .alert, .error, .flash, .form-error');
		return { page: location.host + location.pathname, captcha: challenge, message: alert ? alert.innerText.trim().slice(0, 200) : '' };
	})()`, &state))
	if err != nil {
		return "page state unknown: " + err.Error()
	}
	parts := []string{"on " + state.Page}
	if state.Captcha {
		parts = append(parts, "Harvest asks for a captcha: upload the report PDF by hand instead")
	}
	if state.Message != "" {
		parts = append(parts, fmt.Sprintf("Harvest says %q", state.Message))
	}
	return strings.Join(parts, "; ")
}

// FileName is the report PDF's name for saving it: "Harvest_2026-11_Acme_GmbH.pdf".
func (r *ReportData) FileName() string {
	name := strings.Map(func(c rune) rune {
		if c == ' ' || c == '/' || c == '\\' || c == ':' {
			return '_'
		}
		return c
	}, strings.TrimSpace(r.ClientName))
	if name != "" {
		name = "_" + name
	}
	return fmt.Sprintf("Harvest_%s%s.pdf", r.PeriodFrom.Format("2006-01"), name)
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

// downloadExportPDF logs into Harvest via headless Chrome and downloads the
// PDF export file. It navigates to the export URL (which redirects to the
// Harvest login), submits credentials, and waits for the file download to
// complete in a temporary directory.
func downloadExportPDF(creds HarvestLogin, exportURL string) ([]byte, error) {
	// A browser of its own, which is what it says it is and remembers the Harvest login:
	// Harvest's sign in has an hCaptcha that turned the Mac disguise away (11.11.2026), and
	// a remembered login needs no sign in, so no captcha, at all.
	var opts = []browser.Option{browser.WithGermanLocale(), browser.WithNativeUserAgent()}
	if dir, err := os.UserCacheDir(); err == nil {
		opts = append(opts, browser.WithProfile(filepath.Join(dir, "feed-my-accounting", "harvest-browser")))
	}
	ctx, cancel := browser.NewContext(opts...)
	defer cancel()

	downloadDir, err := os.MkdirTemp("", "harvest-export-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(downloadDir)

	// Set download behavior to save to our directory
	if err := chromedp.Run(ctx,
		cdpbrowser.SetDownloadBehavior(cdpbrowser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(downloadDir).
			WithEventsEnabled(true),
	); err != nil {
		return nil, fmt.Errorf("setting download behavior: %w", err)
	}

	// The export URL downloads the PDF straight away when the login is remembered; else it
	// redirects to Harvest's sign in, and the download follows the sign in.
	if err := chromedp.Run(ctx, chromedp.Navigate(exportURL)); err != nil && !strings.Contains(err.Error(), "net::ERR_ABORTED") {
		return nil, fmt.Errorf("navigating to export: %w", err)
	}

	var downloadedFile string
	signedIn := false
	for i := 0; i < 60 && downloadedFile == ""; i++ {
		time.Sleep(time.Second)
		entries, _ := os.ReadDir(downloadDir)
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) != ".crdownload" {
				downloadedFile = filepath.Join(downloadDir, e.Name())
			}
		}
		if downloadedFile != "" || signedIn {
			continue
		}
		var form bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => { const e = document.querySelector('#email'); return !!e && e.offsetParent !== null; })()`, &form)); err != nil || !form {
			continue
		}
		logger.Println("Signing in to Harvest...")
		if err := chromedp.Run(ctx,
			chromedp.Clear(`#email`, chromedp.ByID),
			chromedp.SendKeys(`#email`, creds.Username, chromedp.ByID),
			chromedp.Clear(`#password`, chromedp.ByID),
			chromedp.SendKeys(`#password`, creds.Password, chromedp.ByID),
			chromedp.Sleep(500*time.Millisecond),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		); err != nil {
			return nil, fmt.Errorf("Harvest login: %w", err)
		}
		signedIn = true
	}

	if downloadedFile == "" {
		return nil, fmt.Errorf("PDF download did not complete within a minute: %s", harvestPageState(ctx))
	}

	data, err := os.ReadFile(downloadedFile)
	if err != nil {
		return nil, fmt.Errorf("reading downloaded file: %w", err)
	}

	if len(data) < 100 {
		return nil, fmt.Errorf("downloaded file too small (%d bytes), likely not a PDF", len(data))
	}

	return data, nil
}
