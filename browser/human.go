package browser

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/chromedp/chromedp"
)

// Pause waits a while between min and max, at random: a person reading the page, or moving
// to the next field — no bot check's even rhythm of a script.
func Pause(ctx context.Context, min, max time.Duration) error {
	d := min
	if max > min {
		d += rand.N(max - min)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// TypeLikeAPerson clicks into the field sel with the mouse, waits a moment, and types value key
// by key, with uneven gaps between them and now and then a longer one: as a person would, not
// a script setting the field's value.
func TypeLikeAPerson(ctx context.Context, sel, value string) error {
	if err := chromedp.Run(ctx, chromedp.Click(sel, chromedp.ByQuery, chromedp.NodeVisible)); err != nil {
		return err
	}
	if err := Pause(ctx, 300*time.Millisecond, 900*time.Millisecond); err != nil {
		return err
	}
	for i, r := range value {
		if err := chromedp.Run(ctx, chromedp.KeyEvent(string(r))); err != nil {
			return err
		}
		gap := 70 * time.Millisecond
		if i > 0 && rand.N(9) == 0 {
			gap = 350 * time.Millisecond // a person's hesitation
		}
		if err := Pause(ctx, gap, gap+150*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}
