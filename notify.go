package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Notifications go by Pushover, to the phone: a scheduled run that failed, and on the closing day
// a month whose receipts are not all there. What runs by hand is seen on the page.

// pushoverURL is where messages go; tests stand in for it.
var pushoverURL = "https://api.pushover.net/1/messages.json"

// notifyOn says whether notifications are set up.
func notifyOn(cfg *Config) bool {
	p := cfg.Notify.Pushover
	return p.Enabled && p.Token != "" && p.User != ""
}

// push sends a notification, if they are set up; a link with it, if one is given.
func push(cfg *Config, title, text string) error {
	if !notifyOn(cfg) {
		return nil
	}
	p := cfg.Notify.Pushover
	form := url.Values{"token": {p.Token}, "user": {p.User}, "title": {title}, "message": {text}}
	if p.Device != "" {
		form.Set("device", p.Device)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, pushoverURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushover: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var answer struct {
			Errors []string `json:"errors"`
		}
		_ = json.NewDecoder(res.Body).Decode(&answer)
		if len(answer.Errors) > 0 {
			return errors.New("pushover: " + strings.Join(answer.Errors, "; "))
		}
		return fmt.Errorf("pushover: HTTP %d", res.StatusCode)
	}
	return nil
}

// pushLogged is push, its failure only logged: what it tells of happened all the same.
func pushLogged(cfg *Config, title, text string) {
	if err := push(cfg, title, text); err != nil {
		log.Printf("notification not sent: %v", err)
	}
}

// monthNames are the months in German, as the notifications are.
var monthNames = []string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}

// remindClosing tells of the sources a month is still waiting for, if any.
func remindClosing(cfg *Config, rows []closingRow, month time.Time) {
	var open []string
	for _, r := range rows {
		if r.Open {
			open = append(open, r.Source)
		}
	}
	name := fmt.Sprintf("%s %d", monthNames[month.Month()-1], month.Year())
	if len(open) == 0 {
		log.Printf("closing: %s complete", name)
		return
	}
	log.Printf("closing: %s still open: %s", name, strings.Join(open, ", "))
	pushLogged(cfg, "Monatsabschluss "+name, "Noch offen: "+strings.Join(open, ", ")+". Siehe Abschluss in feed-my-accounting.")
}
