# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- README: badges for the release, the build, the licence, the Go version and Buy Me a Coffee — the release read from GitHub, no longer a number to forget. A LICENSE file to go with it, the Sponsor button (`.github/FUNDING.yml`) and a build on GitHub Actions: gofmt, vet, tests and the Pi's binary.

## [1.19.0] - 2026-11-13

### Changed

- Kundenrechnung: *Zeitnachweis hochladen* is now *Manuelle Eingabe*. *Leistung von*, *Leistung bis* and *Stunden gesamt* — in that order — are always there to type in, the current month at first; a work log PDF dropped in, Harvest's or one from here, fills them, correctable as typed. The draft takes what the fields say: no upload needed, none kept on the server. Client, article and reference are not repeated there: they are in the settings.
- Kundenrechnung: *Einstellungen Kundenrechnung* is the card's last flap, after *Manuelle Eingabe*.
- Einstellungen Kundenrechnung: the duplicate check is called *Entwurf nicht doppelt anlegen*, in English *No duplicate drafts*.
- Einstellungen Kundenrechnung: *Kunde in Harvest* before *Harvest-API-Token*.
- Manuelle Eingabe: without a *Kunde in sevDesk* saved, *Entwurf in sevDesk anlegen* says so at once instead of starting a run that fails.
- README: screenshots anew; the architecture's Harvest API reads budgets too; the file tree with `auth.go` and `logo.go`.

### Fixed

- Settings without extra mailboxes no longer count as changed at the first edit anywhere — *Nicht gespeichert: Einstellungen* after dropping a PDF, say.

## [1.18.0] - 2026-11-13

### Added

- harvest-invoice: a project of the work log over its budget in Harvest logs a warning, with the link to the project — for *Nur Zeitnachweis* as for the invoice, like the warning about too few hours. Read from Harvest's project budget report: hours or money, as the project's budget is. A monthly budget only for the current month, Harvest reporting only its spending. Without the report, the work log all the same.
- *Abbrechen* left of *Speichern*: drops what is not saved yet — settings, schedule, Zuordnungen, gesperrte Tage, logo — everything as saved again. Greyed out while nothing is changed; not on the Protokoll.

## [1.17.3] - 2026-11-13

### Changed

- Zeitplan: one line under each entry — *zuletzt: Mo., 30.11.2026, 07:32 (manuell) – nächste: …* — the last run started by hand (*manuell*) or by the schedule (*zeitgesteuert*), the same day as *vor 2 h*, a failed one saying so, and the next runs as many as fit whole. The server sends six of them, not three.
- Zeitplan: the explanation starts *Zeitplan nach Cron-Syntax*, links Quartz's CronTrigger page where its forms for days come in, and says *die Weihnachtswoche, falls aktiviert*.

## [1.17.2] - 2026-11-12

### Fixed

- harvest-invoice: the warning about too few hours counts the client's days, not every workday: the days the Reisekosten plan gives the travel client of the sevDesk client's name — with another client's week away in the month, only the rest. Without such a client, every workday as before.
- Tests and comments use made-up client names only.

## [1.17.1] - 2026-11-12

### Changed

- *Sicher?* blinks again, on *Speichern* and *Löschen* — red and grey by turns, the button staying — until it is clicked or gives up after 4 seconds.

## [1.17.0] - 2026-11-12

### Added

- Protokoll: *Verlauf* and *Löschen* where the other tabs have *Speichern*, asking *Sicher?* first, as *Speichern* does: the runs and their files gone, one running goes on.

### Fixed

- Einstellungen › Mitarbeiter: the rows in fixed columns — name and password field in the same place in every row, whether a password is set or not; the placeholders fit.

## [1.16.1] - 2026-11-12

### Changed

- The tabs: *Reisekosten* before *Belege*.
- English: *Login* and *Logout*, and the tagline *Travel expenses & invoices for accounting*.

## [1.16.0] - 2026-11-12

The web interface in English too, and a calmer Zuordnungen form: short words with pixel icons, on one line.

### Added

- Web interface in **English** too: *DE | EN* at the top right, as on rummeyer.com — the browser's language at first, `?lang=en` in the address, then the one chosen. The login page as well, and the server's messages. Run names from before are shown in English too. The PDFs and mails stay German.

### Changed

