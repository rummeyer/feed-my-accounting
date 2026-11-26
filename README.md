# feed-my-accounting

<p align="center">
  <img src="docs/logo.png" alt="feed-my-accounting: a suitcase packed with a boarding pass and the Vodafone, Apple, Harvest and sevDesk invoices, on a pixel world map with a flight path" width="640">
</p>

<p align="center">
  <a href="https://github.com/rummeyer/feed-my-accounting/releases"><img src="https://img.shields.io/github/v/release/rummeyer/feed-my-accounting?label=release" alt="Release"></a>
  <a href="https://github.com/rummeyer/feed-my-accounting/actions/workflows/build.yml"><img src="https://github.com/rummeyer/feed-my-accounting/actions/workflows/build.yml/badge.svg" alt="Build"></a>
  <a href="https://github.com/rummeyer/feed-my-accounting/blob/main/LICENSE"><img src="https://img.shields.io/badge/licence-MIT-blue" alt="Licence"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/rummeyer/feed-my-accounting" alt="Go version"></a>
  <img src="https://img.shields.io/badge/runs%20on-Raspberry%20Pi%20%C2%B7%20Linux-c51a4a" alt="Runs on Raspberry Pi and Linux">
  <a href="https://sevdesk.de"><img src="https://img.shields.io/badge/accounting-sevDesk-orange" alt="For sevDesk"></a>
</p>

<p align="center">
  <a href="https://buymeacoffee.com/rummeyer"><img src="https://img.shields.io/badge/donate-Buy%20Me%20a%20Coffee-yellow" alt="Buy Me a Coffee"></a>
</p>

A single command that closes your books for the month.

