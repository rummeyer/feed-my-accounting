// Package amazon downloads the invoices of an Amazon Business account's orders of a month from
// amazon.de via headless Chrome, and sends them as PDF attachments via SMTP.
package amazon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"feed-my-accounting/browser"
	"feed-my-accounting/email"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

var logger = log.New(os.Stderr, "[amazon-invoice] ", log.LstdFlags)

// SetLogOutput redirects this package's log output to the given writer.
func SetLogOutput(w io.Writer) { logger.SetOutput(w) }

// Config is how to reach the Amazon Business account and where its invoices go.
type Config struct {
	Mail     email.MailConfig
	Username string // the Amazon Business login's email address
	Password string
	// Profile keeps the browser's cookies from run to run, with them Amazon's "Angemeldet
	// bleiben" and the device its one-time code was given on: most runs need no login, and
	// so no code. Empty: a fresh browser each time.
	Profile string
	// Live asks someone to type the one-time code, or to log in by hand, in the web
	// interface's live view. Nil, as on the command line: such a login fails.
	Live *browser.Live
	// Ask, without a live view — on the command line, in a terminal — asks someone for the
	// one-time code there. Nil: such a login fails.
	Ask func(prompt string) (string, error)
}

const site = "https://www.amazon.de"

// Run downloads the invoices of the month's orders FetchedMonth names and sends them via email.
func Run(cfg Config) error {
	attachments, err := Collect(cfg, FetchedMonth(time.Now()))
	if err != nil {
		return err
	}
	return Send(cfg, attachments)
}

// FetchedMonth is the month whose orders a run on now fetches: on its last day now's own — the
// books are closed then, most months — else the month before, complete. Once a month only, a
// month half over would never be fetched again.
func FetchedMonth(now time.Time) time.Time {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	if now.AddDate(0, 0, 1).Month() != now.Month() {
		return first
	}
	return first.AddDate(0, -1, 0)
}

// Send mails the invoices, if there are any.
func Send(cfg Config, attachments []email.Attachment) error {
	return email.SendPDFs(cfg.Mail, "Amazon", attachments)
}

// Collect downloads the invoices of the orders placed in month's month, without sending them.
func Collect(cfg Config, month time.Time) ([]email.Attachment, error) {
	opts := []browser.Option{browser.WithGermanLocale(), browser.WithWindowSize(1000, 720), browser.WithTimeout(10*time.Minute + browser.WaitForHand)}
	if cfg.Profile != "" {
		opts = append(opts, browser.WithProfile(cfg.Profile))
	}
	ctx, cancel := browser.NewContext(opts...)
	defer cancel()

	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined});`).Do(ctx)
		return err
	})); err != nil {
		return nil, fmt.Errorf("injecting script: %w", err)
	}

	first, last := monthBounds(month)
	logger.Printf("Looking for Amazon orders of %s", first.Format("01/2006"))
	orders, err := ordersOf(ctx, cfg, first, last)
	if err != nil {
		return nil, err
	}
	logger.Printf("%d orders in %s", len(orders), first.Format("01/2006"))

	var attachments []email.Attachment
	used := map[string]bool{}
	for _, o := range orders {
		links, err := invoiceLinks(ctx, o)
		if err != nil {
			logger.Printf("Order %s: %v", o.ID, err)
			continue
		}
		if len(links) == 0 {
			logger.Printf("Order %s: no invoice yet", o.ID)
			continue
		}
		for i, link := range links {
			data, err := download(ctx, link)
			if err != nil {
				logger.Printf("Order %s: invoice %d: %v", o.ID, i+1, err)
				continue
			}
			attachments = append(attachments, email.Attachment{Filename: fileName(o, used), Data: data, Key: fmt.Sprintf("%s#%d", o.ID, i+1)})
		}
	}
	logger.Printf("%d invoices downloaded", len(attachments))
	return attachments, nil
}

// monthBounds are the first and the last day of t's month.
func monthBounds(t time.Time) (time.Time, time.Time) {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	return first, first.AddDate(0, 1, -1)
}

// fileName names an invoice as the others are named: its month, then the order's day —
// 11_2026_Rechnung_Amazon_17112026.pdf — and _2, _3 after it for more of the same day; used
// are the names given already.
func fileName(o order, used map[string]bool) string {
	base := fmt.Sprintf("%s_Rechnung_Amazon_%s", o.Date.Format("01_2006"), o.Date.Format("02012006"))
	name := base + ".pdf"
	for n := 2; used[name]; n++ {
		name = fmt.Sprintf("%s_%d.pdf", base, n)
	}
	used[name] = true
	return name
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

// login logs in on the sign-in page the browser is on: email, password, and "Angemeldet
// bleiben"; a one-time code, a captcha or a question it does not know, for someone in the
// live view, with "don't ask on this device again" ticked first.
func login(ctx context.Context, cfg Config) error {
	sent := map[string]bool{}
	for step := 0; step < 6; step++ {
		var url, text, field string
		if err := chromedp.Run(ctx, chromedp.Location(&url), chromedp.Text(`body`, &text, chromedp.ByQuery), chromedp.Evaluate(fieldJS, &field)); err != nil {
			return err
		}
		now := signInPage(url, text, field)
		// A field asked for again: Amazon did not take it. Never sent twice — tries that fail get
		// an account locked — but handed over, with what Amazon says and the page kept.
		if (now == "email" || now == "password") && sent[now] {
			var alert string
			chromedp.Run(ctx, chromedp.Evaluate(alertJS, &alert))
			logger.Printf("Asked for the %s again: %q", now, alert)
			dump(ctx, cfg, "login")
			return byHand(ctx, cfg.Live, "amazon-asks")
		}
		if now == "email" || now == "password" {
			sent[now] = true
		}
		switch now {
		case "in":
			logger.Println("Logged in")
			return nil
		case "otp":
			// "In diesem Browser ist kein Code erforderlich": ticked, so later runs need no code —
			// Amazon's own default; found by its name or its label, the id it once had gone.
			var remember string
			chromedp.Run(ctx, chromedp.Evaluate(rememberJS, &remember))
			logger.Printf("Remember this browser: %s", remember)
			if cfg.Live == nil && cfg.Ask != nil {
				return codeAsked(ctx, cfg)
			}
			return byHand(ctx, cfg.Live, "amazon-otp")
		case "captcha":
			return byHand(ctx, cfg.Live, "amazon-captcha")
		case "refused":
			if cfg.Live != nil {
				return byHand(ctx, cfg.Live, "amazon-refused")
			}
			return errors.New("login refused")
		case "email":
			if cfg.Username == "" || cfg.Password == "" {
				return byHand(ctx, cfg.Live, "amazon-no-password")
			}
			logger.Println("Submitting email...")
			if err := typeInto(ctx, `#ap_email, #ap_email_login, input[name=email]`, cfg.Username); err != nil {
				return err
			}
		case "password":
			if cfg.Password == "" {
				return byHand(ctx, cfg.Live, "amazon-no-password")
			}
			// Typed as a person does, once: Amazon emptied the field when a browser posing as a
			// Mac set it at once (17.11.2026). Asked for again, it goes to the live view.
			logger.Println("Submitting password...")
			chromedp.Run(ctx, chromedp.Evaluate(`(() => { const b = document.querySelector('input[name=rememberMe]'); if (b && !b.checked) b.click(); })()`, nil))
			if err := typeInto(ctx, `#ap_password`, cfg.Password); err != nil {
				logger.Printf("%v", err)
				return byHandFill(ctx, cfg.Live, "amazon-password", cfg.Password)
			}
		default:
			dump(ctx, cfg, "login")
			return byHand(ctx, cfg.Live, "amazon-asks")
		}
		time.Sleep(4 * time.Second)
	}
	return byHand(ctx, cfg.Live, "amazon-asks")
}

// rememberJS ticks the code page's "no code required in this browser" box if it is not, and
// says how it found it: "ticked already", "ticked now", or "not found".
const rememberJS = `(() => {
	const boxes = [...document.querySelectorAll('input[type=checkbox]')];
	const box = boxes.find((b) => /remember/i.test(b.name + ' ' + b.id)) ||
		boxes.find((b) => /kein code|nicht erneut|don.t require|remember/i.test((b.closest('label') || b.parentElement || {}).innerText || ''));
	if (!box) return 'not found';
	if (box.checked) return 'ticked already';
	box.click();
	return box.checked ? 'ticked now' : 'not ticked';
})()`

// codeAsked asks for the one-time code in the terminal and types it in as a person would: twice
// at the most, a wrong code more than that what a lock is made of. Logged in once Amazon sets
// its login cookie, its redirect after the code hanging as it may.
func codeAsked(ctx context.Context, cfg Config) error {
	for try := 1; try <= 2; try++ {
		code, err := cfg.Ask("Amazon one-time code: ")
		if err != nil {
			return fmt.Errorf("one-time code: %w", err)
		}
		code = strings.TrimSpace(code)
		if code == "" {
			return fmt.Errorf("no one-time code given (%w)", browser.ErrNoLiveView)
		}
		if err := typeInto(ctx, `#auth-mfa-otpcode, input[name=otpCode]`, code); err != nil {
			return err
		}
		for end := time.Now().Add(30 * time.Second); time.Now().Before(end); {
			if signedIn(ctx) {
				logger.Println("Logged in with the code")
				stopLoading(ctx)
				return nil
			}
			if err := browser.Pause(ctx, 2*time.Second, 2*time.Second); err != nil {
				return err
			}
		}
		logger.Println("Code not taken")
	}
	return errors.New("one-time code not taken")
}

// fieldJS is the sign-in field the page shows: "otp", "password" or "email", else "".
const fieldJS = `(() => {
	const shown = (sel) => [...document.querySelectorAll(sel)].some((e) => e.offsetParent !== null && e.type !== 'hidden');
	if (shown('#auth-mfa-otpcode, input[name=otpCode]')) return 'otp';
	if (shown('#ap_password')) return 'password';
	if (shown('#ap_email, #ap_email_login, input[name=email]')) return 'email';
	return '';
})()`

// signInPage reads where the login stands, by the address, the page's text and the field it
// shows: "in", off Amazon's sign-in pages; "otp", its one-time code; "captcha", characters
// from a picture or a puzzle; "refused", a wrong password; "email" or "password", the field
// it asks for next; "" for anything else.
func signInPage(url, text, field string) string {
	t := strings.ToLower(strings.Join(strings.Fields(text), " "))
	// Its sign-in pages: /ap/, and /ax/ since its new sign-in (17.11.2026: after the email,
	// /ax/claim, taken for logged in); a sign-in field anywhere is one too.
	if field == "" && !strings.Contains(url, "/ap/") && !strings.Contains(url, "/ax/") && !strings.Contains(url, "/errors/validatecaptcha") {
		return "in"
	}
	switch {
	case field == "otp" || strings.Contains(url, "/ap/mfa") || strings.Contains(t, "zweistufige überprüfung") || strings.Contains(t, "two-step verification"):
		return "otp"
	case strings.Contains(url, "validatecaptcha") || strings.Contains(url, "/ap/cvf") || strings.Contains(t, "zeichen eingeben") || strings.Contains(t, "rätsel lösen") || strings.Contains(t, "solve this puzzle"):
		return "captcha"
	case strings.Contains(t, "passwort ist falsch") || strings.Contains(t, "password is incorrect"):
		return "refused"
	}
	return field
}

// alertJS is what Amazon says in its boxes on the sign-in page, a wrong address, say.
const alertJS = `[...document.querySelectorAll('.a-alert-content, #auth-error-message-box, #auth-warning-message-box, .a-form-error')]
	.filter((e) => e.offsetParent !== null).map((e) => e.innerText.trim()).filter(Boolean).join(' | ').slice(0, 300)`

