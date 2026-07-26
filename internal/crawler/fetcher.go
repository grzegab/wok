package crawler

import (
	"context"
	"log"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

const visibleOffers = 100
const frequency = 6 * time.Hour
const scrollStepDelay = 1 * time.Second
const maxFetchAttempts = 3
const fetchRetryBaseDelay = 2 * time.Second

type OfferFetcher struct {
	Url         string
	Type        string
	FetchedUrls chan []string
}

func NewOfferFetcher(url string) *OfferFetcher {
	of := OfferFetcher{
		Url:         url,
		Type:        "Go",
		FetchedUrls: make(chan []string, visibleOffers),
	}

	return &of
}

func (of *OfferFetcher) Fetch(ctx context.Context) {
	browserCtx, cancel := newBrowserContext(ctx)
	defer cancel()

	randMinute := time.Minute * time.Duration(time.Now().UnixNano()%60)
	tickTime := frequency + randMinute

	ticker := time.NewTicker(tickTime)
	defer ticker.Stop()

	log.Println("Checking web every ", tickTime)

	of.FetchedUrls <- of.getUrls(browserCtx)

	for {
		select {
		case <-ctx.Done():
			close(of.FetchedUrls)
			return
		case <-ticker.C:
			of.FetchedUrls <- of.getUrls(browserCtx)
		}
	}
}

func (of *OfferFetcher) getUrls(ctx context.Context) []string {
	var urlsToScrape []string

	for attempt := 1; attempt <= maxFetchAttempts; attempt++ {
		urls, err := of.scrapeOnce(ctx)
		if err == nil {
			urlsToScrape = urls
			break
		}

		log.Printf("Error loading main page (attempt %d/%d): %v", attempt, maxFetchAttempts, err)

		if attempt == maxFetchAttempts {
			return nil
		}

		backoff := fetchRetryBaseDelay * time.Duration(1<<(attempt-1))
		time.Sleep(backoff)
	}

	log.Printf("Found %d links to scrape. Starting sequential crawl...\n", len(urlsToScrape))

	return urlsToScrape
}

func (of *OfferFetcher) scrapeOnce(ctx context.Context) ([]string, error) {
	// Set a global 3-minute timeout so the script doesn't run forever
	ctx, cancelTimeout := context.WithTimeout(ctx, 3*time.Minute)
	defer cancelTimeout()

	seen := make(map[string]struct{})

	log.Println("Navigating to target page...")

	err := chromedp.Run(ctx,
		chromedp.Navigate(of.Url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// The offer list is virtualized (offer-card nodes are mounted/
			// unmounted as you scroll), so document.body.scrollHeight never
			// grows. We have to collect hrefs after every scroll step
			// instead of waiting for the page to "grow", and stop once
			// window.scrollY itself stops advancing.
			collect := func() error {
				var links []*cdp.Node
				if err := chromedp.Nodes(".offer-card", &links, chromedp.ByQueryAll, chromedp.AtLeast(0)).Do(ctx); err != nil {
					return err
				}
				for _, node := range links {
					if href, ok := node.Attribute("href"); ok {
						seen[href] = struct{}{}
					}
				}
				return nil
			}

			log.Println("Start scrolling...")

			if err := collect(); err != nil {
				return err
			}

			var lastScrollY, scrollY int64
			for {
				if err := chromedp.Evaluate(`window.scrollBy(0, window.innerHeight)`, nil).Do(ctx); err != nil {
					return err
				}

				time.Sleep(scrollStepDelay)

				if err := collect(); err != nil {
					return err
				}

				if err := chromedp.Evaluate(`window.scrollY`, &scrollY).Do(ctx); err != nil {
					return err
				}

				if scrollY == lastScrollY {
					log.Println("Reached the bottom of the infinite scroll.")
					break
				}
				lastScrollY = scrollY
			}
			return nil
		}),
	)

	if err != nil {
		return nil, err
	}

	urlsToScrape := make([]string, 0, len(seen))
	for href := range seen {
		urlsToScrape = append(urlsToScrape, href)
	}

	return urlsToScrape, nil
}
