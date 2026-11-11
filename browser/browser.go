// Package browser provides a shared headless Chrome context for modules
// that need browser automation (vodafone-invoice, harvest-invoice).
package browser

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
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
	)
	if !cfg.nativeUserAgent {
		flags = append(flags, chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"))
	}
	if cfg.profileDir != "" {
		flags = append(flags, chromedp.UserDataDir(cfg.profileDir))
	}

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
	if cfg.nativeUserAgent {
		if err := useNativeUserAgent(ctx, cfg.germanLocale); err != nil {
			log.Printf("browser: native user agent: %v", err)
		}
	}

	return ctx, func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}
}

// useNativeUserAgent shows the browser as what it is — this Chromium, on Linux — only without
// "Headless". Captchas compare the user agent with the client hints and the platform the page
// sees; a Mac Chrome 131 that is really Chromium 156 on a Raspberry Pi fails that.
func useNativeUserAgent(ctx context.Context, german bool) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, product, _, userAgent, _, err := cdpbrowser.GetVersion().Do(ctx)
		if err != nil {
			return err
		}
		full := product[strings.Index(product, "/")+1:]
		major, _, _ := strings.Cut(full, ".")
		override := emulation.SetUserAgentOverride(strings.Replace(userAgent, "HeadlessChrome", "Chrome", 1)).
			WithPlatform("Linux aarch64").
			WithUserAgentMetadata(&emulation.UserAgentMetadata{
				Brands: []*emulation.UserAgentBrandVersion{
					{Brand: "Chromium", Version: major}, {Brand: "Not.A/Brand", Version: "99"}},
				FullVersionList: []*emulation.UserAgentBrandVersion{
					{Brand: "Chromium", Version: full}, {Brand: "Not.A/Brand", Version: "99.0.0.0"}},
				Platform:     "Linux",
				Architecture: "arm",
				Bitness:      "64",
			})
		if german {
			override = override.WithAcceptLanguage("de-DE,de")
		}
		return override.Do(ctx)
	}))
}

type options struct {
	germanLocale    bool
	nativeUserAgent bool
	profileDir      string
}

// Option configures the browser context.
type Option func(*options)

// WithNativeUserAgent keeps the browser's own user agent, minus "Headless", instead of the
// Mac Chrome one the other sites are used to.
func WithNativeUserAgent() Option {
	return func(o *options) { o.nativeUserAgent = true }
}

// WithProfile keeps the browser's profile — cookies, so a login — in dir between runs.
func WithProfile(dir string) Option {
	return func(o *options) { o.profileDir = dir }
}

// WithGermanLocale sets Chrome's UI language to German (de-DE).
func WithGermanLocale() Option {
	return func(o *options) {
		o.germanLocale = true
	}
}
