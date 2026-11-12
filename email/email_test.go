package email

import (
	"testing"

	"github.com/emersion/go-imap"
)

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

func TestFromDomain(t *testing.T) {
	stripe := []*imap.Address{{MailboxName: "invoice+statements", HostName: "stripe.com"}}
	for _, c := range []struct {
		domains string
		want    bool
	}{
		{"mail.anthropic.com", false},
		{"mail.anthropic.com, stripe.com", true},
		{"STRIPE.com", true},
		{" , ", false},
	} {
		if got := fromDomain(stripe, c.domains); got != c.want {
			t.Errorf("fromDomain(%q) = %v, want %v", c.domains, got, c.want)
		}
	}
}
