# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Two-factor login with Google Authenticator**: *2FA einrichten* under *Einstellungen › Mitarbeiter* shows a QR code (and the setup key) for any TOTP app, confirmed with its code. From outside the login then asks for the code after the password, once per browser every 30 days; at home never. Codes count once, wrong ones as failed logins; setting up anew or removing makes every browser give a code again.

### Changed

- **Login lockout doubles**: an address locked out again waits twice as long as before — 15 minutes, 30, an hour … up to 24 hours — instead of 15 minutes each time, so a password can no longer be tried some 480 times a day from one address. Quiet for a day, or logged in, it starts over at 15 minutes.

## [1.28.0] - 2026-11-26

### Added

- **Reply-To for the tax adviser**: *Antwort an (Reply-To)* on *Einstellungen › Steuerberater* — where his answer goes; empty, the sender.

### Changed

- **Buttons that cannot work are off**, saying what is missing: *Aus Harvest anlegen* and *Nur Zeitnachweis* without the Harvest token, *Aus Harvest anlegen* and *Entwurf in sevDesk anlegen* without the sevDesk login, *Ablegen* of the invoices without rclone's folder for invoices or Paperless, *Test senden* without Pushover.
- **Steuerberater**: the fields in a mail's order — Absender, An, Betreff, Antwort an, Kopie, Blindkopie, Text.
- **Code review**: duplicates merged — one rclone copy, one mail delivery, one handler for the mail rules' runs, the rules' fetching shared with the invoices from the mailbox — and every function and type commented.

### Fixed

- README: screenshots beside their explanations — the mail account, the tax adviser, the clients, the assignments, the receipts, Aus Harvest, the invoices from the mailbox; the tax adviser's mail as it is now.

## [1.27.0] - 2026-11-26

### Added

- **Invoices from the mailbox**: *Aus dem Postfach* on the Kundenrechnung card, and the job *invoice-mails* for the schedule — the copy sevDesk mails of each invoice sent is taken by rules, a rule per customer (sender, subject, PDF name: changeable should sevDesk change its mail). The invoice goes to the *Ordner für Rechnungen* as `11_2026_Rechnung_0203.pdf` beside its work log, and to Paperless with the customer as correspondent and its own tags — *Tags für Rechnungen* on *Einstellungen › Paperless*, beside *Tags für Belege*. *PDFs ansehen* to look first.
- **The tax adviser's sender and copy**: *Absender* and *Kopie (CC) an* on *Einstellungen › Steuerberater* — whom the mail to the adviser comes from; empty, *mail.from* as before. The receipts stay with *mail.from*: sevDesk's Autobox takes mails from the account's address only.

### Changed

- **rclone benutzen, Paperless benutzen**: the switches on *Einstellungen*, before *Belege ablegen* and *Belege hochladen*, so named as they hold for the receipts and the invoices alike.
- **The page**: on Belege, Apple first and the further senders' section called *Weitere Rechnungen*; *Weihnachtswoche frei* beneath *Gesperrte Tage*; *Maximale Mail-Anzahl (je Ordner)*; on Kundenrechnung, *Aus dem Postfach* without *Nichts wird gemailt*; the jobs *Weitere Rechnungen abholen*, before *Weitere Absender abholen*, and *Kundenrechnungen aus dem Postfach ablegen*; *Weitere Postfächer* said to be searched for invoices too; shorter notes on Einstellungen.
- **Paperless once**: what Paperless took is noted by its SHA-256 and not queued again.

### Fixed

- README: screenshots anew — Abschluss, Zeitplan, Reisekosten, Kunden, Kundenrechnung, the rules, Firma, the storage, the log.
- README: the work log is attached to the draft since 1.26.0, no longer "attach it manually".

## [1.26.1] - 2026-11-25

### Added

- **Work logs kept by invoice**: *Ordner für Rechnungen* on *Einstellungen › Ablage (rclone)* — the Kundenrechnung run copies the work log there, named after its invoice, `11_2026_Rechnung_0203_WorkLog.pdf`, `{YYYY}` its year. Once: one there already stays.

## [1.26.0] - 2026-11-25

### Added

