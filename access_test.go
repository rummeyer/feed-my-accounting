package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func request(remote, proto string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	if proto != "" {
		r.Header.Set("X-Forwarded-Proto", proto)
	}
	return r
}

func TestHomeAccessOnlyFromTheHomeNetwork(t *testing.T) {
	home := WebConfig{Access: AccessHome}
	for remote, want := range map[string]int{
		"10.0.1.20:51000":     0,
		"192.168.178.5:51000": 0,
		"172.20.0.3:51000":    0,
		"127.0.0.1:51000":     0,
		"[::1]:51000":         0,
		"[fd00::5]:51000":     0,
		"[fe80::1]:51000":     0,
		"84.150.10.2:51000":   http.StatusForbidden,
		"[2a02:810::1]:51000": http.StatusForbidden,
	} {
		if got, why := admit(home, request(remote, "")); got != want {
			t.Errorf("%s: got %d (%s), want %d", remote, got, why, want)
		}
	}
}

func TestProxyAccessOnlyThroughTheProxyWithHTTPSAndAPassword(t *testing.T) {
	proxy := WebConfig{Access: AccessProxy, Proxies: defaultProxies, Password: "secret"}
	cases := []struct {
		remote, proto string
		want          int
	}{
		{"172.18.0.2:40000", "https", 0},                     // Caddy in Docker
		{"127.0.0.1:40000", "https", 0},                      // Caddy on the machine
		{"172.18.0.2:40000", "http", http.StatusForbidden},   // the proxy, but unencrypted
		{"172.18.0.2:40000", "", http.StatusForbidden},       // no proxy headers
		{"10.0.1.20:40000", "https", http.StatusForbidden},   // directly from the home network
		{"84.150.10.2:40000", "https", http.StatusForbidden}, // directly from outside
	}
	for _, c := range cases {
		if got, why := admit(proxy, request(c.remote, c.proto)); got != c.want {
			t.Errorf("%s %q: got %d (%s), want %d", c.remote, c.proto, got, why, c.want)
		}
	}
	proxy.Password = ""
	if got, _ := admit(proxy, request("127.0.0.1:40000", "https")); got != http.StatusServiceUnavailable {
		t.Errorf("without a password the proxy may not open it: got %d", got)
	}
}

func TestAccessDefaultsToHome(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)
	if cfg.Web.Access != AccessHome || len(cfg.Web.Proxies) == 0 {
		t.Errorf("defaults: %+v", cfg.Web)
	}
}
