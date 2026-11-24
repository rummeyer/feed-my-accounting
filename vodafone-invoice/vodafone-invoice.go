// Package vodafone downloads Vodafone invoices (Mobilfunk/Kabel) via headless Chrome
// and sends them as PDF attachments via SMTP.
package vodafone

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"feed-my-accounting/browser"
	"feed-my-accounting/email"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var logger = log.New(os.Stderr, "[vodafone-invoice] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	Mail                email.MailConfig
	Username            string // Vodafone portal username
	Password            string // Vodafone portal password
	FallbackToLastMonth bool   // if false, skip sending when current month invoice not yet available
	// Live asks someone to log in by hand — a captcha, say, or no password stored — in the web
	// interface's live view. Nil, as on the command line: such a login fails.
	Live *browser.Live
}

// waitForHand is how long a run waits for someone to log in by hand in the live view.
const waitForHand = 10 * time.Minute

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

// contractTypes are searched in this order: Mobilfunk first, then Kabel — the same every run.
var contractTypes = []struct{ key, name string }{
	{"mobilfunk", "Mobilfunk"},
	{"kabel", "Kabel"},
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

// Pre-compiled regexes for invoice text parsing.
var (
	invoicePatterns = []*regexp.Regexp{
		regexp.MustCompile(`Rechnung (\p{L}+) (\d{4})`),
		regexp.MustCompile(`Rechnungsdatum[:\s]+\d+\.\s*(\p{L}+)\s+(\d{4})`),
	}
	archiveEntryPattern = regexp.MustCompile(
		`(Januar|Februar|März|April|Mai|Juni|Juli|August|September|Oktober|November|Dezember)\s+\d{2}\.\d{2}\.(\d{4})`,
	)
)

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

// Run downloads Vodafone invoices and sends them via email.
func Run(cfg Config) error {
	attachments, err := Collect(cfg)
	if err != nil {
		return err
	}
	return Send(cfg, attachments)
}

// Send mails the invoices, if there are any.
func Send(cfg Config, attachments []email.Attachment) error {
	return email.SendPDFs(cfg.Mail, "Vodafone", attachments)
}

// Collect downloads the current Vodafone invoices — MeinVodafone has the current month's, or with
// FallbackToLastMonth the last one — without sending them.
func Collect(cfg Config) ([]email.Attachment, error) {
	// A fresh browser each run, nothing kept: a profile kept carried Vodafone's bot check's
	// cookies from run to run, and logins it had taken for a robot with them (17.11.2026).
	ctx, cancel := browser.NewContext(browser.WithGermanLocale(), browser.WithWindowSize(1000, 760), browser.WithTimeout(10*time.Minute+waitForHand))
	defer cancel()

	logger.Println("Logging in to Vodafone...")
	if err := login(ctx, cfg); err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	now := time.Now()
	targetMonth := fmt.Sprintf("%s %d", monthNames[now.Month()], now.Year())
	logger.Printf("Looking for Vodafone invoices: %s", targetMonth)

	var results []invoice
	for _, ct := range contractTypes {
		logger.Printf("Searching %s...", ct.name)
		if inv := downloadInvoice(ctx, ct.key, ct.name, cfg.FallbackToLastMonth); inv != nil {
			results = append(results, *inv)
		}
	}

	if len(results) == 0 {
		logger.Println("No Vodafone invoices found")
		return nil, nil
	}

	attachments := make([]email.Attachment, 0, len(results))
	for _, inv := range results {
		if len(inv.PDFData) > 0 {
			// One invoice a month per contract, as its name says.
			attachments = append(attachments, email.Attachment{Filename: inv.Filename, Data: inv.PDFData, Key: inv.Filename})
		}
	}
	return attachments, nil
}

// ---------------------------------------------------------------------------
// Browser
// ---------------------------------------------------------------------------

func login(ctx context.Context, cfg Config) error {
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(`
			Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
		`).Do(ctx)
		return err
	})); err != nil {
		return fmt.Errorf("injecting script: %w", err)
	}

	// The login page itself, no detour by the services and their redirect by script: what
	// MeinVodafone shows there is what the run goes by — the form, its security code at once,
	// or the services it sends on to with the cookies of a login that stayed.
	logger.Println("Opening MeinVodafone...")
	if err := chromedp.Run(ctx, chromedp.Navigate("https://www.vodafone.de/meinvodafone/account/login")); err != nil {
		return fmt.Errorf("opening the login page: %w", err)
	}
	logger.Println("Waiting for login form...")
	switch loginPage(ctx, 30*time.Second) {
	case "in":
		logger.Println("Still logged in")
		return nil
	case "captcha":
		return byHand(ctx, cfg.Live, "captcha")
	case "unknown":
		return byHand(ctx, cfg.Live, "no-form")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return byHand(ctx, cfg.Live, "no-password")
	}

	// The cookie banner, if it is there, answered as a person would after a look at the page.
	if err := browser.Pause(ctx, 2*time.Second, 4*time.Second); err != nil {
		return err
	}
	chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('#dip-consent-summary-reject-all')?.click()`, nil))
	if err := browser.Pause(ctx, 1500*time.Millisecond, 3*time.Second); err != nil {
		return err
	}

	logger.Println("Submitting credentials...")
	for _, f := range []struct{ sel, value, name string }{
		{"#username-text", cfg.Username, "username"},
		{"#passwordField-input", cfg.Password, "password"},
	} {
		if err := typeInto(ctx, f.sel, f.value, f.name); err != nil {
			// Not sent: what is in the field is not what is stored. Typed again or set by a
			// script, a second try is what a bot check counts: for someone to look instead.
			logger.Printf("%v", err)
			return byHand(ctx, cfg.Live, "asks")
		}
		if err := browser.Pause(ctx, 600*time.Millisecond, 1500*time.Millisecond); err != nil {
			return err
		}
	}
	if err := browser.Pause(ctx, 800*time.Millisecond, 2*time.Second); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#submit`, chromedp.ByID)); err != nil {
		return err
	}

	// Logged in once MeinVodafone leaves its login page; still there, it says why. Up to 20 s:
	// the redirect after a good login takes a few.
	var url, text string
	for i := 0; i < 20; i++ {
		time.Sleep(time.Second)
		if err := chromedp.Run(ctx, chromedp.Location(&url), chromedp.Text(`body`, &text, chromedp.ByQuery)); err != nil {
			return err
		}
		err := loginOutcome(url, text)
		if err == errCaptcha {
			return byHand(ctx, cfg.Live, "captcha")
		}
		if err == nil {
			logger.Println("Logged in")
			return nil
		}
		if err != errStillOnLogin {
			// Turned down — the password, or this address flagged after wrong tries, which
			// MeinVodafone answers alike (14.11.2026): someone to look and log in by hand,
			// where there is a live view; else the reason.
			if cfg.Live != nil {
				logger.Printf("%v", err)
				return byHand(ctx, cfg.Live, "refused")
			}
			return err
		}
	}
	// Neither in nor told why: a captcha, say, or a question not known yet.
	return byHand(ctx, cfg.Live, "asks")
}

