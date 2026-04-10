// Package harvest provides sevDesk invoice creation via browser automation.
//
// sevDesk is an AngularJS SPA using Angular UI Grid for lists. All interactions
// happen through chromedp (headless Chrome):
//
//   - Date fields MUST be set via the daterangepicker jQuery plugin's callback(),
//     not by setting input values directly (Angular model won't update).
//   - Rechnungsdatum is pre-filled with today's date by sevDesk — left unchanged.
//   - The invoice list uses div[role="row"] / div[role="gridcell"] (not <tr>/<td>).
//   - Direct URL navigation to list pages doesn't work (SPA redirects to dashboard),
//     so we use the hamburger menu to navigate.
package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// SevDeskConfig holds sevDesk login and invoice defaults.
type SevDeskConfig struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	ClientName   string `yaml:"clientName"`   // fixed client name (overrides name extracted from email)
	ProductName  string `yaml:"productName"`  // search term for product field (e.g. "Acme Produkt")
	ProductNum   string `yaml:"productNum"`   // article number to select from dropdown (e.g. "0102")
	ReferenceNum string `yaml:"referenceNum"` // Kundenreferenz für E-Rechnung
}

// ---------------------------------------------------------------------------
// Login & navigation helpers
// ---------------------------------------------------------------------------

// sevdeskLogin navigates to sevDesk and logs in with the given credentials.
func sevdeskLogin(ctx context.Context, user, pass string) error {
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://my.sevdesk.de/"),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return err
	}

	dismissCookieBanner(ctx)

	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`input[type="email"], input[placeholder*="E-Mail"]`, chromedp.ByQuery),
		chromedp.Click(`input[type="email"], input[placeholder*="E-Mail"]`, chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.SendKeys(`input[type="email"], input[placeholder*="E-Mail"]`, user, chromedp.ByQuery),
		chromedp.Click(`input[type="password"], input[placeholder*="Passwort"]`, chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.SendKeys(`input[type="password"], input[placeholder*="Passwort"]`, pass, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("filling login form: %w", err)
	}

	clickButton(ctx, `button[type="submit"]`, "Anmelden")

	// Wait for dashboard to load (hamburger menu or sidebar appears after login)
	if err := waitForSelector(ctx, `[class*="hamburger"], [class*="menu-toggle"], [class*="sidebar"]`, 15*time.Second); err != nil {
		// Fallback: if no specific element found, give it a moment
		time.Sleep(5 * time.Second)
	}

	dismissCookieBanner(ctx)
	return nil
}

// dismissCookieBanner clicks the cookie consent button if present.
func dismissCookieBanner(ctx context.Context) {
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const b = [...document.querySelectorAll('button')].find(b =>
			b.innerText.includes('Alle akzeptieren') || b.innerText.includes('Nur Notwendige'));
		if (b) b.click();
	})()`, nil))
	time.Sleep(time.Second)
}

// clickButton clicks a button by CSS selector, falling back to text search.
func clickButton(ctx context.Context, selector, text string) {
	chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const btn = document.querySelector('%s') ||
			[...document.querySelectorAll('button')].find(b => b.innerText.includes('%s'));
		if (btn) btn.click();
	})()`, selector, text), nil))
}