Every month, freelancers and small businesses juggle the same ritual: collect invoices from Amazon, Apple, Vodafone and whoever else mails them, generate travel expense reports, turn Harvest timesheets into outgoing invoices, feed everything into the accounting system — none of it twice — and tell the tax adviser once the month is booked. **feed-my-accounting** automates all of it — from source to [sevDesk](https://sevdesk.de).

It connects its sources — mailboxes, Amazon Business, MeinVodafone, the Harvest API and your calendar — to your sevDesk account, each handled by a dedicated module:

| Module | Source | What it does | Delivery |
|--------|--------|-------------|----------|
| **travel-expense** | Config (clients, distances) | Generates Kilometergelderstattung, Verpflegungsmehraufwand and Übernachtungspauschale PDFs | Email → sevDesk Autobox |
| **apple-invoice** | IMAP inbox and *Rechnungen* | Fetches Apple invoice emails, converts HTML to PDF via headless Chrome | Email → sevDesk Autobox |
| **amazon-invoice** | Amazon Business (amazon.de) | Logs in via headless Chrome, downloads the invoices of the month's orders | Email → sevDesk Autobox |
| **vodafone-invoice** | MeinVodafone portal | Logs in via headless Chrome, downloads Mobilfunk + Kabel invoices | Email → sevDesk Autobox |
| **mail-receipts** | IMAP inbox and *Rechnungen* | Takes the PDF invoices of senders by a rule each — Anthropic's receipts, Strato, the tax adviser, set up on the web interface | Email → sevDesk Autobox |
| **harvest-invoice** | Harvest API | Reads the month's hours of a client from the Harvest API, makes the work log PDF, creates the draft invoice in sevDesk | Direct browser automation |
| **invoice-mails** | IMAP inbox | Takes the copy sevDesk mails of each invoice sent, by a rule per customer | Storage (rclone) and Paperless |

### How it all fits together

<p align="center"><img src="docs/architecture.svg" alt="Architecture: the web interface, the Zeitplan and the command line start feed-my-accounting, one Go binary with one config.yaml. Its sources — mailboxes over IMAP, the Amazon Business and MeinVodafone portals, the Harvest API with time entries and project budgets, and the calendar with clients, vacation and assignments — feed six modules: mail-receipts, apple-invoice, amazon-invoice, vodafone-invoice, harvest-invoice and travel-expense. Five of them send their PDFs by email to the sevDesk Autobox; harvest-invoice creates a draft E-Rechnung in sevDesk with headless Chrome, with the hours the budget allows. A ledger keeps each receipt from going twice and shows the month on Abschluss, with Pushover and the tax adviser; every run's PDFs and log are in the Protokoll. It runs on a Raspberry Pi behind Caddy with HTTPS." width="900"></p>

Five modules — mail-receipts, amazon, apple, vodafone and travel-expense — collect documents and deliver them as email attachments to sevDesk's **Autobox** — an inbox that automatically imports and OCR-parses incoming PDFs as bookkeeping records — each receipt once, none you filed by hand. The sixth (harvest-invoice) goes further: it logs into sevDesk directly and creates a fully populated draft invoice, ready for review.

Run once a month with a single command — or schedule it with cron — and your accounting is fed.

All modules share a single `config.yaml` with a common `mail` block for SMTP/IMAP credentials and addresses.

Run them from the command line or cron as before — or start the **web interface** (`feed-my-accounting serve`): first the month's **closing** — what came in, what is still missing, the tax adviser told — then a register card per area of work, a month calendar for the travel expenses with vacation and client assignments, Harvest PDF upload for the sevDesk draft, a built-in **schedule** that replaces crontab, a live log of every run, and the settings. See [Web interface](#web-interface).

## Requirements

- Go 1.25+
- Google Chrome or Chromium (required by `amazon-invoice`, `apple-invoice`, `vodafone-invoice`, and `harvest-invoice`)

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
feed-my-accounting apple-invoice

# Vodafone invoices → email
feed-my-accounting vodafone-invoice

# PDF invoices by the rules set — Anthropic, Strato, … → email
feed-my-accounting mail-receipts

# Harvest → sevDesk draft invoice (current month, or the one given)
feed-my-accounting harvest-invoice
feed-my-accounting harvest-invoice 10/2026

# The invoices sent, from sevDesk's copies → storage and Paperless (this month and the one before)
feed-my-accounting invoice-mails

# Web interface with schedule (web.listen, default :8595)
feed-my-accounting serve

# Custom config file
feed-my-accounting --config /path/to/config.yaml all 3/2026

# Show version
feed-my-accounting --version
```

## Web interface

```bash
feed-my-accounting serve          # http://<host>:8595
```

<p align="center">
  <img src="docs/closing.png" alt="The Abschluss card for November 2026 with made-up receipts: a road block, the month under way, still open the Kundenrechnung and the tax adviser's invoice; source by source the receipts sent — travel expenses, Apple, Vodafone, Anthropic, Strato — an Amazon invoice filed by hand, each with an (×) to send it again and the next scheduled run; beneath, the tax adviser to be told, with all receipts as a zip" width="760">
</p>

<p align="center">
  <img src="docs/travel.png" alt="The Reisekosten card for December 2026 with fictional companies: each client in its own colour — an overnight trip to Miami for Globex Inc. from the 3rd to the 9th with a moon on each day, day trips to Acme GmbH in light blue and Initech GmbH in green in blocks on the other workdays, three vacation days hatched, the Christmas week grey, and the month's totals" width="760">
</p>

<p align="center">
  <img src="docs/cron.png" alt="The Zeitplan card: six jobs in the order they run next, each with its cron expression and next runs — Vodafone on the 21st, Weitere Absender, Amazon and the Kundenrechnung on the last day and workday" width="380">
  <img src="docs/log.png" alt="The Protokoll card: a run's live output and the three PDFs it made" width="380">
</p>

<p align="center"><sub>Screenshots with made-up customers, people and data — the company, its name and logo, is the author's. The page speaks German or English — the browser's language at first, then *DE | EN* at the top right; the PDFs and mails stay German, for the German books. Dark mode follows the system, or the switch beside it — sun, moon, half disc, as on rummeyer.com: <a href="docs/travel-dark.png">Reisekosten in dark</a>, <a href="docs/invoice.png">Kundenrechnung</a>, <a href="docs/settings.png">Kunden</a>, <a href="docs/company.png">Firma und Mitarbeiter</a>, <a href="docs/worklog.png">a Zeitnachweis</a>, <a href="docs/login.png">the login</a>.</sub></p>

`serve` starts a small web server — one Go binary, the page embedded — with a register card per area of work. Everything it does is also what the command line does; it reads `config.yaml` afresh on every request, so edits by hand count at once.

A button that sends or transfers something — a mail, the draft to sevDesk, a Pushover test — is solid black; one that only shows or edits here is light. Every setting is on the card where it is used. Nothing saves by itself: one red *Speichern* at the bottom of the page saves every change on every card — settings, schedule, Zuordnungen, gesperrte Tage — and says beside it what is not saved yet. The first click turns it into *Sicher?*, the second within four seconds saves. *Abbrechen* beside it drops what is not saved yet, everything as saved again.

| Card | What it does |
|------|--------------|
| **Abschluss** | The first card, the overview: the month's state at a glance — a green tick when every source is done and the invoice drafted in sevDesk, a road block while it is under way, a red cross while nothing has happened yet — then a month's books, source by source: the receipts sent for it, with when, each with an (×) that takes it out of the ledger — several at a time, the next run sends them again, until then struck through; *nichts gefunden* where a run looked and found none; *offen*, in red, where the schedule runs a source that has neither sent nor looked yet; its last run's error, and its next run. Until the closing day the month before, after it the current one; ‹ › for others. Beneath, *Steuerberater informieren*: the mail to the tax adviser as set, *Belege 11/2026* — with *Alle Belege als ZIP anhängen* the month's receipts from the storage zipped beside it — and when he was told. See [Never twice, and the month's closing](#never-twice-and-the-months-closing). |
| **Zeitplan** | The schedule: each job with its cron expression (`minute hour day month weekday`, as in crontab, with Quartz's forms for days — see below), and beneath it in one line when it last ran — *manuell* or *zeitgesteuert*, the same day as *vor 2 h* — and its next runs, as many as fit; and a switch to turn it off. Runs by hand start on their own cards. Replaces the crontab entries — keep only one of the two, or jobs run twice. |
| **Reisekosten** | A month calendar: which client gets which day, day trips and overnight trips, holidays, blocked days, and each day's meal allowance and mileage, with the month's totals. *Zuordnungen* gives date ranges to a client explicitly; *Gesperrte Tage* blocks vacation and the like. Both are edited as drafts — add, *bearbeiten*, *entfernen* — and saved with the *Speichern* at the bottom; until then nothing changes. *PDFs ansehen* generates the two PDFs to look at; *Per Mail senden* sends them to the address under Einstellungen › E-Mail. Below: the clients — country, state, distance, meal rates — the name on the PDFs and the Christmas week. |
| **Belege** | Amazon, Apple and Vodafone invoices on demand, each with its settings: the Amazon and Vodafone logins, the Apple mail filter. Apple with *PDFs ansehen*, fetching them for download, and *Per Mail senden*, sending them too, after asking; Amazon and Vodafone with *Per Mail senden* only, as each run is a login, Vodafone's held to one in 28 days: Amazon and Apple by month — `11/2026`, the current one at first — Vodafone the current month's. The schedule sends the current month's. Last, *Weitere Absender*: the rules for further senders, by month to look at or to send — see [mail-receipts](#module-mail-receipts). |
| **Kundenrechnung** | *Aus Harvest anlegen* runs harvest-invoice for the month given next to it as on the command line (`11/2026`): the hours come from the Harvest API, and the month's work log PDF comes with the run for download — on iCloud Drive, say. *Nur Zeitnachweis* makes only the work log. Fewer hours than the days booked × 8 log a warning with the link to the time entries in Harvest: each day with an entry a workday of 8 hours at least, however many days a week are worked for the client. A project of the work log over its budget in Harvest — hours or money, as Harvest's project budget report has it — logs a warning too, with the link to the project; a monthly budget only for the current month, Harvest reporting only its spending. As a fallback, behind its own flap at the foot of the card, *Manuelle Eingabe*: *Leistung von*, *Leistung bis* and *Stunden gesamt* typed in — the current month at first — or filled from a work log PDF dropped in, Harvest's or one from here, and correctable either way; then *Entwurf in sevDesk anlegen* creates the draft invoice for the client, article and reference of the settings beneath — with the same duplicate check. Then *Aus dem Postfach*: the invoices sent, from the copies sevDesk mails, by month to look at (*PDFs ansehen*) or to store (*Ablegen*), with their rules folded away — see [Invoices from the mailbox](#invoices-from-the-mailbox). Folded away last on the card, *Einstellungen Kundenrechnung*: the sevDesk client, article and reference, the sevDesk login, the Harvest API token and the client's name in Harvest. |
| **Protokoll** | The last 50 runs; a click opens a run's output under it, with the PDFs it made to download. The history is kept in `feed-my-accounting-jobs.json` next to the config; the PDFs until the server restarts, then listed by name only. Web addresses in the output are links. *Löschen* at the bottom, after *Sicher?*, empties it. |
| **Einstellungen** | What all of them share: the **company** — its name, and its logo (PNG or JPEG, kept beside the configuration as `feed-my-accounting-logo.png`), which the work log shows instead of the name — the **employees** — the one ticked is the Lieferant on the travel expenses and whose hours in Harvest make the work log and the invoice, named as in Harvest; shown at the top right as *Mitarbeiter* — the mail account, the storage, **Paperless**, **Pushover** with *Test senden*, and the web interface's address and password. |

Passwords are never sent to the page: leave a password field empty to keep it. A field left empty shows its default as a hint.

**Saving from the web interface rewrites `config.yaml`** — values intact, comments gone — and keeps the previous file as `config.yaml.bak`.

One job runs at a time; a scheduled job due while another runs is tried again every minute for half an hour.

### Running it as a service

[`feed-my-accounting.service`](feed-my-accounting.service) runs the web interface and its schedule at boot. Set `User`, `WorkingDirectory` and the paths to yours, then:

```bash
sudo cp feed-my-accounting.service /etc/systemd/system/
sudo systemctl enable --now feed-my-accounting
journalctl -u feed-my-accounting -f
```

### Cron expressions

Five fields, `minute hour day month weekday`, as in crontab — `*`, numbers, ranges `1-5`, lists `1,15` and steps `*/15` — a day matching when its day of month or its weekday does, both being restricted. Beyond that, what Quartz has for days:

| Form | Field | Means | Example |
|------|-------|-------|---------|
| `L` | day | the month's last day | `0 7 L * *` — the 30th, 31st, 28th… at 7:00 |
| `L-n` | day | n days before it | `0 7 L-2 * *` |
| `nW` | day | the workday nearest the nth, within the month | `0 7 15W * *` |
| `LW` | day | the month's last workday | `0 7 LW * *` |

A workday for `W` and `LW` is one as the Reisekosten count it: Monday to Friday, no public holiday — Germany's, in the state of the first German client — no blocked day, and not in the Christmas week if that is off. With the 31st of December blocked, `LW` in December 2026 is the 30th.
| `nL` | weekday | the month's last such weekday | `0 7 * * 5L` — the last Friday |
| `n#k` | weekday | the kth such weekday | `0 7 * * 1#2` — the second Monday |
| `JAN`–`DEC`, `SUN`–`SAT` | month, weekday | names for the numbers | `0 7 * * MON-FRI` |
| `?` | day, weekday | the same as `*` | |
| `@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly` | — | shorthands, at midnight | `@monthly` |

New entries start at `0 7 1 * *`, the 1st at 7:00.

### Who may reach it

The page holds the keys to your mail, Vodafone, Harvest and sevDesk accounts, so by default it answers **the home network only** (`web.homeOnly`, on the web interface *Nur im Heimnetz erreichbar*): private addresses (`10.…`, `192.168.…`), localhost and local IPv6 — not `172.16–31.…`, where Docker puts its containers: one of them, open to the internet, is not at home. Anything else gets *403*, even if the port is open to the internet by mistake. A VPN such as Tailscale uses addresses outside the private ranges (`100.64.…`) and is not let in.

### Logging in

Every employee can have a **password** (Einstellungen › Mitarbeiter, at least 8 characters, kept only as a bcrypt hash in `staff.logins`). As soon as one has, every page and call needs a session:

- **At home**, the login page asks only for the employee — a dropdown and *Anmelden*, no password. A password lost is set anew from there; no reset by mail is needed.
- **From outside** — `web.homeOnly` switched off, through a reverse proxy with HTTPS — it asks for the employee's name, typed — no list of employees shows — and their password.

<p align="center"><img src="docs/login.png" alt="The login page from outside: the employee's name typed, Max Mustermann, and the password field beneath, with Anmelden" width="480"></p>

As long as no employee has a password, the page answers the home network only, whatever `web.homeOnly` says. A session ends after 24 hours without a request — the page's own polling does not count, so an open tab logs out too and shows the login — and each request renews it; it is in a cookie the page's scripts cannot read, sent over HTTPS only when the browser talks HTTPS, and only from this site's pages; the Pi keeps the sessions' hashes, not their tokens, in `feed-my-accounting-sessions.json`, so a restart logs no one out. A new password ends the employee's other sessions. Five failed logins from one address lock it for 15 minutes; twenty from any addresses within 15 minutes, as a botnet would try, lock all logins from outside for 15 minutes — at home no password is asked, so it still lets in. Uploads are capped: the logo at 2 MB, a Zeitnachweis at 10 MB. Calls that change something are taken only from the page itself, and the page loads only its own scripts and cannot be framed.

### Behind Caddy with HTTPS

A reverse proxy on the Pi or anywhere else in the home network: the web interface takes the client's address from `X-Forwarded-For` — the entry the proxy adds, last — when the request comes from a home address, so a visitor from the internet does not count as at home through it. Caddy sets the header by itself:

```
accounting.example.com {
	reverse_proxy 192.168.178.10:8595   # the Pi; localhost:8595 with Caddy on the Pi
}
```

**Caddy in Docker** reaches the Pi from a Docker address, `172.…`, which counts neither as home nor as a proxy to believe — else any container could claim to be at home. Give it a token: a long random one, `openssl rand -hex 32`, as `web.proxyToken` (on the web interface *Proxy-Token*, under *Nur im Heimnetz erreichbar*) and in the Caddyfile:

```
accounting.example.com {
	reverse_proxy 172.18.0.1:8595 {   # the Docker network's gateway: the Pi
		header_up X-Proxy-Token "the same token"
	}
}
```

Keep the Caddyfile readable for its owner only: it holds the token.

Then switch off *Nur im Heimnetz erreichbar* and give the employees who log in from outside a password.

## Configuration

Copy `config.yaml.example` to `config.yaml` and fill in your details:

```bash
cp config.yaml.example config.yaml
```

The config file is searched in the following order:
1. Path provided via `--config`
2. Current working directory
3. Directory of the executable

## Logging

All module output is written to both stderr and `feed-my-accounting.log` next to the executable. The file is opened in append mode at startup — rotate it externally if it grows too large. Each module uses a prefixed logger (`[apple-invoice]`, `[vodafone-invoice]`, `[harvest-invoice]`, `[travel-expense]`, `[email]`) so you can `grep` per module.

### Full config reference

```yaml
company:
  name: "Muster GmbH"            # the work log's head, unless there is a logo; default: Harvest's account name

staff:                           # who works with the tool
  names: ["Max Mustermann", "Erika Musterfrau"]
  current: Max Mustermann        # the Lieferant on the travel expenses, whose Harvest hours count

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
  mailboxes:                     # optional: further accounts receipts come to, searched after this one
    - username: "max@example.de"
      password: "the-other-password"
      # imapHost, imapPort: optional, default this one's

travel-expense:
  christmasWeekOff: true         # optional, default: true
  mealRates:                     # optional: inland rates, default the legal 14 / 28 / 14
    day: 14
    fullDay: 28
    travelDay: 14
  clients:
    - id: "1"
      name: Acme GmbH
      from: "Stuttgart, Heimatbüro (Your Company GmbH)"
      to: "München, Hauptstr. (Acme GmbH)"
      reason: Projektarbeit
      distance: 42
      province: BW
    - id: "2"
      name: Globex Inc.
      from: "Stuttgart, Heimatbüro (Your Company GmbH)"
      to: "Miami, Brickell Ave (Globex Inc.)"
      reason: Projektarbeit vor Ort
      distance: 25               # the drive to the airport, first and last day
      country: US                # optional, default DE: whose public holidays count
      mealFullDay: 65            # abroad: the country's rates (BMF table)
      mealTravelDay: 44
      lodging: 256               # a night without a hotel invoice (BMF table)
  blocked:                       # optional: no travel on these days
    - from: 2026-08-03
      to: 2026-08-14
      note: Urlaub
  assignments:                   # optional: days given to a client explicitly
    - client: "2"                # id or name
      from: 2026-12-03
      to: 2026-12-09
      overnight: true
      note: Miami

apple-invoice:
  filter:
    count: 1000                    # the month's emails at the most, in each folder
    from: "apple.com"
    subject: "Deine Rechnung von Apple"
    folders: [INBOX, Rechnungen]   # searched in this order: the inbox first

mail-receipts:                     # optional: further senders' PDF invoices, a rule each
  count: 1000                      # default
  folders: [INBOX, Rechnungen]     # default
  rules:
    - name: Anthropic              # in the file names and the mail's subject
      from: "mail.anthropic.com, stripe.com"
      subject: "Your receipt from Anthropic"
      attachment: "receipt"        # the receipt, not the invoice beside it
    - name: Strato
      from: "strato.de"            # sender domains, comma-separated
      subject: "Rechnung"          # optional: the subject contains it, whatever the case
      attachment: ""               # optional: the PDF's name contains it
    - name: Steuerberater
      from: "datev.de"
      subject: "Rechnung"
      off: true                    # switched off: left out

advisor:                           # optional: the tax adviser, told on Abschluss
  to: "steuerberater@example.com"
  from: "me@example.com"           # optional: the sender, mail.from being the tool's own, say
  bcc: "me@example.com"            # optional: a blind copy, to yourself say
  subject: "Belege {MM}/{YYYY}"    # default; {MONAT} is the month's name
  text: "Hallo,\n\nalle Rechnungen & Belege sind für {MM}/{YYYY} in sevDesk gebucht.\n\nViele Grüße"   # default

notify:                            # optional: Pushover, to the phone
  pushover:
    enabled: true
    token: "your-pushover-application-token"
    user: "your-pushover-user-key"
    device: ""                     # optional: one device's name; empty: all
  closingDay: 5                    # default: the 5th, at 9:00, the month before looked over
  closing: true                    # default: tell of a month not complete

vodafone-invoice:
  username: "your-vodafone-email@example.com"
  password: "your-vodafone-password"
  fallbackToLastMonth: true      # optional, default: true

harvest-invoice:
  skipExisting: true               # optional, default: true
  harvest:
    token: "123456.pt.your-personal-access-token"   # id.getharvest.com/developers
    client: "Acme GmbH"            # optional, default: sevdesk.clientName
  sevdesk:
    username: "sevdesk-login@example.com"
    password: "your-sevdesk-password"
    clientName: "Acme GmbH"        # the client in sevDesk
    productName: "Acme Produkt"    # search term for product field
    productNum: "0102"             # article number to select from dropdown
    referenceNum: "REF-123"        # Kundenreferenz für E-Rechnung

web:                               # optional, for feed-my-accounting serve
  listen: ":8595"                  # default
  homeOnly: true                   # default: answer the home network only
  password: ""                     # optional: asked for by the browser

schedule:                          # optional, run by feed-my-accounting serve
  - job: travel-expense            # travel-expense, amazon-invoice, apple-invoice, vodafone-invoice, mail-receipts, harvest-invoice, invoice-mails
    cron: "0 6 5 * *"              # minute hour day month weekday
    enabled: true
  - job: harvest-invoice
    cron: "0 12 25-31 * 1-5"       # as in cron: the 25th–31st or any weekday
    enabled: true
```

### Storage (rclone)

The receipts mailed — Amazon, Apple, Vodafone, the rules' — can be copied to a storage too, by [rclone](https://rclone.org): iCloud Drive, Google Drive, OneDrive, Dropbox, Nextcloud by WebDAV, a NAS by SFTP — any remote set up for it. Each goes into its month's folder (`Belege/2026/11`, by the `11_2026_…` its name begins with), or into one folder for all without `{YYYY}`/`{MM}`. A name taken there is kept: the new one gets *(1)*, *(2)* before its extension, as the Finder does. On *Einstellungen › Ablage (rclone)* or in `config.yaml` — *rclone benutzen* (`enabled`) switches it for the receipts and the customers' invoices alike:

```yaml
archive:
  enabled: true
  remote: icloud                 # the rclone remote
  folder: "Belege/{YYYY}/{MM}"   # default
  invoiceFolder: "Rechnungen/{YYYY}"   # optional: the invoices and work logs, by number
```

With *Ordner für Rechnungen* set, the Kundenrechnung run puts the work log there too, named after its invoice — `11_2026_Rechnung_0203_WorkLog.pdf` in `Rechnungen/2026` — and once sent, the invoice follows from sevDesk's copy by mail, `11_2026_Rechnung_0203.pdf`: see [Invoices from the mailbox](#invoices-from-the-mailbox). Once: one of that name there already stays as it is, as the one attached to the draft in sevDesk does. Without the invoice's number found, it is not kept.

Set the remote up once on the machine, as the user the service runs as, and check it:

```bash
rclone config        # n, a name, the storage, its login
rclone lsd icloud:   # its folders: it works
```

**iCloud Drive** (`iclouddrive`, rclone 1.69 or later): Apple has no API for the Drive, rclone talks to icloud.com as its web page does. *Advanced Data Protection* must be off for the Apple ID. rclone asks for a two-factor code and keeps a trust token in `~/.config/rclone/rclone.conf`; when Apple wants a new code, after a month or two, the copy fails — the mail sent all the same, the run saying *sent by mail; rclone: …*, the log what to do: `rclone config reconnect icloud:` with a new code.

### Paperless

Beside the storage, or instead of it, the receipts mailed can go to [Paperless-ngx](https://docs.paperless-ngx.com) by its API: the sender as the correspondent (Amazon, Apple, a rule's name; none for the travel expenses), the day from the name as created (`…_17112026.pdf`, else Paperless reads it from the PDF), the title from the name (*Rechnung Amazon 17.11.2026*) and the tags for receipts — `Firma`, say, for a workflow there to give it its storage path. A tag or correspondent Paperless does not have yet is made. On *Einstellungen › Paperless* or in `config.yaml` — *Paperless benutzen* (`enabled`) switches it for the receipts and the customers' invoices alike:

```yaml
paperless:
  enabled: true
  url: "http://nas:8000"
  token: "..."                   # Paperless › My Profile › API Auth Token
  tags: [Firma]                  # the receipts'
  invoiceTags: [Firma, Rechnungen]   # the customers' invoices'
```

Paperless on a NAS that is off most of the time is fine: each receipt waits in `feed-my-accounting-paperless/` beside the configuration until Paperless takes it — tried right after sending and every ten minutes while the web interface runs. Not reaching Paperless never fails a run; the log says how many wait. What Paperless took is noted in `feed-my-accounting-paperless/uploaded`, by its SHA-256, and not queued again — and Paperless refuses a file it has already, so a receipt uploaded twice is still one document. The customers' invoices go there too: see [Invoices from the mailbox](#invoices-from-the-mailbox).

<p align="center"><img src="docs/storage.png" alt="Einstellungen: Ablage (rclone) with rclone benutzen ticked, the remote icloud and the folder Belege/{YYYY}/{MM}; beneath, Paperless with Paperless benutzen ticked, the address http://nas:8000, the API token saved and the tag Firma" width="760"></p>

### Never twice, and the month's closing

What goes to sevDesk's Autobox is booked — twice if it comes twice. So every receipt mailed is noted in `feed-my-accounting-sent.json` beside the configuration: its name, its SHA-256, and what it is — the Amazon order and its invoice, the Apple order, the mail and attachment of a rule's receipt — Anthropic's, say. A run, scheduled or by hand, leaves out what was sent before, the same file or the same receipt fetched anew, and says so: *sent on 12.11.2026 11:37 already — not again*. Nothing new: no mail. The travel expenses, made anew each time, go again when sent again — a correction — with a warning in the log. A receipt that should go again: the (×) beside it on *Abschluss*, and the next run sends it.

**Filed by hand**: a receipt downloaded and booked by hand — an Amazon invoice fetched while there was time — and put in the storage as the others are, is not sent by the run either. Before sending, a run fetches the month's receipts from the storage and leaves out the same file, whatever its name; Apple's and Vodafone's also by their name, as their files differ each time they are made — Apple's carries the order's number, Vodafone's the month and contract. Amazon's not by name: the orders of a day are numbered as found. What it finds is noted in the ledger, on *Abschluss* as *von Hand abgelegt*; the (×) sends it all the same. A storage that cannot be read — iCloud wanting a new code — stops the run rather than send blindly.

The ledger also notes, for each source and month, when a run last looked — finding nothing is an answer too. *Abschluss* shows a month from it, and on the closing day — the 5th by default, at 9:00 — the month before is looked over: with Pushover set up, the phone is told what is still *offen*, a source the schedule runs that has neither sent nor looked: *Monatsabschluss November 2026 — Noch offen: Vodafone*. Nothing when it is complete. A scheduled run that fails is told at once, with its error; runs by hand are seen on the page.

**The tax adviser**: once a month is booked, *Steuerberater informieren* on *Abschluss* mails him, as set on *Einstellungen › Steuerberater* — to him only, not to the Autobox; a blind copy to the address set for it, if any. *Alle Belege als ZIP anhängen* fetches the month's receipts from the storage first — in a folder for all, those whose name begins with the month, `11_2026_…`, the ones put there by hand too — and attaches them as `Belege_11_2026.zip`, offered for download with the run.

**Pushover**: an application at [pushover.net/apps](https://pushover.net/apps) for its token, and the user key from pushover.net once logged in, on *Einstellungen › Benachrichtigung*; *Test senden* checks them.

---

## Module: travel-expense

Generates up to three PDF documents per month for German business travel expense reporting:

- **Kilometergelderstattung** — mileage reimbursement (distance × €0.30/km, one-way)
- **Verpflegungsmehraufwand** — meal allowance (€14.00 for trips 8h–24h)
- **Übernachtungspauschale** — the flat rate for each night of an overnight trip without a hotel invoice, see below

### How it works

1. Calculates workdays for the month (excluding weekends, German public holidays, blocked days, and optionally the Christmas/New Year week)
2. Gives the days of explicit **assignments** to their client, and the remaining workdays to the other clients in consecutive blocks of equal size
3. Generates formatted PDF documents with smart page breaks (blocks never split across pages)
4. Sends the PDFs via email as in-memory attachments (no files written to disk)

### PDF output

- `MM_YYYY_Reisekosten_Kilometergelderstattung.pdf`
- `MM_YYYY_Reisekosten_Verpflegungsmehraufwand.pdf`
- `MM_YYYY_Reisekosten_Uebernachtungspauschale.pdf` — only with nights at the flat rate

PDFs include a structured `Beleg-Nr.` (format: `RK-YYYY-MM-XXXX`) and are formatted for sevDesk OCR recognition. The selected employee (`staff.current`) is rendered prominently at the top as the Lieferant.

### Clients

Each client represents a destination:

| Field | Description |
|-------|-------------|
| `id` | Identifier shown in the document |
| `name` | Client name used as section header |
| `from` | Origin address (e.g. `"Stuttgart, Heimatbüro (Your Company GmbH)"`) |
| `to` | Destination address (e.g. `"München, Hauptstr. (Acme GmbH)"`) |
| `reason` | Purpose of the trip (e.g. `Projektarbeit`) |
| `distance` | One-way distance in kilometres; `0` for no mileage |
| `country` | ISO 3166 code of the client's country, default `DE`: its public holidays count (see below) |
| `province` | German state code for holiday calculation, for `DE` only (see below) |

When multiple clients are configured, the workdays are split into consecutive blocks, one per client: with 20 workdays and 2 clients, the first 10 go to one, the other 10 to the next. Mileage is calculated per client based on their distance.

### Assignments and overnight trips

Both ways of splitting the days work together:

- **Equal distribution**: the clients in Europe without assignments share the remaining workdays in consecutive blocks of equal size, in the order they are listed — with two clients and 21 days, the first 11 to one, the other 10 to the next. A day in a client's block that is a public holiday of its own is off.
- **Outside Europe** (`US`, `CA`, `MX`, `BR`, `AR`, `JP`, `TH`, `NZ`, `ZA`): never in the equal distribution — going there and back every day is no way to travel. Such a client only gets the days of its assignments, overnight as a rule. Europe — Switzerland, Austria, France and so on — takes part: a long drive, but a day trip.
- **Assignments**: a date range given to one client explicitly. A client with assignments gets those days and no others; the remaining workdays are shared by the clients without any.
  - **Day trips** (`overnight: false`): the workdays in the range, each with the day rate and the drive there and back.
  - **Overnight** (`overnight: true`): every day of the range counts, weekends and holidays too — the first and last day at the travel day rate (An-/Abreisetag) with the drive to and from the airport or station, the days between at the full day rate, without mileage. A client without `distance` has no mileage at all; a month without driving sends only the Verpflegungsmehraufwand PDF.

Example: 3–9 December in Miami for Globex, overnight; every other workday with Acme. The calendar on the web interface shows the result before anything is sent.

### Meal allowance rates

Every client uses the **inland rates** under `mealRates`, unless it has rates of its own:

| Inland (`mealRates`) | Client | Day | Default |
|----------------------|--------|-----|---------|
| `day` | `mealDay` | day trip, more than 8 hours away | 14 € |
| `fullDay` | `mealFullDay` | a full day away, overnight trips | 28 € |
| `travelDay` | `mealTravelDay` | the day of arrival or departure of an overnight trip | 14 € |

The defaults are the German Verpflegungspauschalen by law. If the law changes, change `mealRates` — on the web interface under *Pauschalen Inland* — and every client without rates of its own follows. For trips abroad, set the BMF's Auslandspauschalen for the country on that client — the table has one rate for the day of arrival or departure *and* for a day of more than 8 hours away, so a client abroad with only one of `mealDay` and `mealTravelDay` uses it for both; only with neither does the inland rate apply, marked red in the calendar: the BMF's table in the [Lohnsteuer-Handbuch of the year](https://lsth.bundesfinanzministerium.de/lsth/2026/B-Anhaenge/Anhang-25/I/inhalt.html) — Miami, say, 65 € for 24 hours and 44 € for the day of arrival or departure. The web interface links the table where the rates are entered, for the year of the month shown — the year before's while the year's handbook is not out yet, in its first weeks — so the link needs no change from year to year.

### Countries

A client's day trips skip the public holidays of its country: in Germany those of its state (`province`), elsewhere the national ones. Known countries:

`DE` Deutschland · `AR` Argentinien · `AT` Österreich · `BE` Belgien · `BG` Bulgarien · `BR` Brasilien · `CA` Kanada · `CH` Schweiz · `CZ` Tschechien · `DK` Dänemark · `ES` Spanien · `FI` Finnland · `FR` Frankreich · `GB` Vereinigtes Königreich · `GR` Griechenland · `HR` Kroatien · `IE` Irland · `IS` Island · `IT` Italien · `JP` Japan · `LT` Litauen · `LU` Luxemburg · `LV` Lettland · `MX` Mexiko · `NL` Niederlande · `NO` Norwegen · `NZ` Neuseeland · `PL` Polen · `RO` Rumänien · `SE` Schweden · `SI` Slowenien · `SK` Slowakei · `TH` Thailand · `UA` Ukraine · `US` USA · `ZA` Südafrika

Abroad the German meal rates do not apply: give the client the country's rates. A run that has to fall back to a German rate for a client abroad says so in the log. Overnight trips count every day regardless of holidays.

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

Defaults to `BW` if omitted or invalid. Only used for `country: DE`.

### Excluded dates

- Weekends (Saturday, Sunday)
- German public holidays for the client's province
- Christmas/New Year week off when `christmasWeekOff: true` (the default):
  - December 24
  - December 27–31

  December 25–26 are already excluded as public holidays (Weihnachten). Set `christmasWeekOff: false` to only exclude public holidays.
- Blocked ranges under `blocked` — vacation, illness, training — on the web interface under *Gesperrte Tage*

A month without any travel day — all of it vacation, say — is logged as nothing to send, not an error, so `all` goes on with the other modules.

### Nights without a hotel invoice

Without a hotel invoice, the employer may pay a flat rate per night instead, tax free (Übernachtungspauschale): 20 € in Germany, abroad the BMF's rate for the place — Miami 256 €, in the same table as the meal rates. Every **overnight assignment** has a night on each of its days but the last: 3–9 December are six nights, from the 3rd to the 4th up to the 8th to the 9th, each in the month it starts in. The rate is the client's `lodging`, else the inland `lodgingRate` (default 20 €; abroad that is marked red, as with the meal rates). An assignment with `hotel: true` — on the web interface *Hotelrechnung* — has no flat rate: its hotel invoice is accounted on its own, not twice. With `nightFlight: true` — *Nachtflug* — the night before the day of arrival is spent on the plane, as on the way home from the USA: 3–9 December are five nights then, the last from the 7th to the 8th; the meals stay, the 8th a full day away. The nights come as a third PDF, *Übernachtungspauschale*, and show in the calendar, after a pixel bed, and in the totals.

### travel-expense config options

| Field | Description | Default |
|-------|-------------|---------|
| `christmasWeekOff` | Exclude Dec 24 and Dec 27–31 | `true` |
| `clients` | List of client/trip configurations | required |
| `mealRates` | Inland meal rates in EUR: `day`, `fullDay`, `travelDay` | 14 / 28 / 14 |
| `lodgingRate` | Inland flat rate for a night without a hotel invoice, in EUR | 20 |
| `clients[].lodging` | The client's flat rate for a night, in EUR; abroad the BMF's | `lodgingRate` |
| `clients[].mealDay`, `mealFullDay`, `mealTravelDay` | The client's own meal rates in EUR, see above | `mealRates` |
| `clients[].country` | ISO 3166 code: whose public holidays count | `DE` |
| `blocked` | Date ranges without travel: `from`, `to` (optional), `note` | — |
| `assignments` | Date ranges for one client: `client` (id or name), `from`, `to`, `overnight`, `hotel` (the nights have a hotel invoice: no flat rate), `nightFlight` (home through the night before arrival: no flat rate for it), `note` | — |

---

## Module: mail-receipts

<p align="center"><img src="docs/rules.png" alt="The rules on Belege › Aus dem Postfach: Anthropic, Strato, the tax adviser by datev.de, a railway rule switched off — each with its sender domains, the word in the subject and in the PDF's name" width="760"></p>

The PDF invoices of senders without a module of their own, by rules set up on *Belege › Aus dem Postfach* — no code for a new one. Each rule: a name, the sender's domains, and if need be what the subject and the PDF's name contain.

1. Searches the folders in order, the inbox first and then *Rechnungen*, where receipts filed by hand are — in the main mailbox and then in each of `mail.mailboxes`; the same mail found twice counts once
2. Takes the month's mails of each rule — the current month's on the schedule, *Weitere Absender abholen*; any month's on the web interface — and their PDF attachments
3. Names each by the rule and the mail's date, as the other receipts are: `11_2026_Rechnung_Strato_03112026.pdf`; a second of the same day `…_2.pdf`
4. Sends each rule's in one email, *Deine PDF-Rechnungen von Strato* — those not sent before

PDFs only: a sender whose invoice is the mail itself, as Apple's, needs a module of its own.

**Anthropic** is such a rule since 1.25 — the module anthropic-invoice of before became one on the first start, its schedule entry *Weitere Absender abholen*: mails from `mail.anthropic.com, stripe.com` — Anthropic Ireland's come from Stripe — whose subject contains *Your receipt from Anthropic*, and of their two PDFs the one whose name contains *receipt*, not the invoice. `feed-my-accounting anthropic-invoice` runs mail-receipts.

| Field | Description | Default |
|-------|-------------|---------|
| `count` | The month's emails to scan at the most, in each folder | `1000` |
| `folders` | Folders to search, in this order | `INBOX`, `Rechnungen` |
| `rules[].name` | The sender, in the file names and the mail's subject | — |
| `rules[].from` | Sender domains, comma-separated | — |
| `rules[].subject` | The subject contains it, whatever the case; empty: any | — |
| `rules[].attachment` | The PDF's name contains it, whatever the case; empty: any PDF | — |
| `rules[].off` | Switched off: left out | `false` |

---

## Module: apple-invoice

Fetches Apple invoice emails from an IMAP inbox, converts their HTML body to PDF using headless Chrome, and sends all PDFs as attachments in a single email.

### How it works

1. Connects to the IMAP server and scans the last N recent emails of each folder, the inbox first and then *Rechnungen*, where invoices filed by hand are
2. Filters by subject, sender domain, and current month; the same invoice found in both folders counts once, the inbox's
3. Extracts the HTML body and converts each to an A4 PDF via headless Chrome
4. Embeds external images as base64 data URIs for reliable rendering
5. Names each PDF as `MM_YYYY_Rechnung_Apple_BESTELLNUMMER.pdf` using the order number extracted from the invoice HTML; falls back to subject-based naming if not found
6. Sends all PDFs as attachments in a single email

### apple-invoice config options

| Field | Description | Default |
|-------|-------------|---------|
| `filter.count` | The month's emails to scan at the most, in each folder — the IMAP server picks the month's, however full the folder | `1000` |
| `filter.from` | Sender domain to match | `apple.com` |
| `filter.subject` | Exact subject line to match | `Deine Rechnung von Apple` |
| `filter.folders` | Folders to search, in this order; the same invoice in two counts once | `INBOX`, `Rechnungen` |

The IMAP host and port are configured under the top-level `mail:` section.

---

## Module: amazon-invoice

Logs into an Amazon Business account on amazon.de via headless Chrome, downloads the invoices of the orders placed in a month — on a month's last day the current one, else the one before — and sends them as PDF attachments via email — once in 28 days at the most, held by the login gate as Vodafone is.

1. Opens the order history with the browser profile kept beside `config.yaml` (`feed-my-accounting-amazon-browser/`): still logged in from the last run, it goes on at once
2. Else it enters the email address from *Einstellungen Amazon* and hands the password page to the live view: *Passwort eintragen* types the stored one key by key, *Anmelden* is clicked by hand, *Angemeldet bleiben* ticked. Amazon's **one-time code** — from the authenticator app or a text message — is typed in the web interface's live view, *Don't require a code on this device* ticked by the run first: the next runs need no code. A captcha, or a page it does not know, goes to the live view too. On the command line, in a terminal, the one-time code is asked there (*Amazon one-time code:*); in cron or a script, with nobody to ask, the run stops saying so — not counted as a login
3. Goes through the order history of that month's year, newest first, for the orders placed in it
4. Fetches each order's invoice PDFs from its invoice popover, as the page, with its cookies; an order with no invoice yet — a seller who has not uploaded one — is logged and skipped
5. Sends them in one email (*Amazon*), named `11_2026_Rechnung_Amazon_17112026.pdf` — the month, then the order's day — numbered `_2`, `_3` for more of the same day

```yaml
amazon-invoice:
  username: "your-amazon-business-email@example.com"
  password: "your-amazon-password"
```

## Module: vodafone-invoice

Logs into the MeinVodafone portal via headless Chrome, downloads Mobilfunk and Kabel invoices, and sends them as PDF attachments via email.

### How it works

1. Launches headless Chrome with bot-detection evasion (new headless mode, custom user agent, `navigator.webdriver` removed via CDP)
2. Opens MeinVodafone in a fresh browser, nothing kept from the last run, and logs in at a person's pace: a look at the page, the fields typed key by key with uneven gaps, a pause before *Anmelden*. A login MeinVodafone turns down stops the run with why: *username or password wrong*, or blocked for now after too many tries. **Once a month at the most:** no login within 28 days of the last (`feed-my-accounting-logins.json` beside `config.yaml` notes each) — Vodafone takes logins that come more often for an attacker, resets the password and blocks the address. Until then *Per Mail senden* is off, with when the next login is allowed beneath it; the schedule skips the runs it would hold back and shows the next one allowed. With a Vodafone schedule on, the schedule has the say: *Per Mail senden* stays off, saying when the schedule fetches them — a login by hand would make that run skip
3. **Logging in by hand:** without a password stored, or when MeinVodafone neither lets it in nor says why — a captcha, as a rule — a run started in the web interface, by hand or on schedule, waits up to 10 minutes. A window opens on the page with the Vodafone page as the run sees it: used as the page itself — a click on the picture clicks there, the keyboard types into the field clicked (a phone's keyboard too, pasting as well), the mouse wheel scrolls; once you are logged in the run goes on by itself, the login kept for the next runs. *Lauf abbrechen* ends it. On the command line such a login fails, saying so
4. Navigates to the invoice page for each contract type, Mobilfunk first, then Kabel
5. Downloads the invoice shown in the "Aktuelle Rechnung" block by intercepting the blob URL created by the browser
6. If that download fails and `fallbackToLastMonth` is true, tries the first entry in the Rechnungsarchiv instead
7. If the shown invoice is not for the current month and `fallbackToLastMonth` is false, skips and sends no email
8. Sends all found invoices as attachments in a single email

### When to run

Run near the **end of the month** (around the 25th or later). Vodafone invoices are typically generated mid-month and may not be available earlier.

### PDF output

- `MM_YYYY_Rechnung_Vodafone_Mobilfunk.pdf`
- `MM_YYYY_Rechnung_Vodafone_Kabel.pdf`

### Adding contract types

Edit `contractTypes` in `vodafone-invoice/vodafone-invoice.go`:

```go
var contractTypes = map[string]string{
    "mobilfunk": "Mobilfunk",
    "kabel":     "Kabel",
    "dsl":       "DSL", // example
}
```

### vodafone-invoice config options

| Field | Description | Default |
|-------|-------------|---------|
| `username` | MeinVodafone login email | required |
| `password` | MeinVodafone password | required |
| `fallbackToLastMonth` | If `true`, send last month's invoice when current month is not yet available. If `false`, skip sending entirely until the current month's invoice is ready. | `true` |

---

## Module: harvest-invoice

Reads a month's hours of a client from the Harvest API and creates a draft invoice (Entwurf) for them in sevDesk via browser automation. The month is the current one, the one given on the command line (`harvest-invoice 10/2026`) or on the web interface (`10/2026`).

<p align="center">
  <img src="docs/worklog.png" alt="A Zeitnachweis for November 2026 with made-up entries: the company's logo at the top right, the period and its hours, and a table by day — Kunde Acme GmbH, Projekt with the times and notes beneath, Tätigkeit, Mitarbeiter Max Mustermann, Stunden" width="560">
</p>

### How it works

1. Reads the time entries of the client (`harvest.client`, else `sevdesk.clientName`) by the selected employee (`staff.current`, named as in Harvest; without one, everyone's) in the month from the Harvest API v2 and sums them — rounded hours, as Harvest's reports count them. Fewer hours than the days booked × 8, or a project of the month over its budget in Harvest, log a warning with the link to Harvest — a warning only, the run goes on
2. Makes the month's **work log**, laid out as Harvest's *Detailed time report*, in German — *Zeitnachweis*: the company's logo, sized and placed as sevDesk does on its invoices (or without one its name: `company.name`, else Harvest's account name), the period, its hours, the hours billable and not billed yet and what is left of the project's budget in hours in Harvest (*151,00 Stunden verbleibend*) — in red when less than a twelfth of the budget is left, a month's share (under 120 of 1440 hours); none left, *Keine Stunden verbleibend* in red. Hours beyond the budget are not billed: with 120 hours left before the month and 154 booked, *120,00 Stunden abrechenbar*, in red *34,00 Stunden nicht abrechenbar*, and the draft in sevDesk takes the 120 — as does the manual entry, from the work log dropped in. What a budget had left before the month is what Harvest has left now plus the month's own hours — then a table by day — Kunde, Projekt with the times and the note beneath, Tätigkeit, Mitarbeiter, Stunden — with the table's head on every page and the total and page number at its foot. The web interface offers it with the run; the command line saves it in the current directory
3. **Guard: `skipExisting`** — logs into sevDesk, navigates to the Rechnungen list via the menu, and checks if any invoice for the same client has a Rechnungsdatum within the month; skips if found (works for Entwurf, Offen, and Bezahlt invoices)
4. Logs into sevDesk and creates a new E-Rechnung draft with:
   - **Rechnungsdatum** pre-filled as today's date by sevDesk
   - **Leistungszeitraum** set to the whole month (e.g. 01.04.2026 - 30.04.2026)
   - **Referenznummer** and **Kundenreferenz** from config
   - **Product** selected via typeahead search (search term + article number)
   - **Menge** set to the total hours
5. Saves the invoice as draft — it is never sent to the client
6. Attaches the work log to the draft under *Dokumente hinzufügen*, as `0203_WorkLog.pdf` beside sevDesk's own `0203.pdf`, and saves it again: sevDesk binds it into the E-Rechnung, and it goes with the invoice's mail

The Harvest API needs a personal access token from [id.getharvest.com/developers](https://id.getharvest.com/developers) — on every plan, the free one too. sevDesk is used through its web interface with headless Chrome — no sevDesk API token needed.

The work log is made before the sevDesk check, so it comes even when the draft exists already. It is named after the invoice it goes with, `10_2026_Rechnung_0202_WorkLog.pdf`, its number read off sevDesk's Rechnungen list — the existing invoice's, or the new draft's once saved; without one found, and with *Nur Zeitnachweis*, `10_2026_Rechnung_WorkLog.pdf`. Uploaded again, it is read like Harvest's own report.

**Attached to the draft**: the draft there already gets the work log too, if it does not have one of that name — run again, it is left as it is; an invoice no longer a draft (*Offen*, *Bezahlt*) is not changed. Attaching failing — sevDesk's page changed, say — the draft stays and is noted, the run says *the draft is there, the work log not attached*: attach it by hand, or run again.

On the web interface, *Manuelle Eingabe* creates the draft without the API: *Leistung von*, *Leistung bis* and *Stunden gesamt* typed in, or filled from a work log PDF dropped in — Harvest's report or one from here: the hours (`157,00 Hours`, `157,00 Stunden`) and the period (the first two dates as `dd.mm.yyyy`) — and correctable either way. The client, article and reference are `sevdesk.*` as for the run; the duplicate check the same.

### harvest-invoice config options

| Field | Description | Default |
|-------|-------------|---------|
| `skipExisting` | Check sevDesk for an existing invoice with the same client and period before creating | `true` |
| `harvest.token` | Personal access token for the Harvest API v2 | required |
| `harvest.accountId` | Harvest account to read | the token's Harvest account |
| `harvest.client` | Client name in Harvest | `sevdesk.clientName` |
| `sevdesk.username` | sevDesk login email | required |
| `sevdesk.password` | sevDesk login password | required |
| `sevdesk.clientName` | Client in sevDesk | required |
| `sevdesk.productName` | Search term typed into the product field (e.g. "Acme Produkt") | required |
| `sevdesk.productNum` | Article number to select from the typeahead dropdown (e.g. "0102") | required |
| `sevdesk.referenceNum` | Kundenreferenz for E-Rechnung | — |

### Safe for cron: duplicate prevention

harvest-invoice can run repeatedly without creating duplicate invoices: with **`skipExisting`** (default: `true`), before opening the invoice form, the module logs into sevDesk via headless Chrome, navigates to the Rechnungen list, and checks if any invoice for the same client has a Rechnungsdatum within the month. This covers all statuses (Entwurf, Offen, Bezahlt). If a match is found, the run is skipped: the first successful run creates the invoice, all later ones are no-ops. Note that a draft made mid-month keeps the hours of that day — delete it, or correct the Menge, at the end of the month.

### Invoices from the mailbox

Sent from sevDesk with a copy to yourself, each invoice comes back: *Kopie von Rechnung Nr. 0203 von … zur Bestellung Nr. …*, from sevdesk.de, with `0203.pdf` and the work log. The job **invoice-mails** — *Rechnungen aus dem Postfach ablegen* on the schedule, or *Ablegen* on the Kundenrechnung card by month — takes them by rules, as mail-receipts does the further senders' receipts, a rule per customer: the sender's domains, what the subject contains — *Kopie von Rechnung*, or the customer's *Bestellung Nr.* to tell customers apart — and what the PDF's name contains, if anything. sevDesk changing its mail, the rule is changed on the card. Each invoice:

- is named after its number, `11_2026_Rechnung_0203.pdf`: the number the PDF's (`0203.pdf`, `RE-0203.pdf`), else the subject's (*Nr. 0203*); the month the work log's in the same mail, else the mail's — none found, it is left out, said so
- goes to the storage's *Ordner für Rechnungen*, if rclone is used, beside its work log — once: one of that name there stays as it is. The mail's work log goes there too, should it be missing
- goes to Paperless, if used: the rule's name — the customer — as correspondent, *Rechnung 0203 Acme GmbH* as title, the day of the mail as created, and the tags for invoices (`paperless.invoiceTags`, *Einstellungen › Paperless*). The work log does not
- is mailed nowhere: the books have it from sevDesk

The schedule's run looks at the current month and the one before, an invoice sent on the first seen still; run daily, say `0 20 * * *`. The rules are on the card, behind *Regeln*:

```yaml
invoice-mails:
  folders: [INBOX]                 # default
  rules:
    - name: "Acme GmbH"            # the customer
      from: "sevdesk.de"
      subject: "Kopie von Rechnung"
```

### Notes

- Harvest's web sign in has an hCaptcha that turns an automated browser away — hence the API
- Chrome runs in headless mode with German locale (`de-DE`)
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
│   ├── travel-expense.go          # Config, day plan, Generate(), Send(), Run()
│   ├── doc.go                     # document content builders
│   └── pdf.go                     # the PDF's layout
├── pdfdoc/
│   └── pdfdoc.go                 # shared PDF basics: A4 pages, blocks never split, Document
├── mail-receipts/
│   └── mailreceipts.go            # rules, the PDFs of their mails and their names
├── apple-invoice/
│   ├── apple-invoice.go            # Config, Run(), invoice parsing
│   └── pdf.go                      # HTML→PDF conversion, HTML cleanup
├── amazon-invoice/
│   └── amazon.go                   # Config, login with the live view, orders, invoice PDFs
├── vodafone-invoice/
│   └── vodafone-invoice.go         # Config, browser automation, Run()
├── harvest-invoice/
│   ├── harvest.go                 # Config, Run(), CreateDraft()
│   ├── api.go                     # Harvest API v2: a month's time entries, the project budgets, the company
│   ├── worklog.go                 # the work log PDF, as Harvest's detailed time report
│   ├── logo.go                    # the company's logo, scaled down for the work log
│   ├── parser.go                  # a dropped work log PDF: hours and period, for Manuelle Eingabe
│   └── sevdesk.go                 # sevDesk browser automation (login, form, save, duplicate check)
├── web/                           # the web interface's page, embedded in the binary
├── main.go                        # CLI entry point and config mapping
├── config.go                      # unified YAML config structs, loading and saving
├── server.go                      # web interface: pages and JSON API
├── auth.go                        # logins per employee, sessions, home network only
├── logins.go                      # the login gate: Amazon and Vodafone not too often
├── jobs.go                        # runs one job at a time, keeps its output
├── schedule.go                    # cron expressions and the built-in scheduler
├── ledger.go                      # every receipt sent, once: feed-my-accounting-sent.json
├── filed.go                       # receipts filed by hand in the storage, not sent again
├── closing.go                     # the month's books per source, and its state
├── notify.go                      # Pushover: failed runs, and the closing day
├── advisor.go                     # the tax adviser's mail, the month's receipts zipped
├── archive.go                     # receipts copied to the storage with rclone
├── paperless.go                   # receipts uploaded to Paperless-ngx, waiting while it is off
├── invoicemails.go                # the customers' invoices from sevDesk's copies, stored
├── feed-my-accounting.service     # systemd unit for the web interface
├── config.yaml.example
└── go.mod
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT — see [LICENSE](LICENSE). If it saves you an evening each month, a [coffee](https://buymeacoffee.com/rummeyer) is welcome.
