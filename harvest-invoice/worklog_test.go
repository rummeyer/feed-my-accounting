package harvest

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
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
	data, err := WorkLog("Muster GmbH", "", day(1), day(31), entries, []Budget{{Project: "Süd", By: "project", Budget: 400, Spent: 249}, {Project: "Kosten", By: "project_cost", Budget: 9000, Spent: 10}})
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
	for _, want := range []string{"Zeitnachweis 10/2026", "Muster GmbH", "Zeitraum", "248,00 Stunden", "243,00 Stunden abrechenbar", "151,00 Stunden verbleibend", "Tätigkeit", "[0042] Beratung", "[7:30 - 12:30] Abstimmung", "Seite 1 von"} {
		if !strings.Contains(text, want) {
			t.Errorf("work log without %q", want)
		}
	}
	if got := WorkLogName(day(1), ""); got != "10_2026_Rechnung_WorkLog.pdf" {
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
	data, err := WorkLog("Muster GmbH", logo, day, day.AddDate(0, 1, -1), entries, nil)
	if err != nil {
		t.Fatal(err)
	}
	text, _ := pdfText(data)
	if strings.Contains(text, "Muster GmbH") || !strings.Contains(text, "Zeitnachweis") {
		t.Errorf("with a logo, the name should not show: %q", text[:min(200, len(text))])
	}
	if _, err := WorkLog("Muster GmbH", filepath.Join(t.TempDir(), "none.png"), day, day, entries, nil); err == nil {
		t.Error("a logo that is not there: no error")
	}
}

func TestLogoShrunk(t *testing.T) {
	// A logo as uploaded: large, transparent around a dark mark.
	img := image.NewNRGBA(image.Rect(0, 0, 1536, 1024))
	for x := 400; x < 1100; x++ {
		for y := 300; y < 700; y++ {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 0x40, 0xff})
		}
	}
	logo := filepath.Join(t.TempDir(), "logo.png")
	f, _ := os.Create(logo)
	png.Encode(f, img)
	f.Close()
	data, err := logoJPEG(logo)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Bounds(); b.Dx() != logoPixels || b.Dy() != 1024*logoPixels/1536 {
		t.Errorf("shrunk to %v, want %d wide, proportions kept", b, logoPixels)
	}
	// Transparent is white, the mark stays dark.
	if r, g, b, _ := got.At(5, 5).RGBA(); r>>8 < 245 || g>>8 < 245 || b>>8 < 245 {
		t.Errorf("transparent corner not white: %d %d %d", r>>8, g>>8, b>>8)
	}
	if _, _, b, _ := got.At(340, 220).RGBA(); b>>8 > 120 {
		t.Errorf("the mark lost: blue %d", b>>8)
	}
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	pdfData, err := WorkLog("Muster GmbH", logo, day, day, []Entry{{Date: day, Client: "Acme GmbH", Hours: 1}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pdfData) > 150<<10 {
		t.Errorf("work log with a logo: %d KB", len(pdfData)>>10)
	}
}

func TestLogoAsUploaded(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, img image.Image, asJPEG bool) string {
		path := filepath.Join(dir, name)
		f, _ := os.Create(path)
		defer f.Close()
		if asJPEG {
			jpeg.Encode(f, img, &jpeg.Options{Quality: 80})
		} else {
			png.Encode(f, img)
		}
		return path
	}
	small := image.NewNRGBA(image.Rect(0, 0, 300, 200))
	small.Set(10, 10, color.NRGBA{0, 0, 0, 0x80})
	paletted := image.NewPaletted(image.Rect(0, 0, 300, 200), color.Palette{color.NRGBA{0, 0, 0, 0}, color.NRGBA{0, 0, 0, 0xff}, color.NRGBA{0x80, 0x80, 0x80, 0x40}})
	for _, c := range []struct {
		name, path, kind string
		asUploaded       bool
	}{
		{"small PNG with alpha", write("small.png", small, false), "PNG", true},
		{"small JPEG", write("small.jpg", small, true), "JPG", true},
		{"palette with transparency", write("palette.png", paletted, false), "JPG", false},
		{"too wide", write("wide.png", image.NewNRGBA(image.Rect(0, 0, logoPixels+1, 100)), false), "JPG", false},
	} {
		uploaded, _ := os.ReadFile(c.path)
		data, kind, err := logoForPDF(c.path)
		if err != nil || kind != c.kind || bytes.Equal(data, uploaded) != c.asUploaded {
			t.Errorf("%s: %s, as uploaded %v, %v; want %s, as uploaded %v", c.name, kind, bytes.Equal(data, uploaded), err, c.kind, c.asUploaded)
		}
	}
}

func TestOverBudget(t *testing.T) {
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	entries := []Entry{{ProjectID: 1}, {ProjectID: 2}, {ProjectID: 3}}
	budgets := []Budget{
		{ProjectID: 1, Budget: 100, Spent: 120},              // over: warned
		{ProjectID: 2, Budget: 100, Spent: 100},              // just spent: not
		{ProjectID: 3, Budget: 10, Spent: 12, Monthly: true}, // monthly, over this month
		{ProjectID: 4, Budget: 10, Spent: 50},                // not in the work log
	}
	ids := func(bs []Budget) (out []int) {
		for _, b := range bs {
			out = append(out, b.ProjectID)
		}
		return out
	}
	if got := ids(overBudget(budgets, entries, nov, nov.AddDate(0, 0, 12))); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Errorf("this month: %v, want [1 3]", got)
	}
	// October's work log in November: the monthly spending is November's, not October's.
	if got := ids(overBudget(budgets, entries, nov.AddDate(0, -1, 0), nov.AddDate(0, 0, 12))); !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("a past month: %v, want [1]", got)
	}
}

