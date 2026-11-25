package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"feed-my-accounting/email"
)

func TestPaperlessTitleAndDate(t *testing.T) {
	for _, c := range []struct{ name, title, date string }{
		{"11_2026_Rechnung_Amazon_17112026.pdf", "Rechnung Amazon 17.11.2026", "2026-11-17"},
		{"11_2026_Rechnung_Strato_03112026_2.pdf", "Rechnung Strato 03.11.2026", "2026-11-03"},
		{"11_2026_Rechnung_Vodafone_Kabel.pdf", "Rechnung Vodafone Kabel", ""},
		{"11_2026_Rechnung_Apple_MNMTYFDFG4.pdf", "Rechnung Apple MNMTYFDFG4", ""},
		{"11_2026_Reisekosten_Verpflegungsmehraufwand.pdf", "Reisekosten Verpflegungsmehraufwand", ""},
		{"Deine Rechnung.pdf", "Deine Rechnung", ""},
		{"11_2026_Rechnung_X_31022026.pdf", "Rechnung X 31.02.2026", ""}, // no such day
	} {
		if got := paperlessTitle(c.name); got != c.title {
			t.Errorf("paperlessTitle(%q) = %q, want %q", c.name, got, c.title)
		}
		if got := receiptDate(c.name); got != c.date {
			t.Errorf("receiptDate(%q) = %q, want %q", c.name, got, c.date)
		}
	}
}

// fakePaperless is Paperless's API as far as the upload uses it.
type fakePaperless struct {
	mu        sync.Mutex
	tags      map[string]int
	corrs     map[string]int
	documents []map[string][]string
	auth      []string
}

func (f *fakePaperless) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	kinds := map[string]map[string]int{"/api/tags/": f.tags, "/api/correspondents/": f.corrs}
	if have, ok := kinds[r.URL.Path]; ok {
		if r.Method == http.MethodGet {
			results := []map[string]any{}
			for name, id := range have {
				if strings.EqualFold(name, r.URL.Query().Get("name__iexact")) {
					results = append(results, map[string]any{"id": id, "name": name})
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"results": results})
			return
		}
		var in struct{ Name string }
		json.NewDecoder(r.Body).Decode(&in)
		have[in.Name] = 100 + len(have)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": have[in.Name]})
		return
	}
	if r.URL.Path == "/api/documents/post_document/" && r.Method == http.MethodPost {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		doc := map[string][]string{}
		for k, v := range r.MultipartForm.Value {
			doc[k] = v
		}
		doc["file"] = []string{r.MultipartForm.File["document"][0].Filename}
		f.documents = append(f.documents, doc)
		json.NewEncoder(w).Encode("task-id")
		return
	}
	http.NotFound(w, r)
}

func waiting(t *testing.T, dir string) int {
	t.Helper()
	metas, _ := filepath.Glob(filepath.Join(dir, paperlessQueue, "*.json"))
	pdfs, _ := filepath.Glob(filepath.Join(dir, paperlessQueue, "*.pdf"))
	if len(metas) != len(pdfs) {
		t.Fatalf("%d meta files beside %d PDFs", len(metas), len(pdfs))
	}
	return len(metas)
}

