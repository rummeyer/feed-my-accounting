package main

import (
	"bytes"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// upload is a PUT of data as the logo, as the settings page sends it.
func upload(t *testing.T, s *server, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("logo", "logo.png")
	part.Write(data)
	form.Close()
	r := httptest.NewRequest(http.MethodPut, "/api/logo", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	s.putLogo(w, r)
	return w
}

func TestLogo(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(config, []byte("mail:\n  to: a@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &server{configPath: config}
	logo := filepath.Join(dir, logoBase+".png")

	if w := upload(t, s, []byte("not an image")); w.Code != http.StatusBadRequest {
		t.Errorf("not an image: %d", w.Code)
	}
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 40, 10)))
	if w := upload(t, s, img.Bytes()); w.Code != http.StatusOK {
		t.Fatalf("a PNG: %d %s", w.Code, w.Body)
	}
	if !fileExists(logo) {
		t.Fatal("the logo is not kept beside the configuration")
	}
	w := httptest.NewRecorder()
	s.getLogo(w, httptest.NewRequest(http.MethodGet, "/api/logo", nil))
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), img.Bytes()) {
		t.Errorf("getting it: %d, %d bytes", w.Code, w.Body.Len())
	}
	cfg, _ := readConfig(config)
	if cfg.logoPath() != logo {
		t.Errorf("logoPath %q", cfg.logoPath())
	}

	// Taken away, it is kept as its .bak, and there is none.
	w = httptest.NewRecorder()
	s.deleteLogo(w, httptest.NewRequest(http.MethodDelete, "/api/logo", nil))
	if w.Code != http.StatusOK || fileExists(logo) || !fileExists(filepath.Join(dir, logoBase+".bak.png")) {
		t.Errorf("deleting: %d, logo there %v", w.Code, fileExists(logo))
	}
	w = httptest.NewRecorder()
	s.getLogo(w, httptest.NewRequest(http.MethodGet, "/api/logo", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("without a logo: %d", w.Code)
	}
}