// navigateToInvoiceList opens the Rechnungen list via the hamburger menu.
// Direct URL navigation to /fi/index/type/RE redirects to the dashboard,
// so we must click through the SPA menu.
func navigateToInvoiceList(ctx context.Context) error {
	// Open the hamburger menu (top-left) and click "Rechnungen".
	// Retry up to 3 times — on slower machines the menu may need more time to open.
	var result string
	for attempt := 0; attempt < 3; attempt++ {
		chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const btn = document.querySelector('[class*="hamburger"], [class*="menu-toggle"]');
			if (btn) { btn.click(); return; }
			const icons = document.querySelectorAll('svg, [class*="icon"]');
			for (const el of icons) {
				const r = el.getBoundingClientRect();
				if (r.left < 100 && r.top < 80 && r.width > 0) { el.click(); return; }
			}
		})()`, nil))
		time.Sleep(3 * time.Second)

		chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			for (const el of document.querySelectorAll('a, [role="menuitem"], [class*="nav"] span, [class*="sidebar"] a')) {
				if (el.textContent.trim() === 'Rechnungen' || el.textContent.trim() === 'Ausgangsrechnungen') {
					el.click();
					return 'ok';
				}
			}
			for (const el of document.querySelectorAll('*')) {
				if (el.children.length === 0 && el.textContent.trim() === 'Rechnungen' && el.offsetParent !== null) {
					el.click();
					return 'ok';
				}
			}
			return 'not found';
		})()`, &result))
		if result == "ok" {
			break
		}
		logger.Printf("Rechnungen menu not found (attempt %d/3), retrying...", attempt+1)
	}
	if result != "ok" {
		logger.Printf("WARNING: Rechnungen menu item not found after 3 attempts")
	}
	time.Sleep(8 * time.Second)
	return nil
}

// waitForSelector waits for a CSS selector to become visible, with a timeout.
func waitForSelector(ctx context.Context, sel string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, chromedp.WaitVisible(sel, chromedp.ByQuery))
}

// ---------------------------------------------------------------------------
// Duplicate check
// ---------------------------------------------------------------------------

// checkInvoiceExists logs into sevDesk, navigates to the Rechnungen list,
// and checks whether any invoice already exists for the given client with
// a Rechnungsdatum within the Leistungszeitraum (from–to).
//
// This is a heuristic: since there's at most one invoice per client per
// month, matching client name + date-in-range is sufficient. The check covers
// all statuses (Entwurf, Offen, Bezahlt, etc.).
func checkInvoiceExists(ctx context.Context, cfg SevDeskConfig, clientName string, from, to time.Time) (bool, error) {
	if err := sevdeskLogin(ctx, cfg.Username, cfg.Password); err != nil {
		return false, fmt.Errorf("sevDesk login: %w", err)
	}

	logger.Println("Navigating to Rechnungen...")
	if err := navigateToInvoiceList(ctx); err != nil {
		return false, fmt.Errorf("navigating to invoices: %w", err)
	}

	rows, err := readInvoiceList(ctx)
	if err != nil {
		return false, err
	}
	logger.Printf("Found %d invoices in list", len(rows))

	fromDay := truncateToDay(from)
	toDay := truncateToDay(to)

	for _, row := range rows {
		if !strings.Contains(row.Text, clientName) {
			continue
		}
		datum := parseDatum(row.Datum)
		if datum.IsZero() || datum.Before(fromDay) || datum.After(toDay) {
			continue
		}
		logger.Printf("Found existing invoice: %s (%s, Datum %s)",
			clientName, row.Status, row.Datum)
		return true, nil
	}

	logger.Println("No matching invoice found in list")
	return false, nil
}

// invoiceRow holds the parsed data from a single row in the invoice list.
type invoiceRow struct {
	Status string // e.g. "Entwurf", "Offen", "Bezahlt"
	Datum  string // e.g. "10.04.26" or "27.02.26"
	Text   string // full row text for client name matching
}

