package harvest

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"feed-my-accounting/pdfdoc"

	"github.com/go-pdf/fpdf"
)

// The work log is laid out as Harvest's detailed time report, in German: a head with the
// company's logo or name, the period and the totals, then a table by day — client, project with
// the note beneath, task, person, hours — its head on every page, the total and the page number
// at each page's foot.

const (
	logMargin  = 12.0
	logLine    = 4.0 // a line of the table's text
	logPad     = 2.6 // above and beneath a row's text
	logFont    = 8.5
	logDayRow  = 7.0
	logHeadRow = 7.0
)

// The table's columns: their titles and widths, which fill the 186 mm between the margins.
var logColumns = []struct {
	title string
	width float64
}{{"Kunde", 36}, {"Projekt", 72}, {"Tätigkeit", 28}, {"Mitarbeiter", 30}, {"Stunden", 20}}

// The logo as sevDesk places it on the invoices (Rechnung 0201, 30.09.2026): 57.6 mm wide, its
// top 4.8 mm from the page's, its right edge 18 mm from the page's — so that the same file looks
// the same on both. Its height by its proportions, no taller than logoMaxHeight.
const (
	logoWidth     = 57.6
	logoTop       = 4.8
	logoRight     = 18.0
	logoMaxHeight = 40.0
)

// WorkLog is the PDF of the entries, Harvest's detailed time report as the company would have
// it for the period from from to to: with its logo, a PNG or JPEG, else its name.
func WorkLog(company, logo string, from, to time.Time, entries []Entry) ([]byte, error) {
	var total, open float64
	for _, e := range entries {
		total += e.Hours
		if e.Billable && !e.Billed {
			open += e.Hours
		}
	}
	var tr func(string) string
	doc := pdfdoc.New(func(pdf *fpdf.Fpdf) {
		tr = pdf.UnicodeTranslatorFromDescriptor("") // cp1252: umlauts, €, –
		pdf.SetMargins(logMargin, logMargin, logMargin)
		pdf.SetAutoPageBreak(false, 22) // room for the total and the page number
		pdf.AliasNbPages("{nb}")
		pdf.SetHeaderFunc(func() {
			if pdf.PageNo() > 1 {
				tableHead(pdf, tr)
			}
		})
		pdf.SetFooterFunc(func() {
			pdf.SetY(-19)
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(40, 40, 40)
			pdf.CellFormat(186-logColumns[4].width, 5, "Summe", "", 0, "R", false, 0, "")
			pdf.CellFormat(logColumns[4].width, 5, hours(total), "", 1, "R", false, 0, "")
			pdf.SetFont("Helvetica", "", 9)
			pdf.CellFormat(186, 6, fmt.Sprintf("Seite %d von {nb}", pdf.PageNo()), "", 0, "C", false, 0, "")
		})
	})
	if err := logHead(doc, tr, company, logo, from, to, total, open, entries); err != nil {
		return nil, err
	}
	tableHead(doc.Fpdf, tr)

	for _, day := range byDay(entries) {
		rows := make([]float64, len(day))
		var sum float64
		for i, e := range day {
			rows[i] = entryHeight(doc.Fpdf, tr, e)
			sum += e.Hours
		}
		// The day's row never alone at a page's foot: with its first entry, or on the next page.
		doc.Keep(logDayRow + rows[0])
		dayRow(doc.Fpdf, day[0].Date, sum)
		for i, e := range day {
			doc.Keep(rows[i])
			entryRow(doc.Fpdf, tr, e, rows[i])
		}
	}
	return doc.Bytes()
}

// WorkLogName is the work log's file name: "10_2026_Rechnung_WorkLog.pdf", or with the number of the
// invoice it goes with, "10_2026_Rechnung_0202_WorkLog.pdf".
func WorkLogName(from time.Time, invoice string) string {
	if invoice == "" {
		return from.Format("01_2006") + "_Rechnung_WorkLog.pdf"
	}
	return from.Format("01_2006") + "_Rechnung_" + invoice + "_WorkLog.pdf"
}

