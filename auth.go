package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Logging in: as an employee. From the home network by choosing the name, no password asked;
// from outside — through Caddy, say — with the password set for them under Einstellungen ›
// Mitarbeiter. As soon as one employee has a password, every page and every call needs a
// session; as long as none has, the web interface answers the home network only, open as before.

const (
	sessionCookie = "fma_session"
	sessionTTL    = 30 * 24 * time.Hour
	maxFailures   = 5                // failed logins from one address before it waits
	lockedFor     = 15 * time.Minute // how long it waits
)

// sessions are the logged-in browsers: the token's SHA-256, never the token, with the employee
// and when it expires. Kept beside the configuration, readable only by its owner, so that a
// restart logs no one out.
type sessions struct {
	path string
	mu   sync.Mutex
	m    map[string]session
}

type session struct {
	Name    string    `json:"name"`
	Expires time.Time `json:"expires"`
}

func newSessions(path string) *sessions {
	s := &sessions{path: path, m: map[string]session{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s.m)
	}
	return s
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// start is a new session for the employee: its token, for the cookie.
func (s *sessions) start(name string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[tokenHash(token)] = session{Name: name, Expires: time.Now().Add(sessionTTL)}
	return token, s.save()
}

// who is the employee the token's session is for; empty if there is none, or it has expired.
func (s *sessions) who(token string) string {
	if token == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	got, ok := s.m[tokenHash(token)]
	if !ok || time.Now().After(got.Expires) {
		return ""
	}
	return got.Name
}

func (s *sessions) end(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, tokenHash(token))
	_ = s.save()
}

// endFor ends the employee's sessions but the one kept: after a new password, or with the
// employee gone. keep is a token, empty for none.
func (s *sessions) endFor(name, keep string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for h, got := range s.m {
		if got.Name == name && (keep == "" || h != tokenHash(keep)) {
			delete(s.m, h)
		}
	}
	_ = s.save()
}

// save writes the sessions not expired; called with mu held.
func (s *sessions) save() error {
	for h, got := range s.m {
		if time.Now().After(got.Expires) {
			delete(s.m, h)
		}
	}
	data, err := json.Marshal(s.m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// failures counts failed logins by address, so that passwords cannot be tried one after another.
type failures struct {
	mu sync.Mutex
	m  map[string]struct {
		n     int
		until time.Time
	}
}

func (f *failures) locked(ip string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return time.Now().Before(f.m[ip].until)
}

func (f *failures) failed(ip string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = map[string]struct {
			n     int
			until time.Time
		}{}
	}
	got := f.m[ip]
	got.n++
	if got.n >= maxFailures {
		got.n, got.until = 0, time.Now().Add(lockedFor)
	}
	f.m[ip] = got
}

func (f *failures) succeeded(ip string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, ip)
}

// dummyHash is a hash to compare with for a name without a password: made once.
var dummyHash = sync.OnceValue(func() string {
	hash, _ := hashPassword("no one's password")
	return hash
})

// hashPassword is the password as kept: bcrypt.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// ---------------------------------------------------------------------------
// Where a request comes from
// ---------------------------------------------------------------------------

// proxied says the request came through a reverse proxy in the home network — Caddy on this
// machine or on another — with the client's address in X-Forwarded-For. Only from there is the
// header believed: from outside, it is the client's own say. A proxy elsewhere in the home
// network must count too, or everyone it lets in from the internet would be at home.
func proxied(r *http.Request) bool {
	ip := net.ParseIP(remoteHost(r.RemoteAddr))
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) &&
		r.Header.Get("X-Forwarded-For") != ""
}

func remoteHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// clientIP is the address the request comes from: through the proxy, the one it added last to
// X-Forwarded-For — the others the client could have written itself.
func clientIP(r *http.Request) string {
	if proxied(r) {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return remoteHost(r.RemoteAddr)
}

// overHTTPS says the browser talks HTTPS: to us, or to the proxy in front.
func overHTTPS(r *http.Request) bool {
	return r.TLS != nil || (proxied(r) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
}

// ---------------------------------------------------------------------------
// Login and logout
// ---------------------------------------------------------------------------

// open are the paths the login page needs before a login.
var open = map[string]bool{
	"/login": true, "/login.html": true, "/login.js": true, "/theme.js": true,
	"/style.css": true, "/logo.webp": true, "/api/login": true, "/api/session": true,
}

// loggedIn is the employee the request's session is for; empty without one.
func (s *server) loggedIn(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return s.sessions.who(cookie.Value)
}

// session says whether logging in is needed, whether with a password, for whom it could be,
// and who is logged in. At home every employee, without a password; from outside those with one.
func (s *server) session(w http.ResponseWriter, r *http.Request) {
	cfg, err := readConfig(s.configPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	atHome := fromHome(r)
	names := []string{}
	for _, n := range cfg.Staff.Names {
		if atHome || cfg.Staff.Logins[n] != "" {
			names = append(names, n)
		}
	}
	reply(w, map[string]any{
		"required": len(cfg.Staff.Logins) > 0, "password": !atHome, "names": names, "user": s.loggedIn(r),
	})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !read(w, r, &in) {
		return
	}
	ip := clientIP(r)
	if s.failures.locked(ip) {
		fail(w, http.StatusTooManyRequests, errors.New("zu viele Versuche — in 15 Minuten wieder"))
		return
	}
	cfg, err := readConfig(s.configPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	// At home the name is enough: one of the employees.
	if fromHome(r) {
		if !slices.Contains(cfg.Staff.Names, in.Name) {
			fail(w, http.StatusUnauthorized, errors.New("diesen Mitarbeiter gibt es nicht"))
			return
		}
		s.startSession(w, r, in.Name)
		return
	}
	hash := cfg.Staff.Logins[in.Name]
	// Compared even without an employee of that name, so the answer takes as long either way.
	if hash == "" {
		hash = dummyHash()
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil || cfg.Staff.Logins[in.Name] == "" {
		s.failures.failed(ip)
		fail(w, http.StatusUnauthorized, errors.New("Name oder Passwort falsch"))
		return
	}
	s.failures.succeeded(ip)
	s.startSession(w, r, in.Name)
}

// startSession logs the employee in: a session, and its cookie — not for the page's scripts, over
// HTTPS only when the browser talks HTTPS, and only sent from this site's pages.
func (s *server) startSession(w http.ResponseWriter, r *http.Request, name string) {
	token, err := s.sessions.start(name)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, Secure: overHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
	reply(w, map[string]string{"user": name})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.end(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: overHTTPS(r), SameSite: http.SameSiteStrictMode})
	reply(w, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------------
// The gate
// ---------------------------------------------------------------------------

// auth guards every request. The home network only, with web.homeOnly — the address seen
// through the proxy, not the proxy's — and always while no employee has a password. A session,
// as soon as one has. Calls that change something only from this page: their Origin, when the
// browser sends one, is this host.
func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		cfg, err := readConfig(s.configPath)
		if err != nil {
			http.Error(w, "config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		required := len(cfg.Staff.Logins) > 0
		if (cfg.Web.homeOnly() || !required) && !fromHome(r) {
			http.Error(w, "only reachable from the home network", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			http.Error(w, "not from this page", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/login" {
			r.URL.Path = "/login.html"
		}
		if required && !open[r.URL.Path] && s.loggedIn(r) == "" {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				fail(w, http.StatusUnauthorized, errors.New("nicht angemeldet"))
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin says a changing call comes from a page of this host: by its Origin, or Referer.
// Neither sent, as by curl, counts: a browser always sends one of them.
func sameOrigin(r *http.Request) bool {
	from := r.Header.Get("Origin")
	if from == "" {
		from = r.Header.Get("Referer")
	}
	if from == "" {
		return true
	}
	u, err := url.Parse(from)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// securityHeaders keep the page from being framed, sniffed or leaking where it is, and let it
// load only its own scripts.
func securityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
}