- The theme switch with rummeyer.com's pixel icons — sun, moon, half disc — its first click always away from what is on screen.
- Zuordnungen: *über Nacht* is *Übernachtung*, *Hotel mit Rechnung* *Hotel*, *Rückflug über Nacht* *Nachtflug*, each with its pixel icon — moon, bed and a new plane; day trips with the car. An entry shows *Hotel* or *Übernachtung*, not both. In the calendar's legend the moon is *Übernachtung* and the bed *Übernachtungspauschale*. On one line with a short note; with an overnight trip's checkboxes, note and *Zuordnen* go beneath; the entries show the icons too.
- The top row: the next run on the left, *DE | EN* and the theme switch on the right.
- Zeitplan: no heading under its own tab, and the text starts with the cron syntax.
- Einstellungen: *Weitere Postfächer* without the Anthropic example; in English *Other mailboxes*.

## [1.15.0] - 2026-11-12

Security fixes from a check of the page on the internet: a Docker container is no longer at home, and the login gives away less. **Caddy in Docker needs the new proxy token** — see *Behind Caddy with HTTPS* in the README; without it, every visitor through it is asked for a password, at home too.

### Security

- Web interface: Docker's addresses, `172.16–31.…`, are no longer home, and `X-Forwarded-For` from there is not believed: a container on the Pi, open to the internet, could have logged in without a password. Caddy in Docker sends a token instead, `web.proxyToken` (*Proxy-Token* on the web interface), in `X-Proxy-Token`.
- The login page from outside shows no list of employees: the name is typed.
- Twenty failed logins from any addresses within 15 minutes lock all logins from outside for 15 minutes, against a botnet; at home no password is asked. Addresses quiet for 15 minutes are forgotten.
- Uploads capped: the logo at 2 MB, a Zeitnachweis at 10 MB, before it is read.

### Changed

- Web interface: a session ends after 24 hours without a request, not after 30 days; every request renews it, and its cookie with it. The page's polling for the next run does not count: a tab left open logs out too, and shows the login. Sessions from before are cut to 24 hours.
- Protokoll: a run's files one under the other.

## [1.14.0] - 2026-11-12

Receipts from Anthropic, from several mailboxes and any month, to look at before they are sent — Vodafone's too; a schedule that knows the last workday; a warning when Harvest has too few hours.

### Added

- **anthropic-invoice**: Anthropic's receipt emails — the inbox first, then the folder *Rechnungen* — and the receipt PDF each brings, named `11_2026_Rechnung_Anthropic_12112026.pdf` by the email's date, sent like Apple's and Vodafone's. On the web interface first under Belege, in the schedule as *Anthropic-Belege abholen*, and part of `all`.
- The mail module searches several folders in turn and reads attachments; the settings page takes lists, comma-separated.
- Schedule: Quartz's forms for days — `L` the month's last day, `L-n`, `15W` the nearest workday, `LW` the last workday, `5L` the last Friday, `1#2` the second Monday — names for months and weekdays, `?`, and `@monthly` and the other shorthands. New entries start at 7:00.
- **Further mailboxes** (`mail.mailboxes`, on the web interface under Einstellungen › E-Mail › *Weitere Postfächer*): accounts receipts come to besides the main one — Anthropic's to another address, say — searched after it, each in the same folders; the mail still goes out from the main one. Their passwords are kept as the others are: never sent back to the page. One failing does not stop the others.
- Belege: Anthropic and Apple by month — `11/2026`, the current one at first; Vodafone shows its month too, not to be changed — with *PDFs ansehen*, the month's PDFs for download without sending, and *Per Mail senden*, after asking, as the Reisekosten have it. The schedule still sends the current month's. The mail module takes any month.
- Vodafone under Belege too: *PDFs ansehen* and *Per Mail senden*, the current month's invoices.
- Schedule: `W` and `LW` count workdays as the Reisekosten do — no public holidays, blocked days or, when off, the Christmas week. `LW` in December 2026, the 31st blocked, is the 30th.
- Sender filters take several domains, comma-separated. Anthropic's default is `mail.anthropic.com, stripe.com`: receipts from Anthropic Ireland come through Stripe.
- apple-invoice searches the folder *Rechnungen* too, after the inbox (`filter.folders`); the same invoice in both is sent once. It scans the latest 50 emails of each folder by default, not 10.
- harvest-invoice: a warning in the log when the month has fewer hours than its workdays × 8 — workdays as the Reisekosten count them, without holidays and blocked days — with the link to the time entries in Harvest, its address from the API (`https://rummeyer.harvestapp.com/time`). Web addresses in the log are links.

### Changed

- Protokoll: a run's output opens under its entry, a second click closes it. 50 runs are kept, not 100, the oldest going first. Files gone with a restart are listed by name, not as links; a page left open across a restart turns its dead links into text when shown again.
- harvest-invoice: the work log without an invoice number — with *Nur Zeitnachweis*, or none found in sevDesk — is `11_2026_Rechnung_WorkLog.pdf`, not `11_2026_WorkLog.pdf`.

