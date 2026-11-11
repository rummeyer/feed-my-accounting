package harvest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// The Harvest API v2, with a personal access token (id.getharvest.com/developers): the hours
// of a report without the web sign in and its captcha. On every plan, the free one too.

var apiClient = &http.Client{Timeout: 30 * time.Second}

// apiGet fetches a Harvest API URL into v.
func apiGet(creds HarvestLogin, accountID, rawURL string, v any) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	req.Header.Set("User-Agent", "feed-my-accounting (github.com/rummeyer/feed-my-accounting)")
	if accountID != "" {
		req.Header.Set("Harvest-Account-Id", accountID)
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Harvest API: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// apiAccount is the Harvest account to read: the one configured, else the token's first
// Harvest account.
func apiAccount(creds HarvestLogin) (string, error) {
	if creds.AccountID != "" {
		return creds.AccountID, nil
	}
	var accounts struct {
		Accounts []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Product string `json:"product"`
		} `json:"accounts"`
	}
	if err := apiGet(creds, "", "https://id.getharvest.com/api/v2/accounts", &accounts); err != nil {
		return "", fmt.Errorf("finding the Harvest account: %w", err)
	}
	for _, a := range accounts.Accounts {
		if a.Product == "harvest" {
			return strconv.Itoa(a.ID), nil
		}
	}
	return "", fmt.Errorf("the token has no Harvest account")
}

// APIHours is the hours booked for client — all clients if empty — from from to to, as
// Harvest's reports count them: rounded, when the account rounds.
func APIHours(creds HarvestLogin, client string, from, to time.Time) (float64, int, error) {
	account, err := apiAccount(creds)
	if err != nil {
		return 0, 0, err
	}
	next := "https://api.harvestapp.com/v2/time_entries?" + url.Values{
		"from":     {from.Format("2006-01-02")},
		"to":       {to.Format("2006-01-02")},
		"per_page": {"2000"},
	}.Encode()
	var hours float64
	entries, matched := 0, false
	for next != "" {
		var page struct {
			TimeEntries []struct {
				RoundedHours float64 `json:"rounded_hours"`
				Client       struct {
					Name string `json:"name"`
				} `json:"client"`
			} `json:"time_entries"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := apiGet(creds, account, next, &page); err != nil {
			return 0, 0, fmt.Errorf("reading time entries: %w", err)
		}
		for _, e := range page.TimeEntries {
			if client == "" || e.Client.Name == client {
				hours += e.RoundedHours
				entries++
				matched = true
			}
		}
		next = page.Links.Next
	}
	if !matched {
		return 0, 0, fmt.Errorf("no time entries for %q from %s to %s", client, from.Format("02.01.2006"), to.Format("02.01.2006"))
	}
	return hours, entries, nil
}
