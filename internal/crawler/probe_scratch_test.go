package crawler

import (
	"context"
	"testing"
	"time"

	"github.com/grzegab/wok/internal/category"
)

// TestProbeMultiCategoryAndMultiOffer reproduces the crash from the VPS log:
// the second category used to fail with "context canceled", and every offer
// after the first would too.
func TestProbeMultiCategoryAndMultiOffer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 280*time.Second)
	defer cancel()

	allocCtx, allocCancel := newBrowserAllocator(ctx)
	defer allocCancel()

	// 1. two listings in a row, from one browser
	fetcher := NewOfferFetcher(category.All)
	var firstBatch []string
	for _, cat := range []category.Category{category.All[0], category.All[1]} {
		urls := fetcher.getUrls(allocCtx, cat.ScrapeURL)
		t.Logf("%-10s collected %d urls", cat.Name, len(urls))
		if len(urls) == 0 {
			t.Errorf("%s: collected no urls", cat.Name)
		}
		if firstBatch == nil {
			firstBatch = urls
		}
	}

	// 2. three offer pages in a row, from that same browser
	if len(firstBatch) < 3 {
		t.Skip("not enough urls to test offer scraping")
	}
	for i, url := range firstBatch[:3] {
		offer, err := scrapeOffer(allocCtx, url, "Go")
		if err != nil {
			t.Errorf("offer %d: %v", i+1, err)
			continue
		}
		t.Logf("offer %d ok: %q (%s)", i+1, offer.JobTitle, offer.Location.String)
	}
}
