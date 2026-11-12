package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	harvest "feed-my-accounting/harvest-invoice"
)

// testServer is the web interface on a configuration of two employees, the first with the
// password "geheim123" when withLogin.
func testServer(t *testing.T, withLogin bool, homeOnly bool) *server {
	t.Helper()
	dir := t.TempDir()
	cfg := "staff:\n    names: [Max Mustermann, Erika Musterfrau]\n    current: Max Mustermann\n"
	if withLogin {
		hash, err := hashPassword("geheim123")
		if err != nil {
			t.Fatal(err)
		}
		cfg += "    logins:\n        Max Mustermann: " + hash + "\n"
	}
	if !homeOnly {
		cfg += "web:\n    homeOnly: false\n"
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return &server{
		configPath: path,
		jobs:       NewJobs(filepath.Join(dir, "jobs.json")),
		uploads:    map[string]*harvest.ReportData{},
		sessions:   newSessions(filepath.Join(dir, "sessions.json")),
	}
}

// call sends a request from addr, through the server's gate.
func call(s *server, method, path, addr, body string, mod ...func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = addr
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	for _, m := range mod {
		m(r)
	}
	w := httptest.NewRecorder()
	s.handler().ServeHTTP(w, r)
	return w
}

const home, outside = "192.168.178.20:50000", "84.150.10.2:50000"

func TestWithoutLoginsOnlyHome(t *testing.T) {
	s := testServer(t, false, false)
	// No password anywhere: open at home, closed outside even with homeOnly off.
	if w := call(s, "GET", "/api/overview", home, ""); w.Code != http.StatusOK {
		t.Errorf("home: %d", w.Code)
	}
	if w := call(s, "GET", "/api/overview", outside, ""); w.Code != http.StatusForbidden {
		t.Errorf("outside without any password: %d", w.Code)
	}
}

func TestLogin(t *testing.T) {
	s := testServer(t, true, true)
	if w := call(s, "GET", "/api/overview", home, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("API without a session: %d", w.Code)
	}
	if w := call(s, "GET", "/", home, ""); w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Errorf("page without a session: %d %s", w.Code, w.Header().Get("Location"))
	}
	for _, path := range []string{"/login", "/style.css", "/login.js", "/theme.js", "/logo.webp", "/api/session"} {
		if w := call(s, "GET", path, home, ""); w.Code != http.StatusOK {
			t.Errorf("%s before logging in: %d", path, w.Code)
		}
	}
	// At home the name is enough, any employee's, a password or not.
	if w := call(s, "POST", "/api/login", home, `{"name":"Erika Musterfrau"}`); w.Code != http.StatusOK {
		t.Errorf("at home, by name: %d %s", w.Code, w.Body)
	}
	if w := call(s, "POST", "/api/login", home, `{"name":"Nobody"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("at home, a name that is no employee's: %d", w.Code)
	}
	if w := call(s, "GET", "/api/session", home, ""); !strings.Contains(w.Body.String(), `"password":false`) || !strings.Contains(w.Body.String(), "Erika Musterfrau") {
		t.Errorf("session at home: %s", w.Body)
	}
	w := call(s, "POST", "/api/login", home, `{"name":"Max Mustermann"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("right password: %d %s", w.Code, w.Body)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure {
		t.Errorf("cookie over plain HTTP at home: %+v", cookie)
	}
	with := func(r *http.Request) { r.AddCookie(cookie) }
	if w := call(s, "GET", "/api/overview", home, "", with); w.Code != http.StatusOK {
		t.Errorf("API with the session: %d", w.Code)
	}
	if w := call(s, "GET", "/api/session", home, "", with); !strings.Contains(w.Body.String(), `"user":"Max Mustermann"`) {
		t.Errorf("session: %s", w.Body)
	}
	// The sessions file holds no token, only its hash.
	data, _ := os.ReadFile(s.sessions.path)
	if bytes.Contains(data, []byte(cookie.Value)) {
		t.Error("the token in the sessions file")
	}
	call(s, "POST", "/api/logout", home, "", with)
	if w := call(s, "GET", "/api/overview", home, "", with); w.Code != http.StatusUnauthorized {
		t.Errorf("after logging out: %d", w.Code)
	}
}

func TestLoginFromOutside(t *testing.T) {
	s := testServer(t, true, false)
	other := "84.150.10.3:50000"
	if w := call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"falsch"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: %d", w.Code)
	}
	if w := call(s, "POST", "/api/login", other, `{"name":"Erika Musterfrau","password":"geheim123"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("an employee without a password: %d", w.Code)
	}
	if w := call(s, "POST", "/api/login", other, `{"name":"Max Mustermann"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("from outside without a password: %d", w.Code)
	}
	if w := call(s, "GET", "/api/session", other, ""); !strings.Contains(w.Body.String(), `"password":true`) || strings.Contains(w.Body.String(), "Erika") {
		t.Errorf("session from outside: only those with a password: %s", w.Body)
	}
	if w := call(s, "POST", "/api/login", other, `{"name":"Max Mustermann","password":"geheim123"}`); w.Code != http.StatusOK {
		t.Errorf("right password: %d", w.Code)
	}
	for i := 0; i < maxFailures; i++ {
		call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"falsch"}`)
	}
	if w := call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"geheim123"}`); w.Code != http.StatusTooManyRequests {
		t.Errorf("after %d failures, even the right password: %d", maxFailures, w.Code)
	}
	// Another address is not locked.
	if w := call(s, "POST", "/api/login", other, `{"name":"Max Mustermann","password":"geheim123"}`); w.Code != http.StatusOK {
		t.Errorf("another address: %d", w.Code)
	}
}

func TestBehindCaddy(t *testing.T) {
	viaCaddy := func(client, proto string) func(*http.Request) {
		return func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", client)
			r.Header.Set("X-Forwarded-Proto", proto)
		}
	}
	// Caddy on the Pi: the request comes from localhost, the client is the one Caddy names.
	s := testServer(t, true, true)
	if w := call(s, "GET", "/login", "127.0.0.1:40000", "", viaCaddy("84.150.10.2", "https")); w.Code != http.StatusForbidden {
		t.Errorf("from outside through Caddy with homeOnly: %d", w.Code)
	}
	s = testServer(t, true, false)
	if w := call(s, "GET", "/login", "127.0.0.1:40000", "", viaCaddy("84.150.10.2", "https")); w.Code != http.StatusOK {
		t.Errorf("from outside through Caddy, homeOnly off: %d", w.Code)
	}
	w := call(s, "POST", "/api/login", "127.0.0.1:40000", `{"name":"Max Mustermann","password":"geheim123"}`, viaCaddy("84.150.10.2", "https"))
	if c := w.Result().Cookies(); w.Code != http.StatusOK || len(c) == 0 || !c[0].Secure {
		t.Errorf("login through Caddy over HTTPS: %d, cookie %+v", w.Code, c)
	}
	// A client's own X-Forwarded-For counts for nothing: only the address Caddy adds, last.
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:40000"
	r.Header.Set("X-Forwarded-For", "192.168.178.5, 84.150.10.2")
	if fromHome(r) {
		t.Error("a forged home address in front of Caddy's")
	}
	// Caddy on another machine of the home network, as the router forwards to: the client counts,
	// not the proxy's home address — else the whole internet would be at home.
	r = httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.178.30:40000"
	r.Header.Set("X-Forwarded-For", "84.150.10.2")
	if fromHome(r) {
		t.Error("through a Caddy elsewhere in the home network, the client taken for home")
	}
	if w := call(s, "POST", "/api/login", "192.168.178.30:40000", `{"name":"Max Mustermann"}`, viaCaddy("84.150.10.2", "https")); w.Code != http.StatusUnauthorized {
		t.Errorf("through another machine's Caddy without a password: %d", w.Code)
	}
	// Without a proxy in front, the header is not believed at all.
	r = httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = outside
	r.Header.Set("X-Forwarded-For", "192.168.178.5")
	if fromHome(r) {
		t.Error("X-Forwarded-For believed from outside")
	}
}