- **Work log attached in sevDesk**: the Kundenrechnung run attaches the month's work log to the draft under *Dokumente hinzufügen*, as `0203_WorkLog.pdf` beside the invoice's `0203.pdf` — sevDesk binds it into the E-Rechnung and sends it with the invoice's mail. A draft there already gets it too, once; an invoice no longer a draft is left as it is.
- **Paperless**: the receipts mailed are uploaded to Paperless-ngx too, beside rclone or instead of it — the sender as correspondent, the day from the name as created, a title from the name and the tags set (`Firma`, say). While Paperless cannot be reached, the NAS off, they wait beside the configuration and go up once it answers: after each sending and every ten minutes. On *Einstellungen › Paperless*.

### Fixed

- Kundenrechnung: sevDesk's cookie banner now comes again on the dashboard, seconds after the login, and sevDesk loads a while longer: the Rechnungen list did not open and the run stopped (25.11.2026). The banner is waited for and closed, and the list counts as open only once it shows rows — else tried again, three times.

## [1.25.1] - 2026-11-24

### Changed

- A button that sends or transfers something is solid: *Steuerberater informieren*, *Aus Harvest anlegen* and *Test senden* now look as *Per Mail senden* does; one that only shows or edits here stays light.

## [1.25.0] - 2026-11-24

### Added

- **Never twice**: every receipt mailed is noted in `feed-my-accounting-sent.json` beside the configuration — its name, its SHA-256 and what it is: the Amazon order, the Apple order, the Anthropic receipt's number. A run leaves out what was sent before and says so; nothing new, no mail. sevDesk's Autobox booked a receipt twice when it came twice — on 17.11.2026 four Amazon invoices came four times. The travel expenses, made anew, go again on asking, with a warning.
- **Filed by hand, not sent**: before sending, a run looks in the storage too — a receipt downloaded and booked by hand, the same file there, or Apple's and Vodafone's by name, is left out and noted as *von Hand abgelegt*. A storage that cannot be read stops the run.
- **Abschluss**: a tab with a month's books, source by source — the receipts sent, *nichts gefunden* where a run looked and found none, *offen* where a scheduled source has done neither, its last run's error and its next run. (×) beside a receipt takes it out of the ledger: the next run sends it again — several at a time.
- Abschluss: the month's **state at a glance** — a green tick when every source is done and the invoice drafted in sevDesk, a road block while it is under way, a red cross while nothing has happened yet.
- **Steuerberater informieren** on Abschluss: the mail to the tax adviser, address, a blind copy, subject and text on *Einstellungen › Steuerberater*; not to the Autobox. *Alle Belege als ZIP anhängen* zips the month's receipts from the storage beside it. When he was told is noted and shown.
- **Pushover**: a scheduled run that fails is told at once; on the closing day — the 5th by default, at 9:00 — what the month before is still missing. On *Einstellungen › Benachrichtigung*, with *Test senden*.
- **mail-receipts**: PDF invoices by rules on the web interface — the sender's domains, words in the subject and in the PDF's name — named as the other receipts, sent once each. On the schedule as *Weitere Absender abholen*, on the command line as `mail-receipts`.

### Changed

- Work log: the hours left of the budget in red when less than a twelfth of it is left — a month's share, under 120 of 1440 hours — and *Keine Stunden verbleibend* in red when none is.
- harvest-invoice: hours beyond the project's budget are not billed. With 120 hours left before the month and 154 booked, the work log says *120,00 Stunden abrechenbar* and in red *34,00 Stunden nicht abrechenbar*, and the draft in sevDesk takes 120 hours — the manual entry too, from the work log dropped in.
- **Anthropic is a mail rule**, not a module: on the first start the module's filter becomes the rule *Anthropic* — the receipt PDF, not the invoice beside it — and its schedule entry *Weitere Absender abholen*. Its receipts sent before count as sent. `anthropic-invoice` on the command line runs mail-receipts.
- harvest-invoice: the warning of too few hours counts the days with time entries in the month × 8, not the client's days as the Reisekosten plan them: four days a week at a client warned every month, the plan counting five.

## [1.24.0] - 2026-11-17

### Added

