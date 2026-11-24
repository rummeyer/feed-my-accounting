package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // logos
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	amazon "feed-my-accounting/amazon-invoice"
	anthropic "feed-my-accounting/anthropic-invoice"
	apple "feed-my-accounting/apple-invoice"
	"feed-my-accounting/browser"
	"feed-my-accounting/email"
	harvest "feed-my-accounting/harvest-invoice"
	travelexpense "feed-my-accounting/travel-expense"
	vodafone "feed-my-accounting/vodafone-invoice"
)

//go:embed web
var webFiles embed.FS

// server is the web interface: a page per area of work, and the JSON API behind it. It reads
// the configuration afresh for every request, so that changes to config.yaml by hand count at
// once, and writes it back when the settings change.
type server struct {
	configPath string
	jobs       *Jobs
	saving     sync.Mutex

	mu sync.Mutex

	sessions *sessions // the logged-in browsers
	failures failures  // failed logins, by address
}

// runServe starts the web interface and serves until it fails.
func runServe(configPath string) error {
	cfg, err := loadConfig("", configPath)
	if err != nil {
		return err
	}
	s := &server{
		configPath: configPath,
		jobs:       NewJobs(filepath.Join(filepath.Dir(configPath), "feed-my-accounting-jobs.json")),
		sessions:   newSessions(filepath.Join(filepath.Dir(configPath), "feed-my-accounting-sessions.json")),
	}
	if err := s.change(moveOldPassword); err != nil {
		return err
	}
	liveView = &browser.Live{} // runs started here, by hand or on schedule, may ask in it
	go s.scheduler()
	httpServer := &http.Server{Addr: cfg.Web.Listen, Handler: s.handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("feed-my-accounting %s: web interface on %s, config %s", version, cfg.Web.Listen, configPath)
	return httpServer.ListenAndServe()
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	web, _ := fs.Sub(webFiles, "web")
	mux.Handle("GET /", fresh(http.FileServerFS(web)))
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/overview", s.overview)
	mux.HandleFunc("GET /api/schedule", s.getSchedule)
	mux.HandleFunc("PUT /api/schedule", s.putSchedule)
	mux.HandleFunc("GET /api/travel/plan", s.travelPlan)
	mux.HandleFunc("POST /api/travel/generate", s.travelRun(false))
	mux.HandleFunc("POST /api/travel/send", s.travelRun(true))
	mux.HandleFunc("PUT /api/travel/blocked", s.saveBlocked)
	mux.HandleFunc("PUT /api/travel/assignments", s.saveAssignments)
	// Anthropic's and Apple's receipts of a month: to look at, or sent — offered for download both.
	anthropicReceipts := receipts{"anthropic-invoice", "Anthropic-Belege",
		func(cfg *Config, m time.Time) ([]email.Attachment, error) {
			return anthropic.Collect(anthropicConfig(cfg), m)
		},
		func(cfg *Config, a []email.Attachment) error { return anthropic.Send(anthropicConfig(cfg), a) }, ""}
	appleReceipts := receipts{"apple-invoice", "Apple-Rechnungen",
		func(cfg *Config, m time.Time) ([]email.Attachment, error) { return apple.Collect(appleConfig(cfg), m) },
		func(cfg *Config, a []email.Attachment) error { return apple.Send(appleConfig(cfg), a) }, ""}
	// Amazon Business: the invoices of a month's orders, from amazon.de — on a month's last day
	// its own, else the one before; the month not chosen, once a month only, as Vodafone's.
	amazonReceipts := receipts{"amazon-invoice", "Amazon-Rechnungen",
		func(cfg *Config, _ time.Time) ([]email.Attachment, error) {
			return amazon.Collect(amazonConfig(cfg), amazon.FetchedMonth(time.Now()))
		},
		func(cfg *Config, a []email.Attachment) error { return amazon.Send(amazonConfig(cfg), a) }, "amazon"}
	mux.HandleFunc("POST /api/amazon/send", s.receiptsRun(amazonReceipts, true))
	mux.HandleFunc("POST /api/anthropic/view", s.receiptsRun(anthropicReceipts, false))
	mux.HandleFunc("POST /api/anthropic/send", s.receiptsRun(anthropicReceipts, true))
	mux.HandleFunc("POST /api/apple/view", s.receiptsRun(appleReceipts, false))
	mux.HandleFunc("POST /api/apple/send", s.receiptsRun(appleReceipts, true))
	// Vodafone's current invoices: the month is not chosen, MeinVodafone has the current one only.
	// Sent only, not to look at: each run is a login, and logins risk the account (17.11.2026).
	vodafoneReceipts := receipts{"vodafone-invoice", "Vodafone-Rechnungen",
		func(cfg *Config, _ time.Time) ([]email.Attachment, error) {
			return vodafone.Collect(vodafoneConfig(cfg))
		},
		func(cfg *Config, a []email.Attachment) error { return vodafone.Send(vodafoneConfig(cfg), a) }, "vodafone"}
	mux.HandleFunc("POST /api/vodafone/send", s.receiptsRun(vodafoneReceipts, true))
	mux.HandleFunc("POST /api/mail/view", s.mailReceiptsRun(false))
	mux.HandleFunc("POST /api/mail/send", s.mailReceiptsRun(true))
	mux.HandleFunc("POST /api/harvest/run", s.harvestRun(false))
	mux.HandleFunc("POST /api/harvest/worklog", s.harvestRun(true))
	mux.HandleFunc("GET /api/bmf-table", s.bmfTable)
	mux.HandleFunc("GET /api/logo", s.getLogo)
	mux.HandleFunc("PUT /api/logo", s.putLogo)
	mux.HandleFunc("DELETE /api/logo", s.deleteLogo)
	mux.HandleFunc("POST /api/harvest/upload", s.harvestUpload)
	mux.HandleFunc("POST /api/harvest/draft", s.harvestDraft)
	mux.HandleFunc("GET /api/closing", s.getClosing)
	mux.HandleFunc("DELETE /api/sent", s.forgetSent)
	mux.HandleFunc("POST /api/notify/test", s.testNotify)
	mux.HandleFunc("GET /api/jobs", s.listJobs)
	mux.HandleFunc("DELETE /api/jobs", s.clearJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("GET /api/jobs/{id}/files/{name}", s.getFile)
	mux.HandleFunc("GET /api/live", liveState)
	mux.HandleFunc("GET /api/live/shot", liveShot)
	mux.HandleFunc("POST /api/live/click", liveClick)
	mux.HandleFunc("POST /api/live/type", liveType)
	mux.HandleFunc("POST /api/live/key", liveKey)
	mux.HandleFunc("POST /api/live/scroll", liveScroll)
	mux.HandleFunc("POST /api/live/clear", liveClear)
	mux.HandleFunc("POST /api/live/fill", liveFill)
	mux.HandleFunc("POST /api/live/cancel", liveCancel)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	return s.auth(mux)
}

// fromHome says whether a request comes from the home network — through a proxy, the client's
// address counts, not the proxy's. Anything else is turned away, even if the port is open to the
// internet.
func fromHome(r *http.Request) bool { return homeAddress(net.ParseIP(clientIP(r))) }

// docker is 172.16.0.0/12, private but where Docker puts its containers: one of them, open to
// the internet, is not at home.
var docker = &net.IPNet{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)}

// homeAddress says whether the address is in the home network: 10.…, 192.168.…, local IPv6,
// loopback or link-local — not Docker's 172.16–31.
func homeAddress(ip net.IP) bool {
	return ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || (ip.IsPrivate() && !docker.Contains(ip)))
}

