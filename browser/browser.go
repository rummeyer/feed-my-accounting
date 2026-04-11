// Package browser provides a shared headless Chrome context for modules
// that need browser automation (vodafone-invoice, harvest-invoice).
package browser

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// filteredLogf wraps log.Printf but suppresses noisy chromedp messages
// like "unhandled node event *dom.EventTopLayerElementsUpdated".
func filteredLogf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "unhandled node event") {
		return
	}
	log.Print(msg)
}

// NewContext creates a headless Chrome browser context with bot-detection
// evasion flags (new headless mode, custom user agent, AutomationControlled
// disabled). The returned cancel function tears down the allocator, browser
// context, and timeout in the correct order.
//
// Options:
//   - GermanLocale: sets Chrome's UI and accept-language to de-DE. Required
//     for sites that render locale-dependent UI (e.g. sevDesk date pickers).
func NewContext(opts ...Option) (context.Context, context.CancelFunc) {
	cfg := options{}
	for _, o := range opts {
		o(&cfg)
	}

	flags := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	)

	if cfg.germanLocale {
		flags = append(flags,
			chromedp.Flag("lang", "de-DE"),
			chromedp.Flag("accept-lang", "de-DE,de"),
		)
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), flags...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(filteredLogf),
		chromedp.WithLogf(filteredLogf),
	)
	ctx, timeoutCancel := context.WithTimeout(ctx, 5*time.Minute)

	return ctx, func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}
}

type options struct {
	germanLocale bool
}

// Option configures the browser context.
type Option func(*options)

// WithGermanLocale sets Chrome's UI language to German (de-DE).
func WithGermanLocale() Option {
	return func(o *options) {
		o.germanLocale = true
	}
}
