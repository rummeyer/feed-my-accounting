package harvest

import (
	"testing"
	"time"
)

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
