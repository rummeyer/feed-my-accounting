package browser

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// WaitForHand is how long a run waits for someone to log in by hand in the live view: the
// runs' browsers are given as long beyond their own time.
const WaitForHand = 10 * time.Minute

// Hand is a login handed over to a person in the live view — a captcha, a code by SMS, no
// password stored — as Amazon's and Vodafone's runs do it: the same waiting, each site with
// its own look at whether it has let them in.
type Hand struct {
	Live   *Live
	Why    string        // the reason's key: the live view says it in the page's language
	Reason string        // the reason in words, for the log and the error without a live view
	Fill   string        // the stored password, offered at a button; "" for none
	Wait   time.Duration // how long to wait for someone; WaitForHand if 0
	Log    *log.Logger
	// In says whether the site has let them in by now: looked at every 2 seconds, it should
	// take a few seconds at most, a page between two loads not holding it up.
	In func(ctx context.Context) bool
}

// ByHand asks someone to log in in the live view, and waits until In says they are — wrong
// tries along the way included, as they type — or they cancel, or Wait has passed. Without a
// live view, the reason is the error: ErrNoLiveView.
func ByHand(ctx context.Context, h Hand) error {
	if h.Live == nil {
		return fmt.Errorf("%s, log in via web interface (%w)", strings.ToLower(h.Reason), ErrNoLiveView)
	}
	if h.Wait == 0 {
		h.Wait = WaitForHand
	}
	h.Log.Printf("%s - waiting for live view", h.Reason)
	cancelled := h.Live.OpenWith(ctx, h.Why, h.Fill)
	defer h.Live.Close()
	deadline := time.After(h.Wait)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-cancelled:
			return ErrGivenUp
		case <-deadline:
			return fmt.Errorf("%w after %d min", ErrNoAnswer, int(h.Wait.Minutes()))
		case <-tick.C:
			if h.In(ctx) {
				h.Log.Println("Logged in by hand")
				return nil
			}
		}
	}
}
