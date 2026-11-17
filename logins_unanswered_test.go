package main

import (
	"errors"
	"fmt"
	"testing"

	"feed-my-accounting/browser"
)

func TestWithLoginUnanswered(t *testing.T) {
	cfg := &Config{dir: t.TempDir()}
	err := withLogin(cfg, "amazon", func() error {
		return fmt.Errorf("login failed: %w", fmt.Errorf("%w after 10 min", browser.ErrNoAnswer))
	})
	if !browser.Unanswered(err) {
		t.Fatalf("not unanswered: %v", err)
	}
	if _, ok := lastLogin(cfg, "amazon"); ok {
		t.Error("a run nobody answered counted as a login")
	}
	withLogin(cfg, "amazon", func() error { return errors.New("no order list found") })
	if _, ok := lastLogin(cfg, "amazon"); !ok {
		t.Error("a run that failed otherwise not counted")
	}
}
