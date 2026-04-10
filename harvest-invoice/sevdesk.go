// Package harvest provides sevDesk invoice creation via browser automation.
//
// sevDesk is an AngularJS single-page application. All form interactions happen
// through chromedp (headless Chrome). Date fields MUST be set via the native
// date picker components — setting input values directly (even via Angular's
// $setViewValue) does not persist correctly on save.
//
// Rechnungsdatum is pre-filled with today's date by sevDesk and left unchanged.
// Leistungszeitraum is set via the daterangepicker jQuery plugin's callback()
// which updates both the input value and the Angular model.
package harvest

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// SevDeskConfig holds sevDesk login and invoice defaults.
type SevDeskConfig struct {
	User         string `yaml:"user"`
	Pass         string `yaml:"pass"`
	ProductName  string `yaml:"productName"`  // search term typed into product field (e.g. "Acme Produkt")
	ProductNum   string `yaml:"productNum"`   // article number to select from the dropdown (e.g. "0102")
	ReferenceNum string `yaml:"referenceNum"` // Kundenreferenz für E-Rechnung
}

// createInvoice logs into sevDesk and creates a draft invoice via the web UI.
// It fills in all fields (customer, dates, product, quantity, references) and
// saves the invoice as "Entwurf" (draft). It never sends the invoice.
func createInvoice(ctx context.Context, cfg SevDeskConfig, data *ReportData) error {
	log.Println("Logging in to sevDesk...")
	if err := sevdeskLogin(ctx, cfg.User, cfg.Pass); err != nil {
		return fmt.Errorf("sevDesk login: %w", err)
	}

	log.Println("Opening invoice form...")
	if err := openInvoiceForm(ctx); err != nil {
		return fmt.Errorf("opening invoice form: %w", err)
	}

	log.Println("Selecting customer...")
	if err := selectCustomer(ctx, data.ClientName); err != nil {
		return fmt.Errorf("selecting customer: %w", err)
	}

	// After selecting a customer, sevDesk shows a "Daten übernehmen" dialog
	// offering to copy address data. We dismiss it with "Abbrechen".
	log.Println("Dismissing data transfer dialog...")
	dismissDataDialog(ctx)

	// Rechnungsdatum is pre-filled with today's date by sevDesk — no action needed.

	// Leistungszeitraum = full Harvest report period (e.g. 01.04.2026 - 30.04.2026)
	log.Println("Setting service period...")
	if err := setServicePeriod(ctx, data.PeriodFrom, data.PeriodTo); err != nil {
		return fmt.Errorf("setting service period: %w", err)
	}

	if cfg.ReferenceNum != "" {
		// Referenznummer field at the top of the form
		log.Println("Setting reference number...")
		setReferenceNumber(ctx, cfg.ReferenceNum)

		// Kundenreferenz field at the bottom (for E-Rechnung)
		log.Println("Setting customer reference...")
		setCustomerReference(ctx, cfg.ReferenceNum)
	}

	log.Println("Adding position...")
	if err := addPosition(ctx, cfg.ProductName, cfg.ProductNum, data.TotalHours); err != nil {
		return fmt.Errorf("adding position: %w", err)
	}

	log.Println("Saving invoice as draft...")
	if err := saveInvoice(ctx); err != nil {
		return fmt.Errorf("saving invoice: %w", err)
	}

	log.Println("sevDesk invoice draft created successfully")
	return nil
}

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

	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const btn = document.querySelector('button[type="submit"]') ||
			[...document.querySelectorAll('button')].find(b => b.innerText.includes('Anmelden'));
		if (btn) btn.click();
	})()`, nil))
	time.Sleep(8 * time.Second)

	dismissCookieBanner(ctx)
	return nil
}

// dismissCookieBanner clicks "Alle akzeptieren" or "Nur Notwendige" if present.
func dismissCookieBanner(ctx context.Context) {
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const b = [...document.querySelectorAll('button')].find(b =>
			b.innerText.includes('Alle akzeptieren') || b.innerText.includes('Nur Notwendige'));
		if (b) b.click();
	})()`, nil))
	time.Sleep(time.Second)
}

// openInvoiceForm navigates directly to the new E-Rechnung form URL.
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

// selectCustomer types the customer name into the contact search and clicks
// the first matching dropdown entry that contains "GmbH" and "KND" (Kundennummer).
func selectCustomer(ctx context.Context, customerName string) error {
	if err := chromedp.Run(ctx,
		chromedp.Click(`input[placeholder="Person oder Organisation"]`, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.SendKeys(`input[placeholder="Person oder Organisation"]`, customerName, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return err
	}

	var result string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		for (const li of document.querySelectorAll('li')) {
			if (li.innerText.includes('GmbH') && li.innerText.includes('KND')) {
				(li.querySelector('a') || li).click();
				return 'ok: ' + li.innerText.trim().substring(0, 60);
			}
		}
		return 'not found';
	})()`, &result))
	log.Printf("Customer: %s", result)
	time.Sleep(3 * time.Second)
	return nil
}

// dismissDataDialog clicks "Abbrechen" on the "Daten übernehmen" dialog
// that appears after selecting a customer.
func dismissDataDialog(ctx context.Context) {
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const btn = [...document.querySelectorAll('button')].find(b => b.innerText.trim() === 'Abbrechen');
		if (btn) btn.click();
	})()`, nil))
	time.Sleep(2 * time.Second)
}

// ---------------------------------------------------------------------------
// Invoice form field setters
// ---------------------------------------------------------------------------

