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
	Distance int    `yaml:"distance"`           // one-way distance in km
	Country  string `yaml:"country,omitempty"`  // ISO 3166 code, default DE: whose public holidays count
	Province string `yaml:"province,omitempty"` // German state abbreviation (e.g., "BW", "BY"), Germany only

	// Meal allowance in EUR; empty for the inland rates (mealRates).
	MealDay       float64 `yaml:"mealDay,omitempty"`       // day trip, more than 8 hours
	MealFullDay   float64 `yaml:"mealFullDay,omitempty"`   // a full day away, overnight trips
	MealTravelDay float64 `yaml:"mealTravelDay,omitempty"` // arrival or departure day
}

// DateRange is days from From to To (YYYY-MM-DD), inclusive; To empty for one day.
type DateRange struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to,omitempty" json:"to"`
	Note string `yaml:"note,omitempty" json:"note"`
}

// Assignment gives a date range to one client, by its id or name: as day trips, or overnight.
type Assignment struct {
	Client    string `yaml:"client" json:"client"`
	From      string `yaml:"from" json:"from"`
	To        string `yaml:"to,omitempty" json:"to"`
	Overnight bool   `yaml:"overnight,omitempty" json:"overnight"`
	Note      string `yaml:"note,omitempty" json:"note"`
}

// MealRates are the meal allowance rates in EUR; zero for the legal German ones.
type MealRates struct {
	Day       float64 `yaml:"day,omitempty"`       // day trip, more than 8 hours: 14
	FullDay   float64 `yaml:"fullDay,omitempty"`   // a full day away: 28
	TravelDay float64 `yaml:"travelDay,omitempty"` // arrival or departure day: 14
}

type TravelExpenseConfig struct {
	Employee         string       `yaml:"employee"`
	MealRates        MealRates    `yaml:"mealRates,omitempty"` // inland rates, for clients without their own
	Clients          []Client     `yaml:"clients"`
	ChristmasWeekOff *bool        `yaml:"christmasWeekOff,omitempty"` // default: true
	Blocked          []DateRange  `yaml:"blocked,omitempty"`          // vacation and other days off
	Assignments      []Assignment `yaml:"assignments,omitempty"`      // days given to a client explicitly
}

// ScheduleEntry runs a job on a cron schedule while the web interface runs: travel-expense,
// apple-invoice, vodafone-invoice or harvest-invoice.
type ScheduleEntry struct {
	Job     string `yaml:"job" json:"job"`
	Cron    string `yaml:"cron" json:"cron"` // minute hour day month weekday, as in crontab
	Enabled bool   `yaml:"enabled" json:"enabled"`
}

// WebConfig is the web interface's: where it listens, whether it answers the home network
// only, and an optional password asked for by the browser (HTTP basic auth, any user name).
type WebConfig struct {
	Listen   string `yaml:"listen,omitempty"`
	HomeOnly *bool  `yaml:"homeOnly,omitempty"` // default: true
	Password string `yaml:"password,omitempty"`
}

// homeOnly says whether only the home network may reach the web interface: unless switched off.
func (w WebConfig) homeOnly() bool { return w.HomeOnly == nil || *w.HomeOnly }

type AppleInvoiceConfig struct {
	Filter struct {
		Count   int    `yaml:"count"`
		From    string `yaml:"from"` // sender domain filter
		Subject string `yaml:"subject"`
	} `yaml:"filter"`
}

type VodafoneInvoiceConfig struct {
	Username            string `yaml:"username"`
	Password            string `yaml:"password"`
	FallbackToLastMonth *bool  `yaml:"fallbackToLastMonth,omitempty"` // default: true
}

type HarvestInvoiceConfig struct {
	CurrentMonthOnly *bool `yaml:"currentMonthOnly,omitempty"` // default: true
	SkipExisting     *bool `yaml:"skipExisting,omitempty"`     // default: true
	Filter           struct {
		Count   int    `yaml:"count"`
		From    string `yaml:"from"`
		Subject string `yaml:"subject"`
	} `yaml:"filter"`
	Harvest struct {
		Username  string `yaml:"username"`  // Harvest login email
		Password  string `yaml:"password"`  // Harvest login password
		Token     string `yaml:"token"`     // personal access token: the API instead of the login
		AccountID string `yaml:"accountId"` // empty: the token's Harvest account
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
	Mail            email.MailConfig      `yaml:"mail"`
	TravelExpense   TravelExpenseConfig   `yaml:"travel-expense"`
	AppleInvoice    AppleInvoiceConfig    `yaml:"apple-invoice"`
	VodafoneInvoice VodafoneInvoiceConfig `yaml:"vodafone-invoice"`
	HarvestInvoice  HarvestInvoiceConfig  `yaml:"harvest-invoice"`
	Web             WebConfig             `yaml:"web,omitempty"`
	Schedule        []ScheduleEntry       `yaml:"schedule,omitempty"`
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

	cfg, err := readConfig(path)
	if err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	return cfg, nil
}

// readConfig reads the configuration as written, without defaults: the web interface edits it.
func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return &cfg, nil
}

// saveConfig writes the configuration, readable only by its owner as it holds passwords. The
// file it replaces is kept beside it as config.yaml.bak, as the web interface rewrites it
// without its comments.
func saveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil {
		if err := os.WriteFile(path+".bak", old, 0o600); err != nil {
			return fmt.Errorf("backing up config: %w", err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// applyDefaults fills in what the configuration leaves out.
func applyDefaults(cfg *Config) {
	if cfg.Mail.From == "" {
		cfg.Mail.From = cfg.Mail.Username
	}
	if cfg.AppleInvoice.Filter.Count == 0 {
		cfg.AppleInvoice.Filter.Count = 10
	}
	if cfg.AppleInvoice.Filter.From == "" {
		cfg.AppleInvoice.Filter.From = "apple.com"
	}
	if cfg.AppleInvoice.Filter.Subject == "" {
		cfg.AppleInvoice.Filter.Subject = "Deine Rechnung von Apple"
	}
	if cfg.VodafoneInvoice.FallbackToLastMonth == nil {
		t := true
		cfg.VodafoneInvoice.FallbackToLastMonth = &t
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
	if cfg.HarvestInvoice.Filter.From == "" {
		cfg.HarvestInvoice.Filter.From = "harvestapp.com"
	}
	if cfg.HarvestInvoice.Filter.Subject == "" {
		cfg.HarvestInvoice.Filter.Subject = "We\u2019ve exported your detailed time report"
	}
	if cfg.Web.Listen == "" {
		cfg.Web.Listen = ":8595"
	}
}
