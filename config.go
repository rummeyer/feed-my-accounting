package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	// Flat rate for a night without a hotel invoice; empty for the inland one (lodgingRate).
	Lodging float64 `yaml:"lodging,omitempty"`
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
	Hotel     bool   `yaml:"hotel,omitempty" json:"hotel"` // the nights have a hotel invoice: no flat rate
	// The flight home goes through the night before the day of arrival: no flat rate for it.
	NightFlight bool   `yaml:"nightFlight,omitempty" json:"nightFlight"`
	Note        string `yaml:"note,omitempty" json:"note"`
}

// MealRates are the meal allowance rates in EUR; zero for the legal German ones.
type MealRates struct {
	Day       float64 `yaml:"day,omitempty"`       // day trip, more than 8 hours: 14
	FullDay   float64 `yaml:"fullDay,omitempty"`   // a full day away: 28
	TravelDay float64 `yaml:"travelDay,omitempty"` // arrival or departure day: 14
}

type TravelExpenseConfig struct {
	Employee         string       `yaml:"employee,omitempty"`    // until 1.10: now staff.current, moved there on reading
	MealRates        MealRates    `yaml:"mealRates,omitempty"`   // inland rates, for clients without their own
	LodgingRate      float64      `yaml:"lodgingRate,omitempty"` // inland flat rate for a night; empty: the legal 20
	Clients          []Client     `yaml:"clients"`
	ChristmasWeekOff *bool        `yaml:"christmasWeekOff,omitempty"` // default: true
	Blocked          []DateRange  `yaml:"blocked,omitempty"`          // vacation and other days off
	Assignments      []Assignment `yaml:"assignments,omitempty"`      // days given to a client explicitly
}

// ScheduleEntry runs a job on a cron schedule while the web interface runs: travel-expense,
// anthropic-invoice, apple-invoice,
// vodafone-invoice or harvest-invoice.
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
	// Password is the page's one password of before 1.13, asked by HTTP basic auth: on starting,
	// it becomes the selected employee's login.
	Password string `yaml:"password,omitempty"`
}

// homeOnly says whether only the home network may reach the web interface: unless switched off.
func (w WebConfig) homeOnly() bool { return w.HomeOnly == nil || *w.HomeOnly }

// MailFilterConfig finds a module's emails: in the inbox first, then where they may have been
// filed away.
type MailFilterConfig struct {
	Count   int      `yaml:"count"`   // how many recent emails to scan, in each folder
	From    string   `yaml:"from"`    // sender domains, comma-separated
	Subject string   `yaml:"subject"` // Anthropic: the subject's start; Apple: the whole subject
	Folders []string `yaml:"folders"` // searched in this order
}

type AnthropicInvoiceConfig struct {
	Filter MailFilterConfig `yaml:"filter"`
}

type AppleInvoiceConfig struct {
	Filter MailFilterConfig `yaml:"filter"`
}

type VodafoneInvoiceConfig struct {
	Username            string `yaml:"username"`
	Password            string `yaml:"password"`
	FallbackToLastMonth *bool  `yaml:"fallbackToLastMonth,omitempty"` // default: true
}

