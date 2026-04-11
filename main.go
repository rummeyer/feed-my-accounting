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

const version = "1.5.0"

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
	apple.SetLogOutput(logOut)
	vodafone.SetLogOutput(logOut)
	harvest.SetLogOutput(logOut)
	travelexpense.SetLogOutput(logOut)
	email.SetLogOutput(logOut)
	log.SetOutput(logOut)
	return f, nil
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

	default:
		fmt.Fprintf(logOut, "Unknown command: %q\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func runTravelExpense(cfg *Config, year int, month time.Month) error {
	clients := make([]travelexpense.Client, len(cfg.TravelExpense.Clients))
	for i, c := range cfg.TravelExpense.Clients {
		clients[i] = travelexpense.Client{
			ID: c.ID, Name: c.Name, From: c.From, To: c.To,
			Reason: c.Reason, Distance: c.Distance, Province: c.Province,
		}
	}
	return travelexpense.Run(travelexpense.Config{
		Mail:             cfg.Mail,
		Employee:         cfg.TravelExpense.Employee,
		Clients:          clients,
		ChristmasWeekOff: cfg.TravelExpense.ChristmasWeekOff,
	}, year, month)
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
	return harvest.Run(harvest.Config{
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
	})
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

Flags:
  --config path             Path to config.yaml (default: auto-detect)
  --version, -v             Print version
`)
}
