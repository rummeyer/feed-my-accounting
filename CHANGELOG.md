# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.4.0] - 2026-04-10

### Added

- `sevdesk.clientName` config option to override the client name extracted from Harvest email
- `[module]` log prefix for all modules (`[harvest]`, `[vodafone]`, `[apple]`, `[travel]`, `[imap]`)
- `CurrentMonthOnly` flag on IMAP filter — apple-invoice-pdf uses it, harvest-invoice has its own period-based guard

### Changed

- **Breaking:** renamed YAML keys: `user`/`pass` → `username`/`password`, `mitarbeiter` → `employee`, `customers` → `clients`
- Default `filter.count` is now `10` for both apple-invoice-pdf and harvest-invoice
- harvest-invoice now passes `Subject` to the IMAP filter (was ignored before)
- Renamed `customer` → `client` throughout harvest-invoice and travel-expense code
- IMAP filter now checks `FromDomain` before `Subject` for faster elimination of non-matching emails

### Fixed

- Harvest email subject uses Unicode right single quotation mark (`'` U+2019) — default and config now use `\u2019` to match correctly

### Removed

- Unused `ProjectName` field from harvest-invoice `ReportData`

---

## [1.3.0] - 2026-04-10

### Added

- **harvest-invoice** module: downloads Harvest time report PDFs from email, extracts hours, and creates draft invoices in sevDesk via browser automation
- Shared `browser` package for headless Chrome context creation (used by vodafone-downloader and harvest-invoice)
- Unified mail config: all modules now share `imap` and `smtp` sections with camelCase YAML keys
- `currentMonthOnly` guard: skip Harvest reports that are not for the current month (default: `true`)
- `skipExisting` guard: navigates to the sevDesk Rechnungen list and checks for duplicate invoices by matching client name and Rechnungsdatum within the Leistungszeitraum — covers all statuses (Entwurf, Offen, Bezahlt) (default: `true`)
- Unit tests for date parsing, day truncation, and duplicate match logic

### Changed

- README rewritten with narrative intro, architecture diagram, and per-module documentation

---

## [1.2.0] - 2026-03-06

### Added

- `all [M/YYYY]` command to run all modules in sequence (travel-expense, apple-invoice-pdf, vodafone-downloader)
- Running with no command now defaults to `all` for the current month

### Fixed

- vodafone-downloader: pre-compile regexes at package level instead of recompiling on every call
- vodafone-downloader: check all `chromedp.Run` errors instead of silently discarding them
- vodafone-downloader: escape contract type name in JS to prevent format-string injection
- apple-invoice-pdf: add 10s timeout to HTTP client for image embedding (was using default client with no timeout)

---

## [1.1.0] - 2026-03-06

### Changed

- vodafone-downloader: "Aktuelle Rechnung" block is now always tried first regardless of month; previously it was skipped if the shown invoice was not for the current month
- vodafone-downloader: archive fallback now skips the current month's entry to avoid re-downloading an invoice that is still being generated

### Added

- vodafone-downloader: `fallbackToLastMonth` config option (default: `true`) — when `false`, no email is sent if the current month's invoice is not yet available
- vodafone-downloader: suppressed noisy `unhandled node event` log output from chromedp

---

## [1.0.0] - 2026-03-06

### Added

- Initial release combining travel-expense, apple-invoice-pdf, and vodafone-downloader into a single tool
- Unified `config.yaml` with shared `smtp`, `email`, and `imap` sections
- Subcommand CLI: `feed-my-accounting travel-expense [M/YYYY]`, `feed-my-accounting apple-invoice-pdf`, `feed-my-accounting vodafone-downloader`
- `--config` flag to specify a custom config file path
- `--version` / `-v` flag
- Shared `email` package providing SMTP sending and IMAP fetching used by all modules
- Config file searched in current directory, then executable directory (enables placing binary + config together)

---

## travel-expense history

Incorporated at reisekosten v1.11.0.

### [1.11.0] - 2026-02-18

- Added configurable `mitarbeiter` field for Lieferant name in PDF header (sevDesk OCR)
- Added email `cc` field for automatic sevDesk inbox forwarding
- Lieferant name rendered prominently at top of PDF for reliable sevDesk OCR recognition
- Footer label changed from "GESAMTBETRAG" to "Rechnungsbetrag" for sevDesk compatibility

### [1.10.0] - 2026-02-13

- Added comprehensive unit test suite (39 test cases across 19 test functions)

### [1.9.0] - 2026-02-13

- Switched configuration format from JSON to YAML
- Renamed SMTP config fields `username`/`password` to `user`/`pass`
- Updated email subject to "Deine Reisekostenabrechnung"

### [1.8.0] - 2026-02-02

- Abrechnungszeitraum now shows actual date range (first to last workday)
- PDF header shows report type with month/year, centered

### [1.7.0] - 2026-02-02

