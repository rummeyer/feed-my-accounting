// Package main is the feed-my-accounting orchestrator.
// It combines travel expense reports, Apple invoice processing,
// and Vodafone invoice downloading into a single tool.
//
// Usage:
//
//	feed-my-accounting [--config path] [command] [args...]
//
// Commands:
//
//	all [M/YYYY]              Run all modules (default when no command given)
//	travel-expense [M/YYYY]   Generate and send monthly travel expense PDFs
//	anthropic-invoice         Fetch Anthropic receipt emails and send their PDFs
//	apple-invoice             Fetch Apple invoice emails and send as PDFs
//	vodafone-invoice          Download Vodafone invoices and send via email
//	harvest-invoice           Create sevDesk invoice from Harvest monthly report
//	serve                     Web interface on web.listen (default :8595)
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	anthropic "feed-my-accounting/anthropic-invoice"
	apple "feed-my-accounting/apple-invoice"
	"feed-my-accounting/email"
	harvest "feed-my-accounting/harvest-invoice"
	"feed-my-accounting/pdfdoc"
	travelexpense "feed-my-accounting/travel-expense"
	vodafone "feed-my-accounting/vodafone-invoice"
)

const version = "1.17.0"

var monthArgRegex = regexp.MustCompile(`^(0?[1-9]|1[0-2])/(20[0-9]{2})$`)

// logOut is where all module log output is written. Defaults to stderr;
// main() tees it to a log file next to the executable at startup.
var logOut io.Writer = os.Stderr

// setupLogging opens <exe-name>.log next to the executable and routes all
// module loggers to tee output to both stderr and the file. On failure,
// logs a warning to stderr and continues with stderr-only output.
func setupLogging() (io.Closer, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(exe)
	base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	path := filepath.Join(dir, base+".log")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	logOut = io.MultiWriter(os.Stderr, f)
	setModuleLogs(logOut)
	return f, nil
}

// setModuleLogs routes every module's log output to w.
func setModuleLogs(w io.Writer) {
	anthropic.SetLogOutput(w)
	apple.SetLogOutput(w)
	vodafone.SetLogOutput(w)
	harvest.SetLogOutput(w)
	travelexpense.SetLogOutput(w)
	email.SetLogOutput(w)
	log.SetOutput(w)
}

