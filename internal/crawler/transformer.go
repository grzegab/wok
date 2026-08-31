package crawler

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type OfferTransformer struct {
	Offers chan database.CreatePossibilityParams
}

func NewOfferTransformer() *OfferTransformer {
	return &OfferTransformer{
		Offers: make(chan database.CreatePossibilityParams),
	}
}

func (ot *OfferTransformer) Transform(ctx context.Context, ch <-chan FetchedOffers) {
	ctx, cancel := newBrowserContext(ctx)
	defer cancel()

	for batch := range ch {
		for i, url := range batch.Urls {
			if ctx.Err() != nil {
				break
			}

			log.Printf("[%s %d/%d] Scraping: %s\n", batch.Category, i+1, len(batch.Urls), url)
			// Be polite to the server with a short pause between requests
			baseSecond := 65 * time.Second
			randSecond := time.Second * time.Duration(time.Now().UnixNano()%60)
			log.Printf("Sleeping for %s\n", baseSecond+randSecond)

			offer, err := scrapeOffer(ctx, url, batch.Category)
			if err != nil {
				log.Printf("Failed to scrape %s: %v\n", url, err)
				continue
			}

			ot.Offers <- offer

			if !pause(ctx, baseSecond+randSecond) {
				break
			}
		}
	}

	close(ot.Offers)
}

// pause waits out the politeness delay between requests, returning false as
// soon as the context is cancelled so shutdown does not sit through the full
// delay.
func pause(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// scrapeOffer navigates to a single job-offer page and extracts the fields
// used to build a CreatePossibilityParams. The selectors below are specific
// to the current DOM structure of the target site (relative XPath from the
// "Job description"/"Tech stack" headings) and are expected to silently stop
// matching if the site is redesigned.
func scrapeOffer(ctx context.Context, url, category string) (database.CreatePossibilityParams, error) {
	var jobTitle string
	var companyName string
	var description string
	var location string
	var body string
	var stack string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Text("h1", &jobTitle, chromedp.ByQuery),
		chromedp.Text("h2", &companyName, chromedp.ByQuery),
		chromedp.Text("body", &body, chromedp.ByQuery),
		chromedp.Text(
			`//h2/ancestor::a/preceding-sibling::div[1]`,
			&location,
			chromedp.BySearch,
		),
		chromedp.Text(
			`//h3[normalize-space()='Job description']/following-sibling::div`,
			&description,
			chromedp.BySearch,
		),
		chromedp.Evaluate(`
				(() => {
					try {
						const result = document.evaluate(
							'//h3[contains(text(),"Tech stack")]/following::h4',
							document,
							null,
							XPathResult.ORDERED_NODE_SNAPSHOT_TYPE,
							null
						);
						const items = [];
						for (let i = 0; i < result.snapshotLength; i++) {
							items.push(result.snapshotItem(i).innerText);
						}
						return items.join(', ');
					} catch (e) {
						return '';
					}
				})()
			`,
			&stack),
	)

	if err != nil {
		return database.CreatePossibilityParams{}, err
	}

	offer := database.CreatePossibilityParams{
		Category:    category,
		Url:         url,
		CompanyName: companyName,
		JobTitle:    jobTitle,
		Location:    optText(location),
		Tech:        optText(stack),
		Description: optText(description),
	}

	if strings.Contains(body, "Remote") {
		offer.IsRemote = true
	}

	return offer, nil
}

func optText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