- Added `--config` command line parameter for custom config file path

### [1.6.0] - 2026-02-02

- Config file search in executable directory as fallback

### [1.5.0] - 2026-02-02

- Added configurable `christmasWeekOff` option to exclude Dec 24, 27–31 (default: `true`)

### [1.4.0] - 2026-02-02

- PDFs generated in memory; email attachments sent directly from memory streams
- Removed temporary file creation and cleanup

### [1.3.0] - 2026-02-02

- Added per-customer `province` setting for German state holidays
- Support for all 16 German states (Bundesländer)
- Holiday calculation uses customer's province instead of a global setting

### [1.2.0] - 2026-02-01

- Added structured document ID format (`RK-YYYY-MM-XXXX`) for sevDesk recognition
- Added sevDesk-friendly labels (Beleg-Nr., Datum, Rechnungsart, Abrechnungszeitraum)
- Professional document layout with ASCII separators
- German decimal format for amounts

### [1.1.0] - 2026-02-01

- Added per-customer `distance` field for mileage calculation
- Multiple customer support with round-robin day distribution
- Smart PDF page breaks (blocks never split across pages)
- Moved credentials and customer info to external config file

### [1.0.0] - 2026-02-01

- Initial release
- Generate Kilometergelderstattung and Verpflegungsmehraufwand PDFs
- German holidays support (Baden-Württemberg)
- Automatic email sending via SMTP
- Month/year argument (`M/YYYY` format)

---

## apple-invoice-pdf history

Incorporated at apple-invoice-pdf v1.5.0.

### [1.5.0] - 2026-03-05

- Added `email.cc` config field for CC recipients (e.g. sevDesk inbox)
- Config file now also searched in executable directory

### [1.4.0] - 2026-02-13

- PDF naming changed to `MM_YYYY_Rechnung_Apple_BESTELLNUMMER.pdf` using order number extracted from invoice HTML
- Falls back to subject-based naming if no order number is found

### [1.2.0] - 2026-02-13

- Restructured config to nested YAML keys (`email.*`, `filter.*`, `user`/`pass`)
- `filter.count` is now optional (omit or set to 0 to scan all messages)
- Only emails from the current month are matched

### [1.1.0] - 2026-02-13

- Switched from wkhtmltopdf to headless Chrome (chromedp) for PDF rendering
- Added configurable outgoing email subject, filter subject/from, sender address, inbox scan count
- Extracted `fetchMatchingUIDs` and `fetchBodies` for readability

### [1.0.0] - 2026-02-13

- Initial release
- IMAP inbox scanning with configurable email count
- HTML-to-PDF conversion using headless Chrome
- Images embedded as base64 data URIs for reliable rendering
- HTML cleanup: removes action buttons, help links, footer link bar; bolds UID-Nr line
- All matching PDFs sent as attachments in a single email via SMTP

---

## vodafone-downloader history

Incorporated at vodafone-downloader v1.8.0.

### [1.8.0] - 2026-03-05

- Added optional `cc` field in email config for additional recipients (e.g. sevDesk mail import)
- Config file now also searched in executable directory

### [1.7.0] - 2026-02-13

- Email body text simplified to "Dokumente anbei."
- Added tests for `loadConfig`, all 12 German months in both `parseInvoiceInfo` patterns, edge cases for `parseArchiveFirstEntry`, `buildMessage`, data integrity, and `sendEmail` — total 68 test cases

### [1.6.0] - 2026-02-13

- Fixed headless Chrome blocked by Vodafone bot detection
- Switched to Chrome's new headless mode (`--headless=new`)
- Added anti-detection: custom user agent, `AutomationControlled` disabled, `navigator.webdriver` removed via CDP

### [1.5.0] - 2026-02-10

- Fixed invoice detection for new Vodafone page layout
- Fixed `parseInvoiceInfo` regex not matching months with umlauts (e.g. März)
- Replaced fixed sleep with content polling (up to 15s) for invoice page loading
- Added archive fallback: automatically downloads first Rechnungsarchiv entry when current month fails
- Added configurable email subject

### [1.4.0] - 2026-02-09

- Replaced raw SMTP/TLS email sending with gomail library
- Extracted `buildMessage` function from `sendEmail` for testability

### [1.3.0] - 2026-02-09

- Added early exit with clear message when invoice is not yet available

### [1.2.0] - 2026-02-09

- PDF filename format changed to `MM_YYYY_Rechnung_Vodafone_Type.pdf`

### [1.1.0] - 2026-02-09

- Only download invoices for current month
- Detect invoice month from page content
- Simplified code structure (~35% reduction)

### [1.0.0] - 2026-02-09

- Initial release
- Login to MeinVodafone portal
- Download Mobilfunk and Kabel invoices
- Send invoices via email with PDF attachments
- In-memory PDF handling (no disk I/O)
- Headless Chrome automation via chromedp