// typeInto types value into the visible field of sel as a person does — clicked into, key by
// key with uneven gaps — checks it holds just that, and sends it with its form's button,
// clicked with the mouse after a moment. Not sent if the field does not hold it. The value is
// never logged.
func typeInto(ctx context.Context, sel, value string) error {
	js, _ := json.Marshal(sel)
	var ok bool
	mark := fmt.Sprintf(`(() => {
		document.querySelectorAll('[data-fma-field], [data-fma-send]').forEach((e) => { e.removeAttribute('data-fma-field'); e.removeAttribute('data-fma-send'); });
		const e = [...document.querySelectorAll(%s)].find((e) => e.offsetParent !== null && e.type !== 'hidden');
		if (!e) return false;
		e.setAttribute('data-fma-field', '1');
		if (e.value) { e.value = ''; e.dispatchEvent(new Event('input', {bubbles: true})); }
		const send = e.form && [...e.form.querySelectorAll('input[type=submit], button[type=submit]')].find((b) => b.offsetParent !== null);
		if (send) send.setAttribute('data-fma-send', '1');
		return true;
	})()`, js)
	if err := chromedp.Run(ctx, chromedp.Evaluate(mark, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("field %s not found", sel)
	}
	if err := browser.TypeChecked(ctx, `[data-fma-field]`, value, ""); err != nil {
		return err
	}
	if err := browser.Pause(ctx, 700*time.Millisecond, 1800*time.Millisecond); err != nil {
		return err
	}
	var send bool
	chromedp.Run(ctx, chromedp.Evaluate(`!!document.querySelector('[data-fma-send]')`, &send))
	if send {
		return chromedp.Run(ctx, chromedp.Click(`[data-fma-send]`, chromedp.ByQuery))
	}
	return chromedp.Run(ctx, chromedp.KeyEvent(kb.Enter))
}

var handReasons = map[string]string{
	"amazon-password":    "Password asked",
	"amazon-otp":         "One-time code asked",
	"amazon-captcha":     "Captcha asked",
	"amazon-refused":     "Login refused",
	"amazon-no-password": "No password stored",
	"amazon-asks":        "Page unknown",
}

// byHand asks someone to go on in the live view, and waits until Amazon has let them in, they
// cancel, or browser.WaitForHand has passed. Without a live view, the reason is the error.
func byHand(ctx context.Context, live *browser.Live, why string) error {
	return byHandFill(ctx, live, why, "")
}

// byHandFill is byHand offering fill, the stored password, at a button in the live view.
func byHandFill(ctx context.Context, live *browser.Live, why, fill string) error {
	seen := ""
	return browser.ByHand(ctx, browser.Hand{
		Live: live, Why: why, Reason: handReasons[why], Fill: fill, Log: logger,
		In: func(ctx context.Context) bool {
			// The login's cookie first, which a page loading does not hold up: after the code,
			// Amazon's redirect hung for three minutes, and with it a look at the page
			// (17.11.2026). Then — the look given a few seconds only — what the page says.
			if signedIn(ctx) {
				stopLoading(ctx)
				return true
			}
			var url, field string
			look, done := context.WithTimeout(ctx, 3*time.Second)
			defer done()
			if err := chromedp.Run(look, chromedp.Location(&url), chromedp.Evaluate(fieldJS, &field)); err != nil {
				return false
			}
			// Each page it comes to, by its path: what Amazon asks after the code, say.
			if path := pathOf(url); path != seen {
				seen = path
				logger.Printf("Live view on %s", path)
			}
			return handedOn(url, field)
		},
	})
}

// signedIn says whether Amazon has given the browser its login, by its cookie: set the moment
// the code is taken, whatever the page shows — after it, Amazon sent the browser to its own
// address on port 8443, which hung for two minutes on the code's page (17.11.2026).
func signedIn(ctx context.Context) bool {
	var cookies []*network.Cookie
	ctx, done := context.WithTimeout(ctx, 3*time.Second)
	defer done()
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = network.GetCookies().WithURLs([]string{site}).Do(ctx)
		return err
	}))
	if err != nil {
		return false
	}
	for _, c := range cookies {
		if c.Name == "at-acbde" || c.Name == "sess-at-acbde" {
			return true
		}
	}
	return false
}

// stopLoading stops a page still loading — Amazon's redirect after the code, hanging — for the
// run to go on to its orders.
func stopLoading(ctx context.Context) {
	ctx, done := context.WithTimeout(ctx, 3*time.Second)
	defer done()
	_ = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error { return page.StopLoading().Do(ctx) }))
}

// pathOf is an address without its query: what the log may say of it.
func pathOf(url string) string {
	if i := strings.IndexAny(url, "?#"); i >= 0 {
		return url[:i]
	}
	return url
}

// signInSteps are the addresses of Amazon's sign-in steps themselves: the email, the password,
// the one-time code, a captcha.
var signInSteps = []string{"/ap/signin", "/ap/mfa", "/ap/cvf", "/ap/challenge", "/ax/claim", "/errors/validatecaptcha"}

