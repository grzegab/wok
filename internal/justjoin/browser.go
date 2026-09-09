package justjoin

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// newBrowser starts a headless Chromium with the setuid sandbox disabled,
// since containers can't provide it without extra privileges and the crawler
// already runs as a non-root user.
//
// chromedp allocates a browser on the first Run of a context, and a context
// derived straight from an allocator allocates one of its own. Running an
// empty batch here brings the process up front, so that contexts derived from
// the returned one inherit this browser and open a tab in it instead of
// launching a Chromium each.
func newBrowser(ctx context.Context) (context.Context, context.CancelFunc, error) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
	)...)

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	cancel := func() {
		browserCancel()
		allocCancel()
	}

	if err := chromedp.Run(browserCtx); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("could not start chromium: %w", err)
	}

	return browserCtx, cancel, nil
}

// newTab opens a fresh tab in browserCtx's browser, time-boxed to timeout.
//
// Every unit of work needs its own tab. chromedp ties a tab to the context it
// was created from, so cancelling a context derived from it - which is what a
// timeout does - closes that tab for good. Sharing one tab across pages means
// the first timeout kills every page after it with "context canceled".
func newTab(browserCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	timedCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)

	return timedCtx, func() {
		timeoutCancel()
		tabCancel()
	}
}