func TestChangesOnlyFromThisPage(t *testing.T) {
	s := testServer(t, false, true)
	evil := func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }
	if w := call(s, "POST", "/api/logout", home, "", evil); w.Code != http.StatusForbidden {
		t.Errorf("from another site: %d", w.Code)
	}
	ours := func(r *http.Request) { r.Header.Set("Origin", "http://example.com") } // httptest's host
	if w := call(s, "POST", "/api/logout", home, "", ours); w.Code != http.StatusOK {
		t.Errorf("from this page: %d", w.Code)
	}
	if w := call(s, "GET", "/api/overview", home, ""); w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("no security headers")
	}
}

func TestPasswordsFromSettings(t *testing.T) {
	s := testServer(t, false, true)
	// A new password: hashed, never written as it is.
	body := `{"Staff":{"Names":["Max Mustermann","Erika Musterfrau"],"Current":"Max Mustermann","Passwords":{"Erika Musterfrau":"sehrgeheim"}}}`
	if w := call(s, "PUT", "/api/settings", home, body); w.Code != http.StatusOK {
		t.Fatalf("saving: %d %s", w.Code, w.Body)
	}
	data, _ := os.ReadFile(s.configPath)
	if bytes.Contains(data, []byte("sehrgeheim")) || !bytes.Contains(data, []byte("Erika Musterfrau: $2")) {
		t.Errorf("config after a new password:\n%s", data)
	}
	// The settings never show the hash; they say who has a password.
	if w := call(s, "GET", "/api/settings", home, ""); w.Code != http.StatusUnauthorized {
		// Logged out now that someone has a password.
		t.Errorf("settings without a session, once there is a password: %d", w.Code)
	}
	cookie := call(s, "POST", "/api/login", home, `{"name":"Erika Musterfrau"}`).Result().Cookies()[0]
	with := func(r *http.Request) { r.AddCookie(cookie) }
	w := call(s, "GET", "/api/settings", home, "", with)
	if strings.Contains(w.Body.String(), "$2") || !strings.Contains(w.Body.String(), `"logins":{"Erika Musterfrau":true}`) {
		t.Errorf("settings: %s", w.Body)
	}
	// Too short: refused.
	short := `{"Staff":{"Names":["Max Mustermann","Erika Musterfrau"],"Passwords":{"Max Mustermann":"kurz"}}}`
	if w := call(s, "PUT", "/api/settings", home, short, with); w.Code == http.StatusOK {
		t.Error("a password of 4 characters taken")
	}
	// None any more: the login gone.
	none := `{"Staff":{"Names":["Max Mustermann","Erika Musterfrau"],"NoLogin":["Erika Musterfrau"]}}`
	if w := call(s, "PUT", "/api/settings", home, none, with); w.Code != http.StatusOK {
		t.Fatalf("removing: %d %s", w.Code, w.Body)
	}
	data, _ = os.ReadFile(s.configPath)
	if bytes.Contains(data, []byte("logins")) {
		t.Errorf("login kept:\n%s", data)
	}
}

