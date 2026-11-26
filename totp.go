package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"
)

// The second factor: an authenticator app — Google Authenticator, or any other for TOTP
// (RFC 6238) — set up per employee under Einstellungen › Mitarbeiter by its QR code. Logging in
// from outside, an employee with one gives its code after the password; the browser is then
// trusted for trustedFor, a cookie of its own, and asked for the password only. At home no
// password is asked, and no code either.

const (
	trustedCookie = "fma_trusted"
	trustedFor    = 30 * 24 * time.Hour // how long a browser that gave a code is not asked again
	totpPeriod    = 30                  // seconds a code lasts, as the apps have it
	totpIssuer    = "feed-my-accounting"
	totpSetupFor  = 10 * time.Minute // how long a QR code shown may be confirmed
)

// totpState are the authenticators being set up — the secret shown, not yet confirmed with a
// code — and the last code used of each employee, so that none counts twice.
type totpState struct {
	mu      sync.Mutex
	pending map[string]pendingTOTP
	used    map[string]int64 // the last time step whose code was taken, by employee
}

// pendingTOTP is a secret shown as a QR code, until when it may be confirmed.
type pendingTOTP struct {
	secret string
	until  time.Time
}

// totpCode is the six-digit code of a base32 secret for a time step.
func totpCode(secret string, step int64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(secret, "=")))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000), nil
}

// totpStep is the time step of a moment.
func totpStep(t time.Time) int64 { return t.Unix() / totpPeriod }

// matchTOTP is the time step whose code the given one is — this one's, the one before or the
// one after, for a clock a little off — or -1 for none.
func matchTOTP(secret, code string, now time.Time) int64 {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != 6 {
		return -1
	}
	for _, step := range []int64{totpStep(now), totpStep(now) - 1, totpStep(now) + 1} {
		if want, err := totpCode(secret, step); err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step
		}
	}
	return -1
}

// check says whether the code is the employee's now, and takes it: the same code, or an older
// one, does not count again.
func (t *totpState) check(name, secret, code string) bool {
	step := matchTOTP(secret, code, time.Now())
	if step < 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.used == nil {
		t.used = map[string]int64{}
	}
	if step <= t.used[name] {
		return false
	}
	t.used[name] = step
	return true
}

// newTOTPSecret is a new random secret: 160 bits, base32 as the apps take it.
func newTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// totpURI is the otpauth address the QR code holds: the app shows the issuer and the employee.
func totpURI(name, secret string) string {
	q := url.Values{"secret": {secret}, "issuer": {totpIssuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + url.PathEscape(totpIssuer+":"+name) + "?" + q.Encode()
}

// totpSetup starts setting up an employee's authenticator: a new secret, shown as a QR code and
// as text, to be confirmed with a code within totpSetupFor. Until then the one before stays.
func (s *server) totpSetup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !read(w, r, &in) {
		return
	}
	cfg, err := readConfig(s.configPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if !slices.Contains(cfg.Staff.Names, in.Name) {
		fail(w, http.StatusBadRequest, errors.New(say(r, "diesen Mitarbeiter gibt es nicht — erst speichern", "no such employee — save first")))
		return
	}
	secret, err := newTOTPSecret()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	uri := totpURI(in.Name, secret)
	code, err := qr.Encode(uri, qr.M)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	code.Scale = 6
	s.totp.mu.Lock()
	if s.totp.pending == nil {
		s.totp.pending = map[string]pendingTOTP{}
	}
	s.totp.pending[in.Name] = pendingTOTP{secret: secret, until: time.Now().Add(totpSetupFor)}
	s.totp.mu.Unlock()
	reply(w, map[string]string{
		"secret": secret, "uri": uri,
		"qr": "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG()),
	})
}

// totpConfirm takes the secret shown once the app gives its code: saved as the employee's, the
// browsers trusted before for the old one asked again.
func (s *server) totpConfirm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	if !read(w, r, &in) {
		return
	}
	s.totp.mu.Lock()
	p, ok := s.totp.pending[in.Name]
	s.totp.mu.Unlock()
	if !ok || time.Now().After(p.until) {
		fail(w, http.StatusBadRequest, errors.New(say(r, "kein QR-Code offen — neu einrichten", "no QR code open — set it up anew")))
		return
	}
	if !s.totp.check(in.Name, p.secret, in.Code) {
		fail(w, http.StatusBadRequest, errors.New(say(r, "Code falsch — die Uhrzeit des Telefons prüfen", "wrong code — check the phone's clock")))
		return
	}
	err := s.change(func(cfg *Config) error {
		if !slices.Contains(cfg.Staff.Names, in.Name) {
			return errors.New(say(r, "diesen Mitarbeiter gibt es nicht", "no such employee"))
		}
		if cfg.Staff.TOTP == nil {
			cfg.Staff.TOTP = map[string]string{}
		}
		cfg.Staff.TOTP[in.Name] = p.secret
		return nil
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.totp.mu.Lock()
	delete(s.totp.pending, in.Name)
	s.totp.mu.Unlock()
	s.trusted.endFor(in.Name, "")
	reply(w, map[string]bool{"ok": true})
}

// totpRemove takes an employee's authenticator away: the password alone again, from outside.
func (s *server) totpRemove(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !read(w, r, &in) {
		return
	}
	err := s.change(func(cfg *Config) error {
		delete(cfg.Staff.TOTP, in.Name)
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	s.trusted.endFor(in.Name, "")
	reply(w, map[string]bool{"ok": true})
}

// trustedBrowser says whether the request's browser gave the employee's code within trustedFor.
func (s *server) trustedBrowser(r *http.Request, name string) bool {
	cookie, err := r.Cookie(trustedCookie)
	if err != nil {
		return false
	}
	who, _ := s.trusted.who(cookie.Value, false)
	return who != "" && who == name
}

// trust marks the browser as having given the employee's code: a cookie for trustedFor, sent
// with logins only, not for the page's scripts.
func (s *server) trust(w http.ResponseWriter, r *http.Request, name string) error {
	token, err := s.trusted.start(name)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: trustedCookie, Value: token, Path: "/api/login", MaxAge: int(trustedFor.Seconds()),
		HttpOnly: true, Secure: overHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
	return nil
}