// readInvoiceList extracts all invoice rows from the currently displayed
// Rechnungen page. sevDesk uses Angular UI Grid with div[role="row"] elements
// containing div[role="gridcell"] children.
func readInvoiceList(ctx context.Context) ([]invoiceRow, error) {
	var listJSON string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const rows = [];
		for (const row of document.querySelectorAll('[role="row"]')) {
			const cells = row.querySelectorAll('[role="gridcell"]');
			if (cells.length === 0) continue;
			const cellTexts = [...cells].map(c => c.innerText.trim());
			rows.push({
				status: cellTexts[0] || '',
				datum:  cellTexts[4] || '',
				text:   row.innerText.trim().substring(0, 300),
			});
		}
		return JSON.stringify(rows);
	})()`, &listJSON))

	if listJSON == "" {
		return nil, fmt.Errorf("could not read invoice list")
	}

	var raw []struct {
		Status string `json:"status"`
		Datum  string `json:"datum"`
		Text   string `json:"text"`
	}
	if err := json.Unmarshal([]byte(listJSON), &raw); err != nil {
		return nil, fmt.Errorf("parsing invoice list: %w", err)
	}

	rows := make([]invoiceRow, len(raw))
	for i, r := range raw {
		rows[i] = invoiceRow{Status: r.Status, Datum: r.Datum, Text: r.Text}
	}
	return rows, nil
}

// ---------------------------------------------------------------------------
// Invoice creation
// ---------------------------------------------------------------------------

// createInvoice logs into sevDesk and creates a draft invoice via the web UI.
// It fills in all fields (client, dates, product, quantity, references) and
// saves as "Entwurf" (draft). It never sends the invoice.
//
// If alreadyLoggedIn is true, the login step is skipped (the duplicate check
// already established a session in the same browser context).
func createInvoice(ctx context.Context, cfg SevDeskConfig, data *ReportData, alreadyLoggedIn bool) error {
	if !alreadyLoggedIn {
		logger.Println("Logging in to sevDesk...")
		if err := sevdeskLogin(ctx, cfg.Username, cfg.Password); err != nil {
			return fmt.Errorf("sevDesk login: %w", err)
		}
	}

	logger.Println("Opening invoice form...")
	if err := openInvoiceForm(ctx); err != nil {
		return fmt.Errorf("opening invoice form: %w", err)
	}

	logger.Println("Selecting client...")
	if err := selectClient(ctx, data.ClientName); err != nil {
		return fmt.Errorf("selecting client: %w", err)
	}

	// sevDesk shows a "Daten übernehmen" dialog after selecting a client.
	logger.Println("Dismissing data transfer dialog...")
	dismissDataDialog(ctx)

	// Rechnungsdatum is pre-filled with today's date — no action needed.

	logger.Println("Setting service period...")
	if err := setServicePeriod(ctx, data.PeriodFrom, data.PeriodTo); err != nil {
		return fmt.Errorf("setting service period: %w", err)
	}

	if cfg.ReferenceNum != "" {
		logger.Println("Setting reference number...")
		setReferenceNumber(ctx, cfg.ReferenceNum)
		logger.Println("Setting client reference...")
		setClientReference(ctx, cfg.ReferenceNum)
	}

	logger.Println("Adding position...")
	if err := addPosition(ctx, cfg.ProductName, cfg.ProductNum, data.TotalHours); err != nil {
		return fmt.Errorf("adding position: %w", err)
	}

	logger.Println("Saving invoice as draft...")
	if err := saveInvoice(ctx); err != nil {
		return fmt.Errorf("saving invoice: %w", err)
	}

	logger.Println("sevDesk invoice draft created successfully")
	return nil
}

// openInvoiceForm navigates directly to the new invoice form.
func openInvoiceForm(ctx context.Context) error {
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://my.sevdesk.de/fi/edit/type/RE/id/"),
		chromedp.Sleep(5*time.Second),
	); err != nil {
		return fmt.Errorf("navigating to invoice form: %w", err)
	}
	dismissCookieBanner(ctx)
	if err := waitForSelector(ctx, `input[placeholder="Person oder Organisation"]`, 15*time.Second); err != nil {
		return fmt.Errorf("invoice form did not load: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Invoice form field setters
// ---------------------------------------------------------------------------

// selectClient types the client name and clicks the matching dropdown entry.
func selectClient(ctx context.Context, clientName string) error {
	if err := chromedp.Run(ctx,
		chromedp.Click(`input[placeholder="Person oder Organisation"]`, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.SendKeys(`input[placeholder="Person oder Organisation"]`, clientName, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return err
	}

	var result string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		for (const li of document.querySelectorAll('li')) {
			if (li.innerText.includes('GmbH') && li.innerText.includes('KND')) {
				(li.querySelector('a') || li).click();
				return 'ok: ' + li.innerText.trim().replace(/\n/g, ' ').substring(0, 80);
			}
		}
		return 'not found';
	})()`, &result))
	logger.Printf("Client: %s", result)
	time.Sleep(3 * time.Second)
	return nil
}