// logHead is the report's title, the company's logo or name, and the summary of the period: its
// totals, and the clients, projects, tasks and people in it.
func logHead(doc *pdfdoc.Doc, tr func(string) string, company, logo string, from, to time.Time, total, open float64, entries []Entry) error {
	pdf := doc.Fpdf
	pdf.SetTextColor(20, 20, 20)
	pdf.SetFont("Helvetica", "B", 20)
	// The month in the title, as the runs name it: "Zeitnachweis 11/2026".
	pdf.CellFormat(110, 10, fmt.Sprintf("Zeitnachweis %d/%d", from.Month(), from.Year()), "", 0, "L", false, 0, "")
	bottom := logMargin + 12
	if logo != "" {
		data, kind, err := logoForPDF(logo)
		if err != nil {
			return fmt.Errorf("the logo %s: %w", logo, err)
		}
		options := fpdf.ImageOptions{ImageType: kind}
		info := pdf.RegisterImageOptionsReader("logo", options, bytes.NewReader(data))
		if err := pdf.Error(); err != nil {
			return fmt.Errorf("the logo %s: %w", logo, err)
		}
		w, h := logoWidth, logoWidth*info.Height()/info.Width()
		if h > logoMaxHeight {
			w, h = logoMaxHeight*info.Width()/info.Height(), logoMaxHeight
		}
		pdf.ImageOptions("logo", 210-logoRight-w, logoTop, w, h, false, options, 0, "")
		bottom = max(bottom, logoTop+h)
	} else {
		pdf.SetFont("Helvetica", "", 16)
		pdf.MultiCell(86, 7, tr(company), "", "R", false)
		bottom = max(bottom, pdf.GetY())
	}
	y := bottom + 4
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(logMargin, y, 210-logMargin, y)
	top := y + 6

	// Left: the period and its hours.
	pdf.SetXY(logMargin, top)
	label(pdf, "Zeitraum", 22)
	bold(pdf, tr(from.Format("02.01.2006")+" – "+to.Format("02.01.2006")), 70, 9)
	pdf.SetX(logMargin)
	label(pdf, "Gesamt", 22)
	bold(pdf, hours(total)+" Stunden", 70, 9)
	pdf.SetX(logMargin + 22)
	bold(pdf, hours(open)+" Stunden abrechenbar, nicht abgerechnet", 70, 8)
	left := pdf.GetY()

	// Right: what the report covers.
	pdf.SetXY(110, top)
	for _, part := range []struct {
		one, many string
		values    []string
	}{
		{"Kunde", "Kunden", distinct(entries, func(e Entry) string { return e.Client })},
		{"Projekt", "Projekte", distinct(entries, projectName)},
		{"Tätigkeit", "Tätigkeiten", distinct(entries, func(e Entry) string { return e.Task })},
		{"Mitarbeiter", "Mitarbeiter", distinct(entries, func(e Entry) string { return e.Person })},
	} {
		title := part.one
		if len(part.values) > 1 {
			title = part.many
		}
		pdf.SetX(110)
		label(pdf, tr(title), 22)
		bold(pdf, tr(strings.Join(part.values, ", ")), 66, 9)
	}
	pdf.SetY(max(left, pdf.GetY()) + 8)
	return nil
}

func label(pdf *fpdf.Fpdf, text string, width float64) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(width, 5, text, "", 0, "L", false, 0, "")
}

func bold(pdf *fpdf.Fpdf, text string, width, size float64) {
	pdf.SetFont("Helvetica", "B", size)
	pdf.SetTextColor(20, 20, 20)
	pdf.MultiCell(width, 5, text, "", "L", false)
}

// tableHead is the row of column titles, on every page.
func tableHead(pdf *fpdf.Fpdf, tr func(string) string) {
	pdf.SetFillColor(240, 240, 240)
	pdf.SetDrawColor(220, 220, 220)
	pdf.SetFont("Helvetica", "", logFont)
	pdf.SetTextColor(60, 60, 60)
	pdf.SetX(logMargin)
	for i, c := range logColumns {
		align := "L"
		if i == len(logColumns)-1 {
			align = "R"
		}
		pdf.CellFormat(c.width, logHeadRow, " "+tr(c.title)+" ", "TB", 0, align, true, 0, "")
	}
	pdf.Ln(logHeadRow)
}

// dayRow is a day's date and its hours, above its entries.
func dayRow(pdf *fpdf.Fpdf, date time.Time, sum float64) {
	pdf.SetFillColor(240, 240, 240)
	pdf.SetDrawColor(220, 220, 220)
	pdf.SetFont("Helvetica", "", logFont)
	pdf.SetTextColor(40, 40, 40)
	pdf.SetX(logMargin)
	last := logColumns[len(logColumns)-1].width
	pdf.CellFormat(186-last, logDayRow, " "+date.Format("02.01.2006"), "B", 0, "L", true, 0, "")
	pdf.CellFormat(last, logDayRow, hours(sum)+" ", "B", 1, "R", true, 0, "")
}

