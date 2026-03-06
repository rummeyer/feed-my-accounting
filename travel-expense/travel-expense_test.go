package travelexpense

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func boolPtr(b bool) *bool {
	return &b
}

// ---------------------------------------------------------------------------
// doc.go — formatting helpers
// ---------------------------------------------------------------------------

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		month    time.Month
		day      int
		expected string
	}{
		{"single digit day and month", 2026, 1, 5, "05.01.2026"},
		{"double digit day and month", 2026, 12, 25, "25.12.2026"},
		{"first day of year", 2026, 1, 1, "01.01.2026"},
		{"last day of year", 2026, 12, 31, "31.12.2026"},
		{"leap year date", 2024, 2, 29, "29.02.2024"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDate(tt.year, tt.month, tt.day)
			if got != tt.expected {
				t.Errorf("formatDate(%d, %d, %d) = %q, want %q", tt.year, tt.month, tt.day, got, tt.expected)
			}
		})
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		expected string
	}{
		{"zero", 0, "0,00"},
		{"integer amount", 14, "14,00"},
		{"decimal amount", 30.60, "30,60"},
		{"large amount", 1234.56, "1234,56"},
		{"small amount", 0.30, "0,30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAmount(tt.amount)
			if got != tt.expected {
				t.Errorf("formatAmount(%v) = %q, want %q", tt.amount, got, tt.expected)
			}
		})
	}
}

func TestRightAlign(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		width    int
		expected string
	}{
		{"shorter than width", "hello", 10, "     hello"},
		{"equal to width", "hello", 5, "hello"},
		{"longer than width", "hello world", 5, "hello world"},
		{"empty string", "", 5, "     "},
		{"width zero", "hello", 0, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rightAlign(tt.s, tt.width)
			if got != tt.expected {
				t.Errorf("rightAlign(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.expected)
			}
		})
	}
}

func TestDocumentID(t *testing.T) {
	id := documentID(2026, 2)

	if !strings.HasPrefix(id, "RK-2026-02-") {
		t.Errorf("documentID(2026, 2) = %q, want prefix RK-2026-02-", id)
	}
	if len(id) != 15 {
		t.Errorf("documentID length = %d, want 15", len(id))
	}
	suffix := id[11:]
	for _, c := range suffix {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Errorf("documentID suffix %q contains invalid character %c", suffix, c)
		}
	}
	// Two calls should (very likely) produce different values
	id2 := documentID(2026, 2)
	if id == id2 {
		t.Logf("Warning: two documentID calls returned same value %q (possible but unlikely)", id)
	}
}

// ---------------------------------------------------------------------------
// doc.go — document content builders
// ---------------------------------------------------------------------------

func TestBuildCustomerHeader(t *testing.T) {
	c := Customer{
		ID:     "1",
		Name:   "Acme Corp",
		From:   "Stuttgart",
		To:     "München",
		Reason: "Projektarbeit",
	}
	got := buildCustomerHeader(c)
	for _, want := range []string{"1) Acme Corp", "Von:    Stuttgart", "Nach:   München", "Grund:  Projektarbeit", lineSingle} {
		if !strings.Contains(got, want) {
			t.Errorf("buildCustomerHeader missing %q in:\n%s", want, got)
		}
	}
}

func TestBuildKilometerEntry(t *testing.T) {
	got := buildKilometerEntry("13.02.2026", 100)
	for _, want := range []string{"13.02.2026", "Fahrkosten (100 km x 0,30 EUR)", "30,00 EUR"} {
		if !strings.Contains(got, want) {
			t.Errorf("buildKilometerEntry missing %q in:\n%s", want, got)
		}
	}
}

func TestBuildKilometerEntryCalculation(t *testing.T) {
	tests := []struct {
		distance int
		amount   string
	}{
		{50, "15,00 EUR"},
		{1, "0,30 EUR"},
		{200, "60,00 EUR"},
	}
	for _, tt := range tests {
		t.Run(tt.amount, func(t *testing.T) {
			got := buildKilometerEntry("01.01.2026", tt.distance)
			if !strings.Contains(got, tt.amount) {
				t.Errorf("buildKilometerEntry with distance %d missing amount %q", tt.distance, tt.amount)
			}
		})
	}
}

func TestBuildMealAllowanceEntry(t *testing.T) {
	got := buildMealAllowanceEntry("13.02.2026")
	for _, want := range []string{"13.02.2026", "07:00 - 17:00", "Verpflegungsmehraufwand (8h - 24h)", "14,00 EUR"} {
		if !strings.Contains(got, want) {
			t.Errorf("buildMealAllowanceEntry missing %q in:\n%s", want, got)
		}
	}
}

