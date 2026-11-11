package travelexpense

import (
	"errors"
	"math"
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

func TestBuildClientHeader(t *testing.T) {
	c := Client{
		ID:     "1",
		Name:   "Acme Corp",
		From:   "Stuttgart",
		To:     "München",
		Reason: "Projektarbeit",
	}
	got := buildClientHeader(c)
	for _, want := range []string{"1) Acme Corp", "Von:    Stuttgart", "Nach:   München", "Grund:  Projektarbeit", lineSingle} {
		if !strings.Contains(got, want) {
			t.Errorf("buildClientHeader missing %q in:\n%s", want, got)
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

func TestGetClientCalendars(t *testing.T) {
	clients := []Client{{Province: "BW"}, {Province: "BY"}, {Province: "BE"}}
	calendars := getClientCalendars(clients)
	if len(calendars) != len(clients) {
		t.Fatalf("getClientCalendars returned %d calendars, want %d", len(calendars), len(clients))
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

func TestPlanSkipsWeekendsHolidaysAndBlockedDays(t *testing.T) {
	cfg := Config{
		Clients: []Client{{Name: "A", Province: "BW"}, {Name: "B", Province: "BW"}},
		Blocked: []BlockedRange{{From: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC), Note: "Urlaub"}},
	}
	days := Plan(cfg, 2026, time.May)
	if len(days) != 31 {
		t.Fatalf("want 31 days, got %d", len(days))
	}
	kinds := map[string]int{}
	for _, d := range days {
		kinds[d.Kind]++
	}
	// May 2026 in BW: 1 May, Ascension 14 May, Whit Monday 25 May; 11–13 May blocked.
	if kinds[DayWeekend] != 10 || kinds[DayHoliday] != 3 || kinds[DayBlocked] != 3 || kinds[DayTravel] != 15 {
		t.Errorf("kinds = %v", kinds)
	}
	if d := days[0]; d.Kind != DayHoliday || d.Note == "" {
		t.Errorf("1 May should be a named holiday: %+v", d)
	}
	if d := days[10]; d.Kind != DayBlocked || d.Note != "Urlaub" || d.Client != -1 {
		t.Errorf("11 May should be blocked: %+v", d)
	}
	// Round-robin over travel days only: the first two go to A then B.
	var travel []Day
	for _, d := range days {
		if d.Kind == DayTravel {
			travel = append(travel, d)
		}
	}
	if travel[0].Client != 0 || travel[1].Client != 1 || travel[2].Client != 0 {
		t.Errorf("round-robin broken: %+v", travel[:3])
	}
}

func TestBlockedRangeSingleDay(t *testing.T) {
	day := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	b := BlockedRange{From: day}
	if !b.Contains(day) || b.Contains(day.AddDate(0, 0, 1)) || b.Contains(day.AddDate(0, 0, -1)) {
		t.Error("a range without To is its From day only")
	}
}

func TestGenerateWithoutTravelDays(t *testing.T) {
	cfg := Config{
		Clients: []Client{{Name: "A", Province: "BW"}},
		Blocked: []BlockedRange{{From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}},
	}
	if _, err := Generate(cfg, 2026, time.August); !errors.Is(err, ErrNoTravelDays) {
		t.Fatalf("want ErrNoTravelDays, got %v", err)
	}
	if err := Run(cfg, 2026, time.August); err != nil {
		t.Fatalf("a month without travel is nothing to send, not an error: %v", err)
	}
}

func TestGenerateTotals(t *testing.T) {
	cfg := Config{Employee: "Max", Clients: []Client{{ID: "1", Name: "A", Distance: 20, Province: "BW"}}}
	r, err := Generate(cfg, 2026, time.May)
	if err != nil {
		t.Fatal(err)
	}
	// 18 workdays in May 2026 in BW; 20 km × 0,30 € a day; 14 € meals a day.
	if r.Days != 18 || math.Abs(r.MileageEUR-18*20*0.30) > 1e-9 || r.MealsEUR != 18*14 {
		t.Errorf("totals: %+v", r)
	}
	if !strings.HasPrefix(r.Mileage.Filename, "05_2026_") || len(r.Meals.Data) == 0 {
		t.Errorf("documents: %q %d bytes", r.Mileage.Filename, len(r.Meals.Data))
	}
}

func date(day int) time.Time { return time.Date(2026, time.December, day, 0, 0, 0, 0, time.UTC) }

// December 2026: an overnight trip to Miami for Globex from the 3rd to the 9th, every other
// workday with Acme. Christmas week off.
func decemberWithMiami() Config {
	return Config{
		Employee: "Max",
		Clients: []Client{
			{ID: "1", Name: "Acme GmbH", Distance: 15, Province: "BW"},
			{ID: "2", Name: "Globex Inc.", Distance: 30, Province: "BW", OnlyAssigned: true,
				MealFullDay: 64, MealTravelDay: 43},
		},
		Assignments: []Assignment{{Client: 1, From: date(3), To: date(9), Overnight: true, Note: "Miami"}},
	}
}

func TestPlanOvernightTripAndEqualDistribution(t *testing.T) {
	days := Plan(decemberWithMiami(), 2026, time.December)
	byDay := func(d int) Day { return days[d-1] }

	// The trip: every day counts, the weekend too; travel day rate and the drive on the 3rd and
	// 9th, full day rate between.
	for d := 3; d <= 9; d++ {
		if got := byDay(d); got.Kind != DayAway || got.Client != 1 || !got.Assigned || got.Note != "Miami" {
			t.Errorf("%d Dec: %+v", d, got)
		}
	}
	if d := byDay(3); d.Meal != 43 || d.MealText != "An-/Abreisetag" || d.Mileage != 30*0.30 {
		t.Errorf("first day: %+v", d)
	}
	if d := byDay(6); d.Meal != 64 || d.MealText != "24h" || d.Mileage != 0 {
		t.Errorf("Sunday away: %+v", d)
	}
	if d := byDay(9); d.Meal != 43 || d.Mileage != 30*0.30 {
		t.Errorf("last day: %+v", d)
	}
	// Every other workday to Acme, Globex only taking its assigned days.
	for _, d := range days {
		if d.Kind == DayTravel && d.Client != 0 {
			t.Errorf("%s went to client %d", d.Date.Format("02.01."), d.Client)
		}
	}
	if d := byDay(1); d.Kind != DayTravel || d.Meal != 14 || d.Mileage != 15*0.30 {
		t.Errorf("1 Dec: %+v", d)
	}
	if d := byDay(24); d.Kind != DayChristmas {
		t.Errorf("24 Dec: %+v", d)
	}
}

func TestGenerateWithOvernightTrip(t *testing.T) {
	r, err := Generate(decemberWithMiami(), 2026, time.December)
	if err != nil {
		t.Fatal(err)
	}
	// Acme: December 2026 has 23 weekdays, less Christmas Day, the 24th and 28th–31st
	// off, and the 3rd, 4th, 7th, 8th and 9th in Miami: 12 day trips.
	travel := 12
	if r.Days != travel+7 {
		t.Errorf("days = %d", r.Days)
	}
	if want := float64(travel)*14 + 2*43 + 5*64; math.Abs(r.MealsEUR-want) > 1e-9 {
		t.Errorf("meals = %.2f, want %.2f", r.MealsEUR, want)
	}
	if want := float64(travel)*15*0.30 + 2*30*0.30; math.Abs(r.MileageEUR-want) > 1e-9 {
		t.Errorf("mileage = %.2f, want %.2f", r.MileageEUR, want)
	}
	if len(r.Documents()) != 2 {
		t.Errorf("want both PDFs, got %d", len(r.Documents()))
	}
}

func TestGenerateAbroadWithoutDriving(t *testing.T) {
	cfg := Config{
		Clients:     []Client{{Name: "Abroad", Province: "BW", OnlyAssigned: true, MealFullDay: 50}},
		Assignments: []Assignment{{Client: 0, From: date(14), To: date(18), Overnight: true}},
	}
	r, err := Generate(cfg, 2026, time.December)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mileage != nil || r.Meals == nil || len(r.Documents()) != 1 {
		t.Errorf("no distance, no mileage PDF: %+v", r)
	}
}

func TestPlanDayAssignmentOverridesEqualDistribution(t *testing.T) {
	cfg := Config{
		Clients:     []Client{{Name: "A", Province: "BW"}, {Name: "B", Province: "BW"}},
		Assignments: []Assignment{{Client: 1, From: date(14), To: date(18)}},
	}
	days := Plan(cfg, 2026, time.December)
	for d := 14; d <= 18; d++ {
		if got := days[d-1]; got.Kind != DayTravel || got.Client != 1 || !got.Assigned {
			t.Errorf("%d Dec: %+v", d, got)
		}
	}
	// The weekend in an assignment of day trips stays off.
	if got := days[19]; got.Kind != DayWeekend {
		t.Errorf("20 Dec: %+v", got)
	}
}

func TestBuildMealEntryAbroad(t *testing.T) {
	got := buildMealEntry("05.12.2026", "", "24h", 64)
	for _, want := range []string{"05.12.2026", "Verpflegungsmehraufwand (24h)", "64,00 EUR"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
