package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOnlyTheHomeNetwork(t *testing.T) {
	for remote, want := range map[string]bool{
		"10.0.1.20:51000":     true,
		"192.168.178.5:51000": true,
		"172.20.0.3:51000":    false, // a Docker container is not at home
		"127.0.0.1:51000":     true,
		"[::1]:51000":         true,
		"[fd00::5]:51000":     true,
		"[fe80::1]:51000":     true,
		"84.150.10.2:51000":   false,
		"[2a02:810::1]:51000": false,
		"100.101.102.103:1":   false, // a VPN's shared address space is not home
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		if got := fromHome(r); got != want {
			t.Errorf("%s: got %v, want %v", remote, got, want)
		}
	}
}

func TestHomeOnlyIsOnUnlessSwitchedOff(t *testing.T) {
	off := false
	if !(WebConfig{}).homeOnly() || (WebConfig{HomeOnly: &off}).homeOnly() {
		t.Error("homeOnly should default to on and follow the switch")
	}
}