func main() {
	args := os.Args[1:]

	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("feed-my-accounting v%s\n", version)
			return
		}
	}

	if closer, err := setupLogging(); err != nil {
		fmt.Fprintf(logOut, "WARNING: could not open log file: %v\n", err)
	} else {
		defer closer.Close()
	}

	// Parse --config flag
	var configPath string
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			configPath = args[i+1]
			args = append(args[:i], args[i+2:]...)
			break
		}
	}

	if configPath == "" {
		found, err := findConfigFile("config.yaml")
		if err != nil {
			fmt.Fprintf(logOut, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		configPath = found
	}
	if abs, err := filepath.Abs(configPath); err == nil {
		configPath = abs
	}
	cfg, err := loadConfig("config.yaml", configPath)
	if err != nil {
		fmt.Fprintf(logOut, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var subcommand string
	var remaining []string
	if len(args) == 0 {
		subcommand = "all"
	} else {
		subcommand = args[0]
		remaining = args[1:]
	}

	switch subcommand {
	case "all":
		year, month := parseMonthArg(remaining)
		if err := runTravelExpense(cfg, year, month); err != nil {
			fmt.Fprintf(logOut, "travel-expense error: %v\n", err)
			os.Exit(1)
		}
		if err := runAnthropicInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "anthropic-invoice error: %v\n", err)
			os.Exit(1)
		}
		if err := runAppleInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "apple-invoice error: %v\n", err)
			os.Exit(1)
		}
		if err := runVodafoneInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "vodafone-invoice error: %v\n", err)
			os.Exit(1)
		}
		if err := harvestFromCommandLine(cfg, year, month); err != nil {
			fmt.Fprintf(logOut, "harvest-invoice error: %v\n", err)
			os.Exit(1)
		}

	case "travel-expense":
		year, month := parseMonthArg(remaining)
		if err := runTravelExpense(cfg, year, month); err != nil {
			fmt.Fprintf(logOut, "travel-expense error: %v\n", err)
			os.Exit(1)
		}

	case "anthropic-invoice":
		if err := runAnthropicInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "anthropic-invoice error: %v\n", err)
			os.Exit(1)
		}

	case "apple-invoice":
		if err := runAppleInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "apple-invoice error: %v\n", err)
			os.Exit(1)
		}

	case "vodafone-invoice":
		if err := runVodafoneInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "vodafone-invoice error: %v\n", err)
			os.Exit(1)
		}

	case "harvest-invoice":
		year, month := parseMonthArg(remaining)
		if err := harvestFromCommandLine(cfg, year, month); err != nil {
			fmt.Fprintf(logOut, "harvest-invoice error: %v\n", err)
			os.Exit(1)
		}

	case "serve":
		if err := runServe(configPath); err != nil {
			fmt.Fprintf(logOut, "serve error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(logOut, "Unknown command: %q\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func runTravelExpense(cfg *Config, year int, month time.Month) error {
	tc, err := travelConfig(cfg)
	if err != nil {
		return err
	}
	return travelexpense.Run(tc, year, month)
}

// travelConfig turns the YAML's travel-expense section into the module's: dates parsed, and
// assignments pointing at their client.
func travelConfig(cfg *Config) (travelexpense.Config, error) {
	te := cfg.TravelExpense
	if cfg.Staff.Current == "" {
		return travelexpense.Config{}, errors.New("travel-expense: no employee for the PDF's Lieferant: add one under Einstellungen › Mitarbeiter (staff.names)")
	}
	clients := make([]travelexpense.Client, len(te.Clients))
	for i, c := range te.Clients {
		clients[i] = travelexpense.Client{
			ID: c.ID, Name: c.Name, From: c.From, To: c.To,
			Reason: c.Reason, Distance: c.Distance, Country: c.Country, Province: c.Province,
			MealDay: c.MealDay, MealFullDay: c.MealFullDay, MealTravelDay: c.MealTravelDay,
			Lodging: c.Lodging,
		}
	}
	for _, c := range te.Clients {
		if code := strings.ToUpper(strings.TrimSpace(c.Country)); code != "" {
			if _, known := travelexpense.Countries[code]; !known {
				return travelexpense.Config{}, fmt.Errorf("travel-expense: client %q: country %q unknown, use an ISO code such as DE, AT, US", c.Name, c.Country)
			}
		}
	}
	tc := travelexpense.Config{
		Mail:             cfg.Mail,
		Employee:         cfg.Staff.Current,
		Clients:          clients,
		ChristmasWeekOff: te.ChristmasWeekOff,
		InlandRates:      travelexpense.MealRates{Day: te.MealRates.Day, FullDay: te.MealRates.FullDay, TravelDay: te.MealRates.TravelDay},
		InlandLodging:    te.LodgingRate,
	}
	for _, b := range te.Blocked {
		from, to, err := parseRange(b.From, b.To)
		if err != nil {
			return tc, fmt.Errorf("travel-expense.blocked: %w", err)
		}
		tc.Blocked = append(tc.Blocked, travelexpense.BlockedRange{From: from, To: to, Note: b.Note})
	}
	for _, a := range te.Assignments {
		from, to, err := parseRange(a.From, a.To)
		if err != nil {
			return tc, fmt.Errorf("travel-expense.assignments: %w", err)
		}
		client := findClient(te.Clients, a.Client)
		if client < 0 {
			return tc, fmt.Errorf("travel-expense.assignments: no client %q", a.Client)
		}
		tc.Assignments = append(tc.Assignments, travelexpense.Assignment{
			Client: client, From: from, To: to, Overnight: a.Overnight, Hotel: a.Hotel, NightFlight: a.NightFlight, Note: a.Note,
		})
	}
	return tc, nil
}

// workdays says which days are workdays, as the Reisekosten have them: Monday to Friday without
// the public holidays at home, the blocked days and, when off, the Christmas week.
func workdays(cfg *Config) func(time.Time) bool {
	te := cfg.TravelExpense
	tc := travelexpense.Config{ChristmasWeekOff: te.ChristmasWeekOff}
	for _, c := range te.Clients {
		tc.Clients = append(tc.Clients, travelexpense.Client{Country: c.Country, Province: c.Province})
	}
	for _, b := range te.Blocked {
		if from, to, err := parseRange(b.From, b.To); err == nil {
			tc.Blocked = append(tc.Blocked, travelexpense.BlockedRange{From: from, To: to})
		}
	}
	return tc.Workdays()
}

// parseRange reads YYYY-MM-DD dates; an empty to is the from day.
func parseRange(from, to string) (time.Time, time.Time, error) {
	f, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return f, f, fmt.Errorf("date %q: want YYYY-MM-DD", from)
	}
	if to == "" {
		return f, f, nil
	}
	t, err := time.Parse(time.DateOnly, to)
	if err != nil {
		return f, t, fmt.Errorf("date %q: want YYYY-MM-DD", to)
	}
	if t.Before(f) {
		return f, t, fmt.Errorf("%s to %s: ends before it starts", from, to)
	}
	return f, t, nil
}

// findClient is the index of the client with this id, or else this name; -1 if none.
func findClient(clients []Client, key string) int {
	for i, c := range clients {
		if c.ID == key {
			return i
		}
	}
	for i, c := range clients {
		if strings.EqualFold(c.Name, key) {
			return i
		}
	}
	return -1
}

func anthropicConfig(cfg *Config) anthropic.Config {
	f := cfg.AnthropicInvoice.Filter
	return anthropic.Config{
		Mail:   cfg.Mail,
		Filter: anthropic.FilterConfig{Count: f.Count, Subject: f.Subject, From: f.From, Folders: f.Folders},
	}
}

func runAnthropicInvoice(cfg *Config) error { return anthropic.Run(anthropicConfig(cfg)) }

func appleConfig(cfg *Config) apple.Config {
	return apple.Config{
		Mail: cfg.Mail,
		Filter: apple.FilterConfig{
			Count:   cfg.AppleInvoice.Filter.Count,
			Subject: cfg.AppleInvoice.Filter.Subject,
			From:    cfg.AppleInvoice.Filter.From,
			Folders: cfg.AppleInvoice.Filter.Folders,
		},
	}
}

func runAppleInvoice(cfg *Config) error { return apple.Run(appleConfig(cfg)) }

func vodafoneConfig(cfg *Config) vodafone.Config {
	return vodafone.Config{
		Mail:                cfg.Mail,
		Username:            cfg.VodafoneInvoice.Username,
		Password:            cfg.VodafoneInvoice.Password,
		FallbackToLastMonth: *cfg.VodafoneInvoice.FallbackToLastMonth,
	}
}

func runVodafoneInvoice(cfg *Config) error { return vodafone.Run(vodafoneConfig(cfg)) }

func harvestConfig(cfg *Config) harvest.Config {
	return harvest.Config{
		SkipExisting: *cfg.HarvestInvoice.SkipExisting,
		Harvest: harvest.HarvestAPI{
			Token:     cfg.HarvestInvoice.Harvest.Token,
			AccountID: cfg.HarvestInvoice.Harvest.AccountID,
			Client:    cfg.HarvestInvoice.Harvest.Client,
			Person:    cfg.Staff.Current,
			Logo:      cfg.logoPath(),
			Company:   cfg.Company.Name,
		},
		SevDesk: harvest.SevDeskConfig{
			Username:     cfg.HarvestInvoice.SevDesk.Username,
			Password:     cfg.HarvestInvoice.SevDesk.Password,
			ClientName:   cfg.HarvestInvoice.SevDesk.ClientName,
			ProductName:  cfg.HarvestInvoice.SevDesk.ProductName,
			ProductNum:   cfg.HarvestInvoice.SevDesk.ProductNum,
			ReferenceNum: cfg.HarvestInvoice.SevDesk.ReferenceNum,
		},
		Workday: workdays(cfg),
	}
}

// runHarvestInvoice makes the sevDesk draft for the month's hours in Harvest, and their work
// log; with logOnly just the work log.
func runHarvestInvoice(cfg *Config, year int, month time.Month, logOnly bool) (*pdfdoc.Document, error) {
	hc := harvestConfig(cfg)
	hc.Month = time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	if logOnly {
		return harvest.WorkLogOnly(hc)
	}
	return harvest.Run(hc)
}

// harvestFromCommandLine is runHarvestInvoice with the work log saved in the current directory.
func harvestFromCommandLine(cfg *Config, year int, month time.Month) error {
	log, err := runHarvestInvoice(cfg, year, month, false)
	if log != nil {
		if werr := os.WriteFile(log.Filename, log.Data, 0o644); werr != nil {
			return errors.Join(err, werr)
		}
		fmt.Fprintf(logOut, "Work log saved as %s\n", log.Filename)
	}
	return err
}

// slashMonth reads a month as the command line takes it, "11/2026"; empty is the current one.
func slashMonth(s string) (int, time.Month, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		now := time.Now()
		return now.Year(), now.Month(), nil
	}
	if !monthArgRegex.MatchString(s) {
		return 0, 0, fmt.Errorf("month %q: want M/YYYY, e.g. 11/2026", s)
	}
	parts := strings.Split(s, "/")
	year, _ := strconv.Atoi(parts[1])
	m, _ := strconv.Atoi(parts[0])
	return year, time.Month(m), nil
}

func parseMonthArg(args []string) (int, time.Month) {
	for _, arg := range args {
		if monthArgRegex.MatchString(arg) {
			parts := strings.Split(arg, "/")
			year, _ := strconv.Atoi(parts[1])
			m, _ := strconv.Atoi(parts[0])
			return year, time.Month(m)
		}
	}
	year, month, _ := time.Now().Date()
	return year, month
}

func printUsage() {
	fmt.Print(`feed-my-accounting - accounting orchestrator

Usage:
  feed-my-accounting [--config path] [command] [args...]

Commands:
  all [M/YYYY]              Run all modules (default when no command given)
  travel-expense [M/YYYY]   Generate and send monthly travel expense PDFs
  anthropic-invoice         Fetch Anthropic receipt emails and send their PDFs
  apple-invoice             Fetch Apple invoice emails and send as PDFs
  vodafone-invoice          Download Vodafone invoices and send via email
  harvest-invoice [M/YYYY]  Create sevDesk draft invoice from the month's Harvest hours
  serve                     Web interface on web.listen (default :8595)

Flags:
  --config path             Path to config.yaml (default: auto-detect)
  --version, -v             Print version
`)
}
