package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"feed-my-accounting/email"
)

// A receipt downloaded and booked by hand — an Amazon invoice fetched while there was time — is
// filed in the storage as the others are. Before a run sends one, it looks there too: the same
// file is booked already. By name as well where the name says which invoice it is and the file
// differs each time it is made: Apple's, made from the mail, with the order's number; Vodafone's,
// a month's per contract. Not Amazon's: the orders of a day are numbered as found, _2, _3.

// filedByName are the sources whose receipts' names tell them apart.
var filedByName = map[string]bool{"Apple": true, "Vodafone": true}

// filedLedger is the ledger, and beyond it the storage's receipts of the months asked about,
// fetched once per run.
type filedLedger struct {
	*Ledger
	archive ArchiveConfig
	months  map[string][]email.Attachment
}

// Sent says whether a receipt went to the books: in the ledger, or filed by hand in the storage —
// then noted in the ledger, too. A storage that cannot be read stops the run: nothing twice.
func (f *filedLedger) Sent(source string, a email.Attachment) (string, bool, error) {
	if how, sent, err := f.Ledger.Sent(source, a); sent || err != nil {
		return how, sent, err
	}
	if f.again(source, a) {
		return "", false, nil
	}
	month := time.Now()
	if m := receiptMonth.FindStringSubmatch(a.Filename); m != nil {
		month, _ = time.ParseInLocation("2006-01", m[2]+"-"+m[1], time.Local)
	}
	key := month.Format("2006-01")
	if f.months == nil {
		f.months = map[string][]email.Attachment{}
	}
	files, ok := f.months[key]
	if !ok {
		var err error
		if files, err = monthReceipts(f.archive, month); err != nil {
			return "", false, fmt.Errorf("the storage, for receipts filed by hand: %w", err)
		}
		f.months[key] = files
	}
	hash := hashOf(a.Data)
	for _, filed := range files {
		if hashOf(filed.Data) == hash || filedByName[source] && sameName(filed.Filename, a.Filename) {
			if err := f.RecordFiled(source, a, filed.Filename); err != nil {
				archiveLog.Printf("WARNING: ledger: %v", err)
			}
			return "filed by hand as " + filed.Filename, true, nil
		}
	}
	return "", false, nil
}

var copyMark = regexp.MustCompile(`(?:\s\(\d+\)|-(\d+))(\.[A-Za-z]+)$`)

// sameName says whether a name in the storage is the receipt's: the same, or as the Finder or a
// hand made it a second time — " (1)", "-2" for "_2".
func sameName(filed, name string) bool {
	norm := func(s string) string {
		return strings.ToLower(copyMark.ReplaceAllStringFunc(s, func(m string) string {
			p := copyMark.FindStringSubmatch(m)
			if p[1] != "" {
				return "_" + p[1] + p[2]
			}
			return p[2]
		}))
	}
	return norm(filed) == norm(name)
}
