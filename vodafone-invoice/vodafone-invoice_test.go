package vodafone

import (
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// parseInvoiceInfo
// ---------------------------------------------------------------------------

func TestParseInvoiceInfo(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantMonth string
		wantYear  string
		wantNil   bool
	}{
		{
			"Aktuelle Rechnung format",
			"Aktuelle Rechnung Februar 2026\nRechnung vom 10.02.2026",
			"02", "2026", false,
		},
		{
			"Rechnungsdatum format",
			"Rechnungsdatum: 01. Januar 2026\nKosten: 24,98€",
			"01", "2026", false,
		},
		{
			"März with umlaut",
			"Aktuelle Rechnung März 2026",
			"03", "2026", false,
		},
		{
			"Dezember end of year",
			"Aktuelle Rechnung Dezember 2025\nRechnung vom 10.12.2025",
			"12", "2025", false,
		},
		{"no match", "Willkommen bei Vodafone. Keine Rechnung.", "", "", true},
		{"empty text", "", "", "", true},
		{"unknown month name", "Aktuelle Rechnung January 2026", "", "", true},
		{
			"picks first match",
			"Aktuelle Rechnung Februar 2026\nRechnungsdatum: 15. Januar 2025",
			"02", "2026", false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := parseInvoiceInfo(tc.text)
			if tc.wantNil {
				if info != nil {
					t.Errorf("expected nil, got month=%s year=%s", info.Month, info.Year)
				}
				return
			}
			if info == nil {
				t.Fatal("expected invoice info, got nil")
			}
			if info.Month != tc.wantMonth {
				t.Errorf("Month = %q, want %q", info.Month, tc.wantMonth)
			}
			if info.Year != tc.wantYear {
				t.Errorf("Year = %q, want %q", info.Year, tc.wantYear)
			}
		})
	}
}

func TestParseInvoiceInfoAllMonths(t *testing.T) {
	for monthName, monthNum := range months {
		t.Run(monthName, func(t *testing.T) {
			info := parseInvoiceInfo("Aktuelle Rechnung " + monthName + " 2026")
			if info == nil {
				t.Fatalf("expected InvoiceInfo for %s, got nil", monthName)
			}
			if info.Month != monthNum {
				t.Errorf("Month = %q, want %q", info.Month, monthNum)
			}
			if info.Year != "2026" {
				t.Errorf("Year = %q, want 2026", info.Year)
			}
			if info.MonthName != monthName {
				t.Errorf("MonthName = %q, want %q", info.MonthName, monthName)
			}
		})
	}
}

func TestParseInvoiceInfoRechnungsdatumAllMonths(t *testing.T) {
	for monthName, monthNum := range months {
		t.Run(monthName, func(t *testing.T) {
			info := parseInvoiceInfo("Rechnungsdatum: 15. " + monthName + " 2025")
			if info == nil {
				t.Fatalf("expected InvoiceInfo for %s, got nil", monthName)
			}
			if info.Month != monthNum {
				t.Errorf("Month = %q, want %q", info.Month, monthNum)
			}
			if info.Year != "2025" {
				t.Errorf("Year = %q, want 2025", info.Year)
			}
		})
	}
}

func TestParseInvoiceInfoEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantMonth string
		wantYear  string
		wantNil   bool
	}{
		{"lowercase month", "Aktuelle Rechnung februar 2026", "", "", true},
		{"year too short", "Aktuelle Rechnung Februar 26", "", "", true},
		{
			"extra whitespace in Rechnungsdatum",
			"Rechnungsdatum:  01.  März  2026",
			"03", "2026", false,
		},
		{
			"Rechnungsdatum without colon",
			"Rechnungsdatum 01. April 2026",
			"04", "2026", false,
		},
		{
			"lots of surrounding content",
			"Hallo Nutzer\nDein Vertrag\nDetails\nAktuelle Rechnung Oktober 2025\nBetrag: 39,99€",
			"10", "2025", false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := parseInvoiceInfo(tc.text)
			if tc.wantNil {
				if info != nil {
					t.Errorf("expected nil, got month=%s year=%s", info.Month, info.Year)
				}
				return
			}
			if info == nil {
				t.Fatal("expected invoice info, got nil")
			}
			if info.Month != tc.wantMonth {
				t.Errorf("Month = %q, want %q", info.Month, tc.wantMonth)
			}
			if info.Year != tc.wantYear {
				t.Errorf("Year = %q, want %q", info.Year, tc.wantYear)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseArchiveFirstEntry
// ---------------------------------------------------------------------------

func TestParseArchiveFirstEntry(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantMonth string
		wantYear  string
		wantName  string
		wantNil   bool
	}{
		{
			"typical archive page",
			"Aktuelle Rechnung Februar 2026\nRechnungsarchiv\nDatum\tBetrag\tRechnung\nJanuar\n04.01.2026\n24,98 €\nDezember\n04.12.2025\n24,98 €",
			"01", "2026", "Januar", false,
		},
		{
			"März with umlaut",
			"Rechnungsarchiv\nMärz\n15.03.2026\n44,98 €",
			"03", "2026", "März", false,
		},
		{
			"picks first entry not second",
			"Rechnungsarchiv\nNovember\n10.11.2025\n44,98 €\nOktober\n09.10.2025\n44,98 €",
			"11", "2025", "November", false,
		},
		{"no Rechnungsarchiv section", "Aktuelle Rechnung Februar 2026\nKeine weiteren Rechnungen.", "", "", "", true},
		{"Rechnungsarchiv but no entries", "Rechnungsarchiv\nKeine Rechnungen vorhanden.", "", "", "", true},
		{"empty text", "", "", "", "", true},
		{
			"ignores current invoice before archive section",
			"Aktuelle Rechnung Februar 2026\nRechnung vom 10.02.2026\nRechnungsarchiv\nJanuar\n04.01.2026\n24,98 €",
			"01", "2026", "Januar", false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := parseArchiveFirstEntry(tc.text, "", "")
			if tc.wantNil {
				if info != nil {
					t.Errorf("expected nil, got month=%s year=%s", info.Month, info.Year)
				}
				return
			}
			if info == nil {
				t.Fatal("expected archive entry, got nil")
			}
			if info.Month != tc.wantMonth {
				t.Errorf("Month = %q, want %q", info.Month, tc.wantMonth)
			}
			if info.Year != tc.wantYear {
				t.Errorf("Year = %q, want %q", info.Year, tc.wantYear)
			}
			if info.MonthName != tc.wantName {
				t.Errorf("MonthName = %q, want %q", info.MonthName, tc.wantName)
			}
		})
	}
}

func TestParseArchiveFirstEntryEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantNil bool
		wantM   string
		wantY   string
		wantN   string
	}{
		{"unknown month", "Rechnungsarchiv\nJanuary\n04.01.2026\n24,98 €", true, "", "", ""},
		{
			"Dezember in archive",
			"Rechnungsarchiv\nDezember\n15.12.2025\n44,98 €",
			false, "12", "2025", "Dezember",
		},
		{"only header text", "Rechnungsarchiv\nDatum\tBetrag\tRechnung", true, "", "", ""},
		{
			"multiple archive sections picks first",
			"Rechnungsarchiv\nApril\n01.04.2026\n30,00 €\nRechnungsarchiv\nMai\n01.05.2026\n35,00 €",
			false, "04", "2026", "April",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := parseArchiveFirstEntry(tc.text, "", "")
			if tc.wantNil {
				if info != nil {
					t.Errorf("expected nil, got month=%s year=%s", info.Month, info.Year)
				}
				return
			}
			if info == nil {
				t.Fatal("expected archive entry, got nil")
			}
			if info.Month != tc.wantM || info.Year != tc.wantY || info.MonthName != tc.wantN {
				t.Errorf("got month=%s year=%s name=%s, want %s/%s/%s",
					info.Month, info.Year, info.MonthName, tc.wantM, tc.wantY, tc.wantN)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Data structure consistency
// ---------------------------------------------------------------------------

func TestMonthsMapCompleteness(t *testing.T) {
	expected := map[string]string{
		"Januar": "01", "Februar": "02", "März": "03", "April": "04",
		"Mai": "05", "Juni": "06", "Juli": "07", "August": "08",
		"September": "09", "Oktober": "10", "November": "11", "Dezember": "12",
	}
	if len(months) != 12 {
		t.Errorf("months map has %d entries, want 12", len(months))
	}
	for name, num := range expected {
		if got, ok := months[name]; !ok {
			t.Errorf("months map missing %q", name)
		} else if got != num {
			t.Errorf("months[%q] = %q, want %q", name, got, num)
		}
	}
}

func TestMonthNamesCompleteness(t *testing.T) {
	if len(monthNames) != 13 {
		t.Fatalf("monthNames has %d entries, want 13 (index 0 is empty)", len(monthNames))
	}
	if monthNames[0] != "" {
		t.Errorf("monthNames[0] = %q, want empty string", monthNames[0])
	}
	expected := []string{"", "Januar", "Februar", "März", "April", "Mai", "Juni",
		"Juli", "August", "September", "Oktober", "November", "Dezember"}
	for i, want := range expected {
		if monthNames[i] != want {
			t.Errorf("monthNames[%d] = %q, want %q", i, monthNames[i], want)
		}
	}
}

func TestContractTypes(t *testing.T) {
	if len(contractTypes) != 2 {
		t.Errorf("contractTypes has %d entries, want 2", len(contractTypes))
	}
	if contractTypes["mobilfunk"] != "Mobilfunk" {
		t.Errorf("contractTypes[mobilfunk] = %q, want Mobilfunk", contractTypes["mobilfunk"])
	}
	if contractTypes["kabel"] != "Kabel" {
		t.Errorf("contractTypes[kabel] = %q, want Kabel", contractTypes["kabel"])
	}
}

func TestMonthsAndMonthNamesConsistency(t *testing.T) {
	for i := 1; i < len(monthNames); i++ {
		name := monthNames[i]
		num, ok := months[name]
		if !ok {
			t.Errorf("monthNames[%d] = %q has no entry in months map", i, name)
			continue
		}
		expected := fmt.Sprintf("%02d", i)
		if num != expected {
			t.Errorf("months[%q] = %q, want %q (index %d)", name, num, expected, i)
		}
	}
}
