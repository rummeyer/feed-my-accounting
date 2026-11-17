package main

import (
	"testing"
	"time"
)

func TestICloudFolder(t *testing.T) {
	now := time.Date(2026, 12, 1, 7, 0, 0, 0, time.Local)
	for _, c := range []struct{ folder, name, want string }{
		{"Belege/{YYYY}/{MM}", "11_2026_Rechnung_Amazon_17112026.pdf", "Belege/2026/11"},
		{"Belege/{YYYY}/{MM}", "10_2026_Rechnung_Vodafone_kabel.pdf", "Belege/2026/10"},
		{"Belege/{YYYY}/{MM}", "Deine Rechnung von Apple.pdf", "Belege/2026/12"}, // no month in its name: now's
		{"/Buchhaltung/", "11_2026_x.pdf", "Buchhaltung"},
		{"{YYYY}-{MM} Belege", "02_2027_x.pdf", "2027-02 Belege"},
	} {
		if got := icloudFolder(c.folder, c.name, now); got != c.want {
			t.Errorf("icloudFolder(%q, %q) = %q, want %q", c.folder, c.name, got, c.want)
		}
	}
}

func TestFreeName(t *testing.T) {
	taken := map[string]bool{"a.pdf": true, "a (1).pdf": true, "b.pdf": false}
	for _, c := range []struct{ name, want string }{
		{"a.pdf", "a (2).pdf"},
		{"b.pdf", "b.pdf"},
		{"c", "c"},
	} {
		if got := freeName(c.name, taken); got != c.want {
			t.Errorf("freeName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
