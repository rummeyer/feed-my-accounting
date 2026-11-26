package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"feed-my-accounting/email"
)

// The receipts mailed go to Paperless-ngx too, if switched on: by its API, with the sender as the
// correspondent, the receipt's date as created, and the tags set — Firma, say, for a workflow
// there to file it by. Paperless may be on a NAS that is off most of the time: each receipt waits
// in a folder beside the configuration until Paperless takes it, tried after each sending and
// every ten minutes. Paperless refuses a file it has, so one taken twice is still one.

// paperlessLog is the upload's log, "[paperless] " first, as the modules' are.
var paperlessLog = log.New(os.Stderr, "[paperless] ", log.LstdFlags)

// paperlessQueue is the folder beside the configuration where the receipts wait for Paperless.
const paperlessQueue = "feed-my-accounting-paperless"

// paperlessMu keeps one upload of the waiting receipts at a time: a run's and the schedule's.
var paperlessMu sync.Mutex

// paperlessWaiting is a receipt waiting, beside its PDF: what Paperless is told of it. Title and
// Created, where set, are taken as they are — a customer's invoice's — else made from the name.
// An invoice gets the tags set for invoices, a receipt those for receipts.
type paperlessWaiting struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Title   string `json:"title,omitempty"`
	Created string `json:"created,omitempty"` // 2026-11-30
	Invoice bool   `json:"invoice,omitempty"`
}

// paperlessUploaded is the record of the files Paperless took, beside the queue: each one's
// SHA-256 a line. One in it is not queued again — Paperless would refuse it, a failed task there.
const paperlessUploaded = "uploaded"

// queueForPaperless puts receipts in the queue, each under its file's SHA-256: the same twice is
// one.
func queueForPaperless(dir, source string, files []email.Attachment) error {
	for _, f := range files {
		if err := queueOneForPaperless(dir, paperlessWaiting{Name: filepath.Base(f.Filename), Source: source}, f.Data); err != nil {
			return err
		}
	}
	return nil
}

// queueOneForPaperless puts one file in the queue, as w says; not one Paperless took before.
func queueOneForPaperless(dir string, w paperlessWaiting, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	queue := filepath.Join(dir, paperlessQueue)
	if err := os.MkdirAll(queue, 0o700); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	if uploadedBefore(queue, id) {
		return nil
	}
	meta, err := json.Marshal(w)
	if err != nil {
		return err
	}
	// The PDF first: a meta file is only ever beside a whole one.
	if err := os.WriteFile(filepath.Join(queue, id+".pdf"), data, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(queue, id+".json"), meta, 0o600)
}

// paperlessUnreachable is Paperless not answering: the NAS off, as a rule. The receipts wait.
var paperlessUnreachable = errors.New("Paperless not reachable")

// flushPaperless uploads the receipts waiting, oldest first, each taken out of the queue once
// Paperless has it. Paperless not answering stops it quietly: they wait for the next try.
func flushPaperless(cfg *Config) {
	pc := cfg.Paperless
	if !pc.Enabled || cfg.dir == "" {
		return
	}
	paperlessMu.Lock()
	defer paperlessMu.Unlock()
	queue := filepath.Join(cfg.dir, paperlessQueue)
	metas, _ := filepath.Glob(filepath.Join(queue, "*.json"))
	if len(metas) == 0 {
		return
	}
	if pc.URL == "" || pc.Token == "" {
		paperlessLog.Printf("%d receipt(s) waiting, but no Paperless address or API token set", len(metas))
		return
	}
	sort.Slice(metas, func(i, j int) bool { return modTime(metas[i]).Before(modTime(metas[j])) })
	p := &paperless{cfg: pc, client: paperlessClient(), ids: map[string]int{}}
	done := 0
	for _, meta := range metas {
		err := p.uploadWaiting(meta)
		if errors.Is(err, paperlessUnreachable) {
			if done > 0 {
				paperlessLog.Printf("%d receipt(s) uploaded", done)
			}
			paperlessLog.Printf("%s: %d receipt(s) wait for it", err, len(metas)-done)
			return
		}
		if err != nil {
			paperlessLog.Printf("%s: %v", filepath.Base(meta), err)
			continue
		}
		done++
	}
	paperlessLog.Printf("%d receipt(s) uploaded", done)
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// paperlessClient gives up soon on an address not there: the NAS off.
func paperlessClient() *http.Client {
	return &http.Client{
		Timeout:   2 * time.Minute,
		Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext},
	}
}

// paperless is one upload of the waiting receipts: the tags' and correspondents' ids, looked up
// once each.
type paperless struct {
	cfg    PaperlessConfig
	client *http.Client
	ids    map[string]int // "tags/Firma", "correspondents/Amazon"
}

// uploadWaiting uploads one receipt of the queue, and takes it out.
func (p *paperless) uploadWaiting(meta string) error {
	raw, err := os.ReadFile(meta)
	if err != nil {
		return err
	}
	var w paperlessWaiting
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	pdf := strings.TrimSuffix(meta, ".json") + ".pdf"
	data, err := os.ReadFile(pdf)
	if err != nil {
		os.Remove(meta)
		return err
	}
	if err := p.upload(w, data); err != nil {
		return err
	}
	os.Remove(meta)
	os.Remove(pdf)
	noteUploaded(filepath.Dir(meta), strings.TrimSuffix(filepath.Base(meta), ".json"))
	paperlessLog.Printf("%s uploaded (%s)", w.Name, w.Source)
	return nil
}