// typeInto types value into the field sel as a person would, emptied first, and checks it holds
// just that: a field the page filled already would have it appended. Not set by a script when
// it does not: an error, and nothing sent. The log says only lengths, never the value.
func typeInto(ctx context.Context, sel, value, name string) error {
	js, _ := json.Marshal(sel)
	clear := fmt.Sprintf(`(() => { const e = document.querySelector(%s); if (e && e.value) { e.value = ''; e.dispatchEvent(new Event('input', {bubbles: true})); } })()`, js)
	if err := chromedp.Run(ctx, chromedp.Evaluate(clear, nil)); err != nil {
		return err
	}
	if err := browser.TypeLikeAPerson(ctx, sel, value); err != nil {
		return err
	}
	var got int
	length := fmt.Sprintf(`(document.querySelector(%s)?.value || '').length`, js)
	if err := chromedp.Run(ctx, chromedp.Evaluate(length, &got)); err != nil {
		return err
	}
	if want := len([]rune(value)); got != want {
		return fmt.Errorf("%s field holds %d of %d chars, not sent", name, got, want)
	}
	return nil
}

// loginPage looks at the login page every second, up to d, for what it shows: "form", the login
// form; "captcha", its security code; "in", the page left for MeinVodafone itself, logged in
// from before; "unknown", none of these by then. A plain look, which a page between two loads
// cannot hold up as chromedp's waits for an element can.
func loginPage(ctx context.Context, d time.Duration) string {
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(time.Second) {
		var url, text string
		var form bool
		if err := chromedp.Run(ctx, chromedp.Location(&url), chromedp.Text(`body`, &text, chromedp.ByQuery),
			chromedp.Evaluate(`(() => { const e = document.querySelector('#username-text'); return !!e && e.offsetParent !== null; })()`, &form)); err != nil {
			continue
		}
		switch out := loginOutcome(url, text); {
		case out == errCaptcha:
			return "captcha"
		case form:
			return "form"
		case out == nil && strings.Contains(url, "/meinvodafone/"):
			return "in"
		}
	}
	return "unknown"
}

