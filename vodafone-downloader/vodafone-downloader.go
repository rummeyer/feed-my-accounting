// Package vodafone downloads Vodafone invoices (Mobilfunk/Kabel) via headless Chrome
// and sends them as PDF attachments via SMTP.
package vodafone

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"feed-my-accounting/email"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	SMTP                email.SMTPConfig
	Email               email.EmailConfig
	User                string // Vodafone portal username
	Pass                string // Vodafone portal password
	FallbackToLastMonth bool   // if false, skip sending when current month invoice not yet available
}

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

var contractTypes = map[string]string{
	"mobilfunk": "Mobilfunk",
	"kabel":     "Kabel",
}

var months = map[string]string{
	"Januar": "01", "Februar": "02", "März": "03", "April": "04",
	"Mai": "05", "Juni": "06", "Juli": "07", "August": "08",
	"September": "09", "Oktober": "10", "November": "11", "Dezember": "12",
}

var monthNames = []string{"", "Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember"}

type invoice struct {
	Filename  string
	Month     string
	Year      string
	MonthName string
	Type      string
	PDFData   []byte
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

// Run downloads Vodafone invoices and sends them via email.
func Run(cfg Config) error {
	ctx, cancel := createBrowserContext()
	defer cancel()

	log.Println("Logging in to Vodafone...")
	if err := login(ctx, cfg.User, cfg.Pass); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	now := time.Now()
	targetMonth := fmt.Sprintf("%s %d", monthNames[now.Month()], now.Year())
	log.Printf("Looking for Vodafone invoices: %s", targetMonth)

	var results []invoice
	for contractType, typeName := range contractTypes {
		log.Printf("Searching %s...", typeName)
		if inv := downloadInvoice(ctx, contractType, typeName, cfg.FallbackToLastMonth); inv != nil {
			results = append(results, *inv)
		}
	}

	if len(results) == 0 {
		log.Println("No Vodafone invoices found")
		return nil
	}

	attachments := make([]email.Attachment, 0, len(results))
	for _, inv := range results {
		if len(inv.PDFData) > 0 {
			attachments = append(attachments, email.Attachment{Filename: inv.Filename, Data: inv.PDFData})
		}
	}

	log.Printf("Sending email with %d Vodafone invoice(s)...", len(attachments))
	return email.Send(cfg.SMTP, cfg.Email, "Deine PDF-Rechnungen von Vodafone", attachments...)
}

// ---------------------------------------------------------------------------
// Browser
// ---------------------------------------------------------------------------

func createBrowserContext() (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	ctx, timeoutCancel := context.WithTimeout(ctx, 5*time.Minute)

	return ctx, func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}
}

func login(ctx context.Context, user, pass string) error {
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(`
			Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
		`).Do(ctx)
		return err
	})); err != nil {
		return fmt.Errorf("injecting script: %w", err)
	}

	log.Println("Navigating to Vodafone login page...")
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.vodafone.de/meinvodafone/account/login"),
	); err != nil {
		return fmt.Errorf("navigating to login page: %w", err)
	}

	log.Println("Waiting for login form...")
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`#username-text`, chromedp.ByID),
	); err != nil {
		return fmt.Errorf("login form not found: %w", err)
	}

	chromedp.Run(ctx, chromedp.Click(`#dip-consent-summary-reject-all`, chromedp.ByID))
	time.Sleep(time.Second)

	log.Println("Submitting credentials...")
	return chromedp.Run(ctx,
		chromedp.SendKeys(`#username-text`, user, chromedp.ByID),
		chromedp.SendKeys(`#passwordField-input`, pass, chromedp.ByID),
		chromedp.Click(`#submit`, chromedp.ByID),
		chromedp.Sleep(5*time.Second),
	)
}

// ---------------------------------------------------------------------------
// Invoice downloading
// ---------------------------------------------------------------------------

const clickCurrentInvoice = `(() => {
	const btn = [...document.querySelectorAll('button')].find(btn =>
		btn.innerText.includes('Rechnung herunterladen') ||
		(btn.innerText.includes('Rechnung') && btn.classList.contains('ws10-button--primary')));
	if (btn) {
		btn.disabled = false;
		btn.classList.remove('ws10-button--disabled', 'disabled');
		btn.removeAttribute('aria-disabled');
		btn.click();
	}
})()`

const clickFirstArchiveEntry = `(() => {
	const links = [...document.querySelectorAll('button, a')].filter(b =>
		b.innerText.trim() === 'Rechnung (PDF)' &&
		b.classList.contains('ws10-button-link'));
	if (links.length > 0) links[0].click();
})()`