// uploadedBefore tells whether Paperless took the file of this SHA-256 before.
func uploadedBefore(queue, id string) bool {
	raw, err := os.ReadFile(filepath.Join(queue, paperlessUploaded))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == id {
			return true
		}
	}
	return false
}

// noteUploaded adds a file's SHA-256 to the record of those Paperless took.
func noteUploaded(queue, id string) {
	f, err := os.OpenFile(filepath.Join(queue, paperlessUploaded), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		paperlessLog.Printf("WARNING: %v", err)
		return
	}
	defer f.Close()
	fmt.Fprintln(f, id)
}

// upload hands one receipt to Paperless; it takes it in in the background.
func (p *paperless) upload(w paperlessWaiting, data []byte) error {
	title, created, tags := w.Title, w.Created, p.cfg.Tags
	if w.Invoice {
		tags = p.cfg.InvoiceTags
	}
	if title == "" {
		title = paperlessTitle(w.Name)
	}
	if created == "" {
		created = receiptDate(w.Name)
	}
	fields := map[string][]string{"title": {title}}
	if created != "" {
		fields["created"] = []string{created}
	}
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag == "" {
			continue
		}
		id, err := p.id("tags", tag)
		if err != nil {
			return err
		}
		fields["tags"] = append(fields["tags"], fmt.Sprint(id))
	}
	// Travel expenses are the company's own: no one sent them.
	if w.Source != "" && w.Source != jobSources["travel-expense"] {
		id, err := p.id("correspondents", w.Source)
		if err != nil {
			return err
		}
		fields["correspondent"] = []string{fmt.Sprint(id)}
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, values := range fields {
		for _, v := range values {
			form.WriteField(name, v)
		}
	}
	part, err := form.CreateFormFile("document", w.Name)
	if err != nil {
		return err
	}
	part.Write(data)
	form.Close()
	return p.call(http.MethodPost, "/api/documents/post_document/", form.FormDataContentType(), &body, nil)
}

// id is a tag's or correspondent's id by its name, made if Paperless has none of that name.
func (p *paperless) id(kind, name string) (int, error) {
	key := kind + "/" + strings.ToLower(name)
	if id, ok := p.ids[key]; ok {
		return id, nil
	}
	var found struct {
		Results []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := p.call(http.MethodGet, "/api/"+kind+"/?name__iexact="+url.QueryEscape(name), "", nil, &found); err != nil {
		return 0, err
	}
	for _, r := range found.Results {
		if strings.EqualFold(r.Name, name) {
			p.ids[key] = r.ID
			return r.ID, nil
		}
	}
	var made struct {
		ID int `json:"id"`
	}
	body, _ := json.Marshal(map[string]any{"name": name})
	if err := p.call(http.MethodPost, "/api/"+kind+"/", "application/json", bytes.NewReader(body), &made); err != nil {
		return 0, fmt.Errorf("making %s %q: %w", strings.TrimSuffix(kind, "s"), name, err)
	}
	paperlessLog.Printf("%s %q made in Paperless", strings.TrimSuffix(kind, "s"), name)
	p.ids[key] = made.ID
	return made.ID, nil
}

// call asks Paperless's API, the answer read into out if given.
func (p *paperless) call(method, path, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequest(method, strings.TrimRight(p.cfg.URL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+p.cfg.Token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) || errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no route to host") {
			return paperlessUnreachable
		}
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout:
		// Starting up still, or a proxy before a NAS asleep.
		return paperlessUnreachable
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%s: the API token is not accepted", resp.Status)
	case resp.StatusCode >= 300:
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// receiptName is a receipt's name as made here: 11_2026_Rechnung_Amazon_17112026.pdf.
var receiptName = regexp.MustCompile(`^\d{2}_\d{4}_(.+?)(?:_(\d{2})(\d{2})(\d{4}))?(?:_\d+)?\.pdf$`)

// paperlessTitle is a receipt's title in Paperless: "Rechnung Amazon 17.11.2026", or its name
// without .pdf where not made here.
func paperlessTitle(name string) string {
	m := receiptName.FindStringSubmatch(name)
	if m == nil {
		return strings.TrimSuffix(name, filepath.Ext(name))
	}
	title := strings.ReplaceAll(m[1], "_", " ")
	if m[2] != "" {
		title += " " + m[2] + "." + m[3] + "." + m[4]
	}
	return title
}

// receiptDate is the day a receipt's name ends in, 2026-11-17, if it does and it is one.
func receiptDate(name string) string {
	m := receiptName.FindStringSubmatch(name)
	if m == nil || m[2] == "" {
		return ""
	}
	day, err := time.Parse("02012006", m[2]+m[3]+m[4])
	if err != nil {
		return ""
	}
	return day.Format("2006-01-02")
}
