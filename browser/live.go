package browser

import (
	"context"
	"errors"
	"sync"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// Live lets a person take over a page for a moment — to solve a captcha, say — while a run
// waits: the page as a picture, and clicks and typing passed on to it. One at a time: the
// web interface runs one job at a time. Nil: nobody to ask, as on the command line.
type Live struct {
	mu     sync.Mutex
	ctx    context.Context // the page; nil while nobody is asked
	reason string
	cancel chan struct{} // closed when the person gives up
}

// ErrGivenUp is the person's answer when they cancel instead.
var ErrGivenUp = errors.New("cancelled in the live view")

// Open asks for help on the page of ctx, for reason. The channel closes if the person cancels.
func (l *Live) Open(ctx context.Context, reason string) <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ctx, l.reason, l.cancel = ctx, reason, make(chan struct{})
	return l.cancel
}

// Close ends the asking.
func (l *Live) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ctx, l.reason, l.cancel = nil, "", nil
}

// Asking is the reason help is asked for, or "" while it is not.
func (l *Live) Asking() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.reason
}

// Cancel gives up: the waiting run ends.
func (l *Live) Cancel() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		close(l.cancel)
		l.cancel = nil
	}
}

var errNotAsking = errors.New("no live view open")

func (l *Live) page() (context.Context, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ctx == nil {
		return nil, errNotAsking
	}
	return l.ctx, nil
}

// Shot is the page as it is now, a JPEG of what the window shows.
func (l *Live) Shot() ([]byte, error) {
	ctx, err := l.page()
	if err != nil {
		return nil, err
	}
	var data []byte
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		data, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatJpeg).WithQuality(75).Do(ctx)
		return err
	}))
	return data, err
}

// Click clicks at x, y on the page, in CSS pixels from its top left.
func (l *Live) Click(x, y float64) error {
	ctx, err := l.page()
	if err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.MouseClickXY(x, y))
}

// Type types text where the focus is.
func (l *Live) Type(text string) error {
	ctx, err := l.page()
	if err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.InsertText(text).Do(ctx)
	}))
}

// liveKeys are the keys the live view passes on as keys, by their names in the browser: what
// the others write is typed.
var liveKeys = map[string]string{
	"Enter": kb.Enter, "Tab": kb.Tab, "Backspace": kb.Backspace, "Delete": kb.Delete,
	"ArrowLeft": kb.ArrowLeft, "ArrowRight": kb.ArrowRight, "ArrowUp": kb.ArrowUp, "ArrowDown": kb.ArrowDown,
	"Home": kb.Home, "End": kb.End, "PageUp": kb.PageUp, "PageDown": kb.PageDown,
}

// Key presses one of liveKeys.
func (l *Live) Key(name string) error {
	key, ok := liveKeys[name]
	if !ok {
		return errors.New("no such key: " + name)
	}
	ctx, err := l.page()
	if err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.KeyEvent(key))
}

// Scroll turns the mouse wheel over the page at x, y by dy CSS pixels: down when positive.
func (l *Live) Scroll(x, y, dy float64) error {
	ctx, err := l.page()
	if err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MouseWheel, x, y).WithDeltaX(0).WithDeltaY(dy).Do(ctx)
	}))
}
