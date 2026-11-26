package browser

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"
)

// TestByHand: no live view says so; the site letting them in ends the wait, as does cancelling,
// and the wait running out.
func TestByHand(t *testing.T) {
	quiet := log.New(io.Discard, "", 0)
	ctx := context.Background()
	if err := ByHand(ctx, Hand{Reason: "Captcha asked", Log: quiet}); !errors.Is(err, ErrNoLiveView) {
		t.Errorf("without a live view: %v", err)
	}
	looks := 0
	in := Hand{Live: &Live{}, Why: "captcha", Reason: "Captcha asked", Wait: 10 * time.Second, Log: quiet,
		In: func(context.Context) bool { looks++; return looks == 2 }}
	if err := ByHand(ctx, in); err != nil || looks != 2 {
		t.Errorf("let in on the second look: %v after %d", err, looks)
	}
	live := &Live{}
	go func() {
		for live.Asking() == "" {
			time.Sleep(10 * time.Millisecond)
		}
		live.Cancel()
	}()
	never := func(context.Context) bool { return false }
	if err := ByHand(ctx, Hand{Live: live, Why: "captcha", Reason: "Captcha asked", Wait: 10 * time.Second, Log: quiet, In: never}); !errors.Is(err, ErrGivenUp) {
		t.Errorf("cancelled: %v", err)
	}
	if err := ByHand(ctx, Hand{Live: &Live{}, Why: "captcha", Reason: "Captcha asked", Wait: 100 * time.Millisecond, Log: quiet, In: never}); !errors.Is(err, ErrNoAnswer) {
		t.Errorf("nobody came: %v", err)
	}
}
