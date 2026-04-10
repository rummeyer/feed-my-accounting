package main

import (
	"fmt"
	"os"
	"path/filepath"

	"feed-my-accounting/email"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Config structs (YAML-facing)
// ---------------------------------------------------------------------------

// Client represents a client with trip details used by the travel-expense module.
type Client struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
	Reason   string `yaml:"reason"`
	Distance int    `yaml:"distance"` // one-way distance in km
	Province string `yaml:"province"` // German state abbreviation (e.g., "BW", "BY")
}

type TravelExpenseConfig struct {
	Employee         string   `yaml:"employee"`
	Clients          []Client `yaml:"clients"`
	ChristmasWeekOff *bool    `yaml:"christmasWeekOff,omitempty"` // default: true
}

type AppleInvoicePDFConfig struct {
	Filter struct {
		Count   int    `yaml:"count"`
		Subject string `yaml:"subject"`
		From    string `yaml:"from"` // sender domain filter
	} `yaml:"filter"`
}

type VodafoneDownloaderConfig struct {
	Username            string `yaml:"username"`
	Password            string `yaml:"password"`
	FallbackToLastMonth *bool  `yaml:"fallbackToLastMonth,omitempty"` // default: true
}

type HarvestInvoiceConfig struct {
	CurrentMonthOnly *bool `yaml:"currentMonthOnly,omitempty"` // default: true
	SkipExisting     *bool `yaml:"skipExisting,omitempty"`     // default: true
	Filter           struct {
		Count   int    `yaml:"count"`
		Subject string `yaml:"subject"`
		From    string `yaml:"from"`
	} `yaml:"filter"`
	Harvest struct {
		Username string `yaml:"username"` // Harvest login email
		Password string `yaml:"password"` // Harvest login password
	} `yaml:"harvest"`
	SevDesk struct {
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
		ClientName   string `yaml:"clientName"`   // fixed client name (overrides name from email)
		ProductName  string `yaml:"productName"`  // search term for product (e.g. "Acme Produkt")
		ProductNum   string `yaml:"productNum"`   // article number to select from dropdown (e.g. "0102")
		ReferenceNum string `yaml:"referenceNum"` // Kundenreferenz für E-Rechnung
	} `yaml:"sevdesk"`
}

// Config is the unified YAML configuration for all modules.
// The "mail" block holds shared SMTP/IMAP credentials and addresses;
// each module has its own section for module-specific settings.
type Config struct {
	Mail               email.MailConfig         `yaml:"mail"`
	TravelExpense      TravelExpenseConfig      `yaml:"travel-expense"`
	AppleInvoicePDF    AppleInvoicePDFConfig    `yaml:"apple-invoice-pdf"`
	VodafoneDownloader VodafoneDownloaderConfig `yaml:"vodafone-downloader"`
	HarvestInvoice     HarvestInvoiceConfig     `yaml:"harvest-invoice"`
}

// ---------------------------------------------------------------------------
// Config loading
// ---------------------------------------------------------------------------

// findConfigFile searches the current directory, then the executable directory.
func findConfigFile(filename string) (string, error) {
	if _, err := os.Stat(filename); err == nil {
		return filename, nil
	}
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		exeConfigPath := filepath.Join(exeDir, filename)
		if _, err := os.Stat(exeConfigPath); err == nil {
			return exeConfigPath, nil
		}
	}
	return "", fmt.Errorf("config file %q not found in current directory or executable directory", filename)
}

// loadConfig reads and parses the YAML configuration file.
// If configPath is non-empty, it is used directly; otherwise the file is auto-located.
func loadConfig(filename, configPath string) (*Config, error) {
	var path string
	var err error

	if configPath != "" {
		path = configPath
	} else {
		path, err = findConfigFile(filename)
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	if cfg.Mail.From == "" {
		cfg.Mail.From = cfg.Mail.Username
	}
	if cfg.AppleInvoicePDF.Filter.Count == 0 {
		cfg.AppleInvoicePDF.Filter.Count = 10
	}
	if cfg.AppleInvoicePDF.Filter.Subject == "" {
		cfg.AppleInvoicePDF.Filter.Subject = "Deine Rechnung von Apple"
	}
	if cfg.AppleInvoicePDF.Filter.From == "" {
		cfg.AppleInvoicePDF.Filter.From = "apple.com"
	}
	if cfg.VodafoneDownloader.FallbackToLastMonth == nil {
		t := true
		cfg.VodafoneDownloader.FallbackToLastMonth = &t
	}
	if cfg.HarvestInvoice.CurrentMonthOnly == nil {
		t := true
		cfg.HarvestInvoice.CurrentMonthOnly = &t
	}
	if cfg.HarvestInvoice.SkipExisting == nil {
		t := true
		cfg.HarvestInvoice.SkipExisting = &t
	}
	if cfg.HarvestInvoice.Filter.Count == 0 {
		cfg.HarvestInvoice.Filter.Count = 10
	}
	if cfg.HarvestInvoice.Filter.Subject == "" {
		cfg.HarvestInvoice.Filter.Subject = "We've exported your detailed time report"
	}
	if cfg.HarvestInvoice.Filter.From == "" {
		cfg.HarvestInvoice.Filter.From = "harvestapp.com"
	}

	return &cfg, nil
}