// setServicePeriod switches the Lieferdatum field to "Zeitraum" mode, then
// sets the date range via the daterangepicker jQuery plugin's callback().
func setServicePeriod(ctx context.Context, from, to time.Time) error {
	// The form defaults to a single "Lieferdatum" field. Clicking the
	// "Zeitraum" link switches it to a date range input.
	var clickResult string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const els = [...document.querySelectorAll('a, button, span, div, [ng-click]')];
		const zeitraum = els.find(el => {
			const t = el.textContent.trim();
			return t === 'Zeitraum' && el.offsetParent !== null;
		});
		if (zeitraum) { zeitraum.click(); return 'clicked'; }
		return 'not found';
	})()`, &clickResult))
	log.Printf("Zeitraum switch: %s", clickResult)
	time.Sleep(2 * time.Second)

	if strings.Contains(clickResult, "not found") {
		log.Println("Zeitraum link not found")
		return nil
	}

	// Set dates via the daterangepicker jQuery plugin: set startDate/endDate
	// on the plugin instance, then call its callback() which updates both the
	// input value and the Angular model. Note: updateElement() does NOT work.
	fromStr := from.Format("02.01.2006")
	toStr := to.Format("02.01.2006")

	var result string
	chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const inputs = document.querySelectorAll('input');
		for (const inp of inputs) {
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
	log.Printf("Leistungszeitraum: %s", result)
	time.Sleep(time.Second)

	// Close the picker
	chromedp.Run(ctx, chromedp.Evaluate(`document.body.click()`, nil))
	time.Sleep(500 * time.Millisecond)

	return nil
}

// setReferenceNumber fills the "Referenznummer eintragen" field at the top.
func setReferenceNumber(ctx context.Context, refNum string) {
	chromedp.Run(ctx,
		chromedp.Click(`input[placeholder="Referenznummer eintragen"]`, chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.SendKeys(`input[placeholder="Referenznummer eintragen"]`, refNum, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
}

// setCustomerReference fills the Kundenreferenz/Auftragsnummer field at the
// bottom of the form (required for E-Rechnung).
func setCustomerReference(ctx context.Context, refNum string) {
	var result string
	chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const inputs = [...document.querySelectorAll('input')];
		const ref = inputs.find(inp =>
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
	log.Printf("Kundenreferenz: %s", result)
}

// addPosition searches for a product by name, selects the matching article
// number from the typeahead dropdown, and sets the quantity (Menge).
//
// searchTerm is typed into the product search field (e.g. "Acme Produkt").
// articleNum is matched against the dropdown entries (e.g. "0102").
// The search term must be a prefix — typing the full product name including
// the article number does not load the price correctly.
func addPosition(ctx context.Context, searchTerm, articleNum string, hours float64) error {
	if err := chromedp.Run(ctx,
		chromedp.Click(`input[placeholder="Produkt suchen"]`, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.SendKeys(`input[placeholder="Produkt suchen"]`, searchTerm, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return err
	}

	// Select the matching entry from the typeahead dropdown
	escapedKeyword := strings.ReplaceAll(articleNum, `"`, `\"`)
	var result string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const matches = document.querySelectorAll('li.uib-typeahead-match');
		for (const li of matches) {
			if (li.innerText.trim().includes("%s")) {
				(li.querySelector('a') || li).click();
				return 'clicked: ' + li.innerText.trim();
			}
		}
		if (matches.length > 0) {
			(matches[0].querySelector('a') || matches[0]).click();
			return 'fallback: ' + matches[0].innerText.trim();
		}
		return 'no matches';
	})()`, escapedKeyword), &result)); err != nil {
		return err
	}
	log.Printf("Product: %s", result)
	time.Sleep(3 * time.Second)

	var priceCheck string
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const p = document.querySelector('input[ng-model="position.priceNet"]');
		return p ? 'priceNet=' + p.value : 'not found';
	})()`, &priceCheck))
	log.Printf("Price: %s", priceCheck)

	// Set Menge (quantity) via the Angular ng-model input
	hoursStr := strings.ReplaceAll(fmt.Sprintf("%.2f", hours), ".", ",")
	var mengeResult string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const byModel = document.querySelector('input[ng-model*="quantity"]');
		if (byModel) {
			byModel.focus();
			byModel.select();
			document.execCommand('selectAll');
			document.execCommand('insertText', false, '%s');
			byModel.dispatchEvent(new Event('input', {bubbles: true}));
			byModel.dispatchEvent(new Event('change', {bubbles: true}));
			byModel.blur();
			return 'set: ' + byModel.value;
		}
		return 'menge not found';
	})()`, hoursStr), &mengeResult)); err != nil {
		return err
	}
	log.Printf("Menge: %s", mengeResult)
	time.Sleep(time.Second)
	return nil
}

// saveInvoice clicks the toolbar save button (floppy disk icon) to save the
// invoice as a draft. The save button is the second tooltip-wrapped button in
// the toolbar (after the preview/eye button).
func saveInvoice(ctx context.Context) error {
	chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, 0)`, nil))
	time.Sleep(time.Second)

	var result string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const tooltipBtns = [...document.querySelectorAll('[class*="tooltip-children"] button')];
		if (tooltipBtns.length >= 2) {
			const saveBtn = tooltipBtns[1];
			saveBtn.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}));
			return 'clicked save button';
		}
		return 'not found (tooltip btns: ' + tooltipBtns.length + ')';
	})()`, &result)); err != nil {
		return err
	}
	log.Printf("Save: %s", result)
	time.Sleep(10 * time.Second)

	return nil
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

// waitForSelector waits for a CSS selector to become visible, with a timeout.
func waitForSelector(ctx context.Context, sel string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, chromedp.WaitVisible(sel, chromedp.ByQuery))
}