- amazon-invoice on the command line, in a terminal: Amazon's one-time code is asked there — *Amazon one-time code:* — and typed in as in the live view; a wrong one once more, then the run stops. In cron or a script, with nobody to ask, it stops saying so.

### Fixed

- Login gate: a run with no live view to ask in — on the command line — is not counted as a login either: the one by hand, with the code, may follow at once.

## [1.23.0] - 2026-11-17

### Changed

- Questions and notices in the page's own look, not the browser's: *Per Mail senden*, the travel expenses sent, the sevDesk draft, cancelling a run in the live view, an error — the live view's frame, the answer on its button: *Senden*, *Anlegen*, *Lauf abbrechen* or *Weiter warten*. Escape says no.
- Zeitplan: the entries in the order they run next, as the page loads them; one added stays where it is put until the next load. Entries that never run come last.

## [1.22.0] - 2026-11-17

### Changed

- The copy of the receipts mailed is not iCloud Drive's only: any storage rclone has a remote for — Google Drive, OneDrive, Dropbox, Nextcloud, a NAS by SFTP. *Einstellungen › Ablage (rclone)*, `archive:` in `config.yaml` — an `icloud:` block of 1.21 read as it, and saved so — the log `[rclone]`, the place by its kind: `"iclouddrive://…"`, `"drive://…"`; a failed copy *sent by mail; rclone: …*.

## [1.21.0] - 2026-11-17

### Added

- **iCloud Drive**: the receipts mailed — Amazon, Anthropic, Apple, Vodafone — are copied to iCloud Drive too, if switched on in *Einstellungen › iCloud Drive*: each into its month's folder, `Belege/{YYYY}/{MM}` unless set otherwise, by the month its name begins with. By [rclone](https://rclone.org)'s `iclouddrive` remote (1.69 or later), set up once with `rclone config` and a two-factor code. A file there by the name already is kept: the new one gets *(1)*, *(2)* before its extension, as the Finder does. A copy that fails leaves the mail sent, and says so: *sent by mail; iCloud Drive: …*, the log adding what to do when Apple wants a new code: `rclone config reconnect icloud:` on the Pi.
- Login gate: a run that ended for want of someone in the live view — the one-time code not given, nobody there at the schedule's run, or *Abbrechen* — is not counted as a login: one by hand, with the code, may follow at once.


### Fixed

- anthropic-invoice, apple-invoice: the month's emails are asked of the IMAP server by their date, however full the folder — not the last fifty scanned, behind which a month's receipts could lie unseen (17.11.2026). *Höchstens so viele Mails des Monats* is a limit now, 1000 by default: more in a folder, the latest so many are looked at, and the log says so.

## [1.20.0] - 2026-11-17

### Added

