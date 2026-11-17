package browser

import (
	"context"
	"errors"
	"sync"
	"time"

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
	fill   string        // what the person may have put into a field at a click: a password
	mouseX float64       // where the mouse is on the page, for the next click to move from
	mouseY float64
}

// ErrGivenUp is the person's answer when they cancel instead.
var ErrGivenUp = errors.New("cancelled in the live view")

// ErrNoAnswer is a run's wait in the live view that nobody came to.
var ErrNoAnswer = errors.New("live view timed out")

// Unanswered says whether err is a run ended for want of someone in the live view: cancelled
// there, or nobody came. No login made — a code not given, a captcha not solved.
func Unanswered(err error) bool {
	return errors.Is(err, ErrGivenUp) || errors.Is(err, ErrNoAnswer)
}

// Open asks for help on the page of ctx, for reason. The channel closes if the person cancels.
func (l *Live) Open(ctx context.Context, reason string) <-chan struct{} {
	return l.OpenWith(ctx, reason, "")
}

// OpenWith asks as Open does, offering fill — the stored password — for the person to put into
// the field they choose, by a button: never shown, never typed unasked.
func (l *Live) OpenWith(ctx context.Context, reason, fill string) <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ctx, l.reason, l.cancel, l.fill = ctx, reason, make(chan struct{}), fill
	return l.cancel
}

// Fills says whether there is something to put into a field.
func (l *Live) Fills() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fill != ""
}

// Fill puts it into the field the focus is in, as typed.
func (l *Live) Fill() error {
	l.mu.Lock()
	fill := l.fill
	l.mu.Unlock()
	if fill == "" {
		return errors.New("nothing to fill in")
	}
	return l.Type(fill)
}

// Close ends the asking.
func (l *Live) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ctx, l.reason, l.cancel, l.fill = nil, "", nil, ""
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

// Click clicks at x, y on the page, in CSS pixels from its top left, as a mouse does: moved
// there from where it was in a few steps, then pressed and let go a moment later. A click out
// of nowhere, no move before it, a sign-in's bot check took for none (Amazon, 17.11.2026).
func (l *Live) Click(x, y float64) error {
	ctx, err := l.page()
	if err != nil {
		return err
	}
	l.mu.Lock()
	fromX, fromY := l.mouseX, l.mouseY
	l.mouseX, l.mouseY = x, y
	l.mu.Unlock()
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		const steps = 8
		for i := 1; i <= steps; i++ {
			f := float64(i) / steps
			if err := input.DispatchMouseEvent(input.MouseMoved, fromX+(x-fromX)*f, fromY+(y-fromY)*f).Do(ctx); err != nil {
				return err
			}
			if err := Pause(ctx, 10*time.Millisecond, 30*time.Millisecond); err != nil {
				return err
			}
		}
		if err := Pause(ctx, 80*time.Millisecond, 200*time.Millisecond); err != nil {
			return err
		}
		if err := input.DispatchMouseEvent(input.MousePressed, x, y).WithButton(input.Left).WithClickCount(1).Do(ctx); err != nil {
			return err
		}
		if err := Pause(ctx, 60*time.Millisecond, 140*time.Millisecond); err != nil {
			return err
		}
		return input.DispatchMouseEvent(input.MouseReleased, x, y).WithButton(input.Left).WithClickCount(1).Do(ctx)
	}))
}

// Type types text where the focus is, key by key with a person's gaps: keys pressed, not text
// set — a field whose bot check counts keystrokes sees them.
func (l *Live) Type(text string) error {
	ctx, err := l.page()
	if err != nil {
		return err
	}
	// No field with the cursor — the page's own focus gone after it loaded (Amazon's code, 17.11.
	// 2026: the code typed into nothing) — the first input field shown gets it.
	chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const a = document.activeElement;
		if (a && /^(INPUT|TEXTAREA)$/.test(a.tagName)) return;
		const f = [...document.querySelectorAll('input, textarea')].find((e) => e.offsetParent !== null && !/^(hidden|checkbox|radio|submit|button|image)$/.test(e.type));
		if (f) f.focus();
	})()`, nil))
	for _, r := range text {
		if err := chromedp.Run(ctx, chromedp.KeyEvent(string(r))); err != nil {
			return err
		}
		if err := Pause(ctx, 50*time.Millisecond, 160*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
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
	// Keys that edit or move as the keyboard does, a raw key with its editing command: a page's
	// own field — Amazon's sign-in (17.11.2026) — took chromedp's key event for none.
	if e, ok := editing[name]; ok {
		return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			down := input.DispatchKeyEvent(input.KeyRawDown).WithKey(name).WithCode(name).
				WithWindowsVirtualKeyCode(e.code).WithNativeVirtualKeyCode(e.code).WithCommands([]string{e.command})
			if err := down.Do(ctx); err != nil {
				return err
			}
			return input.DispatchKeyEvent(input.KeyUp).WithKey(name).WithCode(name).
				WithWindowsVirtualKeyCode(e.code).WithNativeVirtualKeyCode(e.code).Do(ctx)
		}))
	}
	return chromedp.Run(ctx, chromedp.KeyEvent(key))
}

// editing are the keys that edit or move in a field, with their key code and editing command.
var editing = map[string]struct {
	code    int64
	command string
}{
	"Backspace":  {8, "deleteBackward"},
	"Delete":     {46, "deleteForward"},
	"ArrowLeft":  {37, "moveLeft"},
	"ArrowRight": {39, "moveRight"},
	"ArrowUp":    {38, "moveUp"},
	"ArrowDown":  {40, "moveDown"},
	"Home":       {36, "moveToBeginningOfLine"},
	"End":        {35, "moveToEndOfLine"},
}

// Clear empties the field the focus is in, as select all and delete would.
func (l *Live) Clear() error {
	ctx, err := l.page()
	if err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const e = document.activeElement;
		if (!e || !('value' in e)) return;
		e.select?.();
		if (!document.execCommand('delete')) {
			Object.getOwnPropertyDescriptor(Object.getPrototypeOf(e), 'value').set.call(e, '');
			e.dispatchEvent(new Event('input', {bubbles: true}));
		}
	})()`, nil))
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
