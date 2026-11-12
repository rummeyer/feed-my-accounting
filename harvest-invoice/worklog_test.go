package harvest

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
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
	data, err := WorkLog("Muster GmbH", "", day(1), day(31), entries)
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
	for _, want := range []string{"Zeitnachweis", "Muster GmbH", "Zeitraum", "248,00 Stunden", "243,00 Stunden abrechenbar", "Tätigkeit", "[0042] Beratung", "[7:30 - 12:30] Abstimmung", "Seite 1 von"} {
		if !strings.Contains(text, want) {
			t.Errorf("work log without %q", want)
		}
	}
	if got := WorkLogName(day(1), ""); got != "10_2026_WorkLog.pdf" {
		t.Errorf("name %q", got)
	}
	if got := WorkLogName(day(1), "0202"); got != "10_2026_Rechnung_0202_WorkLog.pdf" {
		t.Errorf("name with invoice %q", got)
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

func TestWorkLogLogo(t *testing.T) {
	// A wide logo, as a company's is: it takes the company's name's place.
	img := image.NewRGBA(image.Rect(0, 0, 400, 100))
	for x := 0; x < 400; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{0x2e, 0x2e, 0x2e, 0xff})
		}
	}
	logo := filepath.Join(t.TempDir(), "logo.png")
	f, err := os.Create(logo)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	entries := []Entry{{Date: day, Client: "Acme GmbH", Project: "Beratung", Task: "Analyse", Person: "Max Mustermann", Hours: 2}}
	data, err := WorkLog("Muster GmbH", logo, day, day.AddDate(0, 1, -1), entries)
	if err != nil {
		t.Fatal(err)
	}
	text, _ := pdfText(data)
	if strings.Contains(text, "Muster GmbH") || !strings.Contains(text, "Zeitnachweis") {
		t.Errorf("with a logo, the name should not show: %q", text[:min(200, len(text))])
	}
	if _, err := WorkLog("Muster GmbH", filepath.Join(t.TempDir(), "none.png"), day, day, entries); err == nil {
		t.Error("a logo that is not there: no error")
	}
}
