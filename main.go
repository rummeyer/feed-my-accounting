// Package main is the feed-my-accounting orchestrator.
// It combines travel expense reports, Apple invoice processing,
// and Vodafone invoice downloading into a single tool.
//
// Usage:
//
//	feed-my-accounting [--config path] <command> [args...]
//
// Commands:
//
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
	travelexpense "feed-my-accounting/travel-expense"
	vodafone "feed-my-accounting/vodafone-downloader"
)

const version = "1.0.0"

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

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := loadConfig("config.yaml", configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	subcommand := args[0]
	remaining := args[1:]

	switch subcommand {
	case "travel-expense":
		year, month := parseMonthArg(remaining)
		customers := make([]travelexpense.Customer, len(cfg.TravelExpense.Customers))
		for i, c := range cfg.TravelExpense.Customers {
			customers[i] = travelexpense.Customer{
				ID: c.ID, Name: c.Name, From: c.From, To: c.To,
				Reason: c.Reason, Distance: c.Distance, Province: c.Province,
			}
		}
		if err := travelexpense.Run(travelexpense.Config{
			SMTP:             cfg.SMTP,
			Email:            cfg.Email,
			Mitarbeiter:      cfg.TravelExpense.Mitarbeiter,
			Customers:        customers,
			ChristmasWeekOff: cfg.TravelExpense.ChristmasWeekOff,
		}, year, month); err != nil {
			fmt.Fprintf(os.Stderr, "travel-expense error: %v\n", err)
			os.Exit(1)
		}

	case "apple-invoice-pdf":
		if err := apple.Run(apple.Config{
			SMTP:  cfg.SMTP,
			Email: cfg.Email,
			IMAP:  cfg.IMAP,
			User:  cfg.AppleInvoicePDF.User,
			Pass:  cfg.AppleInvoicePDF.Pass,
			Filter: apple.FilterConfig{
				Count:   cfg.AppleInvoicePDF.Filter.Count,
				Subject: cfg.AppleInvoicePDF.Filter.Subject,
				From:    cfg.AppleInvoicePDF.Filter.From,
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "apple error: %v\n", err)
			os.Exit(1)
		}

	case "vodafone-downloader":
		if err := vodafone.Run(vodafone.Config{
			SMTP:                cfg.SMTP,
			Email:               cfg.Email,
			User:                cfg.VodafoneDownloader.User,
			Pass:                cfg.VodafoneDownloader.Pass,
			FallbackToLastMonth: *cfg.VodafoneDownloader.FallbackToLastMonth,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "vodafone error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", subcommand)
		printUsage()
		os.Exit(1)
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
  feed-my-accounting [--config path] <command> [args...]

Commands:
  travel-expense [M/YYYY]   Generate and send monthly travel expense PDFs
  apple-invoice-pdf         Fetch Apple invoice emails and send as PDFs
  vodafone-downloader       Download Vodafone invoices and send via email

Flags:
  --config path             Path to config.yaml (default: auto-detect)
  --version, -v             Print version
`)
}
