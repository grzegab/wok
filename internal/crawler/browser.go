package crawler

import (
	"context"

	"github.com/chromedp/chromedp"
)

// newBrowserContext starts a headless Chromium context with the setuid
// sandbox disabled, since containers can't provide it without extra
// privileges and the crawler already runs as a non-root user.
func newBrowserContext(ctx context.Context) (context.Context, context.CancelFunc) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
	)...)

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	return browserCtx, func() {
		browserCancel()
		allocCancel()
	}
}