### Fixed

- apple-invoice: App Store invoices — an app or in-app purchase — are named by their order number as well, `11_2026_Rechnung_Apple_MK5YH4MJ4X.pdf`, not by the subject: their label is *BESTELLNR.*, the number in the next cell, where subscription invoices have *Bestellnummer:*.

## [1.13.0] - 2026-11-12

### Added

- Web interface: **logging in**. Every employee can have a password (Einstellungen › Mitarbeiter, kept as a bcrypt hash in `staff.logins`); as soon as one has, the page asks for a login — at home just the employee, from outside the employee and their password. Sessions of 30 days in a cookie scripts cannot read, Secure over HTTPS, SameSite strict; five failed logins lock an address for 15 minutes; changing calls only from the page itself; security headers (CSP, no framing). *Abmelden* at the top.
- Behind a reverse proxy — Caddy, on the Pi or elsewhere in the home network — the client's address counts, from `X-Forwarded-For`, not the proxy's: a visitor from the internet is not at home through it.

### Changed

- Web interface: the employee — and *Abmelden*, once logged in — sits centred above the page, bigger and bold, no longer in its corner.

### Removed

- `web.password` and HTTP basic auth: on starting, a password set there becomes the selected employee's login.

### Fixed

- Web interface: the month next to *Aus Harvest anlegen* always starts at the current month — the browser restored an emptied field over it — and its hint follows the month too.

## [1.12.0] - 2026-11-12

### Changed

- Web interface: the link to the BMF's table of the rates abroad goes to the table in the Lohnsteuer-Handbuch of the year of the month shown — the year before's while the year's is not out yet — through `/api/bmf-table`: no new link each year.
- Web interface: on Kundenrechnung, *Einstellungen Kundenrechnung* comes right after *Aus Harvest*, and the PDF upload, the fallback, is folded away beneath it behind *Zeitnachweis hochladen* — Harvest's report or one from here.
- Web interface: the Reisekosten totals say *Übernachtung* without the count of nights, every box's heading one line high.
- Web interface: the Reisekosten calendar shows each client in its own colour, in the order of the client list — light blue, green, sand, lavender, rose, turquoise — the legend naming them; an overnight trip's days carry a moon, as the colour no longer tells them. Pixel icons of 16 × 16, shaded, mark what an amount is for — fork and knife the meals, a car the mileage, a bed the night — in the calendar, the totals and a second line of the legend; in shades of the text's colour, light on dark in the dark theme.

## [1.11.0] - 2026-11-12

### Added

- travel-expense: **Übernachtungspauschale** — the flat rate for each night of an overnight assignment without a hotel invoice, as a third PDF. Every day of the trip but the last starts a night, in the month it starts in. The rate is the client's `lodging` — abroad the BMF's for the place, Miami 256 € — else the inland `lodgingRate`, 20 € by law. An assignment with `hotel` (*Hotel mit Rechnung*) has none; one with `nightFlight` (*Rückflug über Nacht*, from the USA say) none for the night on the plane before the day of arrival. The calendar shows each night's rate, the totals the nights and their sum.

### Changed

- Web interface: the logo as a card, as sevDesk has it — empty, a button and room to drop a file; with a logo, a red × over it while the mouse is on it, and *Firmenlogo löschen* or *Abbrechen* before it goes. At most 2 MB.
- Work log: a logo small already — no wider than 680 px, no larger than 150 KB, a JPEG or a PNG with an alpha channel — goes in as uploaded; a PNG with a palette and transparent colours is shrunk, as fpdf draws those colours opaque.
- Web interface: *Verpflegung Inland* is *Pauschalen Inland*, with the night's rate.

### Fixed

- travel-expense: umlauts in the PDFs — the Lieferant, a client's address, "Übernachtung" — are written in the PDF's own encoding, not as garbled UTF-8.
- Web interface: the browser asks for the page's files again each time — a page kept from before an update ran its old script and dropped a new field when saving.

## [1.10.0] - 2026-11-12

### Added

- **Company** (`company.name`) and its **logo**, on the web interface under Einstellungen › Firma: the work log shows the logo at its head — a PNG or JPEG kept beside the configuration — sized and placed as sevDesk does on its invoices — else the name, else Harvest's account name. The PDF carries the logo shrunk to 680 px — 300 dpi at its size — as a JPEG on white: some 60 KB, not the megabyte an uploaded one may have; the file uploaded stays as it is. A logo replaced or removed is kept as its `.bak`.
- The work log is in German: *Zeitnachweis*, Zeitraum, Kunde, Projekt, Tätigkeit, Mitarbeiter, Stunden, Seite x von y. The upload reads it too.
- **Employees** (`staff.names`, `staff.current`), on the web interface under Einstellungen › Mitarbeiter, the selected one shown at the top right. The selected employee is the Lieferant on the travel expenses, and only their hours in Harvest make the work log and the invoice — named as in Harvest. `travel-expense.employee` moves there by itself, as the first employee.

