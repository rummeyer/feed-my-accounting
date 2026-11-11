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
//	apple-invoice             Fetch Apple invoice emails and send as PDFs
//	vodafone-invoice          Download Vodafone invoices and send via email
//	harvest-invoice           Create sevDesk invoice from Harvest monthly report
//	serve                     Web interface on web.listen (default :8595)
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	apple "feed-my-accounting/apple-invoice"
	"feed-my-accounting/email"
	harvest "feed-my-accounting/harvest-invoice"
	travelexpense "feed-my-accounting/travel-expense"
	vodafone "feed-my-accounting/vodafone-invoice"
)

const version = "1.9.0"

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
		if err := runAppleInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "apple-invoice error: %v\n", err)
			os.Exit(1)
		}
		if err := runVodafoneInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "vodafone-invoice error: %v\n", err)
			os.Exit(1)
		}
		if err := runHarvestInvoice(cfg); err != nil {
			fmt.Fprintf(logOut, "harvest-invoice error: %v\n", err)
			os.Exit(1)
		}

	case "travel-expense":
		year, month := parseMonthArg(remaining)
		if err := runTravelExpense(cfg, year, month); err != nil {
			fmt.Fprintf(logOut, "travel-expense error: %v\n", err)
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
		if err := runHarvestInvoice(cfg); err != nil {
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
	clients := make([]travelexpense.Client, len(te.Clients))
	for i, c := range te.Clients {
		clients[i] = travelexpense.Client{
			ID: c.ID, Name: c.Name, From: c.From, To: c.To,
			Reason: c.Reason, Distance: c.Distance, Country: c.Country, Province: c.Province,
			MealDay: c.MealDay, MealFullDay: c.MealFullDay, MealTravelDay: c.MealTravelDay,
			OnlyAssigned: c.OnlyAssigned,
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
		Employee:         te.Employee,
		Clients:          clients,
		ChristmasWeekOff: te.ChristmasWeekOff,
		InlandRates:      travelexpense.MealRates{Day: te.MealRates.Day, FullDay: te.MealRates.FullDay, TravelDay: te.MealRates.TravelDay},
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
			Client: client, From: from, To: to, Overnight: a.Overnight, Note: a.Note,
		})
	}
	return tc, nil
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

func runAppleInvoice(cfg *Config) error {
	return apple.Run(apple.Config{
		Mail: cfg.Mail,
		Filter: apple.FilterConfig{
			Count:   cfg.AppleInvoice.Filter.Count,
			Subject: cfg.AppleInvoice.Filter.Subject,
			From:    cfg.AppleInvoice.Filter.From,
		},
	})
}

func runVodafoneInvoice(cfg *Config) error {
	return vodafone.Run(vodafone.Config{
		Mail:                cfg.Mail,
		Username:            cfg.VodafoneInvoice.Username,
		Password:            cfg.VodafoneInvoice.Password,
		FallbackToLastMonth: *cfg.VodafoneInvoice.FallbackToLastMonth,
	})
}

func runHarvestInvoice(cfg *Config) error {
	return harvest.Run(harvestConfig(cfg))
}

func harvestConfig(cfg *Config) harvest.Config {
	return harvest.Config{
		Mail:             cfg.Mail,
		CurrentMonthOnly: *cfg.HarvestInvoice.CurrentMonthOnly,
		SkipExisting:     *cfg.HarvestInvoice.SkipExisting,
		Harvest: harvest.HarvestLogin{
			Username: cfg.HarvestInvoice.Harvest.Username,
			Password: cfg.HarvestInvoice.Harvest.Password,
		},
		SevDesk: harvest.SevDeskConfig{
			Username:     cfg.HarvestInvoice.SevDesk.Username,
			Password:     cfg.HarvestInvoice.SevDesk.Password,
			ClientName:   cfg.HarvestInvoice.SevDesk.ClientName,
			ProductName:  cfg.HarvestInvoice.SevDesk.ProductName,
			ProductNum:   cfg.HarvestInvoice.SevDesk.ProductNum,
			ReferenceNum: cfg.HarvestInvoice.SevDesk.ReferenceNum,
		},
		Filter: harvest.FilterConfig{
			Count:   cfg.HarvestInvoice.Filter.Count,
			Subject: cfg.HarvestInvoice.Filter.Subject,
			From:    cfg.HarvestInvoice.Filter.From,
		},
	}
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
  apple-invoice             Fetch Apple invoice emails and send as PDFs
  vodafone-invoice          Download Vodafone invoices and send via email
  harvest-invoice           Create sevDesk invoice from Harvest monthly report
  serve                     Web interface on web.listen (default :8595)

Flags:
  --config path             Path to config.yaml (default: auto-detect)
  --version, -v             Print version
`)
}
