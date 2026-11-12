package email

import "testing"

func TestAccounts(t *testing.T) {
	cfg := MailConfig{IMAPHost: "imap.strato.de", IMAPPort: 993, Username: "max@example.com", Password: "a",
		Mailboxes: []Mailbox{
			{Username: "max@example.de", Password: "b"},
			{Username: " "}, // a row left empty
			{Username: "max@other.example", Password: "c", IMAPHost: "imap.other.example", IMAPPort: 143},
		}}
	got := cfg.accounts()
	if len(got) != 3 {
		t.Fatalf("%d accounts, want the main one and two further", len(got))
	}
	// The main account first; the further ones on its server unless they name their own.
	for i, want := range []struct {
		user, pass, host string
		port             int
	}{
		{"max@example.com", "a", "imap.strato.de", 993},
		{"max@example.de", "b", "imap.strato.de", 993},
		{"max@other.example", "c", "imap.other.example", 143},
	} {
		a := got[i]
		if a.Username != want.user || a.Password != want.pass || a.IMAPHost != want.host || a.IMAPPort != want.port || len(a.Mailboxes) != 0 {
			t.Errorf("account %d: %+v", i, a)
		}
	}
}