- **Login gate**: Vodafone and Amazon are logged into once a month at the most — not again within 28 days, counted in days — the books closed on the last day of each month, at any hour, February included. Its bot check took the runs' logins for an attacker: three on three days, and Vodafone reset the password (13.11.2026); a day of tries with the password it had reset, and it blocked the address; one more, and it reset the password again (17.11.2026). Each login is noted beside `config.yaml` (`feed-my-accounting-logins.json`); until the next is allowed, *Per Mail senden* is off, beside it *manuell ab 15.12.2026 möglich* — with the schedule on, *per Zeitplan am 21.12.2026* — and a button off takes no hover. The schedule skips the runs the gate would hold back — not started to fail — and shows the next one allowed; on the command line a login too soon fails saying when. By hand, *Per Mail senden* is off too while the schedule's next run would come within 28 days — it would make that run skip, and a month's invoice be missed, logged in on the 19th with last month's — and says when the schedule fetches them instead; switch that schedule off to run it by hand.
- Live view: the page with one row beneath it — a text field whose *Eintragen ↵* types into the page's field key by key (the first input field, if none has the cursor) and sends it, ⌫ ← → Enter, clear, *Abbrechen*; the stored password at a button where asked. Clicks move the mouse there first, then press and let go; keys go as a keyboard's, with their editing commands. The page as large as the window has room for, nothing to scroll; a button off takes no hover.
- Belege: Amazon and Vodafone have *Per Mail senden* only, no *PDFs ansehen*: each run is a login, and a login to look is not worth the account. The PDFs sent are in the Protokoll to download as before.
- vodafone-invoice: logs in at a person's pace — a look at the page before the cookie banner, the fields clicked into and typed key by key with uneven gaps, a pause before *Anmelden*, and a few seconds, varying, on each page after.
- **amazon-invoice**: the invoices of an Amazon Business account's orders of a month — on its last day the current one's, the books closed then, else the month before, complete — from amazon.de, on the *Belege* tab before Anthropic, with *Per Mail senden*, on the schedule and on the command line (`amazon-invoice`). Held by the login gate as Vodafone is: once in 28 days, the schedule having the say. It types the email address and the password from *Einstellungen Amazon* as a person would, once each; Amazon's one-time code is typed in the live view, *In diesem Browser ist kein Code erforderlich* ticked, and the browser profile kept beside `config.yaml` (`feed-my-accounting-amazon-browser/`): later runs go on logged in. The live view closes the moment Amazon sets its login cookie — its redirect after the code hangs for minutes. Business's order cards are read by their order number field; cancelled orders are skipped. Each order's invoice PDFs come from its invoice popover, named as the others are: `11_2026_Rechnung_Amazon_17112026.pdf`, the month and the order's day, `_2`, `_3` for more of the same day; an order with none yet is logged and skipped.
- harvest-invoice: the work log shows what is left of the project's budget in hours in Harvest, beneath the hours billable and not billed: *151,00 Stunden verbleibend*, or *12,00 Stunden über Budget*; with several, each with its project's name. As Harvest's project budget report has it at the run; a monthly budget only for the current month, a budget in money not.
- **Logging in by hand, in a live view.** Without a Vodafone password stored, or when MeinVodafone neither lets the run in nor says why — a captcha — a run started in the web interface, by hand or on schedule, waits up to 10 minutes: a window opens on the page with the Vodafone page as the run sees it, used directly: a click on the picture clicks there and the keyboard types into the page, a phone's keyboard too; the mouse wheel scrolls. Once logged in, the run goes on by itself; *Lauf abbrechen* ends it. On the command line such a login fails, saying so.
- vodafone-invoice: MeinVodafone's security code — *Login - Sicherheitscode*, text in a picture, after wrong tries — hands the login over to the live view, to type the code and click *Weiter*; known by its address, `vodafone.de/cprx/captcha`, or its text; it never counts as logged in.
- vodafone-invoice: a login MeinVodafone refuses — the password, or the address flagged after wrong tries, which it answers alike — opens the live view in the web interface, to look and log in by hand. On the command line it fails with the reason, as before.
- vodafone-invoice: the run opens MeinVodafone's login page itself, no detour by the services and their redirect by script — chromedp's wait for the form hung after it for good — and goes by what it shows, looked at every second for 30 s at most: the form, the security code, or MeinVodafone itself when the login stayed. The cookie banner is answered only if it is there.
- vodafone-invoice: username and password go into fields emptied first, and are checked to be just what is stored — a field the page filled already would have the password appended. If one is not, nothing is sent: the page goes to the live view. The log says only lengths, never the value.
- README: badges for the release, the build, the licence, the Go version and Buy Me a Coffee — the release read from GitHub, no longer a number to forget. A LICENSE file to go with it, the Sponsor button (`.github/FUNDING.yml`) and a build on GitHub Actions: gofmt, vet, tests and the Pi's binary.

### Fixed

- The browser says what it is: Chromium, its version, on Linux, without "Headless" — not Chrome 131 on a Mac, which its client hints contradicted, a robot to a bot check.
- vodafone-invoice: a fresh browser each run again, nothing kept, no *Eingeloggt bleiben*: the profile kept carried Vodafone's bot check's cookies from run to run.
- vodafone-invoice: a login MeinVodafone turns down stops the run with why — *username or password wrong*, or blocked for now after too many tries — instead of going on logged out and reporting *no invoice found* (14.11.2026). An invoice page that does not load within 30 s is an error too, not an empty page. Mobilfunk is searched first, then Kabel, every run the same.

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