func TestBudgetWarning(t *testing.T) {
	got := budgetWarning(Budget{ProjectID: 7, Project: "Beta", By: "project", Budget: 1440, Spent: 1500}, "https://example.harvestapp.com")
	want := "the budget of project Beta exceeded: 1500.00 h of 1440.00 h spent, 60.00 h over: see https://example.harvestapp.com/projects/7"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if got := budgetWarning(Budget{ProjectID: 7, Project: "Beta", By: "project_cost", Budget: 1000, Spent: 1100}, "u"); !strings.Contains(got, "1100.00 of 1000.00 spent") {
		t.Errorf("cost budget: %q", got)
	}
}

func TestBudgetLines(t *testing.T) {
	one := []Budget{{Project: "A", By: "project", Budget: 240, Spent: 89}}
	if got := budgetLines(one); !reflect.DeepEqual(got, []budgetLine{{"151,00 Stunden verbleibend", false}}) {
		t.Errorf("one: %+v", got)
	}
	// Less than a twelfth of 1440 h left: short; 120 h just not. A monthly budget only spent beyond.
	short := []Budget{{Project: "A", By: "project", Budget: 1440, Spent: 1330}, {Project: "B", By: "project", Budget: 1440, Spent: 1320}, {Project: "C", By: "project", Monthly: true, Budget: 160, Spent: 155}}
	if got := budgetLines(short); !reflect.DeepEqual(got, []budgetLine{{"110,00 Stunden verbleibend (A)", true}, {"120,00 Stunden verbleibend (B)", false}, {"5,00 Stunden verbleibend (C)", false}}) {
		t.Errorf("short: %+v", got)
	}
	two := []Budget{{Project: "A", By: "project", Budget: 100, Spent: 112}, {Project: "B", By: "task", Budget: 50, Spent: 20}, {Project: "C", By: "project_cost", Budget: 5000, Spent: 1}}
	if got := budgetLines(two); !reflect.DeepEqual(got, []budgetLine{{"Keine Stunden verbleibend (A)", true}, {"30,00 Stunden verbleibend (B)", false}}) {
		t.Errorf("two: %+v", got)
	}
}

func TestBookedDays(t *testing.T) {
	day := func(d int, h float64) Entry {
		return Entry{Date: time.Date(2026, 11, d, 0, 0, 0, 0, time.Local), Hours: h}
	}
	// Four days a week: Monday to Thursday, two entries on one of them, and one of 0 hours.
	entries := []Entry{day(2, 4), day(2, 4.5), day(3, 8), day(4, 8), day(5, 8), day(6, 0)}
	if n := bookedDays(entries); n != 4 {
		t.Errorf("bookedDays() = %d, want 4", n)
	}
	if n := bookedDays(nil); n != 0 {
		t.Errorf("bookedDays(nil) = %d, want 0", n)
	}
}

// 154 h booked, 120 h of the budget left before the month: 120 h billable, 34 h not.
func TestBeyondTheBudget(t *testing.T) {
	d := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	var entries []Entry
	for i := 0; i < 14; i++ {
		entries = append(entries, Entry{Date: d.AddDate(0, 0, i), Client: "Muster GmbH", Project: "Beratung", ProjectID: 1, Task: "Beratung", Person: "Max", Hours: 11, Billable: true})
	}
	// Harvest counts the month as spent: 1320 before it, 1474 now.
	budget := []Budget{{ProjectID: 1, Project: "Beratung", By: "project", Budget: 1440, Spent: 1474}}
	if in, beyond := Billable(entries, budget); in != 120 || beyond != 34 {
		t.Fatalf("want 120 in, 34 beyond: %v %v", in, beyond)
	}
	// Spent beyond before the month: none of it.
	if in, beyond := Billable(entries, []Budget{{ProjectID: 1, By: "project", Budget: 1440, Spent: 1600}}); in != 0 || beyond != 154 {
		t.Errorf("none left before: %v %v", in, beyond)
	}
	// Room enough, or no budget in hours: all of it.
	if in, beyond := Billable(entries, []Budget{{ProjectID: 1, By: "project", Budget: 1440, Spent: 500}}); in != 154 || beyond != 0 {
		t.Errorf("room enough: %v %v", in, beyond)
	}
	if in, _ := Billable(entries, []Budget{{ProjectID: 1, By: "project_cost", Budget: 10, Spent: 99}}); in != 154 {
		t.Errorf("a budget in money caps nothing: %v", in)
	}

	data, err := WorkLog("Muster Consulting", "", d, d.AddDate(0, 0, 27), entries, budget)
	if err != nil {
		t.Fatal(err)
	}
	text, err := pdfText(data)
	if err != nil {
		t.Fatal(err)
	}
	// The red part a text of its own in the PDF.
	for _, want := range []string{"154,00 Stunden", "120,00 Stunden abrechenbar,", "34,00 Stunden nicht abrechenbar", "Keine Stunden verbleibend"} {
		if !strings.Contains(text, want) {
			t.Errorf("work log without %q", want)
		}
	}
	// Uploaded, it gives the billable hours, as the draft takes them.
	if r, err := ParseReportPDF(data); err != nil || r.TotalHours != 120 {
		t.Errorf("read back %v, want 120: %v", r, err)
	}
}
