package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// The login gate: a site whose bot check takes a run's logins for an attacker — Vodafone reset
// the password after three on three days, and blocked the address after a day of tries with it
// (13.–17.11.2026) — is logged into once in 28 days at the most: once a month, February's too.
// Each login is noted beside the configuration, in feed-my-accounting-logins.json.

const loginsFile = "feed-my-accounting-logins.json"

// gapDays is how many days after a login another is held back, counted in days, not hours:
// the books closed on the 31st of January in the evening, February's 28th in the morning is
// 28 days on all the same.
const gapDays = 28

// allowedFrom is when a login after one at last is allowed: the start of the day gapDays on.
func allowedFrom(last time.Time) time.Time {
	return time.Date(last.Year(), last.Month(), last.Day()+gapDays, 0, 0, 0, 0, last.Location())
}

var loginsMu sync.Mutex

// errGated is a login held back by the gate: run anyway only when someone says so.
var errGated = errors.New("login gate")

// gatedError says when the last login was, and why another is held back.
type gatedError struct {
	site string
	last time.Time
}

func (e gatedError) Error() string {
	return fmt.Sprintf("%s: logged in already on %s — once a month only, the next from %s; another login this soon risks the account being blocked",
		siteName(e.site), e.last.Format("02.01.2006 15:04"), allowedFrom(e.last).Format("02.01.2006"))
}

func (e gatedError) Unwrap() error { return errGated }

func siteName(site string) string {
	switch site {
	case "vodafone":
		return "Vodafone"
	case "amazon":
		return "Amazon"
	}
	return site
}

func loginsPath(cfg *Config) string {
	if cfg.dir == "" {
		return ""
	}
	return filepath.Join(cfg.dir, loginsFile)
}

// readLogins are the logins noted, by site, oldest first.
func readLogins(cfg *Config) map[string][]time.Time {
	logins := map[string][]time.Time{}
	if path := loginsPath(cfg); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &logins)
		}
	}
	return logins
}

// lastLogin is the site's latest login noted, if there is one.
func lastLogin(cfg *Config, site string) (time.Time, bool) {
	loginsMu.Lock()
	defer loginsMu.Unlock()
	times := readLogins(cfg)[site]
	if len(times) == 0 {
		return time.Time{}, false
	}
	return times[len(times)-1], true
}

// loginGate holds a login to site back within 28 days of the last.
func loginGate(cfg *Config, site string, now time.Time) error {
	last, ok := lastLogin(cfg, site)
	if ok && now.Before(allowedFrom(last)) {
		return gatedError{site, last}
	}
	return nil
}

// nextLogin is when the gate lets a login to site through again; zero if it does now.
func nextLogin(cfg *Config, site string, now time.Time) time.Time {
	if last, ok := lastLogin(cfg, site); ok && now.Before(allowedFrom(last)) {
		return allowedFrom(last)
	}
	return time.Time{}
}

// recordLogin notes a login to site at at, the last twelve kept.
func recordLogin(cfg *Config, site string, at time.Time) {
	path := loginsPath(cfg)
	if path == "" {
		return
	}
	loginsMu.Lock()
	defer loginsMu.Unlock()
	logins := readLogins(cfg)
	times := append(logins[site], at)
	if len(times) > 12 {
		times = times[len(times)-12:]
	}
	logins[site] = times
	if data, err := json.MarshalIndent(logins, "", "  "); err == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
}