- harvest-invoice: the month's **work log** as PDF (`10_2026_Rechnung_0202_WorkLog.pdf`, after the number of the invoice in sevDesk; `10_2026_WorkLog.pdf` without one), made from the Harvest API's time entries and laid out as Harvest's *Detailed time report* — period, hours, uninvoiced billable hours, and a table by day with the times and notes. It comes with every run, also when the draft exists already: for download on the web interface, saved in the current directory from the command line. *Nur Zeitnachweis* on the web interface makes only the work log. The upload reads it like Harvest's own.
- `pdfdoc`: what the PDFs have in common — A4 pages, blocks never split across them, the `Document` — for the travel expenses and the work log.

- harvest-invoice: the hours come from the Harvest API v2, with a personal access token (`harvest.token`, required; optionally `harvest.accountId`): the time entries of the client (`harvest.client`, else `sevdesk.clientName`) in the month — the current one, the one given (`harvest-invoice 10/2026`) or the one given on the web interface next to *Aus Harvest anlegen*, as `10/2026`. Works on the free plan.

### Removed

- harvest-invoice: the export mail and the Harvest web login — Harvest's sign in has an hCaptcha that turns the automated browser away. Gone with them: `currentMonthOnly`, `filter`, `harvest.username`, `harvest.password`, the PDF download and offering it with the run. A Harvest PDF report goes in only by upload on the web interface, as before.

### Changed

- travel-expense: a client with assignments gets only its assigned days; the remaining workdays are shared by the clients without assignments in consecutive blocks of equal size, in the order they are listed, rather than alternating day by day. `onlyAssigned` is gone — the assignments say it.
- Web interface: the header shows the next scheduled run; Zuordnungen start without a client chosen and need one.
- Web interface: Zuordnungen and gesperrte Tage are edited as drafts — add, *bearbeiten*, *entfernen*.
- Web interface: one red *Speichern* at the bottom of the page saves every change on every card, after asking *Sicher?*; the separate save buttons are gone, and switching cards keeps changes not saved yet.

- travel-expense: clients outside Europe never take part in the equal distribution — only their assigned days count. Clients in Europe still do.

- Web interface: the first card is called Zeitplan.

### Fixed

- Web interface: the header's logo is the same size in the dark theme — its light patch took its room from the logo, and the page beneath jumped.
- harvest-invoice: the duplicate check waits for sevDesk's Rechnungen list to fill in — read at once, it showed no invoices, and a draft there already could be made again — and a check that fails stops the run instead of going on to make the invoice.
- harvest-invoice: the work log's invoice number is read as sevDesk's list writes it beneath the client, "Rechnung Nr. 0203"; a draft without one takes the number after the highest of the invoices that are not drafts. After saving, the list is opened again until it shows the new draft.
- harvest-invoice: the sevDesk login waits for the cookie banner, which comes seconds after the login form and kept it from taking input ("Element is not focusable").
- travel-expense abroad: a day trip of more than 8 hours takes the client's travel day rate when it has no day trip rate, and the other way round — the BMF's table has one rate for both. Only with neither does the inland rate apply; the web interface marks such days red, names the missing rate under the totals and frames the empty field.

## [1.9.1] - 2026-11-11

### Changed

- `web.access` and `web.proxies` give way to one switch, `web.homeOnly` (default: on), on the web interface *Nur im Heimnetz erreichbar*. The proxy mode is gone.

## [1.9.0] - 2026-11-11

### Added

- `web.access`: `home` (default) answers only the home network — private, loopback and link-local addresses — and gives everything else *403*; `proxy` answers only requests a reverse proxy such as Caddy passes on from `web.proxies` with HTTPS, and only with `web.password` set — never directly. Read afresh for every request; on the web interface under Einstellungen › Oberfläche, with a warning before switching to `proxy`.

## [1.8.0] - 2026-11-11

### Added

- Inland meal rates (`travel-expense.mealRates`: `day`, `fullDay`, `travelDay`), used by every client without rates of its own; on the web interface under *Verpflegung Inland*. Left out, they are the legal 14 / 28 / 14 €, so a change in the law is one edit rather than a new version.

## [1.7.1] - 2026-11-11

### Changed