// dismissDataDialog clicks "Abbrechen" on the "Daten übernehmen" dialog
// that appears after selecting a client.
func dismissDataDialog(ctx context.Context) {
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const btn = [...document.querySelectorAll('button')].find(b => b.innerText.trim() === 'Abbrechen');
		if (btn) btn.click();
	})()`, nil))
	time.Sleep(2 * time.Second)
}

// setServicePeriod switches the Lieferdatum to "Zeitraum" mode and sets the
// date range via the daterangepicker jQuery plugin's callback(). Using
// callback() is the only way to update both the input value and Angular model;
// setStartDate/setEndDate + updateElement() does NOT persist on save.
func setServicePeriod(ctx context.Context, from, to time.Time) error {
	// Switch from single Lieferdatum to Zeitraum (date range)
	var clickResult string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const els = [...document.querySelectorAll('a, button, span, div, [ng-click]')];
		const zeitraum = els.find(el =>
			el.textContent.trim() === 'Zeitraum' && el.offsetParent !== null);
		if (zeitraum) { zeitraum.click(); return 'clicked'; }
		return 'not found';
	})()`, &clickResult))
	logger.Printf("Zeitraum switch: %s", clickResult)
	time.Sleep(2 * time.Second)

	if clickResult == "not found" {
		return nil
	}

	// Set dates via the daterangepicker plugin instance
	fromStr := from.Format("02.01.2006")
	toStr := to.Format("02.01.2006")
	var result string
	chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		for (const inp of document.querySelectorAll('input')) {
			try {
				const dp = $(inp).data('daterangepicker');
				if (!dp) continue;
				dp.setStartDate(moment('%s', 'DD.MM.YYYY'));
				dp.setEndDate(moment('%s', 'DD.MM.YYYY'));
				dp.callback(dp.startDate, dp.endDate, 'custom');
				return 'set: ' + inp.value;
			} catch(e) { return 'error: ' + e.message; }
		}
		return 'no daterangepicker found';
	})()`, fromStr, toStr), &result))
	logger.Printf("Leistungszeitraum: %s", result)
	time.Sleep(time.Second)

	// Close the picker overlay
	chromedp.Run(ctx, chromedp.Evaluate(`document.body.click()`, nil))
	time.Sleep(500 * time.Millisecond)
	return nil
}

// setReferenceNumber selects "Bestellung vom Käufer BT-13" from the custom
// dropdown left of the "Referenznummer eintragen" field, then fills the field.
func setReferenceNumber(ctx context.Context, refNum string) {
	// Step 1: Click the custom dropdown button next to the reference field
	var dropdownResult string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const refInput = document.querySelector('input[placeholder="Referenznummer eintragen"]');
		if (!refInput) return 'refInput not found';
		const container = refInput.parentElement.parentElement;
		const btn = container.querySelector('button[class*="select-button"]') ||
			container.querySelector('[class*="select-module"] button') ||
			container.querySelector('[class*="select"] button');
		if (btn) { btn.click(); return 'opened'; }
		return 'button not found';
	})()`, &dropdownResult))
	time.Sleep(500 * time.Millisecond)

	// Step 2: Click the "Bestellung vom Käufer BT-13" option in the opened dropdown
	if dropdownResult == "opened" {
		chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			for (const el of document.querySelectorAll('li, [role="option"], [class*="option"], [class*="menu-item"]')) {
				if (el.innerText.includes('BT-13') || el.innerText.includes('Bestellung')) {
					el.click();
					return 'set: ' + el.innerText.trim().replace(/\n/g, ' ');
				}
			}
			return 'BT-13 option not found';
		})()`, &dropdownResult))
	}
	logger.Printf("Reference type: %s", dropdownResult)
	time.Sleep(500 * time.Millisecond)

	// Step 3: Fill the reference number
	chromedp.Run(ctx,
		chromedp.Click(`input[placeholder="Referenznummer eintragen"]`, chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.SendKeys(`input[placeholder="Referenznummer eintragen"]`, refNum, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
}

// setClientReference fills the Kundenreferenz/Auftragsnummer field
// (required for E-Rechnung/XRechnung). Uses execCommand to trigger Angular model update.
func setClientReference(ctx context.Context, refNum string) {
	var result string
	chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const ref = [...document.querySelectorAll('input')].find(inp =>
			inp.placeholder.includes('Auftragsnummer') ||
			inp.placeholder.includes('Kundenreferenz'));
		if (ref) {
			ref.focus();
			ref.select();
			document.execCommand('selectAll');
			document.execCommand('insertText', false, '%s');
			ref.dispatchEvent(new Event('change', {bubbles: true}));
			ref.blur();
			return 'set: ' + ref.value;
		}
		return 'field not found';
	})()`, strings.ReplaceAll(refNum, `"`, `\"`)), &result))
	logger.Printf("Kundenreferenz: %s", result)
}

