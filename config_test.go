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
mail:
  smtpHost: smtp.example.com
  smtpPort: 587
  imapHost: imap.example.com
  imapPort: 993
  username: user@example.com
  password: secret
  from: user@example.com
  to: boss@example.com
  cc: cc@example.com

travel-expense:
  employee: Max Mustermann
  clients:
    - id: "1"
      name: Acme GmbH
      from: Stuttgart
      to: München
      reason: Projektarbeit
      distance: 42
      province: BW

apple-invoice:
  filter:
    count: 50
    subject: "Deine Rechnung von Apple"
    from: "apple.com"

vodafone-invoice:
  username: vodafone@example.com
  password: vodapass
`), 0644)

	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Mail.SMTPHost != "smtp.example.com" {
		t.Errorf("Mail.SMTPHost = %q, want smtp.example.com", cfg.Mail.SMTPHost)
	}
	if cfg.Mail.SMTPPort != 587 {
		t.Errorf("Mail.SMTPPort = %d, want 587", cfg.Mail.SMTPPort)
	}
	if cfg.Mail.IMAPHost != "imap.example.com" {
		t.Errorf("Mail.IMAPHost = %q, want imap.example.com", cfg.Mail.IMAPHost)
	}
	if cfg.Mail.IMAPPort != 993 {
		t.Errorf("Mail.IMAPPort = %d, want 993", cfg.Mail.IMAPPort)
	}
	if cfg.Mail.From != "user@example.com" {
		t.Errorf("Mail.From = %q, want user@example.com", cfg.Mail.From)
	}
	if cfg.Mail.CC != "cc@example.com" {
		t.Errorf("Mail.CC = %q, want cc@example.com", cfg.Mail.CC)
	}
	if cfg.TravelExpense.Employee != "Max Mustermann" {
		t.Errorf("TravelExpense.Employee = %q, want Max Mustermann", cfg.TravelExpense.Employee)
	}
	if len(cfg.TravelExpense.Clients) != 1 {
		t.Fatalf("TravelExpense.Clients len = %d, want 1", len(cfg.TravelExpense.Clients))
	}
	if cfg.TravelExpense.Clients[0].Distance != 42 {
		t.Errorf("Client.Distance = %d, want 42", cfg.TravelExpense.Clients[0].Distance)
	}
	if cfg.AppleInvoice.Filter.Count != 50 {
		t.Errorf("AppleInvoice.Filter.Count = %d, want 50", cfg.AppleInvoice.Filter.Count)
	}
	if cfg.VodafoneInvoice.Username != "vodafone@example.com" {
		t.Errorf("VodafoneInvoice.Username = %q, want vodafone@example.com", cfg.VodafoneInvoice.Username)
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
mail:
  username: smtp@example.com
  to: recipient@example.com
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Mail.From != "smtp@example.com" {
		t.Errorf("Mail.From default = %q, want smtp@example.com", cfg.Mail.From)
	}
}

func TestLoadConfig_DefaultAppleFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`mail:
  smtpHost: smtp.example.com
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.AppleInvoice.Filter.Subject != "Deine Rechnung von Apple" {
		t.Errorf("AppleInvoice.Filter.Subject default = %q", cfg.AppleInvoice.Filter.Subject)
	}
	if cfg.AppleInvoice.Filter.From != "apple.com" {
		t.Errorf("AppleInvoice.Filter.From default = %q", cfg.AppleInvoice.Filter.From)
	}
}

func TestLoadConfig_ExplicitPathTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myconfig.yaml")
	os.WriteFile(path, []byte(`
mail:
  smtpHost: explicit.example.com
  smtpPort: 465
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Mail.SMTPHost != "explicit.example.com" {
		t.Errorf("Mail.SMTPHost = %q, want explicit.example.com", cfg.Mail.SMTPHost)
	}
}

func TestLoadConfig_ChristmasWeekOffDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
travel-expense:
  employee: Test
`), 0644)
	cfg, err := loadConfig("config.yaml", path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.TravelExpense.ChristmasWeekOff != nil {
		t.Errorf("ChristmasWeekOff should be nil (omitted) by default, got %v", *cfg.TravelExpense.ChristmasWeekOff)
	}
}