func (s *server) config() (*Config, error) { return loadConfig("", s.configPath) }

// say is the message in the page's language: German, or English when the page asks so in X-Lang.
func say(r *http.Request, de, en string) string {
	if r.Header.Get("X-Lang") == "en" {
		return en
	}
	return de
}

// fresh has the browser ask for the page's files again each time: after an update, a page kept
// from before would run its old script with the new page — and drop what it does not know of
// when saving (12.11.2026: a new checkbox, not saved).
func fresh(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Overview
// ---------------------------------------------------------------------------

func (s *server) overview(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	last := map[string]Job{}
	for _, job := range s.jobs.List() {
		if _, seen := last[job.Kind]; !seen {
			last[job.Kind] = job
		}
	}
	clients := make([]map[string]any, 0, len(cfg.TravelExpense.Clients))
	for _, c := range cfg.TravelExpense.Clients {
		clients = append(clients, map[string]any{"id": c.ID, "name": c.Name})
	}
	// Whether a login-gated site may be logged into by hand — its send button off if not — and
	// why: the schedule's next run, when it fetches them, or when the next login is allowed.
	gates := map[string]any{}
	for _, site := range []string{"amazon", "vodafone"} {
		now := time.Now()
		gate := map[string]any{"closed": manualGate(cfg, site, now) != nil}
		if at := scheduledLoginRun(cfg, site, now); !at.IsZero() {
			gate["scheduled"] = at
		} else if next := nextLogin(cfg, site, now); !next.IsZero() {
			gate["next"] = next
		}
		gates[site] = gate
	}
	reply(w, map[string]any{
		"gates":    gates,
		"version":  version,
		"employee": cfg.Staff.Current,
		"next":     nextRun(cfg, time.Now()),
		"clients":  clients,
		"sevdesk":  cfg.HarvestInvoice.SevDesk.ClientName,
		"mail":     map[string]string{"to": cfg.Mail.To, "cc": cfg.Mail.CC},
		"running":  s.jobs.Running(),
		"last":     last,
	})
}

// ---------------------------------------------------------------------------
// Schedule
// ---------------------------------------------------------------------------

func (s *server) getSchedule(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	last := map[string]Job{}
	for _, job := range s.jobs.List() {
		if _, seen := last[job.Kind]; !seen {
			last[job.Kind] = job
		}
	}
	type entry struct {
		ScheduleEntry
		Title string   `json:"title"`
		Next  []string `json:"next"`
		Error string   `json:"error,omitempty"`
		Last  *Job     `json:"last,omitempty"`
	}
	entries := []entry{}
	for _, e := range cfg.Schedule {
		out := entry{ScheduleEntry: e, Title: scheduledJobs[e.Job].title, Next: []string{}}
		if job, ok := last[e.Job]; ok {
			out.Last = &job
		}
		if spec, err := scheduleSpec(cfg, e.Cron); err != nil {
			out.Error = err.Error()
		} else {
			// As many as the page has room for, it shows; those the login gate holds back left out.
			for _, t := range runTimes(cfg, e.Job, spec, time.Now(), 6) {
				out.Next = append(out.Next, t.Format(time.RFC3339))
			}
		}
		entries = append(entries, out)
	}
	jobs := map[string]string{}
	for name, j := range scheduledJobs {
		jobs[name] = j.title
	}
	reply(w, map[string]any{"entries": entries, "jobs": jobs})
}

func (s *server) putSchedule(w http.ResponseWriter, r *http.Request) {
	var in []ScheduleEntry
	if !read(w, r, &in) {
		return
	}
	for _, e := range in {
		if _, ok := scheduledJobs[e.Job]; !ok {
			fail(w, http.StatusBadRequest, fmt.Errorf("no job %q", e.Job))
			return
		}
		if _, err := parseCron(e.Cron); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	done(w, s.change(func(cfg *Config) error {
		cfg.Schedule = in
		return nil
	}))
}

// ---------------------------------------------------------------------------
// Travel expenses
// ---------------------------------------------------------------------------

type planDay struct {
	Date     string  `json:"date"`
	Weekday  string  `json:"weekday"`
	Kind     string  `json:"kind"`
	Client   string  `json:"client,omitempty"`
	ClientNo int     `json:"clientNo,omitempty"` // its place in the client list, from 1: its colour
	Assigned bool    `json:"assigned,omitempty"`
	Note     string  `json:"note,omitempty"`
	Meal     float64 `json:"meal,omitempty"`
	MealText string  `json:"mealText,omitempty"`
	Mileage  float64 `json:"mileage,omitempty"`
	Missing  string  `json:"missing,omitempty"`
	// The night's flat rate, for a night that starts on the day; lodgingMissing: the inland one
	// abroad.
	Lodging        float64 `json:"lodging,omitempty"`
	LodgingMissing bool    `json:"lodgingMissing,omitempty"`
}

var weekdays = [...]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}

func (s *server) travelPlan(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	year, month, err := monthParam(r.URL.Query().Get("month"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	tc, err := travelConfig(cfg)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	var days []planDay
	totals := map[string]float64{}
	perClient := map[string]int{}
	for _, d := range travelexpense.Plan(tc, year, month) {
		day := planDay{
			Date: d.Date.Format(time.DateOnly), Weekday: weekdays[d.Date.Weekday()], Kind: d.Kind,
			Assigned: d.Assigned, Note: d.Note, Meal: d.Meal, MealText: d.MealText, Mileage: d.Mileage,
			Missing: d.Missing, Lodging: d.Lodging, LodgingMissing: d.LodgingMissing,
		}
		if d.Client >= 0 {
			day.Client, day.ClientNo = tc.Clients[d.Client].Name, d.Client+1
			perClient[day.Client]++
			totals["days"]++
		}
		totals["meals"] += d.Meal
		totals["mileage"] += d.Mileage
		totals["lodging"] += d.Lodging
		if d.Lodging > 0 {
			totals["nights"]++
		}
		days = append(days, day)
	}
	te := cfg.TravelExpense
	reply(w, map[string]any{
		"month":       fmt.Sprintf("%d-%02d", year, month),
		"days":        days,
		"totals":      totals,
		"perClient":   perClient,
		"clients":     clientNames(tc.Clients),
		"blocked":     orEmpty(te.Blocked),
		"assignments": orEmpty(te.Assignments),
	})
}

// clientNames are the clients' names in the order of the list, for the calendar's legend.
func clientNames(clients []travelexpense.Client) []string {
	names := make([]string, len(clients))
	for i, c := range clients {
		names[i] = c.Name
	}
	return names
}

func (s *server) travelRun(send bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Month string `json:"month"`
		}
		if !read(w, r, &in) {
			return
		}
		year, month, err := monthParam(in.Month)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		cfg, err := s.config()
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		tc, err := travelConfig(cfg)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		title := fmt.Sprintf("Reisekosten %02d/%d erzeugen", month, year)
		if send {
			title = fmt.Sprintf("Reisekosten %02d/%d senden", month, year)
		}
		s.start(w, "travel-expense", title, func() ([]File, error) {
			result, err := travelexpense.Generate(tc, year, month)
			if err != nil {
				return nil, err
			}
			var files []File
			for _, d := range result.Documents() {
				files = append(files, File{Name: d.Filename, Size: len(d.Data), data: d.Data})
			}
			if send {
				return files, travelexpense.Send(tc, year, month, result)
			}
			return files, nil
		})
	}
}

func (s *server) saveBlocked(w http.ResponseWriter, r *http.Request) {
	var in []DateRange
	if !read(w, r, &in) {
		return
	}
	for _, b := range in {
		if _, _, err := parseRange(b.From, b.To); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	done(w, s.change(func(cfg *Config) error {
		cfg.TravelExpense.Blocked = in
		return nil
	}))
}

func (s *server) saveAssignments(w http.ResponseWriter, r *http.Request) {
	var in []Assignment
	if !read(w, r, &in) {
		return
	}
	done(w, s.change(func(cfg *Config) error {
		for _, a := range in {
			if _, _, err := parseRange(a.From, a.To); err != nil {
				return err
			}
			if findClient(cfg.TravelExpense.Clients, a.Client) < 0 {
				return fmt.Errorf("no client %q", a.Client)
			}
		}
		cfg.TravelExpense.Assignments = in
		return nil
	}))
}

// ---------------------------------------------------------------------------
// Belege (Amazon, Anthropic, Apple, Vodafone), Harvest
// ---------------------------------------------------------------------------

// receipts are a mail-based kind of receipt: how to collect a month's PDFs, and send them.
type receipts struct {
	kind, title string
	collect     func(*Config, time.Time) ([]email.Attachment, error)
	send        func(*Config, []email.Attachment) error
	gate        string // the site whose logins the login gate holds back; "" for none
}

// receiptsRun collects the receipts of a month — "11/2026", as on the command line, empty the
// current one — and offers them for download; with send, it mails them too.
func (s *server) receiptsRun(r receipts, send bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var in struct {
			Month string `json:"month"`
			Force bool   `json:"force"` // past the login gate, asked first
		}
		if !read(w, req, &in) {
			return
		}
		year, month, err := slashMonth(in.Month)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		cfg, err := s.config()
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		// A login the gate holds back: said, for the page to ask whether to run anyway.
		if r.gate != "" && !in.Force {
			if err := manualGate(cfg, r.gate, time.Now()); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "gated": true})
				return
			}
		}
		at := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
		verb := "ansehen"
		if send {
			verb = "senden"
		}
		s.start(w, r.kind, fmt.Sprintf("%s %d/%d %s", r.title, month, year, verb), func() ([]File, error) {
			var pdfs []email.Attachment
			collect := func() (err error) { pdfs, err = r.collect(cfg, at); return err }
			var err error
			if r.gate != "" {
				err = withLogin(cfg, r.gate, collect)
			} else {
				err = collect()
			}
			if err != nil {
				return nil, err
			}
			var files []File
			for _, p := range pdfs {
				files = append(files, File{Name: p.Filename, Size: len(p.Data), data: p.Data})
			}
			if send {
				err = r.send(cfg, pdfs)
				// Amazon's month is not chosen: the one it fetched.
				if r.kind == "amazon-invoice" {
					at = amazon.FetchedMonth(time.Now())
				}
				err = checked(cfg, r.kind, at, err)
			}
			return files, err
		})
	}
}

