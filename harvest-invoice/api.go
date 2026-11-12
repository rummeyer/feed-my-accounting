package harvest

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The Harvest API v2, with a personal access token: a month's hours, without Harvest's web
// sign in and its captcha.

var apiClient = &http.Client{Timeout: 30 * time.Second}

// apiGet fetches a Harvest API URL into v.
func apiGet(creds HarvestAPI, accountID, rawURL string, v any) error {
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
func apiAccount(creds HarvestAPI) (string, error) {
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

// Entry is one time entry, as Harvest's detailed time report lists it.
type Entry struct {
	Date     time.Time
	Started  string // "7:00", empty without timestamps
	Ended    string
	Client   string
	Project  string
	Code     string // the project's code, "0043"
	Task     string
	Person   string
	Notes    string
	Hours    float64 // rounded, when the account rounds, as Harvest's reports count
	Billable bool
	Billed   bool
}

// APIEntries is the time entries of client and person — all clients, everyone, if empty — from
// from to to, in the order of the day: by date, then by start.
func APIEntries(creds HarvestAPI, client, person string, from, to time.Time) ([]Entry, error) {
	account, err := apiAccount(creds)
	if err != nil {
		return nil, err
	}
	next := "https://api.harvestapp.com/v2/time_entries?" + url.Values{
		"from":     {from.Format("2006-01-02")},
		"to":       {to.Format("2006-01-02")},
		"per_page": {"2000"},
	}.Encode()
	type named struct {
		Name string `json:"name"`
	}
	var entries []Entry
	others := map[string]bool{} // the client's people other than person, should none match
	for next != "" {
		var page struct {
			TimeEntries []struct {
				SpentDate    string  `json:"spent_date"`
				StartedTime  string  `json:"started_time"`
				EndedTime    string  `json:"ended_time"`
				RoundedHours float64 `json:"rounded_hours"`
				Notes        string  `json:"notes"`
				Billable     bool    `json:"billable"`
				IsBilled     bool    `json:"is_billed"`
				Client       named   `json:"client"`
				Task         named   `json:"task"`
				User         named   `json:"user"`
				Project      struct {
					Name string `json:"name"`
					Code string `json:"code"`
				} `json:"project"`
			} `json:"time_entries"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := apiGet(creds, account, next, &page); err != nil {
			return nil, fmt.Errorf("reading time entries: %w", err)
		}
		for _, e := range page.TimeEntries {
			if client != "" && e.Client.Name != client {
				continue
			}
			if person != "" && !strings.EqualFold(strings.TrimSpace(e.User.Name), strings.TrimSpace(person)) {
				others[e.User.Name] = true
				continue
			}
			date, err := time.ParseInLocation("2006-01-02", e.SpentDate, time.Local)
			if err != nil {
				return nil, fmt.Errorf("time entry of %q: %w", e.SpentDate, err)
			}
			entries = append(entries, Entry{
				Date: date, Started: e.StartedTime, Ended: e.EndedTime,
				Client: e.Client.Name, Project: e.Project.Name, Code: e.Project.Code, Task: e.Task.Name,
				Person: e.User.Name, Notes: strings.TrimSpace(e.Notes), Hours: e.RoundedHours,
				Billable: e.Billable, Billed: e.IsBilled,
			})
		}
		next = page.Links.Next
	}
	if len(entries) == 0 && len(others) > 0 {
		names := slices.Sorted(maps.Keys(others))
		return nil, fmt.Errorf("no time entries of %q for %q from %s to %s; Harvest has some of %s — is the employee named as in Harvest?",
			person, client, from.Format("02.01.2006"), to.Format("02.01.2006"), strings.Join(names, ", "))
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no time entries for %q from %s to %s", client, from.Format("02.01.2006"), to.Format("02.01.2006"))
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].Date.Equal(entries[j].Date) {
			return entries[i].Date.Before(entries[j].Date)
		}
		return minutes(entries[i].Started) < minutes(entries[j].Started)
	})
	return entries, nil
}

// minutes is "7:30" in minutes since midnight; -1 without a time, so that such entries come first.
func minutes(clock string) int {
	h, m, ok := strings.Cut(clock, ":")
	if !ok {
		return -1
	}
	hours, err1 := strconv.Atoi(h)
	mins, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil {
		return -1
	}
	return hours*60 + mins
}

// Company is the Harvest account's company: its name, for the report's head, and where it is on
// the web, https://rummeyer.harvestapp.com.
type Company struct {
	Name    string `json:"name"`
	BaseURI string `json:"base_uri"`
}

// APICompany is the account's company.
func APICompany(creds HarvestAPI) (Company, error) {
	var company Company
	account, err := apiAccount(creds)
	if err != nil {
		return company, err
	}
	if err := apiGet(creds, account, "https://api.harvestapp.com/v2/company", &company); err != nil {
		return company, fmt.Errorf("reading the company: %w", err)
	}
	return company, nil
}