type HarvestInvoiceConfig struct {
	SkipExisting *bool `yaml:"skipExisting,omitempty"` // default: true
	Harvest      struct {
		Token     string `yaml:"token"`     // personal access token for the Harvest API v2
		AccountID string `yaml:"accountId"` // empty: the token's Harvest account
		Client    string `yaml:"client"`    // client in Harvest; empty: sevdesk.clientName
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
// CompanyConfig is the company the tool works for, as its documents name it.
type CompanyConfig struct {
	Name string `yaml:"name,omitempty"` // empty: the work log takes Harvest's account name
}

// StaffConfig is who works with the tool: the employees, and the one selected — the Lieferant on
// the travel expenses, and whose hours in Harvest make the work log and the invoice.
type StaffConfig struct {
	Names   []string `yaml:"names"`
	Current string   `yaml:"current"`
	// Logins are the employees' passwords for the web interface, by name: bcrypt hashes. One is
	// enough for the web interface to ask for a login.
	Logins map[string]string `yaml:"logins,omitempty"`
	// Passwords are new ones, from the settings page, by name: hashed into Logins on saving,
	// never kept as they are. NoLogin are the names whose password goes.
	Passwords map[string]string `yaml:"-"`
	NoLogin   []string          `yaml:"-"`
}

type Config struct {
	Company          CompanyConfig          `yaml:"company,omitempty"`
	Staff            StaffConfig            `yaml:"staff"`
	Mail             email.MailConfig       `yaml:"mail"`
	TravelExpense    TravelExpenseConfig    `yaml:"travel-expense"`
	AnthropicInvoice AnthropicInvoiceConfig `yaml:"anthropic-invoice"`
	AppleInvoice     AppleInvoiceConfig     `yaml:"apple-invoice"`
	VodafoneInvoice  VodafoneInvoiceConfig  `yaml:"vodafone-invoice"`
	HarvestInvoice   HarvestInvoiceConfig   `yaml:"harvest-invoice"`
	Web              WebConfig              `yaml:"web,omitempty"`
	Schedule         []ScheduleEntry        `yaml:"schedule,omitempty"`

	dir string // the folder the configuration is in, where the logo is kept beside it
}

// logoBase is the logo's file name beside the configuration, without its extension.
const logoBase = "feed-my-accounting-logo"

// logoTypes are the logo's extensions, as the PDFs can show them.
var logoTypes = []string{".png", ".jpg"}

// logoPath is the logo kept beside the configuration; empty without one.
func (c *Config) logoPath() string {
	for _, ext := range logoTypes {
		if path := filepath.Join(c.dir, logoBase+ext); fileExists(path) {
			return path
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
	normalizeStaff(&cfg)
	cfg.dir = filepath.Dir(path)
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
// normalizeStaff is the employees as read, on the web interface as well: the one name of before
// 1.10 among them, empty rows and names given twice left out, and one selected if there are any.
func normalizeStaff(cfg *Config) {
	// The one name of before 1.10, the travel expenses' Lieferant, is the first employee.
	if old := strings.TrimSpace(cfg.TravelExpense.Employee); old != "" {
		if !slices.Contains(cfg.Staff.Names, old) {
			cfg.Staff.Names = append([]string{old}, cfg.Staff.Names...)
		}
		if cfg.Staff.Current == "" {
			cfg.Staff.Current = old
		}
		cfg.TravelExpense.Employee = ""
	}
	// Rows added on the web interface and left empty, and names given twice, are not employees.
	var names []string
	for _, n := range cfg.Staff.Names {
		if n = strings.TrimSpace(n); n != "" && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	cfg.Staff.Names, cfg.Staff.Current = names, strings.TrimSpace(cfg.Staff.Current)
	// An employee gone takes their login along.
	for name := range cfg.Staff.Logins {
		if !slices.Contains(names, name) {
			delete(cfg.Staff.Logins, name)
		}
	}
	if !slices.Contains(cfg.Staff.Names, cfg.Staff.Current) {
		cfg.Staff.Current = ""
		if len(cfg.Staff.Names) > 0 {
			cfg.Staff.Current = cfg.Staff.Names[0]
		}
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Mail.From == "" {
		cfg.Mail.From = cfg.Mail.Username
	}
	if cfg.AnthropicInvoice.Filter.Count == 0 {
		cfg.AnthropicInvoice.Filter.Count = 50
	}
	if cfg.AnthropicInvoice.Filter.From == "" {
		cfg.AnthropicInvoice.Filter.From = "mail.anthropic.com, stripe.com"
	}
	if cfg.AnthropicInvoice.Filter.Subject == "" {
		cfg.AnthropicInvoice.Filter.Subject = "Your receipt from Anthropic"
	}
	if len(cfg.AnthropicInvoice.Filter.Folders) == 0 {
		cfg.AnthropicInvoice.Filter.Folders = []string{"INBOX", "Rechnungen"}
	}
	if cfg.AppleInvoice.Filter.Count == 0 {
		cfg.AppleInvoice.Filter.Count = 50
	}
	if cfg.AppleInvoice.Filter.From == "" {
		cfg.AppleInvoice.Filter.From = "apple.com"
	}
	if cfg.AppleInvoice.Filter.Subject == "" {
		cfg.AppleInvoice.Filter.Subject = "Deine Rechnung von Apple"
	}
	if len(cfg.AppleInvoice.Filter.Folders) == 0 {
		cfg.AppleInvoice.Filter.Folders = []string{"INBOX", "Rechnungen"}
	}
	if cfg.VodafoneInvoice.FallbackToLastMonth == nil {
		t := true
		cfg.VodafoneInvoice.FallbackToLastMonth = &t
	}
	if cfg.HarvestInvoice.SkipExisting == nil {
		t := true
		cfg.HarvestInvoice.SkipExisting = &t
	}
	if cfg.Web.Listen == "" {
		cfg.Web.Listen = ":8595"
	}
}