// mailReceiptsRun takes the further senders' receipts of a month, by their rules: to look at,
// or sent — offered for download both.
func (s *server) mailReceiptsRun(send bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var in struct {
			Month string `json:"month"`
		}
		if !read(w, req, &in) {
			return
		}
		year, month, err := slashMonth(in.Month)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		cfg, err := s.config()
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		verb := "ansehen"
		if send {
			verb = "senden"
		}
		at := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
		s.start(w, "mail-receipts", fmt.Sprintf("Weitere Absender %d/%d %s", month, year, verb), func() ([]File, error) {
			var pdfs []email.Attachment
			err := mailReceipts(cfg, at, send, &pdfs)
			var files []File
			for _, p := range pdfs {
				files = append(files, File{Name: p.Filename, Size: len(p.Data), data: p.Data})
			}
			return files, err
		})
	}
}

// harvestRun makes the sevDesk draft for a month's hours in Harvest and their work log — with
// logOnly just the work log. The month is "11/2026", as on the command line; empty is the
// current one.
func (s *server) harvestRun(logOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Month string `json:"month"`
		}
		if !read(w, r, &in) {
			return
		}
		year, month, err := slashMonth(in.Month)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		cfg, err := s.config()
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		if cfg.HarvestInvoice.Harvest.Token == "" {
			fail(w, http.StatusBadRequest, errors.New(say(r, "kein Harvest-API-Token: unter „Einstellungen Kundenrechnung“ eintragen", "no Harvest API token: enter it under “Invoice settings”")))
			return
		}
		title := fmt.Sprintf("Kundenrechnung %d/%d aus Harvest", month, year)
		if logOnly {
			title = fmt.Sprintf("Zeitnachweis %d/%d aus Harvest", month, year)
		}
		s.start(w, "harvest-invoice", title, func() ([]File, error) {
			return withFile(runHarvestInvoice(cfg, year, month, logOnly))
		})
	}
}

// ---------------------------------------------------------------------------
// The BMF's table
// ---------------------------------------------------------------------------

// bmfTableURL is the BMF's table of the rates abroad — meals and nights, by country — in the
// Lohnsteuer-Handbuch of a year: the same address each year, but for the year.
func bmfTableURL(year int) string {
	return fmt.Sprintf("https://lsth.bundesfinanzministerium.de/lsth/%d/B-Anhaenge/Anhang-25/I/inhalt.html", year)
}

// bmfTables keeps the years whose table was found: it stays where it is.
var bmfTables sync.Map

// bmfTable sends the browser to the year's table, or to the year before's while the year's is
// not out yet — the handbook comes in the year's first weeks.
func (s *server) bmfTable(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || year < 2025 || year > 2100 {
		year = time.Now().Year()
	}
	for _, y := range []int{year, year - 1} {
		if _, found := bmfTables.Load(y); found || bmfTableThere(r.Context(), bmfTableURL(y)) {
			bmfTables.Store(y, true)
			http.Redirect(w, r, bmfTableURL(y), http.StatusFound)
			return
		}
	}
	// Neither found — the BMF's site slow or down, say: the year's, as asked.
	http.Redirect(w, r, bmfTableURL(year), http.StatusFound)
}

// bmfTableThere says whether the page is there: a year's handbook not out yet is a 404.
func bmfTableThere(ctx context.Context, url string) bool {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

// ---------------------------------------------------------------------------
// Logo
// ---------------------------------------------------------------------------

// getLogo is the company's logo, for the settings page to show; 404 without one.
func (s *server) getLogo(w http.ResponseWriter, r *http.Request) {
	cfg, err := readConfig(s.configPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	path := cfg.logoPath()
	if path == "" {
		fail(w, http.StatusNotFound, errors.New("no logo"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}

// maxLogo is the largest logo taken: plenty for print, where the work log shrinks it to some
// 60 KB anyway. The upload is kept as it is.
const maxLogo = 2 << 20

// putLogo keeps an uploaded PNG or JPEG as the company's logo, beside the configuration. The
// logo it replaces is kept as its .bak.
func (s *server) putLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogo+64<<10) // the form around it too
	file, _, err := r.FormFile("logo")
	if err != nil {
		fail(w, http.StatusBadRequest, uploadError(r, err, "no logo uploaded"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxLogo+1))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if len(data) > maxLogo {
		fail(w, http.StatusBadRequest, errors.New(say(r, "das Logo ist größer als 2 MB", "the logo is larger than 2 MB")))
		return
	}
	// What the PDFs can show: PNG and JPEG, read as such, whatever the file was called.
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	ext := map[string]string{"png": ".png", "jpeg": ".jpg"}[format]
	if err != nil || ext == "" {
		fail(w, http.StatusBadRequest, errors.New(say(r, "das Logo muss ein PNG oder JPEG sein", "the logo must be a PNG or JPEG")))
		return
	}
	cfg, err := readConfig(s.configPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if err := setAsideLogo(cfg); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if err := os.WriteFile(filepath.Join(cfg.dir, logoBase+ext), data, 0o644); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, map[string]bool{"ok": true})
}

// deleteLogo takes the logo away: the work log shows the company's name again. The file is kept
// as its .bak.
func (s *server) deleteLogo(w http.ResponseWriter, r *http.Request) {
	cfg, err := readConfig(s.configPath)
	if err == nil {
		err = setAsideLogo(cfg)
	}
	done(w, err)
}

// setAsideLogo renames the logo there is, if any, to its .bak: feed-my-accounting-logo.bak.png.
func setAsideLogo(cfg *Config) error {
	path := cfg.logoPath()
	if path == "" {
		return nil
	}
	ext := filepath.Ext(path)
	return os.Rename(path, strings.TrimSuffix(path, ext)+".bak"+ext)
}

// maxReport is the largest Harvest report taken: a month's is some 100 KB.
const maxReport = 10 << 20

// uploadError is why an upload was not taken: too big, or not there.
func uploadError(r *http.Request, err error, missing string) error {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return fmt.Errorf(say(r, "zu groß: höchstens %d MB", "too large: %d MB at most"), tooBig.Limit>>20)
	}
	return errors.New(missing)
}

// harvestUpload reads an uploaded work log PDF, Harvest's or one from here: its hours and period,
// to fill Manuelle Eingabe's fields. Nothing is kept: the draft takes what the fields say.
func (s *server) harvestUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxReport)
	file, _, err := r.FormFile("report")
	if err != nil {
		fail(w, http.StatusBadRequest, uploadError(r, err, "no PDF uploaded"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	report, err := harvest.ParseReportPDF(data)
	if err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("not a Harvest report: %w", err))
		return
	}
	reply(w, map[string]any{
		"hours": report.TotalHours,
		"from":  dateOrEmpty(report.PeriodFrom),
		"to":    dateOrEmpty(report.PeriodTo),
	})
}

// harvestDraft creates the sevDesk draft for a period and its hours as Manuelle Eingabe has them:
// typed, or filled from a PDF and checked.
func (s *server) harvestDraft(w http.ResponseWriter, r *http.Request) {
	var in struct {
		From  string  `json:"from"`
		To    string  `json:"to"`
		Hours float64 `json:"hours"`
	}
	if !read(w, r, &in) {
		return
	}
	from, to, err := parseRange(in.From, in.To)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if in.Hours <= 0 {
		fail(w, http.StatusBadRequest, errors.New(say(r, "Stunden fehlen", "hours missing")))
		return
	}
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	report := harvest.ReportData{TotalHours: in.Hours, PeriodFrom: from, PeriodTo: to}
	title := fmt.Sprintf("Rechnungsentwurf %s – %s, %.2f h", from.Format("02.01."), to.Format("02.01.2006"), in.Hours)
	s.start(w, "harvest-invoice", title, func() ([]File, error) {
		if err := harvest.CreateDraft(harvestConfig(cfg), &report); err != nil {
			return nil, err
		}
		recordDraft(cfg, time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.Local), "")
		return nil, nil
	})
}

// ---------------------------------------------------------------------------
// Live view: a page a run hands over for a moment — Vodafone's login with a captcha
// ---------------------------------------------------------------------------

// liveState says whether a run asks for someone, and why: the page polls it.
func liveState(w http.ResponseWriter, r *http.Request) {
	reason := ""
	if liveView != nil {
		reason = liveView.Asking()
	}
	reply(w, map[string]any{"asking": reason != "", "reason": reason, "fill": liveView != nil && liveView.Fills()})
}

// liveShot is the page as it is now.
func liveShot(w http.ResponseWriter, r *http.Request) {
	if liveView == nil {
		fail(w, http.StatusNotFound, errors.New("no live view"))
		return
	}
	data, err := liveView.Shot()
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func liveClick(w http.ResponseWriter, r *http.Request) {
	var in struct{ X, Y float64 }
	if read(w, r, &in) && liveOn(w) {
		done(w, liveView.Click(in.X, in.Y))
	}
}

func liveType(w http.ResponseWriter, r *http.Request) {
	var in struct{ Text string }
	if read(w, r, &in) && liveOn(w) {
		done(w, liveView.Type(in.Text))
	}
}

func liveKey(w http.ResponseWriter, r *http.Request) {
	var in struct{ Key string }
	if read(w, r, &in) && liveOn(w) {
		done(w, liveView.Key(in.Key))
	}
}

func liveScroll(w http.ResponseWriter, r *http.Request) {
	var in struct{ X, Y, DY float64 }
	if read(w, r, &in) && liveOn(w) {
		done(w, liveView.Scroll(in.X, in.Y, in.DY))
	}
}

func liveFill(w http.ResponseWriter, r *http.Request) {
	if liveOn(w) {
		log.Printf("Live view: stored password filled in")
		done(w, liveView.Fill())
	}
}

func liveClear(w http.ResponseWriter, r *http.Request) {
	if liveOn(w) {
		log.Printf("Live view: field cleared")
		done(w, liveView.Clear())
	}
}

func liveCancel(w http.ResponseWriter, r *http.Request) {
	if liveOn(w) {
		liveView.Cancel()
		done(w, nil)
	}
}

func liveOn(w http.ResponseWriter) bool {
	if liveView == nil {
		fail(w, http.StatusNotFound, errors.New("no live view"))
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

func (s *server) start(w http.ResponseWriter, kind, title string, work work) {
	job, err := s.jobs.Start(kind, title, work)
	if err != nil {
		fail(w, http.StatusConflict, err)
		return
	}
	reply(w, job)
}

func (s *server) listJobs(w http.ResponseWriter, r *http.Request) { reply(w, s.jobs.List()) }

func (s *server) clearJobs(w http.ResponseWriter, r *http.Request) {
	s.jobs.Clear()
	reply(w, map[string]bool{"ok": true})
}

func (s *server) getJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.jobs.Get(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, errors.New("no such job"))
		return
	}
	reply(w, job)
}

func (s *server) getFile(w http.ResponseWriter, r *http.Request) {
	f, ok := s.jobs.File(r.PathValue("id"), r.PathValue("name"))
	if !ok {
		fail(w, http.StatusNotFound, errors.New("no such file: files are kept until the server restarts"))
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", f.Name))
	_, _ = w.Write(f.data)
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

// secrets are the configuration's passwords: never sent to the page, and kept when it sends
// them back empty.
func secrets(cfg *Config) []*string {
	return []*string{
		&cfg.Mail.Password,
		&cfg.VodafoneInvoice.Password,
		&cfg.HarvestInvoice.SevDesk.Password,
		&cfg.HarvestInvoice.Harvest.Token,
		&cfg.Web.ProxyToken,
		&cfg.AmazonInvoice.Password,
		&cfg.Notify.Pushover.Token,
		&cfg.Notify.Pushover.User,
	}
}

func (s *server) getSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := readConfig(s.configPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	set := make([]bool, 0, 4)
	for _, p := range secrets(cfg) {
		set = append(set, *p != "")
		*p = ""
	}
	// Who has a password, not the hashes.
	logins := map[string]bool{}
	for name := range cfg.Staff.Logins {
		logins[name] = true
	}
	cfg.Staff.Logins = nil
	// Which further mailbox has a password, not the passwords.
	mailboxes := make([]bool, len(cfg.Mail.Mailboxes))
	for i := range cfg.Mail.Mailboxes {
		mailboxes[i] = cfg.Mail.Mailboxes[i].Password != ""
		cfg.Mail.Mailboxes[i].Password = ""
	}
	reply(w, map[string]any{"config": cfg, "secretsSet": set, "logins": logins, "mailboxes": mailboxes, "path": s.configPath, "countries": travelexpense.Countries})
}

func (s *server) putSettings(w http.ResponseWriter, r *http.Request) {
	var in Config
	if !read(w, r, &in) {
		return
	}
	var token string
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		token = cookie.Value
	}
	done(w, s.change(func(cfg *Config) error {
		for i, p := range secrets(&in) {
			if *p == "" {
				*p = *secrets(cfg)[i]
			}
		}
		// Blocked days, assignments and the schedule are edited on their own pages.
		in.TravelExpense.Blocked, in.TravelExpense.Assignments = cfg.TravelExpense.Blocked, cfg.TravelExpense.Assignments
		in.Schedule = cfg.Schedule
		in.Web.Password = cfg.Web.Password
		// The logins kept, new passwords hashed into them, those to go gone — and with a new
		// password or none, the employee's other sessions ended: the caller's own stays.
		in.Staff.Logins = cfg.Staff.Logins
		if in.Staff.Logins == nil {
			in.Staff.Logins = map[string]string{}
		}
		for name, password := range in.Staff.Passwords {
			if password = strings.TrimSpace(password); password == "" || !slices.Contains(in.Staff.Names, name) {
				continue
			}
			if len(password) < 8 {
				return fmt.Errorf(say(r, "das Passwort für %s ist kürzer als 8 Zeichen", "the password for %s is shorter than 8 characters"), name)
			}
			hash, err := hashPassword(password)
			if err != nil {
				return err
			}
			in.Staff.Logins[name] = hash
			s.sessions.endFor(name, token)
		}
		for _, name := range in.Staff.NoLogin {
			delete(in.Staff.Logins, name)
			s.sessions.endFor(name, "")
		}
		in.Staff.Passwords, in.Staff.NoLogin = nil, nil
		// A further mailbox's password left empty: the one saved for that address.
		for i, m := range in.Mail.Mailboxes {
			if m.Password != "" {
				continue
			}
			for _, old := range cfg.Mail.Mailboxes {
				if strings.EqualFold(old.Username, m.Username) {
					in.Mail.Mailboxes[i].Password = old.Password
				}
			}
		}
		*cfg = in
		return nil
	}))
}

// moveOldPassword makes the page's one password of before, web.password, the selected
// employee's login — the first employee's without one selected — unless they have one already.
func moveOldPassword(cfg *Config) error {
	if cfg.Web.Password == "" || len(cfg.Staff.Names) == 0 {
		return nil
	}
	name := cfg.Staff.Current
	if name == "" {
		name = cfg.Staff.Names[0]
	}
	if cfg.Staff.Logins == nil {
		cfg.Staff.Logins = map[string]string{}
	}
	if cfg.Staff.Logins[name] == "" {
		hash, err := hashPassword(cfg.Web.Password)
		if err != nil {
			return err
		}
		cfg.Staff.Logins[name] = hash
		log.Printf("web.password is now the login of %s", name)
	}
	cfg.Web.Password = ""
	return nil
}

// change edits the configuration as written and saves it, unless edit fails.
func (s *server) change(edit func(*Config) error) error {
	s.saving.Lock()
	defer s.saving.Unlock()
	cfg, err := readConfig(s.configPath)
	if err != nil {
		return err
	}
	if err := edit(cfg); err != nil {
		return err
	}
	return saveConfig(s.configPath, cfg)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// monthParam reads "2026-12"; empty is this month.
func monthParam(s string) (int, time.Month, error) {
	if s == "" {
		now := time.Now()
		return now.Year(), now.Month(), nil
	}
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return 0, 0, fmt.Errorf("month %q: want YYYY-MM", s)
	}
	return t.Year(), t.Month(), nil
}

func dateOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

func read(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(into); err != nil {
		fail(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func done(w http.ResponseWriter, err error) {
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w, map[string]bool{"ok": true})
}

func fail(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// lastJobs are the latest run of each job.
func (s *server) lastJobs() map[string]Job {
	last := map[string]Job{}
	for _, job := range s.jobs.List() {
		if _, seen := last[job.Kind]; !seen && job.Status != "running" {
			last[job.Kind] = job
		}
	}
	return last
}

// remind looks over the month before now's, and tells of what it still waits for.
func (s *server) remind(cfg *Config, now time.Time) {
	month := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.Local)
	d, err := ledgerOf(cfg).Data()
	if err != nil {
		log.Printf("closing: %v", err)
		return
	}
	remindClosing(cfg, closing(cfg, d, month, s.lastJobs(), now), month)
}

// getClosing is a month's books, source by source: ?month=2026-11, the current one without.
func (s *server) getClosing(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	now := time.Now()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	if m := r.URL.Query().Get("month"); m != "" {
		if month, err = time.ParseInLocation("2006-01", m, time.Local); err != nil {
			fail(w, http.StatusBadRequest, fmt.Errorf("month %q: want YYYY-MM", m))
			return
		}
	}
	d, err := ledgerOf(cfg).Data()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, map[string]any{
		"month":      month.Format("2006-01"),
		"rows":       closing(cfg, d, month, s.lastJobs(), now),
		"closingDay": cfg.Notify.closingDay(),
		"notify":     notifyOn(cfg),
	})
}

// forgetSent takes a receipt out of the ledger: the next run may send it again.
func (s *server) forgetSent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Source string `json:"source"`
		ID     string `json:"id"` // its hash, or its name for one without
	}
	if !read(w, r, &in) {
		return
	}
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	found, err := ledgerOf(cfg).Forget(in.Source, in.ID)
	switch {
	case err != nil:
		fail(w, http.StatusInternalServerError, err)
	case !found:
		fail(w, http.StatusNotFound, errors.New(say(r, "nicht im Verzeichnis", "not in the ledger")))
	default:
		reply(w, map[string]bool{"ok": true})
	}
}

// testNotify sends a notification to see that they arrive, with the settings as saved.
func (s *server) testNotify(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if !notifyOn(cfg) {
		fail(w, http.StatusBadRequest, errors.New(say(r, "Pushover ist nicht eingerichtet: einschalten, Token und User-Key eintragen und speichern", "Pushover is not set up: switch it on, enter token and user key, and save")))
		return
	}
	if err := push(cfg, "feed-my-accounting: Test", "So kommen die Nachrichten an."); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	reply(w, map[string]bool{"ok": true})
}