func TestBuildDocumentFooter(t *testing.T) {
	got := buildDocumentFooter(150.00)
	for _, want := range []string{"Rechnungsbetrag:", "150,00 EUR", lineDouble} {
		if !strings.Contains(got, want) {
			t.Errorf("buildDocumentFooter missing %q in:\n%s", want, got)
		}
	}
}

func TestBuildDocumentFooterZero(t *testing.T) {
	got := buildDocumentFooter(0)
	if !strings.Contains(got, "0,00 EUR") {
		t.Errorf("buildDocumentFooter(0) missing 0,00 EUR in:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// pdf.go
// ---------------------------------------------------------------------------

func TestCreatePDF(t *testing.T) {
	data, err := createPDF("Test Lieferant", "Test Header\n", []string{"Block 1\nLine 2\n", "Block 2\n"}, "Footer\n")
	if err != nil {
		t.Fatalf("createPDF() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("createPDF() returned empty data")
	}
	if len(data) < 4 || string(data[:4]) != "%PDF" {
		t.Error("createPDF() output does not start with PDF magic bytes")
	}
}

func TestCreatePDFEmpty(t *testing.T) {
	data, err := createPDF("", "", nil, "")
	if err != nil {
		t.Fatalf("createPDF() with empty input error = %v", err)
	}
	if len(data) == 0 {
		t.Error("createPDF() with empty input returned empty data")
	}
}

// ---------------------------------------------------------------------------
// travel-expense.go — calendar logic
// ---------------------------------------------------------------------------

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		month    time.Month
		expected int
	}{
		{"January", 2026, 1, 31},
		{"February non-leap", 2025, 2, 28},
		{"February leap", 2024, 2, 29},
		{"March", 2026, 3, 31},
		{"April", 2026, 4, 30},
		{"May", 2026, 5, 31},
		{"June", 2026, 6, 30},
		{"July", 2026, 7, 31},
		{"August", 2026, 8, 31},
		{"September", 2026, 9, 30},
		{"October", 2026, 10, 31},
		{"November", 2026, 11, 30},
		{"December", 2026, 12, 31},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daysInMonth(tt.year, tt.month)
			if got != tt.expected {
				t.Errorf("daysInMonth(%d, %d) = %d, want %d", tt.year, tt.month, got, tt.expected)
			}
		})
	}
}

func TestChristmasWeekOffEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected bool
	}{
		{"nil defaults to true", Config{ChristmasWeekOff: nil}, true},
		{"explicit true", Config{ChristmasWeekOff: boolPtr(true)}, true},
		{"explicit false", Config{ChristmasWeekOff: boolPtr(false)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.christmasWeekOffEnabled()
			if got != tt.expected {
				t.Errorf("christmasWeekOffEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewBusinessCalendar(t *testing.T) {
	if cal := newBusinessCalendar("BY"); cal == nil {
		t.Fatal("newBusinessCalendar(BY) returned nil")
	}
	if cal := newBusinessCalendar("INVALID"); cal == nil {
		t.Fatal("newBusinessCalendar(INVALID) returned nil")
	}
	if cal := newBusinessCalendar(""); cal == nil {
		t.Fatal("newBusinessCalendar('') returned nil")
	}
}

func TestGetCustomerCalendars(t *testing.T) {
	customers := []Customer{{Province: "BW"}, {Province: "BY"}, {Province: "BE"}}
	calendars := getCustomerCalendars(customers)
	if len(calendars) != len(customers) {
		t.Fatalf("getCustomerCalendars returned %d calendars, want %d", len(calendars), len(customers))
	}
	for i, c := range calendars {
		if c == nil {
			t.Errorf("calendar[%d] is nil", i)
		}
	}
}

func TestIsWorkday(t *testing.T) {
	cal := newBusinessCalendar("BW")
	tests := []struct {
		name             string
		date             time.Time
		christmasWeekOff bool
		expected         bool
	}{
		{"regular weekday", time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), true, true},
		{"Saturday", time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC), true, false},
		{"Sunday", time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC), true, false},
		{"New Years Day", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), true, false},
		{"Christmas Eve off", time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC), true, false},
		{"Christmas Eve on", time.Date(2025, 12, 24, 0, 0, 0, 0, time.UTC), false, true},
		{"Dec 28 off", time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC), true, false},
		{"Dec 28 on", time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC), false, true},
		{"Dec 31 off", time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), true, false},
		{"Dec 31 on", time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), false, true},
		{"regular Dec day", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWorkday(cal, tt.date, tt.christmasWeekOff)
			if got != tt.expected {
				t.Errorf("isWorkday(%s, christmasWeekOff=%v) = %v, want %v",
					tt.date.Format("2006-01-02 Monday"), tt.christmasWeekOff, got, tt.expected)
			}
		})
	}
}