// addPosition searches for a product, selects it from the typeahead dropdown,
// and sets the quantity (Menge = hours).
func addPosition(ctx context.Context, searchTerm, articleNum string, hours float64) error {
	if err := chromedp.Run(ctx,
		chromedp.Click(`input[placeholder="Produkt suchen"]`, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.SendKeys(`input[placeholder="Produkt suchen"]`, searchTerm, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return err
	}

	// Select matching article from typeahead dropdown
	escapedNum := strings.ReplaceAll(articleNum, `"`, `\"`)
	var result string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const matches = document.querySelectorAll('li.uib-typeahead-match');
		for (const li of matches) {
			if (li.innerText.trim().includes("%s")) {
				(li.querySelector('a') || li).click();
				return 'clicked: ' + li.innerText.trim().replace(/\n/g, ' ');
			}
		}
		if (matches.length > 0) {
			(matches[0].querySelector('a') || matches[0]).click();
			return 'fallback: ' + matches[0].innerText.trim().replace(/\n/g, ' ');
		}
		return 'no matches';
	})()`, escapedNum), &result)); err != nil {
		return err
	}
	logger.Printf("Product: %s", result)
	time.Sleep(3 * time.Second)

	// Verify price loaded
	var priceCheck string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const p = document.querySelector('input[ng-model="position.priceNet"]');
		return p ? 'priceNet=' + p.value : 'not found';
	})()`, &priceCheck))
	logger.Printf("Price: %s", priceCheck)

	// Set quantity via execCommand to trigger Angular model update
	hoursStr := strings.ReplaceAll(fmt.Sprintf("%.2f", hours), ".", ",")
	var mengeResult string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const inp = document.querySelector('input[ng-model*="quantity"]');
		if (inp) {
			inp.focus();
			inp.select();
			document.execCommand('selectAll');
			document.execCommand('insertText', false, '%s');
			inp.dispatchEvent(new Event('input', {bubbles: true}));
			inp.dispatchEvent(new Event('change', {bubbles: true}));
			inp.blur();
			return 'set: ' + inp.value;
		}
		return 'not found';
	})()`, hoursStr), &mengeResult)); err != nil {
		return err
	}
	logger.Printf("Menge: %s", mengeResult)
	time.Sleep(time.Second)
	return nil
}

// saveInvoice clicks the toolbar save button (floppy disk icon, second
// tooltip-wrapped button after the preview/eye button).
func saveInvoice(ctx context.Context) error {
	chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, 0)`, nil))
	time.Sleep(time.Second)

	var result string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const btns = [...document.querySelectorAll('[class*="tooltip-children"] button')];
		if (btns.length >= 2) {
			btns[1].dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}));
			return 'clicked save button';
		}
		return 'not found (btns: ' + btns.length + ')';
	})()`, &result)); err != nil {
		return err
	}
	logger.Printf("Save: %s", result)
	time.Sleep(10 * time.Second)
	return nil
}

// ---------------------------------------------------------------------------
// Date parsing helpers
// ---------------------------------------------------------------------------

// parseDatum parses German date strings in DD.MM.YYYY or DD.MM.YY format.
func parseDatum(s string) time.Time {
	for _, layout := range []string{"2.1.2006", "02.01.2006", "2.1.06", "02.01.06"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// truncateToDay strips the time component from a timestamp.
func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