// handReasons are why a login is handed over, for the log; the live view says them in the
// page's language, by their keys.
var handReasons = map[string]string{
	"no-password": "No password stored",
	"no-form":     "No login form",
	"asks":        "Page unknown",
	"captcha":     "Security code asked",
	"refused":     "Login refused",
}

// byHand asks someone to log in in the live view, and waits until MeinVodafone has let them in
// — wrong tries along the way included, as they type — or until they cancel, or for
// waitForHand. Without a live view, the reason is the error.
func byHand(ctx context.Context, live *browser.Live, why string) error {
	reason := handReasons[why]
	if live == nil {
		return fmt.Errorf("%s, log in via web interface (%w)", strings.ToLower(reason), browser.ErrNoLiveView)
	}
	// The cookie banner out of the way first: it covers the login form.
	chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('#dip-consent-summary-reject-all')?.click()`, nil))
	logger.Printf("%s - waiting for live view", reason)
	cancelled := live.Open(ctx, why)
	defer live.Close()
	deadline := time.After(waitForHand)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-cancelled:
			return browser.ErrGivenUp
		case <-deadline:
			return fmt.Errorf("%w after %d min", browser.ErrNoAnswer, int(waitForHand.Minutes()))
		case <-tick.C:
			var url, text string
			if err := chromedp.Run(ctx, chromedp.Location(&url), chromedp.Text(`body`, &text, chromedp.ByQuery)); err != nil {
				continue // the page between two loads
			}
			if loginOutcome(url, text) == nil {
				logger.Println("Logged in by hand")
				return nil
			}
		}
	}
}

var errStillOnLogin = errors.New("still on the login page")

// errCaptcha is MeinVodafone asking for its security code: for someone to type, in the live view.
var errCaptcha = errors.New("security code required")

// loginOutcome reads the page after the credentials went in: nil once it is no longer the
// login page; the reason MeinVodafone gives when it says one — wrong credentials, or locked
// after too many tries, which it does for the address they came from (14.11.2026);
// errStillOnLogin while it says nothing yet.
func loginOutcome(url, text string) error {
	t := strings.Join(strings.Fields(text), " ")
	// Its security code — a captcha, text in a picture — after wrong tries, on a page of its
	// own, vodafone.de/cprx/captcha (14.11.2026): not logged in. By its address, or its text.
	if strings.Contains(url, "/cprx/captcha") || strings.Contains(t, "Sicherheitscode eingeben") || strings.Contains(t, "Login - Sicherheitscode") {
		return errCaptcha
	}
	if !strings.Contains(url, "/meinvodafone/account/login") {
		return nil
	}
	lower := strings.ToLower(t)
	switch {
	case strings.Contains(lower, "gesperrt") || strings.Contains(lower, "zu viele") || strings.Contains(lower, "zu oft"):
		return errors.New("login blocked, too many tries")
	case strings.Contains(lower, "nicht korrekt") || strings.Contains(lower, "das hat leider nicht geklappt"):
		return errors.New("wrong username or password")
	}
	return errStillOnLogin
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
		logger.Printf("%s: failed to navigate to invoice page: %v", typeName, err)
		return nil
	}

	var pageText string
	if err := chromedp.Run(ctx, chromedp.Text(`body`, &pageText, chromedp.ByQuery)); err != nil {
		logger.Printf("%s: failed to read page text: %v", typeName, err)
		return nil
	}

	now := time.Now()
	currentMonth := fmt.Sprintf("%02d", now.Month())
	currentYear := fmt.Sprintf("%d", now.Year())

	info := parseInvoiceInfo(pageText)
	if info != nil {
		isCurrentMonth := info.Month == currentMonth && info.Year == currentYear
		if isCurrentMonth || fallbackToLastMonth {
			logger.Printf("Downloading %s %s %s...", typeName, info.MonthName, info.Year)
			pdfData, err := capturePDF(ctx, clickCurrentInvoice)
			if err == nil {
				info.Type = typeName
				info.Filename = fmt.Sprintf("%s_%s_Rechnung_Vodafone_%s.pdf", info.Month, info.Year, contractType)
				info.PDFData = pdfData
				return info
			}
			logger.Printf("%s current invoice download failed, trying archive...", typeName)
		} else {
			logger.Printf("%s: no invoice for %s %s yet, skipping", typeName, monthNames[now.Month()], currentYear)
			return nil
		}
	}

	if !fallbackToLastMonth {
		logger.Printf("%s: no current month invoice found, skipping", typeName)
		return nil
	}

	archiveInfo := parseArchiveFirstEntry(pageText, currentMonth, currentYear)
	if archiveInfo == nil {
		logger.Printf("%s: no archive entry found", typeName)
		return nil
	}

	logger.Printf("Downloading %s %s %s from archive...", typeName, archiveInfo.MonthName, archiveInfo.Year)
	pdfData, err := capturePDF(ctx, clickFirstArchiveEntry)
	if err != nil {
		logger.Printf("%s archive download failed!", typeName)
		return nil
	}

	archiveInfo.Type = typeName
	archiveInfo.Filename = fmt.Sprintf("%s_%s_Rechnung_Vodafone_%s.pdf", archiveInfo.Month, archiveInfo.Year, contractType)
	archiveInfo.PDFData = pdfData
	return archiveInfo
}

func navigateToInvoicePage(ctx context.Context, typeName string) error {
	if err := chromedp.Run(ctx, chromedp.Navigate("https://www.vodafone.de/meinvodafone/services/")); err != nil {
		return err
	}
	if err := browser.Pause(ctx, 4*time.Second, 8*time.Second); err != nil {
		return err
	}

	// Click the contract type heading. The name is passed as a JS string
	// argument to avoid format-string injection via typeName.
	contractName := typeName + "-Vertrag"
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`((name) => {
			document.querySelectorAll('h2').forEach(h => {
				if (h.innerText.includes(name)) (h.closest('a') || h.parentElement).click();
			});
		})("`+strings.ReplaceAll(contractName, `"`, `\"`)+`")`, nil),
	); err != nil {
		return err
	}
	if err := browser.Pause(ctx, 4*time.Second, 8*time.Second); err != nil {
		return err
	}

	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			[...document.querySelectorAll('a, button')].find(el =>
				el.innerText.includes('Rechnungen'))?.click();
		`, nil),
	); err != nil {
		return err
	}

	// Poll for invoice page content to finish loading.
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		var hasContent bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`
			document.body.innerText.includes('Aktuelle Rechnung') ||
			document.body.innerText.includes('Deine Rechnungen')
		`, &hasContent)); err != nil {
			return err
		}
		if hasContent {
			return nil
		}
	}
	var url string
	_ = chromedp.Run(ctx, chromedp.Location(&url))
	return fmt.Errorf("invoice page not loaded after 30 s (%s)", url)
}

func capturePDF(ctx context.Context, clickJS string) ([]byte, error) {
	// Intercept PDF blob URLs so we can capture the raw data.
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
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
	`, nil)); err != nil {
		return nil, fmt.Errorf("injecting PDF capture hook: %w", err)
	}

	if err := chromedp.Run(ctx, chromedp.Evaluate(clickJS, nil)); err != nil {
		return nil, fmt.Errorf("clicking download button: %w", err)
	}
	time.Sleep(8 * time.Second)

	var captured []string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`window._capturedPDFs || []`, &captured)); err != nil {
		return nil, fmt.Errorf("reading captured PDFs: %w", err)
	}

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

	for _, matches := range archiveEntryPattern.FindAllStringSubmatch(archiveText, -1) {
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
	for _, re := range invoicePatterns {
		if matches := re.FindStringSubmatch(text); len(matches) >= 3 {
			if month, ok := months[matches[1]]; ok {
				return &invoice{Month: month, Year: matches[2], MonthName: matches[1]}
			}
		}
	}
	return nil
}