// handedOn says whether someone in the live view is through Amazon's sign-in: no sign-in field
// on the page, and not on one of its steps. A page after the code — a passkey offered, a
// notice — is past it (17.11.2026: the live view stayed open on one, the right code taken).
func handedOn(url, field string) bool {
	// A page between two, or Chrome's own error page, is no page past the sign-in (17.11.2026:
	// chrome-error://chromewebdata/ taken for one).
	if field != "" || !strings.HasPrefix(url, "https://") {
		return false
	}
	for _, step := range signInSteps {
		if strings.Contains(url, step) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

// order is an order on the list of the month's orders: its number, its day, and where its
// invoice links are.
type order struct {
	ID   string
	Date time.Time
	// Popover is where the order's invoice links are, as its card says; empty: the usual one.
	Popover string
	// Cancelled is an order its card says is cancelled: no invoice to it, ever.
	Cancelled bool
}

// cardsJS reads the order cards of an order history page: each card's text and the address
// of its invoice popover.
// Amazon Business's cards have no class of their own (17.11.2026): each order's box is the
// .a-box-group around its order number field; a shop's own, .order-card, where there is one.
const cardsJS = `(() => {
	const cards = new Set([...document.querySelectorAll('[id=orderIdField]')].map((f) => f.closest('.a-box-group')).filter(Boolean));
	document.querySelectorAll('.order-card, .js-order-card').forEach((c) => cards.add(c));
	return [...cards];
})().map((c) => {
	const pop = [...c.querySelectorAll('[data-a-popover]')].map((p) => {
		try { return JSON.parse(p.getAttribute('data-a-popover')).url || ''; } catch (e) { return ''; }
	}).find((u) => /invoice/i.test(u)) || '';
	return {text: c.innerText, popover: pop};
})`

// card is an order's card as the page shows it: its text, and the popover of its invoice links.
type card struct {
	Text    string `json:"text"`
	Popover string `json:"popover"`
}

// ordersOf are the orders placed between first and last, from the order history by year,
// newest first: logged in on the way if Amazon asks.
func ordersOf(ctx context.Context, cfg Config, first, last time.Time) ([]order, error) {
	var orders []order
	seen := map[string]bool{}
	for start := 0; start < 500; start += 10 {
		// Amazon's own default, the last three months: the month fetched is at most the last
		// one, and Business knows no year-2026 filter (17.11.2026).
		url := fmt.Sprintf("%s/your-orders/orders?startIndex=%d", site, start)
		if err := open(ctx, url); err != nil {
			return nil, fmt.Errorf("opening the orders: %w", err)
		}
		time.Sleep(2 * time.Second)
		var at, field string
		if err := chromedp.Run(ctx, chromedp.Location(&at), chromedp.Evaluate(fieldJS, &field)); err != nil {
			return nil, err
		}
		if signInPage(at, "", field) != "in" {
			logger.Println("Logging in to Amazon...")
			if err := login(ctx, cfg); err != nil {
				return nil, fmt.Errorf("login failed: %w", err)
			}
			if err := open(ctx, url); err != nil {
				return nil, fmt.Errorf("opening the orders: %w", err)
			}
			time.Sleep(2 * time.Second)
		} else if start == 0 {
			logger.Println("Still logged in")
		}
		var cards []card
		if err := chromedp.Run(ctx, chromedp.Evaluate(cardsJS, &cards)); err != nil {
			return nil, err
		}
		if len(cards) == 0 {
			if start == 0 {
				dump(ctx, cfg, "orders")
				return nil, errors.New("no order list found")
			}
			break
		}
		older := false
		for _, c := range cards {
			o, ok := parseCard(c)
			if !ok || seen[o.ID] {
				continue
			}
			seen[o.ID] = true
			switch {
			case o.Date.After(last):
			case o.Date.Before(first):
				older = true
			case o.Cancelled:
				logger.Printf("Order %s: cancelled, skipped", o.ID)
			default:
				orders = append(orders, o)
			}
		}
		if older || len(cards) < 10 {
			break
		}
	}
	return orders, nil
}

// open loads url, once more after a few seconds if the network failed on the way — a fresh
// browser's first load had its connection closed (17.11.2026). A page, not a login: no harm.
func open(ctx context.Context, url string) error {
	err := chromedp.Run(ctx, chromedp.Navigate(url))
	if err == nil || !strings.Contains(err.Error(), "net::") {
		return err
	}
	logger.Printf("Loading %s: %v, once more", pathOf(url), err)
	if err := browser.Pause(ctx, 4*time.Second, 7*time.Second); err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.Navigate(url))
}

// dump keeps the page and a picture of it beside the browser profile, for a look at what
// Amazon showed: feed-my-accounting-amazon-<what>.html and .jpg.
func dump(ctx context.Context, cfg Config, what string) {
	if cfg.Profile == "" {
		return
	}
	var html, url string
	var shot []byte
	if chromedp.Run(ctx, chromedp.Location(&url), chromedp.OuterHTML(`html`, &html, chromedp.ByQuery), chromedp.CaptureScreenshot(&shot)) != nil {
		return
	}
	base := filepath.Join(filepath.Dir(cfg.Profile), "feed-my-accounting-amazon-"+what)
	os.WriteFile(base+".jpg", shot, 0o600)
	if os.WriteFile(base+".html", []byte(html), 0o600) == nil {
		logger.Printf("Page %s kept in %s.html", url, base)
	}
}

var (
	orderIDPattern = regexp.MustCompile(`\b(\d{3}-\d{7}-\d{7}|D\d{2}-\d{7}-\d{7})\b`)
	// cancelledPattern is a cancelled order's word on its card — "Storniert" — not "Artikel
	// stornieren", the button every open order has.
	cancelledPattern = regexp.MustCompile(`(?i)\bstorniert\b|\bcancell?ed\b`)
	datePattern      = regexp.MustCompile(`(\d{1,2})\.\s*(Januar|Februar|März|April|Mai|Juni|Juli|August|September|Oktober|November|Dezember)\s+(\d{4})`)
	months           = map[string]time.Month{
		"Januar": 1, "Februar": 2, "März": 3, "April": 4, "Mai": 5, "Juni": 6,
		"Juli": 7, "August": 8, "September": 9, "Oktober": 10, "November": 11, "Dezember": 12,
	}
)

// parseCard reads an order card: its number and the date it was placed, the first date on it.
func parseCard(c card) (order, bool) {
	id := orderIDPattern.FindString(c.Text)
	m := datePattern.FindStringSubmatch(c.Text)
	if id == "" || m == nil {
		return order{}, false
	}
	var day int
	fmt.Sscan(m[1], &day)
	var year int
	fmt.Sscan(m[3], &year)
	return order{ID: id, Date: time.Date(year, months[m[2]], day, 0, 0, 0, 0, time.Local), Popover: c.Popover,
		Cancelled: cancelledPattern.MatchString(c.Text)}, true
}

// ---------------------------------------------------------------------------
// Invoices
// ---------------------------------------------------------------------------

// invoiceLinks are the order's invoice PDFs, from its invoice popover.
func invoiceLinks(ctx context.Context, o order) ([]string, error) {
	popover := o.Popover
	if popover == "" {
		popover = "/your-orders/invoice/popover?orderId=" + o.ID
	}
	html, err := fetchText(ctx, popover)
	if err != nil {
		return nil, err
	}
	return pdfLinks(html), nil
}

var hrefPattern = regexp.MustCompile(`href="([^"]+)"`)

// pdfLinks are the invoices among a popover's links: the PDFs, not the order summary to print
// nor the request for one.
func pdfLinks(html string) []string {
	var links []string
	seen := map[string]bool{}
	for _, m := range hrefPattern.FindAllStringSubmatch(html, -1) {
		href := strings.ReplaceAll(m[1], "&amp;", "&")
		lower := strings.ToLower(href)
		if !strings.Contains(lower, "invoice.pdf") && !strings.Contains(lower, "/documents/download/") && !strings.Contains(lower, "generated_invoices") {
			continue
		}
		if !seen[href] {
			seen[href] = true
			links = append(links, href)
		}
	}
	return links
}

// evaluateAsync evaluates a script that returns a promise, waiting for it to settle.
func evaluateAsync(expr string, out any) chromedp.Action {
	return chromedp.Evaluate(expr, out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) })
}

// fetchText fetches an address on amazon.de as the page, with its cookies.
func fetchText(ctx context.Context, url string) (string, error) {
	js, _ := json.Marshal(url)
	var text string
	err := chromedp.Run(ctx, evaluateAsync(fmt.Sprintf(`fetch(%s, {credentials: 'include'}).then((r) => r.ok ? r.text() : Promise.reject(new Error('HTTP ' + r.status)))`, js), &text))
	return text, err
}

// download fetches a PDF as the page, with its cookies.
func download(ctx context.Context, url string) ([]byte, error) {
	js, _ := json.Marshal(url)
	var data string
	err := chromedp.Run(ctx, evaluateAsync(fmt.Sprintf(`fetch(%s, {credentials: 'include'})
		.then((r) => r.ok ? r.blob() : Promise.reject(new Error('HTTP ' + r.status)))
		.then((b) => new Promise((ok) => { const f = new FileReader(); f.onload = () => ok(f.result); f.readAsDataURL(b); }))`, js), &data))
	if err != nil {
		return nil, err
	}
	pdf, err := base64.StdEncoding.DecodeString(data[strings.Index(data, ",")+1:])
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		return nil, errors.New("not a PDF")
	}
	return pdf, nil
}
