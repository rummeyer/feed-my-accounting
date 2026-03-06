package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
smtp:
  host: smtp.example.com
  port: 587
  user: user@example.com
  pass: secret

imap:
  host: imap.example.com
  port: 993

email:
  from: user@example.com
  to: boss@example.com
  cc: cc@example.com

travel-expense:
  mitarbeiter: Max Mustermann
  customers:
    - id: "1"
      name: Acme GmbH
      from: Stuttgart
      to: München
      reason: Projektarbeit
      distance: 42
      province: BW

apple-invoice-pdf:
  filter:
    count: 50
    subject: "Deine Rechnung von Apple"
    from: "apple.com"

vodafone-downloader:
  user: vodafone@example.com
  pass: vodapass
`), 0644)

	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.SMTP.Host != "smtp.example.com" {
		t.Errorf("SMTP.Host = %q, want smtp.example.com", cfg.SMTP.Host)
	}
	if cfg.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want 587", cfg.SMTP.Port)
	}
	if cfg.IMAP.Host != "imap.example.com" {
		t.Errorf("IMAP.Host = %q, want imap.example.com", cfg.IMAP.Host)
	}
	if cfg.Email.From != "user@example.com" {
		t.Errorf("Email.From = %q, want user@example.com", cfg.Email.From)
	}
	if cfg.Email.CC != "cc@example.com" {
		t.Errorf("Email.CC = %q, want cc@example.com", cfg.Email.CC)
	}
	if cfg.TravelExpense.Mitarbeiter != "Max Mustermann" {
		t.Errorf("TravelExpense.Mitarbeiter = %q, want Max Mustermann", cfg.TravelExpense.Mitarbeiter)
	}
	if len(cfg.TravelExpense.Customers) != 1 {
		t.Fatalf("TravelExpense.Customers len = %d, want 1", len(cfg.TravelExpense.Customers))
	}
	if cfg.TravelExpense.Customers[0].Distance != 42 {
		t.Errorf("Customer.Distance = %d, want 42", cfg.TravelExpense.Customers[0].Distance)
	}
	if cfg.AppleInvoicePDF.Filter.Count != 50 {
		t.Errorf("AppleInvoicePDF.Filter.Count = %d, want 50", cfg.AppleInvoicePDF.Filter.Count)
	}
	if cfg.VodafoneDownloader.User != "vodafone@example.com" {
		t.Errorf("VodafoneDownloader.User = %q, want vodafone@example.com", cfg.VodafoneDownloader.User)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := loadConfig("config.yaml", "/nonexistent/config.yaml")
	if err == nil {
		t.Error("loadConfig() expected error for missing file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)
	_, err := loadConfig("config.yaml", path)
	if err == nil {
		t.Error("loadConfig() expected error for invalid YAML")
	}
}

func TestLoadConfig_DefaultEmailFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
smtp:
  user: smtp@example.com
email:
  to: recipient@example.com
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Email.From != "smtp@example.com" {
		t.Errorf("Email.From default = %q, want smtp@example.com", cfg.Email.From)
	}
}

func TestLoadConfig_DefaultAppleFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`smtp:
  host: smtp.example.com
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.AppleInvoicePDF.Filter.Subject != "Deine Rechnung von Apple" {
		t.Errorf("AppleInvoicePDF.Filter.Subject default = %q", cfg.AppleInvoicePDF.Filter.Subject)
	}
	if cfg.AppleInvoicePDF.Filter.From != "apple.com" {
		t.Errorf("AppleInvoicePDF.Filter.From default = %q", cfg.AppleInvoicePDF.Filter.From)
	}
}

func TestLoadConfig_ExplicitPathTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myconfig.yaml")
	os.WriteFile(path, []byte(`
smtp:
  host: explicit.example.com
  port: 465
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.SMTP.Host != "explicit.example.com" {
		t.Errorf("SMTP.Host = %q, want explicit.example.com", cfg.SMTP.Host)
	}
}

func TestLoadConfig_ChristmasWeekOffDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
travel-expense:
  mitarbeiter: Test
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.TravelExpense.ChristmasWeekOff != nil {
		t.Errorf("ChristmasWeekOff should be nil (omitted) by default, got %v", *cfg.TravelExpense.ChristmasWeekOff)
	}
}
