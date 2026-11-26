package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
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

// TypeChecked types value into the field sel as TypeLikeAPerson does, emptied first, and checks
// it holds just that: a field the page filled already would have it appended, a key lost would
// be missing. An error then, for nothing to be sent; it says only lengths, never the value.
// name is the field's, for the error.
func TypeChecked(ctx context.Context, sel, value, name string) error {
	js, _ := json.Marshal(sel)
	clear := fmt.Sprintf(`(() => { const e = document.querySelector(%s); if (e && e.value) { e.value = ''; e.dispatchEvent(new Event('input', {bubbles: true})); } })()`, js)
	if err := chromedp.Run(ctx, chromedp.Evaluate(clear, nil)); err != nil {
		return err
	}
	if err := TypeLikeAPerson(ctx, sel, value); err != nil {
		return err
	}
	var got int
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(document.querySelector(%s)?.value || '').length`, js), &got)); err != nil {
		return err
	}
	if want := len([]rune(value)); got != want {
		return fmt.Errorf("%s holds %d of %d chars, not sent", strings.TrimSpace(name+" field"), got, want)
	}
	return nil
}
