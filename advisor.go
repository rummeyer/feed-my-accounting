package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"feed-my-accounting/email"
)

// Once a month is booked, the tax adviser is told by mail — the receipts of the month zipped
// beside it, if asked, from the storage they were copied to.

const (
	defaultAdvisorSubject = "Belege {MM}/{YYYY}"
	defaultAdvisorText    = "Hallo,\n\nalle Rechnungen & Belege sind für {MM}/{YYYY} in sevDesk gebucht.\n\nViele Grüße"
)

// advisorMail is the mail to the tax adviser for a month, as set, its placeholders filled in.
func advisorMail(cfg *Config, month time.Time) email.Mail {
	a := cfg.Advisor
	fill := strings.NewReplacer("{MM}", month.Format("01"), "{YYYY}", month.Format("2006"), "{MONAT}", monthNames[month.Month()-1]).Replace
	subject, text := a.Subject, a.Text
	if strings.TrimSpace(subject) == "" {
		subject = defaultAdvisorSubject
	}
	if strings.TrimSpace(text) == "" {
		text = defaultAdvisorText
	}
	return email.Mail{To: strings.TrimSpace(a.To), Bcc: strings.TrimSpace(a.Bcc), Subject: fill(subject), Text: fill(text)}
}

// monthReceipts are the receipts of a month in the storage: in the month's folder, or those
// whose name begins with the month, 11_2026_, in a folder for all.
func monthReceipts(ac ArchiveConfig, month time.Time) ([]email.Attachment, error) {
	if !ac.Enabled || ac.Remote == "" {
		return nil, errors.New("no storage set up to take the receipts from: Einstellungen › Ablage (rclone)")
	}
	prefix := month.Format("01_2006_")
	folder := archiveFolder(ac.Folder, prefix, month)
	include := prefix + "*"
	if strings.Contains(ac.Folder, "{MM}") {
		include = "*.pdf" // the month's folder: all of it
	}
	dir, err := os.MkdirTemp("", "feed-my-accounting-zip-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	archiveLog.Printf("Fetching the receipts of %s from \"%s://%s\"...", month.Format("01/2006"), ac.Remote, folder)
	out, err := exec.CommandContext(ctx, "rclone", "copy", "--include", include, ac.Remote+":"+folder, dir).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("rclone copy from %s:%s: %v: %s (if it wants a new login: rclone config reconnect %s:)", ac.Remote, folder, err, lastLine(string(out)), ac.Remote)
	}
	names, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []email.Attachment
	for _, n := range names {
		if n.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, n.Name()))
		if err != nil {
			return nil, err
		}
		files = append(files, email.Attachment{Filename: n.Name(), Data: data})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })
	archiveLog.Printf("%d receipt(s) of %s", len(files), month.Format("01/2006"))
	return files, nil
}

// zipped is the files in one zip: Belege_11_2026.zip.
func zipped(files []email.Attachment, month time.Time) (email.Attachment, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := z.CreateHeader(&zip.FileHeader{Name: f.Filename, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return email.Attachment{}, err
		}
		if _, err := w.Write(f.Data); err != nil {
			return email.Attachment{}, err
		}
	}
	if err := z.Close(); err != nil {
		return email.Attachment{}, err
	}
	return email.Attachment{Filename: "Belege_" + month.Format("01_2006") + ".zip", Data: buf.Bytes()}, nil
}

// informAdvisor tells the tax adviser the month is booked, the receipts zipped beside it if
// asked; the zip offered for download too.
func informAdvisor(cfg *Config, month time.Time, withZip bool) ([]File, error) {
	m := advisorMail(cfg, month)
	if m.To == "" {
		return nil, errors.New("no address for the tax adviser: Einstellungen › Steuerberater")
	}
	var files []File
	if withZip {
		receipts, err := monthReceipts(cfg.Archive, month)
		if err != nil {
			return nil, err
		}
		if len(receipts) == 0 {
			return nil, fmt.Errorf("no receipts of %s in the storage: nothing to zip", month.Format("01/2006"))
		}
		z, err := zipped(receipts, month)
		if err != nil {
			return nil, err
		}
		archiveLog.Printf("%s: %d receipt(s), %d KB", z.Filename, len(receipts), (len(z.Data)+1023)/1024)
		m.Attachments = []email.Attachment{z}
		files = append(files, File{Name: z.Filename, Size: len(z.Data), data: z.Data})
	}
	mc := cfg.Mail
	if from := strings.TrimSpace(cfg.Advisor.From); from != "" {
		mc.From = from
	}
	if err := email.SendMail(mc, m); err != nil {
		return files, err
	}
	if cfg.dir != "" {
		if err := ledgerOf(cfg).Inform(month); err != nil {
			archiveLog.Printf("WARNING: sent, but not noted: %v", err)
		}
	}
	return files, nil
}

// advisorSend tells the tax adviser a month is booked: {month: "2026-11", zip: true}.
func (s *server) advisorSend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Month string `json:"month"`
		Zip   bool   `json:"zip"`
	}
	if !read(w, r, &in) {
		return
	}
	month, err := time.ParseInLocation("2006-01", in.Month, time.Local)
	if err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("month %q: want YYYY-MM", in.Month))
		return
	}
	cfg, err := s.config()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(cfg.Advisor.To) == "" {
		fail(w, http.StatusBadRequest, errors.New(say(r, "keine Adresse für den Steuerberater: unter Einstellungen › Steuerberater eintragen", "no address for the tax adviser: enter it under Settings › Tax adviser")))
		return
	}
	s.start(w, "advisor", fmt.Sprintf("Steuerberater informieren %s", month.Format("01/2006")), func() ([]File, error) {
		return informAdvisor(cfg, month, in.Zip)
	})
}