func TestOldPasswordMoves(t *testing.T) {
	cfg := &Config{Staff: StaffConfig{Names: []string{"Max Mustermann"}, Current: "Max Mustermann"}, Web: WebConfig{Password: "altesgeheim"}}
	if err := moveOldPassword(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Password != "" || !strings.HasPrefix(cfg.Staff.Logins["Max Mustermann"], "$2") {
		t.Errorf("after moving: %+v", cfg)
	}
}

func TestMailboxPasswordsKept(t *testing.T) {
	s := testServer(t, false, true)
	save := func(body string) {
		if w := call(s, "PUT", "/api/settings", home, body); w.Code != http.StatusOK {
			t.Fatalf("saving: %d %s", w.Code, w.Body)
		}
	}
	save(`{"Mail":{"Mailboxes":[{"Username":"max@example.de","Password":"postfach-geheim"}]}}`)
	// Sent back to the page: whether there is one, not the password.
	w := call(s, "GET", "/api/settings", home, "")
	if strings.Contains(w.Body.String(), "postfach-geheim") || !strings.Contains(w.Body.String(), `"mailboxes":[true]`) {
		t.Errorf("settings: %s", w.Body)
	}
	// Saved again with the field empty: kept.
	save(`{"Mail":{"Mailboxes":[{"Username":"max@example.de"}]}}`)
	cfg, _ := readConfig(s.configPath)
	if len(cfg.Mail.Mailboxes) != 1 || cfg.Mail.Mailboxes[0].Password != "postfach-geheim" {
		t.Errorf("mailboxes after saving empty: %+v", cfg.Mail.Mailboxes)
	}
}

func TestSessionEndsWhenIdle(t *testing.T) {
	s := newSessions(filepath.Join(t.TempDir(), "sessions.json"))
	token, err := s.start("Max Mustermann")
	if err != nil {
		t.Fatal(err)
	}
	h := tokenHash(token)

	// A request half an hour before the end renews it for another sessionIdle.
	s.m[h] = session{Name: "Max Mustermann", Expires: time.Now().Add(30 * time.Minute)}
	if name, renewed := s.who(token, true); name != "Max Mustermann" || !renewed {
		t.Fatalf("who = %q, renewed %v", name, renewed)
	}
	if left := time.Until(s.m[h].Expires); left < sessionIdle-time.Minute {
		t.Errorf("renewed for %v only", left)
	}

	// The page's own polling does not.
	s.m[h] = session{Name: "Max Mustermann", Expires: time.Now().Add(30 * time.Minute)}
	if _, renewed := s.who(token, false); renewed {
		t.Error("renewed without touch")
	}

	// Past its end, no one.
	s.m[h] = session{Name: "Max Mustermann", Expires: time.Now().Add(-time.Second)}
	if name, _ := s.who(token, true); name != "" {
		t.Errorf("expired session still %q", name)
	}
}

func TestDockerContainerIsNotHome(t *testing.T) {
	s := testServer(t, true, false)
	const container = "172.18.0.5:40000"
	forged := func(r *http.Request) { r.Header.Set("X-Forwarded-For", "192.168.178.5") }
	// Straight from a container, or claiming a home address: a password is asked, as from outside.
	if w := call(s, "POST", "/api/login", container, `{"name":"Max Mustermann"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("container without a password: %d", w.Code)
	}
	if w := call(s, "POST", "/api/login", container, `{"name":"Max Mustermann"}`, forged); w.Code != http.StatusUnauthorized {
		t.Errorf("container with a forged home address: %d", w.Code)
	}
	if w := call(s, "GET", "/api/session", container, "", forged); strings.Contains(w.Body.String(), "Max Mustermann") {
		t.Errorf("names shown to a container: %s", w.Body)
	}
}

func TestProxyToken(t *testing.T) {
	s := testServer(t, true, false)
	if err := s.change(func(cfg *Config) error { cfg.Web.ProxyToken = "caddy-secret"; return nil }); err != nil {
		t.Fatal(err)
	}
	const caddy = "172.18.0.2:40000"
	via := func(token, client string) func(*http.Request) {
		return func(r *http.Request) {
			r.Header.Set(proxyTokenHeader, token)
			r.Header.Set("X-Forwarded-For", client)
			r.Header.Set("X-Forwarded-Proto", "https")
		}
	}
	// Caddy in Docker with the token: the visitor at home counts as at home.
	if w := call(s, "POST", "/api/login", caddy, `{"name":"Max Mustermann"}`, via("caddy-secret", "192.168.178.5")); w.Code != http.StatusOK {
		t.Errorf("home visitor through Caddy with the token: %d %s", w.Code, w.Body)
	}
	// A wrong token: the header is the container's own say.
	if w := call(s, "POST", "/api/login", caddy, `{"name":"Max Mustermann"}`, via("guessed", "192.168.178.5")); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong token taken: %d", w.Code)
	}
	// From outside through Caddy: the password, and a Secure cookie.
	w := call(s, "POST", "/api/login", caddy, `{"name":"Max Mustermann","password":"geheim123"}`, via("caddy-secret", "84.150.10.2"))
	if c := w.Result().Cookies(); w.Code != http.StatusOK || len(c) == 0 || !c[0].Secure {
		t.Errorf("outside through Caddy: %d, cookie %+v", w.Code, c)
	}
}

func TestManyAddressesTrying(t *testing.T) {
	s := testServer(t, true, false)
	for i := range maxAllFailures {
		addr := fmt.Sprintf("84.150.10.%d:50000", i+1)
		call(s, "POST", "/api/login", addr, `{"name":"Max Mustermann","password":"falsch123"}`)
	}
	// Everyone from outside waits, the right password too; at home no password is asked.
	if w := call(s, "POST", "/api/login", "84.150.99.1:50000", `{"name":"Max Mustermann","password":"geheim123"}`); w.Code != http.StatusTooManyRequests {
		t.Errorf("after %d failures from many addresses: %d", maxAllFailures, w.Code)
	}
	if w := call(s, "POST", "/api/login", home, `{"name":"Max Mustermann"}`); w.Code != http.StatusOK {
		t.Errorf("at home during the wait: %d", w.Code)
	}
}

func TestUploadTooBig(t *testing.T) {
	s := testServer(t, false, false)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("report", "report.pdf")
	_, _ = part.Write(bytes.Repeat([]byte("x"), maxReport+1))
	_ = form.Close()
	w := call(s, "POST", "/api/harvest/upload", home, "", func(r *http.Request) {
		r.Body = io.NopCloser(&body)
		r.Header.Set("Content-Type", form.FormDataContentType())
	})
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "zu groß") {
		t.Errorf("a report over %d MB: %d %s", maxReport>>20, w.Code, w.Body)
	}
}
