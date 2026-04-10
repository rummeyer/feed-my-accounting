package harvest

import (
	"testing"
	"time"
)

func TestParseEmail_FullExport(t *testing.T) {
	html := `<html><body>
		<p>Your detailed time report from 01.03.2026 to 31.03.2026 is ready.</p>
		<ul>
			<li>to Acme GmbH</li>
			<li>to the project Website Redesign</li>
		</ul>
		<a href="https://example.harvestapp.com/exports/12345">Download</a>
	</body></html>`

	data, err := ParseEmail(html)
	if err != nil {
		t.Fatalf("ParseEmail() error = %v", err)
	}
	if data.ExportURL != "https://example.harvestapp.com/exports/12345" {
		t.Errorf("ExportURL = %q", data.ExportURL)
	}
	if data.PeriodFrom != time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("PeriodFrom = %v", data.PeriodFrom)
	}
	if data.PeriodTo != time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC) {
		t.Errorf("PeriodTo = %v", data.PeriodTo)
	}
	if data.ClientName != "Acme GmbH" {
		t.Errorf("ClientName = %q, want Acme GmbH", data.ClientName)
	}
}

func TestParseEmail_SchemaOrgFallback(t *testing.T) {
	html := `<html><body>
		<p>Report from 01.04.2026 to 30.04.2026</p>
		<link itemprop="url" href="https://example.harvestapp.com/exports/99999" />
	</body></html>`

	data, err := ParseEmail(html)
	if err != nil {
		t.Fatalf("ParseEmail() error = %v", err)
	}
	if data.ExportURL != "https://example.harvestapp.com/exports/99999" {
		t.Errorf("ExportURL = %q", data.ExportURL)
	}
}

func TestParseEmail_NoExportURL(t *testing.T) {
	html := `<html><body>
		<p>Report from 01.04.2026 to 30.04.2026</p>
		<a href="https://example.com/other">Click here</a>
	</body></html>`

	_, err := ParseEmail(html)
	if err == nil {
		t.Error("ParseEmail() expected error for missing export URL")
	}
}

func TestParseEmail_NoDates(t *testing.T) {
	html := `<html><body>
		<p>Your report is ready.</p>
		<a href="https://example.harvestapp.com/exports/12345">Download</a>
	</body></html>`

	_, err := ParseEmail(html)
	if err == nil {
		t.Error("ParseEmail() expected error for missing dates")
	}
}

func TestParseEmail_NoClient(t *testing.T) {
	html := `<html><body>
		<p>Report from 01.01.2026 to 31.01.2026</p>
		<a href="https://example.harvestapp.com/exports/12345">Download</a>
	</body></html>`

	data, err := ParseEmail(html)
	if err != nil {
		t.Fatalf("ParseEmail() error = %v", err)
	}
	if data.ClientName != "" {
		t.Errorf("ClientName = %q, want empty", data.ClientName)
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		match []string
		want  time.Time
	}{
		{[]string{"15.03.2026", "15", "03", "2026"}, time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)},
		{[]string{"01.12.2025", "01", "12", "2025"}, time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := parseDate(tt.match)
		if err != nil {
			t.Errorf("parseDate(%v) error = %v", tt.match, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("parseDate(%v) = %v, want %v", tt.match, got, tt.want)
		}
	}
}

func TestParseDate_Invalid(t *testing.T) {
	_, err := parseDate([]string{"bad"})
	if err == nil {
		t.Error("parseDate() expected error for short match")
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"157,00", 157.0},
		{"157.00", 157.0},
		{"42,50", 42.5},
		{"0.25", 0.25},
		{" 160,75 ", 160.75},
	}
	for _, tt := range tests {
		got, err := parseFloat(tt.input)
		if err != nil {
			t.Errorf("parseFloat(%q) error = %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseFloat(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseFloat_Invalid(t *testing.T) {
	_, err := parseFloat("abc")
	if err == nil {
		t.Error("parseFloat(\"abc\") expected error")
	}
}