- Web interface: every setting is on the card where it is used — clients on Reisekosten, Harvest and sevDesk on Kundenrechnung, Apple and Vodafone on Belege — each part with its own save button. Einstellungen keeps only the mail account and the web interface. Empty fields show their default.

## [1.7.0] - 2026-11-11

### Added

- travel-expense clients in any of 36 countries (`country`, ISO 3166 code, default `DE`): day trips skip that country's public holidays; the German state (`province`) applies to `DE` only. An unknown code is an error.
- A run warns when a client abroad falls back to a German meal rate.
- Settings: a country per client, the state only for Germany, and the country's rates asked for abroad.

## [1.6.1] - 2026-11-11

### Fixed

- travel-expense PDFs: every amount is flush right, in line with the total; a long label such as `Verpflegungsmehraufwand (An-/Abreisetag)` no longer runs into its amount.

## [1.6.0] - 2026-11-11

### Added

- **Web interface** (`feed-my-accounting serve`, `web.listen`, default `:8595`): register cards for Cron, Reisekosten, Kundenrechnung, Belege, Protokoll and Einstellungen; every run with its live output and PDFs; optional password (`web.password`).
- **Built-in schedule** (`schedule`): cron expressions per job, run by `serve` — replaces the crontab. One job at a time; a job due while another runs is retried every minute for half an hour.
- **Travel expense assignments** (`travel-expense.assignments`): date ranges given to a client explicitly, as day trips or **overnight** — every day counts, An-/Abreisetag and full day rates, mileage only for the drive there and back. The remaining workdays are still distributed equally; `onlyAssigned` keeps a client out of that.
- **Meal allowance rates per client** (`mealDay`, `mealFullDay`, `mealTravelDay`), for trips abroad. Defaults remain 14 / 28 / 14 €.
- **Blocked days** (`travel-expense.blocked`): vacation and the like, without travel.
- **Harvest PDF upload**: hours and period read from an uploaded report, checked by hand, then the sevDesk draft — `harvest.CreateDraft` and `harvest.ParseReportPDF`.
- `feed-my-accounting.service`, a systemd unit for the web interface.

### Changed

- travel-expense is split into `Plan`, `Generate` and `Send`; `Run` does all three. A month without travel days is logged as nothing to send instead of producing empty PDFs; a month without driving sends only the Verpflegungsmehraufwand PDF.
- Saving from the web interface rewrites `config.yaml` without comments and keeps the previous file as `config.yaml.bak`.

## [1.5.0] - 2026-04-11

### Changed

- **Breaking:** unified module naming — renamed `apple-invoice-pdf` → `apple-invoice` and `vodafone-downloader` → `vodafone-invoice`. Affects the subcommand names, the YAML config keys under the top-level config, and the module directory names.
- Log prefixes now match module names: `[apple-invoice]`, `[vodafone-invoice]`, `[harvest-invoice]`, `[travel-expense]` (was `[apple]`, `[vodafone]`, `[harvest]`, `[travel]`). Shared email package prefix renamed `[imap]` → `[email]`.

### Added

- Automatic log file: all module output is tee'd to `feed-my-accounting.log` next to the executable (append mode), in addition to stderr. If the file can't be opened, a warning is printed and the tool continues with stderr-only output.
- Each module package exposes a `SetLogOutput(io.Writer)` function so main can redirect logs at startup.
- `email.Send` now logs subject, recipient, and attachment count before dialing SMTP, plus a success line after.

## [1.4.0] - 2026-04-10

### Added

- `sevdesk.clientName` config option to override the client name extracted from Harvest email
- `[module]` log prefix for all modules (`[harvest]`, `[vodafone]`, `[apple]`, `[travel]`, `[imap]`)
- `CurrentMonthOnly` flag on IMAP filter — apple-invoice-pdf uses it, harvest-invoice has its own period-based guard
- E-Rechnung reference type dropdown: automatically selects "Bestellung (vom Käufer) BT-13" when setting reference number
- Suppress noisy chromedp `unhandled node event` log messages

### Changed

- **Breaking:** renamed YAML keys: `user`/`pass` → `username`/`password`, `mitarbeiter` → `employee`, `customers` → `clients`
- Default `filter.count` is now `10` for both apple-invoice-pdf and harvest-invoice
- harvest-invoice now passes `Subject` to the IMAP filter (was ignored before)
- Renamed `customer` → `client` throughout harvest-invoice and travel-expense code
- IMAP filter now checks `FromDomain` before `Subject` for faster elimination of non-matching emails
- Replaced fixed 8s sleep after sevDesk login with `WaitVisible` for faster navigation
- Rechnungen menu navigation retries up to 3 times for slower machines (e.g. Raspberry Pi)

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
