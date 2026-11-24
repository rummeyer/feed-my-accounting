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

// The receipts mailed are copied to a storage of rclone's too, if switched on — iCloud Drive,
// Google Drive, OneDrive, Nextcloud, a NAS by SFTP: whatever remote is set up for it on the
// machine with `rclone config`. iCloud Drive's, iclouddrive, talks to icloud.com as its web page
// does, Apple having no API for the Drive; when Apple wants a new two-factor code, the copy
// fails saying so, the mail sent all the same.

// archiveLog is the copy's log, "[rclone] " first, as the modules' are.
var archiveLog = log.New(os.Stderr, "[rclone] ", log.LstdFlags)

// receiptMonth is a receipt's month as its name begins: 11_2026_Rechnung_….pdf.
var receiptMonth = regexp.MustCompile(`^(\d{2})_(\d{4})_`)

// archiveFolder is where in the storage a receipt goes: folder with {YYYY} and {MM} its month's,
// by its name — or now's, if its name does not say.
func archiveFolder(folder, name string, now time.Time) string {
	year, month := fmt.Sprintf("%04d", now.Year()), fmt.Sprintf("%02d", int(now.Month()))
	if m := receiptMonth.FindStringSubmatch(name); m != nil {
		month, year = m[1], m[2]
	}
	return strings.Trim(strings.NewReplacer("{YYYY}", year, "{MM}", month).Replace(folder), "/")
}

// belegeMail is the mail configuration the receipts go out with: none mailed twice, none filed by
// hand in the storage, and with the copy switched on, what is mailed is copied there too.
func belegeMail(cfg *Config) email.MailConfig {
	mail := ledgerMail(cfg)
	// Filed by hand in the storage: booked, not sent.
	if l, ok := mail.Ledger.(*Ledger); ok && cfg.Archive.Enabled && cfg.Archive.Remote != "" {
		mail.Ledger = &filedLedger{Ledger: l, archive: cfg.Archive}
	}
	if cfg.Archive.Enabled {
		ac := cfg.Archive
		mail.Archive = func(vendor string, files []email.Attachment) error {
			err := copyToArchive(ac, files)
			if err != nil {
				// A login the storage wants anew, as a rule — Apple's code, a token run out: what
				// to do, in the log and with the run.
				archiveLog.Printf("Copy failed: %v", err)
				archiveLog.Printf("If it wants a new login, run on the Pi: rclone config reconnect %s:", ac.Remote)
				return fmt.Errorf("%w (if it wants a new login: rclone config reconnect %s:)", err, ac.Remote)
			}
			return nil
		}
	}
	return mail
}

// copyToArchive copies files to the storage, each into its month's folder, never over one there:
// a name taken gets " (1)", " (2)" before its extension, as the Finder does. Written to a
// folder of their own first, under the names they get, then one rclone copy per folder there.
func copyToArchive(ac ArchiveConfig, files []email.Attachment) error {
	if ac.Remote == "" {
		return errors.New("no rclone remote set")
	}
	kind := remoteType(ac.Remote)
	byFolder := map[string][]email.Attachment{}
	for _, f := range files {
		folder := archiveFolder(ac.Folder, f.Filename, time.Now())
		byFolder[folder] = append(byFolder[folder], f)
	}
	folders := make([]string, 0, len(byFolder))
	for folder := range byFolder {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	for _, folder := range folders {
		dir, err := os.MkdirTemp("", "feed-my-accounting-archive-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		taken, err := remoteNames(ac.Remote + ":" + folder)
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
		out, err := exec.CommandContext(ctx, "rclone", "copy", dir, ac.Remote+":"+folder).CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("rclone copy to \"%s://%s\": %v: %s", kind, folder, err, lastLine(string(out)))
		}
		archiveLog.Printf("%d file(s) copied to \"%s://%s\"", len(byFolder[folder]), kind, folder)
	}
	return nil
}

// remoteNames are the names of the files in a folder of the storage; none if it is not there yet.
func remoteNames(remote string) (map[string]bool, error) {
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

// remoteType is the kind of storage a remote is — iclouddrive, drive, onedrive, webdav — as
// rclone has it set up, for the log to say where; the remote's name if rclone does not say.
func remoteType(remote string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "rclone", "listremotes", "--long").Output()
	if err != nil {
		return remote
	}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) == 2 && strings.TrimSuffix(f[0], ":") == remote {
			return f[1]
		}
	}
	return remote
}

// lastLine is the last line rclone wrote that says something: its error, as a rule.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