// cells is each column's lines of an entry, wrapped to its width; the project's with the note
// beneath it, after an empty line's worth of space.
func cells(pdf *fpdf.Fpdf, tr func(string) string, e Entry) [][]string {
	pdf.SetFont("Helvetica", "", logFont)
	wrap := func(text string, width float64) []string { return wrapText(pdf, tr, text, width-2.4) }
	project := wrap(projectName(e), logColumns[1].width)
	if note := noteOf(e); note != "" {
		project = append(append(project, ""), wrap(note, logColumns[1].width)...)
	}
	return [][]string{
		wrap(e.Client, logColumns[0].width),
		project,
		wrap(e.Task, logColumns[2].width),
		wrap(e.Person, logColumns[3].width),
		{hours(e.Hours)},
	}
}

// wrapText breaks text into lines as wide as width at most, at spaces — a word wider than
// that, at any letter — and hands them back in the PDF's encoding. fpdf's SplitText reads
// UTF-8, which the encoded text no longer is.
func wrapText(pdf *fpdf.Fpdf, tr func(string) string, text string, width float64) []string {
	fits := func(s string) bool { return pdf.GetStringWidth(tr(s)) <= width }
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "" && fits(word):
			line = word
		case line != "" && fits(line+" "+word):
			line += " " + word
		default:
			if line != "" {
				lines = append(lines, tr(line))
				line = ""
			}
			for !fits(word) {
				runes := []rune(word)
				n := len(runes) - 1
				for n > 1 && !fits(string(runes[:n])) {
					n--
				}
				lines = append(lines, tr(string(runes[:n])))
				word = string(runes[n:])
			}
			line = word
		}
	}
	if line != "" || len(lines) == 0 {
		lines = append(lines, tr(line))
	}
	return lines
}

// entryHeight is how tall an entry's row is: its tallest cell.
func entryHeight(pdf *fpdf.Fpdf, tr func(string) string, e Entry) float64 {
	lines := 1
	for _, cell := range cells(pdf, tr, e) {
		lines = max(lines, len(cell))
	}
	return float64(lines)*logLine + 2*logPad
}

// entryRow is one time entry, its cells side by side, a rule beneath.
func entryRow(pdf *fpdf.Fpdf, tr func(string) string, e Entry, height float64) {
	top := pdf.GetY()
	x := logMargin
	pdf.SetTextColor(40, 40, 40)
	for i, cell := range cells(pdf, tr, e) {
		align := "L"
		if i == len(logColumns)-1 {
			align = "R"
		}
		for j, line := range cell {
			// The note gap is half a line, not a whole one.
			y := top + logPad + float64(j)*logLine
			if i == 1 && j > 0 && cell[j-1] == "" {
				y -= logLine / 2
			}
			if line == "" {
				continue
			}
			pdf.SetXY(x, y)
			pdf.CellFormat(logColumns[i].width, logLine, " "+line+" ", "", 0, align, false, 0, "")
		}
		x += logColumns[i].width
	}
	pdf.SetDrawColor(225, 225, 225)
	pdf.Line(logMargin, top+height, 210-logMargin, top+height)
	pdf.SetXY(logMargin, top+height)
}

// byDay groups the entries, sorted already, by their date.
func byDay(entries []Entry) [][]Entry {
	var days [][]Entry
	for _, e := range entries {
		if n := len(days); n > 0 && days[n-1][0].Date.Equal(e.Date) {
			days[n-1] = append(days[n-1], e)
		} else {
			days = append(days, []Entry{e})
		}
	}
	return days
}

// projectName is "[0043] EBIL Pro", as Harvest names a project with a code.
func projectName(e Entry) string {
	if e.Code == "" {
		return e.Project
	}
	return "[" + e.Code + "] " + e.Project
}

// noteOf is "[7:00 - 9:00] Feedback": the note, after the times when there are some.
func noteOf(e Entry) string {
	if e.Started == "" || e.Ended == "" {
		return e.Notes
	}
	return strings.TrimSpace("[" + e.Started + " - " + e.Ended + "] " + e.Notes)
}

// distinct is each value once, sorted.
func distinct(entries []Entry, of func(Entry) string) []string {
	seen := map[string]bool{}
	var values []string
	for _, e := range entries {
		if v := of(e); v != "" && !seen[v] {
			seen[v] = true
			values = append(values, v)
		}
	}
	sort.Strings(values)
	return values
}

// hours is 105.5 as "105,50", as Harvest's German reports write it.
func hours(h float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", h), ".", ",", 1)
}
