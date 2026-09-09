package justjoin

import (
	"context"
	"log"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/grzegab/wok/internal/category"
)

const visibleOffers = 100
const frequency = 2 * time.Hour
const maxFrequencyJitter = 1 * time.Hour
const scrollStepDelay = 1 * time.Second
const listingTimeout = 3 * time.Minute
const maxFetchAttempts = 3
const fetchRetryBaseDelay = 2 * time.Second

// FetchedOffers carries the offer links found for a single category, so the
// transformer knows which category the offers it scrapes belong to.
type FetchedOffers struct {
	Category string
	Urls     []string
}

type OfferFetcher struct {
	categories  []category.Category
	FetchedUrls chan FetchedOffers
}

func NewOfferFetcher(categories []category.Category) *OfferFetcher {
	of := OfferFetcher{
		categories:  categories,
		FetchedUrls: make(chan FetchedOffers, visibleOffers),
	}

	return &of
}

func (of *OfferFetcher) Fetch(ctx context.Context) {
	for {
		of.fetchAll(ctx)

		// The wait is re-drawn after every pass, so the crawl doesn't settle
		// into a fixed rhythm the site could recognise.
		wait := frequency + rand.N(maxFrequencyJitter)
		log.Printf("Checking web again in %s\n", wait)

		if !pause(ctx, wait) {
			close(of.FetchedUrls)
			return
		}
	}
}

// fetchAll collects the offer links of every configured category, one
// category at a time so a single browser is enough. The browser only lives
// for the pass, so a crashed one cannot poison the passes that follow.
func (of *OfferFetcher) fetchAll(ctx context.Context) {
	browserCtx, closeBrowser, err := newBrowser(ctx)
	if err != nil {
		log.Printf("Skipping pass: %v\n", err)
		return
	}
	defer closeBrowser()

	for _, cat := range of.categories {
		log.Printf("Crawling %s offers at %s ...\n", cat.Name, cat.ScrapeURL)
		offers := FetchedOffers{Category: cat.Name, Urls: of.getUrls(browserCtx, cat.ScrapeURL)}

		// A blocking send would keep Fetch inside this loop once the buffer
		// fills up, so it would never reach the ctx.Done() case that closes
		// FetchedUrls and shutdown would stall.
		select {
		case of.FetchedUrls <- offers:
		case <-ctx.Done():
			return
		}
	}
}

func (of *OfferFetcher) getUrls(ctx context.Context, url string) []string {
	var urlsToScrape []string

	for attempt := 1; attempt <= maxFetchAttempts; attempt++ {
		urls, err := of.scrapeOnce(ctx, url)
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

	// The listing is sorted newest first, so it is walked back to front: the
	// offers then land in the database oldest first and ordering the panel by
	// "date added" descending puts the newest offer on top, the way the site
	// shows them.
	slices.Reverse(urlsToScrape)

	log.Printf("Found %d links to scrape. Starting sequential crawl...\n", len(urlsToScrape))

	return urlsToScrape
}

func (of *OfferFetcher) scrapeOnce(browserCtx context.Context, url string) ([]string, error) {
	// Own tab per listing, time-boxed so a stuck page can't run forever.
	ctx, cancelTab := newTab(browserCtx, listingTimeout)
	defer cancelTab()

	links := newOrderedLinks()

	log.Println("Navigating to target page...")

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return of.scrollAndCollectLinks(ctx, links)
		}),
	)

	if err != nil {
		return nil, err
	}

	return links.urls, nil
}

// orderedLinks collects offer links in the order the listing renders them,
// skipping the duplicates that reappear while scrolling. A plain map would
// lose that order, and the listing order is what tells us which offers are
// the newest.
type orderedLinks struct {
	seen map[string]struct{}
	urls []string
}

func newOrderedLinks() *orderedLinks {
	return &orderedLinks{seen: make(map[string]struct{})}
}

func (l *orderedLinks) add(href string) {
	if _, ok := l.seen[href]; ok {
		return
	}

	l.seen[href] = struct{}{}
	l.urls = append(l.urls, href)
}

// scrollAndCollectLinks scrolls the page to the bottom, collecting offer-card
// hrefs into seen along the way. The offer list is virtualized (offer-card
// nodes are mounted/unmounted as you scroll), so document.body.scrollHeight
// never grows. We have to collect hrefs after every scroll step instead of
// waiting for the page to "grow", and stop once window.scrollY itself stops
// advancing.
func (of *OfferFetcher) scrollAndCollectLinks(ctx context.Context, links *orderedLinks) error {
	log.Println("Start scrolling...")

	if err := collectOfferLinks(ctx, links); err != nil {
		return err
	}

	var lastScrollY, scrollY int64
	for {
		if err := chromedp.Evaluate(`window.scrollBy(0, window.innerHeight)`, nil).Do(ctx); err != nil {
			return err
		}

		time.Sleep(scrollStepDelay)

		if err := collectOfferLinks(ctx, links); err != nil {
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
}

// collectOfferLinks reads the hrefs of the currently mounted offer-card
// nodes, keeping the order they appear in.
func collectOfferLinks(ctx context.Context, links *orderedLinks) error {
	var nodes []*cdp.Node
	if err := chromedp.Nodes(".offer-card", &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)).Do(ctx); err != nil {
		return err
	}
	for _, node := range nodes {
		if href, ok := node.Attribute("href"); ok {
			links.add(href)
		}
	}
	return nil
}
