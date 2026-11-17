package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"feed-my-accounting/email"
)

// The receipts mailed are copied to iCloud Drive too, if switched on: by rclone, whose
// iclouddrive remote talks to icloud.com as its web page does — Apple has no API for the Drive.
// Set up once on the Pi with `rclone config` and a two-factor code; when Apple wants one again,
// the copy fails saying so, the mail sent all the same.

// icloudLog is the iCloud Drive copy's log, "[icloud] " first, as the modules' are.
var icloudLog = log.New(os.Stderr, "[icloud] ", log.LstdFlags)

// receiptMonth is a receipt's month as its name begins: 11_2026_Rechnung_….pdf.
var receiptMonth = regexp.MustCompile(`^(\d{2})_(\d{4})_`)

// icloudFolder is where in the Drive a receipt goes: folder with {YYYY} and {MM} its month's,
// by its name — or now's, if its name does not say.
func icloudFolder(folder, name string, now time.Time) string {
	year, month := fmt.Sprintf("%04d", now.Year()), fmt.Sprintf("%02d", int(now.Month()))
	if m := receiptMonth.FindStringSubmatch(name); m != nil {
		month, year = m[1], m[2]
	}
	return strings.Trim(strings.NewReplacer("{YYYY}", year, "{MM}", month).Replace(folder), "/")
}

// belegeMail is the mail configuration the receipts go out with: with iCloud Drive switched on,
// what is mailed is copied there too.
func belegeMail(cfg *Config) email.MailConfig {
	mail := cfg.Mail
	if cfg.ICloud.Enabled {
		ic := cfg.ICloud
		mail.Archive = func(vendor string, files []email.Attachment) error {
			err := copyToICloud(ic, files)
			if err != nil {
				// Apple's trust token gone, as a rule: what to do, in the log and with the run.
				icloudLog.Printf("Copy failed: %v", err)
				icloudLog.Printf("If Apple wants a new code, run on the Pi: rclone config reconnect %s:", ic.Remote)
				return fmt.Errorf("%w (if Apple wants a new code: rclone config reconnect %s:)", err, ic.Remote)
			}
			return nil
		}
	}
	return mail
}

// copyToICloud copies files to the Drive, each into its month's folder, never over one there:
// a name taken gets " (1)", " (2)" before its extension, as the Finder does. Written to a
// folder of their own first, under the names they get, then one rclone copy per Drive folder.
func copyToICloud(ic ICloudConfig, files []email.Attachment) error {
	byFolder := map[string][]email.Attachment{}
	for _, f := range files {
		folder := icloudFolder(ic.Folder, f.Filename, time.Now())
		byFolder[folder] = append(byFolder[folder], f)
	}
	folders := make([]string, 0, len(byFolder))
	for folder := range byFolder {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	for _, folder := range folders {
		dir, err := os.MkdirTemp("", "feed-my-accounting-icloud-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		taken, err := icloudNames(ic.Remote + ":" + folder)
		if err != nil {
			return err
		}
		for _, f := range byFolder[folder] {
			name := freeName(filepath.Base(f.Filename), taken)
			taken[name] = true
			if err := os.WriteFile(filepath.Join(dir, name), f.Data, 0o600); err != nil {
				return err
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		out, err := exec.CommandContext(ctx, "rclone", "copy", dir, ic.Remote+":"+folder).CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("rclone copy to \"iclouddrive://%s\": %v: %s", folder, err, lastLine(string(out)))
		}
		icloudLog.Printf("%d file(s) copied to \"iclouddrive://%s\"", len(byFolder[folder]), folder)
	}
	return nil
}

// icloudNames are the names of the files in a Drive folder; none if it is not there yet.
func icloudNames(remote string) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "rclone", "lsf", "--files-only", remote).Output()
	if err != nil {
		var exit *exec.ExitError
		// rclone's code for a folder not found: copy makes it.
		if errors.As(err, &exit) && exit.ExitCode() == 3 {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("rclone lsf %s: %v", remote, err)
	}
	names := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names[line] = true
		}
	}
	return names, nil
}

// freeName is name, or with " (1)", " (2)", … before its extension, the first not taken.
func freeName(name string, taken map[string]bool) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for n := 1; taken[name]; n++ {
		name = fmt.Sprintf("%s (%d)%s", base, n, ext)
	}
	return name
}

// lastLine is the last line rclone wrote that says something: its error, as a rule.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
