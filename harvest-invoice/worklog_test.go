package harvest

import (
	"strings"
	"testing"
	"time"

	"github.com/go-pdf/fpdf"
)

func TestWorkLog(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.Local) }
	var entries []Entry
	for d := 1; d <= 31; d++ {
		for _, slot := range [][3]string{{"7:30", "12:30", "5"}, {"13:00", "16:00", "3"}} {
			hours := 5.0
			if slot[2] == "3" {
				hours = 3
			}
			entries = append(entries, Entry{
				Date: day(d), Started: slot[0], Ended: slot[1], Client: "Acme GmbH", Project: "Beratung & Umsetzung – Süd",
				Code: "0042", Task: "Dokumentation", Person: "Max Mustermann", Hours: hours, Billable: true,
				Notes: strings.Repeat("Abstimmung Größenänderung ", d%4+1),
			})
		}
	}
	entries[0].Billed = true
	data, err := WorkLog("Muster GmbH", day(1), day(31), entries)
	if err != nil {
		t.Fatal(err)
	}
	// What the upload reads off a Harvest report, it reads off this one too.
	r, err := ParseReportPDF(data)
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalHours != 248 || r.PeriodFrom.Format("02.01.2006") != "01.10.2026" || r.PeriodTo.Format("02.01.2006") != "31.10.2026" {
		t.Errorf("read back %v hours, %s – %s; want 248, 01.10. – 31.10.", r.TotalHours, r.PeriodFrom, r.PeriodTo)
	}
	text, err := pdfText(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Detailed time report", "Muster GmbH", "243,00 Uninvoiced billable hours", "[0042] Beratung", "[7:30 - 12:30] Abstimmung", "Page 1 of"} {
		if !strings.Contains(text, want) {
			t.Errorf("work log without %q", want)
		}
	}
	if got := WorkLogName(day(1)); got != "10_2026_WorkLog.pdf" {
		t.Errorf("name %q", got)
	}
}

func TestWrapText(t *testing.T) {
	doc := newTestPDF()
	same := func(s string) string { return s }
	lines := wrapText(doc, same, "EB-27079, EB-27078, EB-27077, EB-26492 Tests", 40)
	if len(lines) < 2 || strings.Join(lines, " ") != "EB-27079, EB-27078, EB-27077, EB-26492 Tests" {
		t.Errorf("wrapped %q", lines)
	}
	for _, l := range wrapText(doc, same, strings.Repeat("x", 200), 30) {
		if doc.GetStringWidth(l) > 30 {
			t.Errorf("line %q wider than 30 mm", l)
		}
	}
	if got := wrapText(doc, same, "", 30); len(got) != 1 || got[0] != "" {
		t.Errorf("empty text: %q", got)
	}
}

func newTestPDF() *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", logFont)
	return pdf
}
