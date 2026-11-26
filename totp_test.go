package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestTOTPCode: the codes of RFC 6238's SHA-1 test vectors, their last six digits.
func TestTOTPCode(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // "12345678901234567890"
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := totpCode(secret, totpStep(time.Unix(unix, 0)))
		if err != nil || got != want {
			t.Errorf("at %d: %s, %v; want %s", unix, got, err, want)
		}
	}
	now := time.Unix(1111111109, 0)
	if matchTOTP(secret, "081 804", now) < 0 {
		t.Error("a code with a space is not taken")
	}
	if matchTOTP(secret, "000000", now) >= 0 {
		t.Error("a wrong code is taken")
	}
	// The step before, for a clock a little off; two before, no more.
	before, _ := totpCode(secret, totpStep(now)-1)
	old, _ := totpCode(secret, totpStep(now)-2)
	if matchTOTP(secret, before, now) < 0 || matchTOTP(secret, old, now) >= 0 {
		t.Error("the window is not one step either side")
	}
}

// TestTOTPLogin: an authenticator set up in the settings asks for its code from outside, once
// per browser for 30 days; at home never; a code counts once.
func TestTOTPLogin(t *testing.T) {
	s := testServer(t, true, false)
	// Logged in at home, the authenticator set up.
	w := call(s, "POST", "/api/login", home, `{"name":"Max Mustermann"}`)
	cookie := w.Result().Cookies()[0]
	withSession := func(r *http.Request) { r.AddCookie(cookie) }
	w = call(s, "POST", "/api/totp/setup", home, `{"name":"Max Mustermann"}`, withSession)
	var setup struct{ Secret, URI, QR string }
	if err := json.NewDecoder(w.Body).Decode(&setup); err != nil || setup.Secret == "" || !strings.HasPrefix(setup.QR, "data:image/png;base64,") || !strings.Contains(setup.URI, "issuer=feed-my-accounting") {
		t.Fatalf("setup: %d %+v %v", w.Code, setup, err)
	}
	if w := call(s, "POST", "/api/totp/confirm", home, `{"name":"Max Mustermann","code":"000000"}`, withSession); w.Code != http.StatusBadRequest {
		t.Errorf("confirmed with a wrong code: %d", w.Code)
	}
	code, _ := totpCode(setup.Secret, totpStep(time.Now()))
	if w := call(s, "POST", "/api/totp/confirm", home, `{"name":"Max Mustermann","code":"`+code+`"}`, withSession); w.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", w.Code, w.Body)
	}
	if w := call(s, "GET", "/api/settings", home, "", withSession); !strings.Contains(w.Body.String(), `"totp":{"Max Mustermann":true}`) || strings.Contains(w.Body.String(), setup.Secret) {
		t.Errorf("settings: whether set, never the secret")
	}
	// Saving the settings keeps it.
	if w := call(s, "PUT", "/api/settings", home, `{"Staff":{"Names":["Max Mustermann","Erika Musterfrau"],"Current":"Max Mustermann"},"Web":{"HomeOnly":false}}`, withSession); w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}

	// From outside: the password alone asks for the code, not counted as a failure.
	w = call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"geheim123"}`)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), `"code":true`) || len(w.Result().Cookies()) > 0 {
		t.Fatalf("without code: %d %s", w.Code, w.Body)
	}
	// The code of before, already used for confirming, does not count again.
	if w := call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"geheim123","code":"`+code+`"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("a code used twice: %d", w.Code)
	}
	// A wrong password with a code: still the password's answer.
	if w := call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"falsch","code":"123456"}`); strings.Contains(w.Body.String(), `"code":true`) {
		t.Errorf("wrong password asks for the code: %s", w.Body)
	}
	// The next step's code: in, and the browser trusted.
	s.totp.used["Max Mustermann"] = 0
	w = call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"geheim123","code":"`+code+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("with code: %d %s", w.Code, w.Body)
	}
	var trusted *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == trustedCookie {
			trusted = c
		}
	}
	if trusted == nil || trusted.MaxAge != int(trustedFor.Seconds()) || !trusted.HttpOnly || trusted.Path != "/api/login" {
		t.Fatalf("trusted cookie: %+v", trusted)
	}
	// That browser: the password alone, for 30 days; another still asked.
	if w := call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"geheim123"}`, func(r *http.Request) { r.AddCookie(trusted) }); w.Code != http.StatusOK {
		t.Errorf("trusted browser: %d %s", w.Code, w.Body)
	}
	if w := call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"geheim123"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("another browser: %d", w.Code)
	}
	// Removed: the trusted browsers forgotten, the password alone again.
	if w := call(s, "POST", "/api/totp/remove", home, `{"name":"Max Mustermann"}`, withSession); w.Code != http.StatusOK {
		t.Fatalf("remove: %d", w.Code)
	}
	if name, _ := s.trusted.who(trusted.Value, false); name != "" {
		t.Error("still trusted after removing")
	}
	if w := call(s, "POST", "/api/login", outside, `{"name":"Max Mustermann","password":"geheim123"}`); w.Code != http.StatusOK {
		t.Errorf("after removing: %d %s", w.Code, w.Body)
	}
}