func downloadInvoice(ctx context.Context, contractType, typeName string, fallbackToLastMonth bool) *invoice {
	if err := navigateToInvoicePage(ctx, typeName); err != nil {
		return nil
	}

	var pageText string
	chromedp.Run(ctx, chromedp.Text(`body`, &pageText, chromedp.ByQuery))

	now := time.Now()
	currentMonth := fmt.Sprintf("%02d", now.Month())
	currentYear := fmt.Sprintf("%d", now.Year())

	info := parseInvoiceInfo(pageText)
	if info != nil {
		isCurrentMonth := info.Month == currentMonth && info.Year == currentYear
		if isCurrentMonth || fallbackToLastMonth {
			log.Printf("Downloading %s %s %s...", typeName, info.MonthName, info.Year)
			pdfData, err := capturePDF(ctx, clickCurrentInvoice)
			if err == nil {
				info.Type = typeName
				info.Filename = fmt.Sprintf("%s_%s_Rechnung_Vodafone_%s.pdf", info.Month, info.Year, contractTypes[contractType])
				info.PDFData = pdfData
				return info
			}
			log.Printf("%s current invoice download failed, trying archive...", typeName)
		} else {
			log.Printf("%s: no invoice for %s %s yet, skipping", typeName, monthNames[now.Month()], currentYear)
			return nil
		}
	}

	if !fallbackToLastMonth {
		log.Printf("%s: no current month invoice found, skipping", typeName)
		return nil
	}

	archiveInfo := parseArchiveFirstEntry(pageText, currentMonth, currentYear)
	if archiveInfo == nil {
		log.Printf("%s: no archive entry found", typeName)
		return nil
	}

	log.Printf("Downloading %s %s %s from archive...", typeName, archiveInfo.MonthName, archiveInfo.Year)
	pdfData, err := capturePDF(ctx, clickFirstArchiveEntry)
	if err != nil {
		log.Printf("%s archive download failed!", typeName)
		return nil
	}

	archiveInfo.Type = typeName
	archiveInfo.Filename = fmt.Sprintf("%s_%s_Rechnung_Vodafone_%s.pdf", archiveInfo.Month, archiveInfo.Year, contractTypes[contractType])
	archiveInfo.PDFData = pdfData
	return archiveInfo
}

func navigateToInvoicePage(ctx context.Context, typeName string) error {
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.vodafone.de/meinvodafone/services/"),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return err
	}

	contractName := typeName + "-Vertrag"
	chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			document.querySelectorAll('h2').forEach(h => {
				if (h.innerText.includes('%s')) (h.closest('a') || h.parentElement).click();
			});
		`, contractName), nil),
		chromedp.Sleep(3*time.Second),
	)

	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			[...document.querySelectorAll('a, button')].find(el =>
				el.innerText.includes('Rechnungen'))?.click();
		`, nil),
	); err != nil {
		return err
	}

	for i := 0; i < 15; i++ {
		time.Sleep(time.Second)
		var hasContent bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			document.body.innerText.includes('Aktuelle Rechnung') ||
			document.body.innerText.includes('Deine Rechnungen')
		`, &hasContent))
		if hasContent {
			return nil
		}
	}
	return nil
}

func capturePDF(ctx context.Context, clickJS string) ([]byte, error) {
	chromedp.Run(ctx, chromedp.Evaluate(`
		window._capturedPDFs = [];
		if (!window._origCreateObjectURL) window._origCreateObjectURL = URL.createObjectURL;
		URL.createObjectURL = function(blob) {
			if (blob?.type === 'application/pdf') {
				const reader = new FileReader();
				reader.onload = () => window._capturedPDFs.push(reader.result);
				reader.readAsDataURL(blob);
			}
			return window._origCreateObjectURL.call(URL, blob);
		};
	`, nil))

	chromedp.Run(ctx, chromedp.Evaluate(clickJS, nil))
	time.Sleep(5 * time.Second)

	var captured []string
	chromedp.Run(ctx, chromedp.Evaluate(`window._capturedPDFs || []`, &captured))

	if len(captured) == 0 {
		return nil, fmt.Errorf("no PDF captured")
	}

	pdfBase64 := strings.TrimPrefix(captured[0], "data:application/pdf;base64,")
	return base64.StdEncoding.DecodeString(pdfBase64)
}

// parseArchiveFirstEntry finds the first archive entry after the "Rechnungsarchiv"
// section heading, skipping any entry that matches skipMonth/skipYear (the current
// month, which should be downloaded via the primary button instead).
func parseArchiveFirstEntry(text, skipMonth, skipYear string) *invoice {
	idx := strings.Index(text, "Rechnungsarchiv")
	if idx == -1 {
		return nil
	}
	archiveText := text[idx:]

	allMonths := "Januar|Februar|März|April|Mai|Juni|Juli|August|September|Oktober|November|Dezember"
	pattern := regexp.MustCompile(`(` + allMonths + `)\s+\d{2}\.\d{2}\.(\d{4})`)
	for _, matches := range pattern.FindAllStringSubmatch(archiveText, -1) {
		if len(matches) < 3 {
			continue
		}
		monthName := matches[1]
		year := matches[2]
		month, ok := months[monthName]
		if !ok {
			continue
		}
		if month == skipMonth && year == skipYear {
			continue
		}
		return &invoice{Month: month, Year: year, MonthName: monthName}
	}
	return nil
}

func parseInvoiceInfo(text string) *invoice {
	patterns := []string{
		`Rechnung (\p{L}+) (\d{4})`,
		`Rechnungsdatum[:\s]+\d+\.\s*(\p{L}+)\s+(\d{4})`,
	}
	for _, pattern := range patterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(text); len(matches) >= 3 {
			if month, ok := months[matches[1]]; ok {
				return &invoice{Month: month, Year: matches[2], MonthName: matches[1]}
			}
		}
	}
	return nil
}
