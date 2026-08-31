package crawler

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

// newBrowserAllocator starts a headless Chromium with the setuid sandbox
// disabled, since containers can't provide it without extra privileges and
// the crawler already runs as a non-root user. The browser stays up for the
// lifetime of the returned context; individual pages run in tabs opened by
// newTab.
func newBrowserAllocator(ctx context.Context) (context.Context, context.CancelFunc) {
	return chromedp.NewExecAllocator(ctx, append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
	)...)
}

// newTab opens a fresh tab, time-boxed to timeout.
//
// Every unit of work needs its own tab. chromedp ties a tab to the context it
// was created from, so cancelling a context derived from it - which is what a
// timeout does - closes that tab for good. Sharing one tab across pages means
// the first timeout kills every page after it with "context canceled".
func newTab(allocCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	timedCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)

	return timedCtx, func() {
		timeoutCancel()
		tabCancel()
	}
}