func TestPaperlessUploads(t *testing.T) {
	fake := &fakePaperless{tags: map[string]int{"Firma": 1}, corrs: map[string]int{"amazon": 7}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	dir := t.TempDir()
	cfg := &Config{dir: dir, Paperless: PaperlessConfig{Enabled: true, URL: srv.URL + "/", Token: "secret", Tags: []string{"firma", " "}}}

	files := []email.Attachment{
		{Filename: "11_2026_Rechnung_Amazon_17112026.pdf", Data: []byte("%PDF a")},
		{Filename: "11_2026_Rechnung_Amazon_18112026.pdf", Data: []byte("%PDF b")},
		{Filename: "again.pdf", Data: []byte("%PDF a")}, // the same file: one
	}
	if err := queueForPaperless(dir, "Amazon", files); err != nil {
		t.Fatal(err)
	}
	if err := queueForPaperless(dir, "Strato", []email.Attachment{{Filename: "11_2026_Rechnung_Strato_03112026.pdf", Data: []byte("%PDF c")}}); err != nil {
		t.Fatal(err)
	}
	if err := queueForPaperless(dir, "Reisekosten", []email.Attachment{{Filename: "11_2026_Reisekosten_Verpflegungsmehraufwand.pdf", Data: []byte("%PDF d")}}); err != nil {
		t.Fatal(err)
	}
	if n := waiting(t, dir); n != 4 {
		t.Fatalf("%d waiting, want 4", n)
	}
	flushPaperless(cfg)
	if n := waiting(t, dir); n != 0 {
		t.Fatalf("%d still waiting", n)
	}
	if len(fake.documents) != 4 {
		t.Fatalf("%d documents uploaded, want 4", len(fake.documents))
	}
	byFile := map[string]map[string][]string{}
	for _, d := range fake.documents {
		byFile[d["file"][0]] = d
	}
	amazon := byFile["11_2026_Rechnung_Amazon_18112026.pdf"]
	if amazon == nil || amazon["title"][0] != "Rechnung Amazon 18.11.2026" || amazon["created"][0] != "2026-11-18" ||
		amazon["correspondent"][0] != "7" || len(amazon["tags"]) != 1 || amazon["tags"][0] != "1" {
		t.Errorf("Amazon's: %v", amazon)
	}
	strato := byFile["11_2026_Rechnung_Strato_03112026.pdf"]
	if id, ok := fake.corrs["Strato"]; !ok || strato == nil || strato["correspondent"][0] != "101" || id != 101 {
		t.Errorf("Strato made a correspondent: %v, %v", fake.corrs, strato)
	}
	if travel := byFile["11_2026_Reisekosten_Verpflegungsmehraufwand.pdf"]; travel == nil || travel["correspondent"] != nil {
		t.Errorf("travel expenses with a correspondent: %v", travel)
	}
	for _, a := range fake.auth {
		if a != "Token secret" {
			t.Fatalf("Authorization %q", a)
		}
	}
}

func TestPaperlessWaitsWhileOff(t *testing.T) {
	// An address nothing listens on: the NAS off.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	off := "http://" + l.Addr().String()
	l.Close()
	dir := t.TempDir()
	cfg := &Config{dir: dir, Paperless: PaperlessConfig{Enabled: true, URL: off, Token: "secret", Tags: []string{"Firma"}}}
	mail := belegeMail(cfg)
	if mail.Archive == nil {
		t.Fatal("no hook for Paperless")
	}
	if err := mail.Archive("Amazon", []email.Attachment{{Filename: "11_2026_Rechnung_Amazon_17112026.pdf", Data: []byte("%PDF a")}}); err != nil {
		t.Fatalf("Paperless off failed the run: %v", err)
	}
	if n := waiting(t, dir); n != 1 {
		t.Fatalf("%d waiting, want 1", n)
	}

	// Back on: it goes up.
	fake := &fakePaperless{tags: map[string]int{"Firma": 1}, corrs: map[string]int{}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg.Paperless.URL = srv.URL
	flushPaperless(cfg)
	if n := waiting(t, dir); n != 0 || len(fake.documents) != 1 {
		t.Fatalf("%d waiting, %d uploaded", n, len(fake.documents))
	}
}

func TestPaperlessTokenRefusedKeepsWaiting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"Invalid token."}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	dir := t.TempDir()
	cfg := &Config{dir: dir, Paperless: PaperlessConfig{Enabled: true, URL: srv.URL, Token: "wrong", Tags: []string{"Firma"}}}
	if err := queueForPaperless(dir, "Amazon", []email.Attachment{{Filename: "a.pdf", Data: []byte("%PDF a")}}); err != nil {
		t.Fatal(err)
	}
	flushPaperless(cfg)
	if n := waiting(t, dir); n != 1 {
		t.Fatalf("%d waiting, want 1", n)
	}
}

func TestPaperlessOffNoQueue(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{dir: dir}
	if mail := belegeMail(cfg); mail.Archive != nil {
		t.Fatal("a hook with neither rclone nor Paperless on")
	}
	if _, err := os.Stat(filepath.Join(dir, paperlessQueue)); !os.IsNotExist(err) {
		t.Fatal("queue made")
	}
}
