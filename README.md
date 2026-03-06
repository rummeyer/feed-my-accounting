# feed-my-accounting

[![Version](https://img.shields.io/badge/version-1.2.0-blue.svg)](CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

An accounting orchestrator that automates the generation and delivery of monthly documents to sevDesk. Combines three tools into one:

- **travel-expense** — generates monthly travel expense PDFs (Kilometergelderstattung + Verpflegungsmehraufwand) and sends them via email
- **apple-invoice-pdf** — fetches Apple invoice emails from IMAP, converts HTML to PDF via headless Chrome, and forwards as attachments
- **vodafone-downloader** — logs into MeinVodafone via headless Chrome, downloads Mobilfunk and Kabel invoices, and sends them via email

All three modules share a single `config.yaml` with common SMTP, email, and IMAP settings.

## Requirements

- Go 1.25+
- Google Chrome or Chromium (required by `apple-invoice-pdf` and `vodafone-downloader`)

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
smtp:
  host: "smtp.example.com"
  port: 587
  user: "user@example.com"
  pass: "smtp-password"

imap:
  host: "imap.example.com"
  port: 993

email:
  from: "user@example.com"       # optional, falls back to smtp.user
  to: "recipient@example.com"
  cc: "autobox@sevdesk.email"    # optional

travel-expense:
  mitarbeiter: Max Mustermann
  christmasWeekOff: true         # optional, default: true
  customers:
    - id: "1"
      name: Acme GmbH
      from: "Stuttgart, Heimatbüro (Your Company GmbH)"
      to: "München, Hauptstr. (Acme GmbH)"
      reason: Projektarbeit
      distance: 42
      province: BW

apple-invoice-pdf:
  user: "user@example.com"       # optional, falls back to smtp.user
  pass: "app-specific-password"  # optional, falls back to smtp.pass
  filter:
    count: 10
    subject: "Deine Rechnung von Apple"
    from: "apple.com"

vodafone-downloader:
  user: "your-vodafone-email@example.com"
  pass: "your-vodafone-password"
  fallbackToLastMonth: true      # optional, default: true
```

---

## Module: travel-expense

Generates two PDF documents per month for German business travel expense reporting:

- **Kilometergelderstattung** — mileage reimbursement (distance × €0.30/km, one-way)
- **Verpflegungsmehraufwand** — meal allowance (€14.00 for trips 8h–24h)

### How it works

1. Calculates workdays for the month (excluding weekends, German public holidays, and optionally the Christmas/New Year week)
2. Distributes workdays equally among configured customers using round-robin assignment
3. Generates formatted PDF documents with smart page breaks (blocks never split across pages)
4. Sends both PDFs via email as in-memory attachments (no files written to disk)

### PDF output

- `MM_YYYY_Reisekosten_Kilometergelderstattung.pdf`
- `MM_YYYY_Reisekosten_Verpflegungsmehraufwand.pdf`

PDFs include a structured `Beleg-Nr.` (format: `RK-YYYY-MM-XXXX`) and are formatted for sevDesk OCR recognition. The `mitarbeiter` name is rendered prominently at the top as the Lieferant.

### Customers

Each customer represents a client/destination:

| Field | Description |
|-------|-------------|
| `id` | Identifier shown in the document |
| `name` | Client name used as section header |
| `from` | Origin address (e.g. `"Stuttgart, Heimatbüro (Your Company GmbH)"`) |
| `to` | Destination address (e.g. `"München, Hauptstr. (Acme GmbH)"`) |
| `reason` | Purpose of the trip (e.g. `Projektarbeit`) |
| `distance` | One-way distance in kilometres |
| `province` | German state code for holiday calculation (see below) |

When multiple customers are configured, workdays are distributed round-robin. With 20 workdays and 2 customers each gets 10 days. Mileage is calculated per customer based on their distance.

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
- German public holidays for the customer's province
- Christmas/New Year week off when `christmasWeekOff: true` (the default):
  - December 24
  - December 27–31

  December 25–26 are already excluded as public holidays (Weihnachten). Set `christmasWeekOff: false` to only exclude public holidays.

### travel-expense config options

| Field | Description | Default |
|-------|-------------|---------|
| `mitarbeiter` | Employee name shown as Lieferant in PDF header for sevDesk OCR | — |
| `christmasWeekOff` | Exclude Dec 24 and Dec 27–31 | `true` |
| `customers` | List of customer/trip configurations | required |

---

## Module: apple-invoice-pdf

Fetches Apple invoice emails from an IMAP inbox, converts their HTML body to PDF using headless Chrome, and sends all PDFs as attachments in a single email.

### How it works

1. Connects to the IMAP server and scans the last N emails (or all if `count` is 0)
2. Filters by subject, sender domain, and current month
3. Extracts the HTML body and converts each to an A4 PDF via headless Chrome
4. Embeds external images as base64 data URIs for reliable rendering
5. Names each PDF as `MM_YYYY_Rechnung_Apple_BESTELLNUMMER.pdf` using the order number extracted from the invoice HTML; falls back to subject-based naming if not found
6. Sends all PDFs as attachments in a single email

### apple-invoice-pdf config options

| Field | Description | Default |
|-------|-------------|---------|
| `user` | IMAP username | falls back to `smtp.user` |
| `pass` | IMAP password | falls back to `smtp.pass` |
| `filter.count` | Number of recent emails to scan (0 = all) | `0` |
| `filter.subject` | Exact subject line to match | `Deine Rechnung von Apple` |
| `filter.from` | Sender domain to match | `apple.com` |

The IMAP host and port are configured under the top-level `imap:` key.

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
| `user` | MeinVodafone login email | required |
| `pass` | MeinVodafone password | required |
| `fallbackToLastMonth` | If `true`, send last month's invoice when current month is not yet available. If `false`, skip sending entirely until the current month's invoice is ready. | `true` |

---

## Project structure

```
feed-my-accounting/
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
├── main.go                        # CLI entry point and config mapping
├── config.go                      # unified YAML config structs
├── config.yaml.example
└── go.mod
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT
