package main

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	harvest "feed-my-accounting/harvest-invoice"
	travelexpense "feed-my-accounting/travel-expense"
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

	mu      sync.Mutex
	uploads map[string]*harvest.ReportData // Harvest PDFs uploaded, by id, until used
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
		uploads:    map[string]*harvest.ReportData{},
	}
	go s.scheduler()
	httpServer := &http.Server{Addr: cfg.Web.Listen, Handler: s.handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("feed-my-accounting %s: web interface on %s, config %s", version, cfg.Web.Listen, configPath)
	return httpServer.ListenAndServe()
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	web, _ := fs.Sub(webFiles, "web")
	mux.Handle("GET /", http.FileServerFS(web))
	mux.HandleFunc("GET /api/overview", s.overview)
	mux.HandleFunc("GET /api/schedule", s.getSchedule)
	mux.HandleFunc("PUT /api/schedule", s.putSchedule)
	mux.HandleFunc("GET /api/travel/plan", s.travelPlan)
	mux.HandleFunc("POST /api/travel/generate", s.travelRun(false))
	mux.HandleFunc("POST /api/travel/send", s.travelRun(true))
	mux.HandleFunc("PUT /api/travel/blocked", s.saveBlocked)
	mux.HandleFunc("PUT /api/travel/assignments", s.saveAssignments)
	mux.HandleFunc("POST /api/apple/run", s.simpleRun("apple-invoice", "Apple-Rechnungen abholen", runAppleInvoice))
	mux.HandleFunc("POST /api/vodafone/run", s.simpleRun("vodafone-invoice", "Vodafone-Rechnungen abholen", runVodafoneInvoice))
	mux.HandleFunc("POST /api/harvest/run", s.simpleRun("harvest-invoice", "Kundenrechnung aus Harvest-Mail", runHarvestInvoice))
	mux.HandleFunc("POST /api/harvest/upload", s.harvestUpload)
	mux.HandleFunc("POST /api/harvest/draft", s.harvestDraft)
	mux.HandleFunc("GET /api/jobs", s.listJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("GET /api/jobs/{id}/files/{name}", s.getFile)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	return s.auth(mux)
}

// auth asks for web.password, if one is set, by HTTP basic auth; any user name will do.
func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg, err := readConfig(s.configPath)
		if err == nil && cfg.Web.Password != "" {
			_, given, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(given), []byte(cfg.Web.Password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="feed-my-accounting"`)
				http.Error(w, "password required", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) config() (*Config, error) { return loadConfig("", s.configPath) }

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
		clients = append(clients, map[string]any{"id": c.ID, "name": c.Name, "onlyAssigned": c.OnlyAssigned})
	}
	reply(w, map[string]any{
		"version":  version,
		"employee": cfg.TravelExpense.Employee,
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
		if spec, err := parseCron(e.Cron); err != nil {
			out.Error = err.Error()
		} else {
			for t, i := time.Now(), 0; i < 3; i++ {
				if t = spec.next(t); t.IsZero() {
					break
				}
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
	Assigned bool    `json:"assigned,omitempty"`
	Note     string  `json:"note,omitempty"`
	Meal     float64 `json:"meal,omitempty"`
	MealText string  `json:"mealText,omitempty"`
	Mileage  float64 `json:"mileage,omitempty"`
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
		}
		if d.Client >= 0 {
			day.Client = tc.Clients[d.Client].Name
			perClient[day.Client]++
			totals["days"]++
		}
		totals["meals"] += d.Meal
		totals["mileage"] += d.Mileage
		days = append(days, day)
	}
	te := cfg.TravelExpense
	reply(w, map[string]any{
		"month":       fmt.Sprintf("%d-%02d", year, month),
		"days":        days,
		"totals":      totals,
		"perClient":   perClient,
		"blocked":     orEmpty(te.Blocked),
		"assignments": orEmpty(te.Assignments),
	})
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
// Apple, Vodafone, Harvest
// ---------------------------------------------------------------------------

// simpleRun runs a module as the command line does, with the configuration as it is.
func (s *server) simpleRun(kind, title string, run func(*Config) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := s.config()
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		s.start(w, kind, title, func() ([]File, error) { return nil, run(cfg) })
	}
}

// harvestUpload reads an uploaded Harvest report PDF: its hours and period, for checking before
// the draft is made.
func (s *server) harvestUpload(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("report")
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("no PDF uploaded"))
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
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	id := newJobID()
	s.mu.Lock()
	s.uploads = map[string]*harvest.ReportData{id: report} // the latest upload only
	s.mu.Unlock()
	sd := cfg.HarvestInvoice.SevDesk
	reply(w, map[string]any{
		"id":        id,
		"hours":     report.TotalHours,
		"from":      dateOrEmpty(report.PeriodFrom),
		"to":        dateOrEmpty(report.PeriodTo),
		"client":    sd.ClientName,
		"product":   strings.TrimSpace(sd.ProductNum + " " + sd.ProductName),
		"reference": sd.ReferenceNum,
	})
}

func (s *server) harvestDraft(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID    string  `json:"id"`
		Hours float64 `json:"hours"`
		From  string  `json:"from"`
		To    string  `json:"to"`
	}
	if !read(w, r, &in) {
		return
	}
	s.mu.Lock()
	uploaded := s.uploads[in.ID]
	s.mu.Unlock()
	if uploaded == nil {
		fail(w, http.StatusBadRequest, errors.New("upload the report again: it is no longer here"))
		return
	}
	from, to, err := parseRange(in.From, in.To)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	report := *uploaded
	report.TotalHours, report.PeriodFrom, report.PeriodTo = in.Hours, from, to
	title := fmt.Sprintf("Rechnungsentwurf %s – %s, %.2f h", from.Format("02.01."), to.Format("02.01.2006"), in.Hours)
	s.start(w, "harvest-invoice", title, func() ([]File, error) {
		return nil, harvest.CreateDraft(harvestConfig(cfg), &report)
	})
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
		&cfg.HarvestInvoice.Harvest.Password,
		&cfg.HarvestInvoice.SevDesk.Password,
		&cfg.Web.Password,
	}
}

func (s *server) getSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := readConfig(s.configPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	set := make([]bool, 0, 5)
	for _, p := range secrets(cfg) {
		set = append(set, *p != "")
		*p = ""
	}
	reply(w, map[string]any{"config": cfg, "secretsSet": set, "path": s.configPath, "countries": travelexpense.Countries})
}

func (s *server) putSettings(w http.ResponseWriter, r *http.Request) {
	var in Config
	if !read(w, r, &in) {
		return
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
		*cfg = in
		return nil
	}))
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
