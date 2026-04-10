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
//	apple-invoice-pdf         Fetch Apple invoice emails and send as PDFs
//	vodafone-downloader       Download Vodafone invoices and send via email
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	apple "feed-my-accounting/apple-invoice-pdf"
	harvest "feed-my-accounting/harvest-invoice"
	travelexpense "feed-my-accounting/travel-expense"
	vodafone "feed-my-accounting/vodafone-downloader"
)

const version = "1.3.0"

var monthArgRegex = regexp.MustCompile(`^(0?[1-9]|1[0-2])/(20[0-9]{2})$`)

func main() {
	args := os.Args[1:]

	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("feed-my-accounting v%s\n", version)
			return
		}
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
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "travel-expense error: %v\n", err)
			os.Exit(1)
		}
		if err := runAppleInvoicePDF(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "apple error: %v\n", err)
			os.Exit(1)
		}
		if err := runVodafoneDownloader(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "vodafone error: %v\n", err)
			os.Exit(1)
		}
		if err := runHarvestInvoice(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "harvest-invoice error: %v\n", err)
			os.Exit(1)
		}

	case "travel-expense":
		year, month := parseMonthArg(remaining)
		if err := runTravelExpense(cfg, year, month); err != nil {
			fmt.Fprintf(os.Stderr, "travel-expense error: %v\n", err)
			os.Exit(1)
		}

	case "apple-invoice-pdf":
		if err := runAppleInvoicePDF(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "apple error: %v\n", err)
			os.Exit(1)
		}

	case "vodafone-downloader":
		if err := runVodafoneDownloader(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "vodafone error: %v\n", err)
			os.Exit(1)
		}

	case "harvest-invoice":
		if err := runHarvestInvoice(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "harvest-invoice error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func runTravelExpense(cfg *Config, year int, month time.Month) error {
	customers := make([]travelexpense.Customer, len(cfg.TravelExpense.Customers))
	for i, c := range cfg.TravelExpense.Customers {
		customers[i] = travelexpense.Customer{
			ID: c.ID, Name: c.Name, From: c.From, To: c.To,
			Reason: c.Reason, Distance: c.Distance, Province: c.Province,
		}
	}
	return travelexpense.Run(travelexpense.Config{
		Mail:             cfg.Mail,
		Mitarbeiter:      cfg.TravelExpense.Mitarbeiter,
		Customers:        customers,
		ChristmasWeekOff: cfg.TravelExpense.ChristmasWeekOff,
	}, year, month)
}

func runAppleInvoicePDF(cfg *Config) error {
	return apple.Run(apple.Config{
		Mail: cfg.Mail,
		Filter: apple.FilterConfig{
			Count:   cfg.AppleInvoicePDF.Filter.Count,
			Subject: cfg.AppleInvoicePDF.Filter.Subject,
			From:    cfg.AppleInvoicePDF.Filter.From,
		},
	})
}

func runVodafoneDownloader(cfg *Config) error {
	return vodafone.Run(vodafone.Config{
		Mail:                cfg.Mail,
		User:                cfg.VodafoneDownloader.User,
		Pass:                cfg.VodafoneDownloader.Pass,
		FallbackToLastMonth: *cfg.VodafoneDownloader.FallbackToLastMonth,
	})
}

func runHarvestInvoice(cfg *Config) error {
	return harvest.Run(harvest.Config{
		Mail:             cfg.Mail,
		CurrentMonthOnly: *cfg.HarvestInvoice.CurrentMonthOnly,
		SkipExisting:     *cfg.HarvestInvoice.SkipExisting,
		Harvest: harvest.HarvestLogin{
			User: cfg.HarvestInvoice.Harvest.User,
			Pass: cfg.HarvestInvoice.Harvest.Pass,
		},
		SevDesk: harvest.SevDeskConfig{
			User:         cfg.HarvestInvoice.SevDesk.User,
			Pass:         cfg.HarvestInvoice.SevDesk.Pass,
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
  apple-invoice-pdf         Fetch Apple invoice emails and send as PDFs
  vodafone-downloader       Download Vodafone invoices and send via email
  harvest-invoice           Create sevDesk invoice from Harvest monthly report

Flags:
  --config path             Path to config.yaml (default: auto-detect)
  --version, -v             Print version
`)
}
