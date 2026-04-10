# feed-my-accounting

[![Version](https://img.shields.io/badge/version-1.4.0-blue.svg)](CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

A single command that closes your books for the month.

Every month, freelancers and small businesses juggle the same ritual: collect invoices from Apple and Vodafone, generate travel expense reports, turn Harvest timesheets into outgoing invoices, and feed everything into the accounting system. **feed-my-accounting** automates all of it — from source to [sevDesk](https://sevdesk.de).

It connects four data sources to your sevDesk account, each handled by a dedicated module:

| Module | Source | What it does | Delivery |
|--------|--------|-------------|----------|
| **travel-expense** | Config (clients, distances) | Generates Kilometergelderstattung + Verpflegungsmehraufwand PDFs | Email → sevDesk Autobox |
| **apple-invoice-pdf** | IMAP inbox | Fetches Apple invoice emails, converts HTML to PDF via headless Chrome | Email → sevDesk Autobox |
| **vodafone-downloader** | MeinVodafone portal | Logs in via headless Chrome, downloads Mobilfunk + Kabel invoices | Email → sevDesk Autobox |
| **harvest-invoice** | IMAP inbox + Harvest | Downloads time report PDF, extracts hours, creates draft invoice in sevDesk | Direct browser automation |

### How it all fits together

```
                        feed-my-accounting
                       ────────────────────
                                │
           ┌────────────────────┼────────────────────┐
           │                    │                    │
  ┌────────────────┐  ┌─────────────────┐  ┌─────────────────┐
  │ travel-expense │  │ apple-invoice-  │  │ vodafone-       │
  │                │  │ pdf             │  │ downloader      │
  │ Config →       │  │ IMAP → Chrome → │  │ Chrome →        │
  │ generate PDFs  │  │ HTML to PDF     │  │ download PDFs   │
  └───────┬────────┘  └────────┬────────┘  └────────┬────────┘
          │                    │                    │
          └────────────────────┼────────────────────┘
                               │
                        Email + SMTP
                               │
                               ▼
                  ┌───────────────────────┐
                  │  sevDesk Autobox      │
                  │  autobox@sevdesk.email│
                  │                       │
                  │  Auto-imports PDFs    │
                  │  via OCR parsing      │
                  └───────────────────────┘

  ┌─────────────────┐              ┌─────────────────┐
  │ harvest-invoice │    Chrome    │ sevDesk Web UI  │
  │                 │  ─────────►  │                 │
  │ IMAP → Harvest  │              │ Creates draft   │
  │ PDF → extract   │              │ E-Rechnung with │
  │ total hours     │              │ hours + period  │
  └─────────────────┘              └─────────────────┘
```

Three modules collect documents and deliver them as email attachments to sevDesk's **Autobox** — an inbox that automatically imports and OCR-parses incoming PDFs as bookkeeping records. The fourth module (harvest-invoice) goes further: it logs into sevDesk directly and creates a fully populated draft invoice, ready for review.

Run once a month with a single command — or schedule it with cron — and your accounting is fed.

All modules share a single `config.yaml` with a common `mail` block for SMTP/IMAP credentials and addresses.

## Requirements

- Go 1.25+
- Google Chrome or Chromium (required by `apple-invoice-pdf`, `vodafone-downloader`, and `harvest-invoice`)

## Installation

```bash
go build -o feed-my-accounting .
```

## Usage

```bash
feed-my-accounting [--config path] [command] [args...]

# Run all modules for the current month (default when no command is given)
feed-my-accounting
feed-my-accounting all

# Run all modules for a specific month
feed-my-accounting all 3/2026

# Travel expense report for current month
feed-my-accounting travel-expense

# Travel expense report for a specific month
feed-my-accounting travel-expense 3/2026
feed-my-accounting travel-expense 12/2025

# Apple invoice emails → PDF → email
feed-my-accounting apple-invoice-pdf

# Vodafone invoices → email
feed-my-accounting vodafone-downloader

# Harvest report → sevDesk draft invoice
feed-my-accounting harvest-invoice

# Custom config file
feed-my-accounting --config /path/to/config.yaml all 3/2026

# Show version
feed-my-accounting --version
```

## Configuration

Copy `config.yaml.example` to `config.yaml` and fill in your details:

```bash
cp config.yaml.example config.yaml
```

The config file is searched in the following order:
1. Path provided via `--config`
2. Current working directory
3. Directory of the executable

### Full config reference

```yaml
mail:
  smtpHost: "smtp.example.com"
  smtpPort: 587
  imapHost: "imap.example.com"
  imapPort: 993
  username: "user@example.com"
  password: "your-password"
  from: "user@example.com"       # optional, falls back to username
  to: "recipient@example.com"
  cc: "autobox@sevdesk.email"    # optional

travel-expense:
  employee: Max Mustermann
  christmasWeekOff: true         # optional, default: true
  clients:
    - id: "1"
      name: Acme GmbH
      from: "Stuttgart, Heimatbüro (Your Company GmbH)"
      to: "München, Hauptstr. (Acme GmbH)"
      reason: Projektarbeit
      distance: 42
      province: BW

apple-invoice-pdf:
  filter:
    count: 10
    subject: "Deine Rechnung von Apple"
    from: "apple.com"

vodafone-downloader:
  username: "your-vodafone-email@example.com"
  password: "your-vodafone-password"
  fallbackToLastMonth: true      # optional, default: true

harvest-invoice:
  currentMonthOnly: true           # optional, default: true
  skipExisting: true               # optional, default: true
  filter:
    count: 10
    subject: "We've exported your detailed time report"
    from: "harvestapp.com"
  harvest:
    username: "harvest-login@example.com"
    password: "your-harvest-password"
  sevdesk:
    username: "sevdesk-login@example.com"
    password: "your-sevdesk-password"
    clientName: "Acme GmbH"        # optional, overrides client name from Harvest email
    productName: "Acme Produkt"    # search term for product field
    productNum: "0102"             # article number to select from dropdown
    referenceNum: "REF-123"        # Kundenreferenz für E-Rechnung
```

---

## Module: travel-expense

Generates two PDF documents per month for German business travel expense reporting:

- **Kilometergelderstattung** — mileage reimbursement (distance × €0.30/km, one-way)
- **Verpflegungsmehraufwand** — meal allowance (€14.00 for trips 8h–24h)

### How it works

1. Calculates workdays for the month (excluding weekends, German public holidays, and optionally the Christmas/New Year week)
2. Distributes workdays equally among configured clients using round-robin assignment
3. Generates formatted PDF documents with smart page breaks (blocks never split across pages)
4. Sends both PDFs via email as in-memory attachments (no files written to disk)

### PDF output

- `MM_YYYY_Reisekosten_Kilometergelderstattung.pdf`
- `MM_YYYY_Reisekosten_Verpflegungsmehraufwand.pdf`

PDFs include a structured `Beleg-Nr.` (format: `RK-YYYY-MM-XXXX`) and are formatted for sevDesk OCR recognition. The `employee` name is rendered prominently at the top as the Lieferant.

### Clients

Each client represents a destination:

| Field | Description |
|-------|-------------|
| `id` | Identifier shown in the document |
| `name` | Client name used as section header |
| `from` | Origin address (e.g. `"Stuttgart, Heimatbüro (Your Company GmbH)"`) |
| `to` | Destination address (e.g. `"München, Hauptstr. (Acme GmbH)"`) |
| `reason` | Purpose of the trip (e.g. `Projektarbeit`) |
| `distance` | One-way distance in kilometres |
| `province` | German state code for holiday calculation (see below) |

When multiple clients are configured, workdays are distributed round-robin. With 20 workdays and 2 clients each gets 10 days. Mileage is calculated per client based on their distance.

### Province codes (Bundesland)

| Code | State (German) | State (English) |
|------|----------------|-----------------|
| BW | Baden-Württemberg | Baden-Württemberg |
| BY | Bayern | Bavaria |
| BE | Berlin | Berlin |
| BB | Brandenburg | Brandenburg |
| HB | Bremen | Bremen |
| HH | Hamburg | Hamburg |
| HE | Hessen | Hesse |
| MV | Mecklenburg-Vorpommern | Mecklenburg-Western Pomerania |
| NI | Niedersachsen | Lower Saxony |
| NW | Nordrhein-Westfalen | North Rhine-Westphalia |
| RP | Rheinland-Pfalz | Rhineland-Palatinate |
| SL | Saarland | Saarland |
| SN | Sachsen | Saxony |
| ST | Sachsen-Anhalt | Saxony-Anhalt |
| SH | Schleswig-Holstein | Schleswig-Holstein |
| TH | Thüringen | Thuringia |

Defaults to `BW` if omitted or invalid.

### Excluded dates

- Weekends (Saturday, Sunday)
- German public holidays for the client's province
- Christmas/New Year week off when `christmasWeekOff: true` (the default):
  - December 24
  - December 27–31

  December 25–26 are already excluded as public holidays (Weihnachten). Set `christmasWeekOff: false` to only exclude public holidays.

### travel-expense config options

| Field | Description | Default |
|-------|-------------|---------|
| `employee` | Employee name shown as Lieferant in PDF header for sevDesk OCR | — |
| `christmasWeekOff` | Exclude Dec 24 and Dec 27–31 | `true` |
| `clients` | List of client/trip configurations | required |

---

## Module: apple-invoice-pdf

Fetches Apple invoice emails from an IMAP inbox, converts their HTML body to PDF using headless Chrome, and sends all PDFs as attachments in a single email.

### How it works

1. Connects to the IMAP server and scans the last N recent emails
2. Filters by subject, sender domain, and current month
3. Extracts the HTML body and converts each to an A4 PDF via headless Chrome
4. Embeds external images as base64 data URIs for reliable rendering
5. Names each PDF as `MM_YYYY_Rechnung_Apple_BESTELLNUMMER.pdf` using the order number extracted from the invoice HTML; falls back to subject-based naming if not found
6. Sends all PDFs as attachments in a single email

### apple-invoice-pdf config options

| Field | Description | Default |
|-------|-------------|---------|
| `filter.count` | Number of recent emails to scan | `10` |
| `filter.subject` | Exact subject line to match | `Deine Rechnung von Apple` |
| `filter.from` | Sender domain to match | `apple.com` |

The IMAP host and port are configured under the top-level `mail:` section.

---

## Module: vodafone-downloader

Logs into the MeinVodafone portal via headless Chrome, downloads Mobilfunk and Kabel invoices, and sends them as PDF attachments via email.

### How it works

1. Launches headless Chrome with bot-detection evasion (new headless mode, custom user agent, `navigator.webdriver` removed via CDP)
2. Logs into `meinvodafone.de` and navigates to the invoice page for each contract type (Mobilfunk, Kabel)
3. Downloads the invoice shown in the "Aktuelle Rechnung" block by intercepting the blob URL created by the browser
4. If that download fails and `fallbackToLastMonth` is true, tries the first entry in the Rechnungsarchiv instead
5. If the shown invoice is not for the current month and `fallbackToLastMonth` is false, skips and sends no email
6. Sends all found invoices as attachments in a single email

### When to run

Run near the **end of the month** (around the 25th or later). Vodafone invoices are typically generated mid-month and may not be available earlier.

### PDF output

- `MM_YYYY_Rechnung_Vodafone_Mobilfunk.pdf`
- `MM_YYYY_Rechnung_Vodafone_Kabel.pdf`

### Adding contract types

Edit `contractTypes` in `vodafone-downloader/vodafone-downloader.go`:

```go
var contractTypes = map[string]string{
    "mobilfunk": "Mobilfunk",
    "kabel":     "Kabel",
    "dsl":       "DSL", // example
}
```

### vodafone-downloader config options

| Field | Description | Default |
|-------|-------------|---------|
| `username` | MeinVodafone login email | required |
| `password` | MeinVodafone password | required |
| `fallbackToLastMonth` | If `true`, send last month's invoice when current month is not yet available. If `false`, skip sending entirely until the current month's invoice is ready. | `true` |

---

## Module: harvest-invoice

Fetches Harvest monthly time report emails from IMAP, downloads the PDF export via headless Chrome, extracts the total hours, and creates a draft invoice (Entwurf) in sevDesk via browser automation.

### How it works

1. Connects to the IMAP server and scans recent emails matching the Harvest export subject/sender
2. Parses the export email HTML to extract the download URL, date range, and client name
3. **Guard: `currentMonthOnly`** — skips the report if the period is not the current month
4. **Guard: `skipExisting`** — logs into sevDesk, navigates to the Rechnungen list via the menu, and checks if any invoice for the same client has a Rechnungsdatum within the Leistungszeitraum; skips if found (works for Entwurf, Offen, and Bezahlt invoices)
5. Logs into Harvest via headless Chrome and downloads the PDF report
6. Extracts the total hours from the PDF
7. Logs into sevDesk and creates a new E-Rechnung draft with:
   - **Rechnungsdatum** pre-filled as today's date by sevDesk
   - **Leistungszeitraum** set to the full report period (e.g. 01.04.2026 - 30.04.2026)
   - **Referenznummer** and **Kundenreferenz** from config
   - **Product** selected via typeahead search (search term + article number)
   - **Menge** set to the total hours from the PDF
8. Saves the invoice as draft — it is never sent to the client

IMAP credentials are taken from the top-level `mail.username` / `mail.password`. The `skipExisting` guard uses the same headless Chrome browser to log into sevDesk and inspect the Rechnungen list — no API token needed.

### Harvest report setup

The Harvest time report **must be filtered to a single client** in Harvest's report settings. The client name in the export email is extracted from the `"to <ClientName>"` line and used to:

- Select the client in the sevDesk invoice form
- Match against existing invoices for duplicate detection

The client name in Harvest must match the client name in sevDesk (e.g. both `"Acme GmbH"`). If names differ, set `sevdesk.clientName` in the config to use a fixed name instead of the one extracted from the email.

### harvest-invoice config options

| Field | Description | Default |
|-------|-------------|---------|
| `currentMonthOnly` | Only process reports where the period matches the current month | `true` |
| `skipExisting` | Check sevDesk for an existing invoice with the same client and period before creating | `true` |
| `filter.count` | Number of recent emails to scan | `10` |
| `filter.subject` | Subject line to match | `We've exported your detailed time report` |
| `filter.from` | Sender domain to match | `harvestapp.com` |
| `harvest.username` | Harvest login email | required |
| `harvest.password` | Harvest login password | required |
| `sevdesk.username` | sevDesk login email | required |
| `sevdesk.password` | sevDesk login password | required |
| `sevdesk.clientName` | Fixed client name for sevDesk (overrides name from Harvest email) | — |
| `sevdesk.productName` | Search term typed into the product field (e.g. "Acme Produkt") | required |
| `sevdesk.productNum` | Article number to select from the typeahead dropdown (e.g. "0102") | required |
| `sevdesk.referenceNum` | Kundenreferenz for E-Rechnung | — |

### Safe for cron: duplicate prevention

harvest-invoice is designed to be run repeatedly (e.g. via daily cron) without creating duplicate invoices. Two guard checks run **before** the expensive browser automation:

- **`currentMonthOnly`** (default: `true`) — If the most recent Harvest export email covers a past month, it is silently skipped. This prevents old emails from being re-processed when polling the inbox via cron. Disable this (`false`) if you want to process exports for any month.

- **`skipExisting`** (default: `true`) — Before downloading the PDF or opening the invoice form, the module logs into sevDesk via headless Chrome, navigates to the Rechnungen list, and checks if any invoice for the same client has a Rechnungsdatum within the report's Leistungszeitraum. This covers all statuses (Entwurf, Offen, Bezahlt). If a match is found, the run is skipped. This means cron can safely run every day: the first successful run creates the invoice, and all subsequent runs are no-ops.

Both checks are lightweight and exit early. No Harvest PDF download or sevDesk form automation happens when either guard triggers.

### Notes

- The PDF report is downloaded but **not** attached to the sevDesk invoice — attach it manually if needed
- Chrome runs in headless mode with German locale (`de-DE`)
- Both Harvest login and sevDesk automation share the same browser session
- Date fields are set via the native date picker components (uib-datepicker and daterangepicker) to ensure Angular model consistency

---

## Project structure

```
feed-my-accounting/
├── browser/
│   └── browser.go                # shared headless Chrome context
├── email/
│   └── email.go                  # shared SMTP sending + IMAP fetching
├── travel-expense/
│   ├── travel-expense.go          # Config, calendar logic, Run()
│   ├── doc.go                     # document content builders
│   └── pdf.go                     # PDF generation
├── apple-invoice-pdf/
│   ├── apple-invoice-pdf.go       # Config, Run(), invoice parsing
│   └── pdf.go                     # HTML→PDF conversion, HTML cleanup
├── vodafone-downloader/
│   └── vodafone-downloader.go     # Config, browser automation, Run()
├── harvest-invoice/
│   ├── harvest.go                 # Config, Run(), PDF download via Chrome
│   ├── parser.go                  # email HTML parsing, PDF hours extraction
│   └── sevdesk.go                 # sevDesk browser automation (login, form, save, duplicate check)
├── main.go                        # CLI entry point and config mapping
├── config.go                      # unified YAML config structs
├── config.yaml.example
└── go.mod
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT
