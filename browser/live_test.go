package browser

import (
	"context"
	"testing"
)

func TestLiveAsking(t *testing.T) {
	var l Live
	if l.Asking() != "" {
		t.Fatal("asking before Open")
	}
	if _, err := l.Shot(); err != errNotAsking {
		t.Errorf("Shot before Open: %v, want errNotAsking", err)
	}
	if err := l.Click(1, 2); err != errNotAsking {
		t.Errorf("Click before Open: %v", err)
	}
	cancelled := l.Open(context.Background(), "Captcha")
	if l.Asking() != "Captcha" {
		t.Errorf("Asking() = %q", l.Asking())
	}
	l.Cancel()
	select {
	case <-cancelled:
	default:
		t.Error("Cancel did not close the channel")
	}
	l.Cancel() // twice: no panic
	l.Close()
	if l.Asking() != "" {
		t.Error("still asking after Close")
	}
}

func TestLiveKeys(t *testing.T) {
	var l Live
	if err := l.Key("Escape"); err == nil {
		t.Error("a key not offered was taken")
	}
	if err := l.Key("Enter"); err != errNotAsking {
		t.Errorf("Enter before Open: %v", err)
	}
}
