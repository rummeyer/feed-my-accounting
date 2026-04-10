package harvest

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"feed-my-accounting/browser"
	"feed-my-accounting/email"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

// Config holds all configuration for the harvest-invoice module.
type Config struct {
	Mail             email.MailConfig
	CurrentMonthOnly bool
	SkipExisting     bool
	Harvest          HarvestLogin  `yaml:"harvest"`
	SevDesk          SevDeskConfig `yaml:"sevdesk"`
	Filter           FilterConfig  `yaml:"filter"`
}

// HarvestLogin holds credentials for the Harvest web login.
type HarvestLogin struct {
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
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
	log.Println("Fetching Harvest report emails...")
	messages, err := email.FetchHTMLEmails(cfg.Mail, email.IMAPFilter{
		Count:      cfg.Filter.Count,
		FromDomain: cfg.Filter.From,
	})
	if err != nil {
		return fmt.Errorf("fetching emails: %w", err)
	}

	if len(messages) == 0 {
		log.Println("No Harvest report emails found")
		return nil
	}

	// Find the most recent export email (skip password change etc.)
	var report *ReportData
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		log.Printf("Checking: %q (%s)", msg.Subject, msg.Date.Format("2006-01-02"))
		r, err := ParseEmail(msg.HTMLBody)
		if err != nil {
			continue // not an export email
		}
		report = r
		log.Printf("Found export: %q", msg.Subject)
		break
	}
	if report == nil {
		log.Println("No Harvest export emails found (checked all matches)")
		return nil
	}

	log.Printf("Export URL: %s", report.ExportURL)
	log.Printf("Period: %s – %s", report.PeriodFrom.Format("02.01.2006"), report.PeriodTo.Format("02.01.2006"))
	if report.ClientName != "" {
		log.Printf("Client: %s", report.ClientName)
	}

	// Guard: only process reports for the current month
	if cfg.CurrentMonthOnly {
		now := time.Now()
		if report.PeriodFrom.Year() != now.Year() || report.PeriodFrom.Month() != now.Month() {
			log.Printf("Skipping: report period %s is not current month (%s)",
				report.PeriodFrom.Format("01/2006"), now.Format("01/2006"))
			return nil
		}
	}

	// Create browser context (shared for sevDesk check, Harvest download, and invoice creation)
	ctx, cancel := browser.NewContext(browser.WithGermanLocale())
	defer cancel()

	// Guard: check if sevDesk already has an invoice for this customer+period
	sevdeskLoggedIn := false
	if cfg.SkipExisting {
		log.Println("Checking sevDesk for existing invoice...")
		exists, err := checkInvoiceExists(ctx, cfg.SevDesk, report.ClientName, report.PeriodFrom, report.PeriodTo)
		if err != nil {
			log.Printf("WARNING: duplicate check failed: %v (continuing anyway)", err)
		} else if exists {
			log.Printf("Skipping: sevDesk already has an invoice for %s (%s – %s)",
				report.ClientName,
				report.PeriodFrom.Format("02.01.2006"),
				report.PeriodTo.Format("02.01.2006"))
			return nil
		}
		sevdeskLoggedIn = true
	}

	log.Println("Logging into Harvest to download report PDF...")
	pdfData, err := downloadExportPDF(ctx, cfg.Harvest, report.ExportURL)
	if err != nil {
		return fmt.Errorf("downloading PDF: %w", err)
	}
	report.PDFData = pdfData
	log.Printf("Downloaded PDF: %d bytes", len(pdfData))

	totalHours, err := ParsePDFHours(pdfData)
	if err != nil {
		return fmt.Errorf("parsing PDF hours: %w", err)
	}
	report.TotalHours = totalHours

	log.Printf("Total hours: %.2f", report.TotalHours)

	// Create sevDesk invoice via browser (reuse the same browser context)
	log.Println("Creating sevDesk invoice...")
	if err := createInvoice(ctx, cfg.SevDesk, report, sevdeskLoggedIn); err != nil {
		return fmt.Errorf("creating sevDesk invoice: %w", err)
	}

	return nil
}

// downloadExportPDF logs into Harvest via headless Chrome and downloads the
// PDF export file. It navigates to the export URL (which redirects to the
// Harvest login), submits credentials, and waits for the file download to
// complete in a temporary directory.
func downloadExportPDF(ctx context.Context, creds HarvestLogin, exportURL string) ([]byte, error) {
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

	// Navigate to export URL → redirects to Harvest login
	if err := chromedp.Run(ctx,
		chromedp.Navigate(exportURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return nil, fmt.Errorf("navigating to export: %w", err)
	}

	// Login — after submit, the download is triggered automatically
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`#email`, chromedp.ByID),
		chromedp.Clear(`#email`, chromedp.ByID),
		chromedp.SendKeys(`#email`, creds.User, chromedp.ByID),
		chromedp.Clear(`#password`, chromedp.ByID),
		chromedp.SendKeys(`#password`, creds.Pass, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("Harvest login: %w", err)
	}

	// Wait for download to complete
	var downloadedFile string
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		entries, _ := os.ReadDir(downloadDir)
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) != ".crdownload" {
				downloadedFile = filepath.Join(downloadDir, e.Name())
			}
		}
		if downloadedFile != "" {
			break
		}
	}

	if downloadedFile == "" {
		return nil, fmt.Errorf("PDF download did not complete within 30 seconds")
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

