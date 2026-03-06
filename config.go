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

// Customer represents a client with trip details used by the reisekosten module.
type Customer struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
	Reason   string `yaml:"reason"`
	Distance int    `yaml:"distance"` // one-way distance in km
	Province string `yaml:"province"` // German state abbreviation (e.g., "BW", "BY")
}

type TravelExpenseConfig struct {
	Mitarbeiter      string     `yaml:"mitarbeiter"`
	Customers        []Customer `yaml:"customers"`
	ChristmasWeekOff *bool      `yaml:"christmasWeekOff,omitempty"` // default: true
}

type AppleInvoicePDFConfig struct {
	User   string `yaml:"user"` // optional IMAP auth override, falls back to smtp.user
	Pass   string `yaml:"pass"` // optional IMAP auth override, falls back to smtp.pass
	Filter struct {
		Count   int    `yaml:"count"`
		Subject string `yaml:"subject"`
		From    string `yaml:"from"` // sender domain filter
	} `yaml:"filter"`
}

type VodafoneDownloaderConfig struct {
	User                string `yaml:"user"`
	Pass                string `yaml:"pass"`
	FallbackToLastMonth *bool  `yaml:"fallbackToLastMonth,omitempty"` // default: true
}

// Config is the unified YAML configuration for all modules.
// smtp, email, and imap are shared; each module has its own section.
type Config struct {
	SMTP               email.SMTPConfig         `yaml:"smtp"`
	Email              email.EmailConfig         `yaml:"email"`
	IMAP               email.IMAPConfig          `yaml:"imap"` // used by apple-invoice-pdf
	TravelExpense      TravelExpenseConfig       `yaml:"travel-expense"`
	AppleInvoicePDF    AppleInvoicePDFConfig     `yaml:"apple-invoice-pdf"`
	VodafoneDownloader VodafoneDownloaderConfig  `yaml:"vodafone-downloader"`
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
	if cfg.Email.From == "" {
		cfg.Email.From = cfg.SMTP.User
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

	return &cfg, nil
}
