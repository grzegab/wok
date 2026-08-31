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

// offerTimeout caps the time spent on a single offer page, so one unexpected
// page cannot stall the whole crawl.
const offerTimeout = 90 * time.Second

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
// used to build a CreatePossibilityParams. The selectors are specific to the
// current DOM structure of the target site and will stop matching if it is
// redesigned, so only the title and the company name are read with Text
// (which waits for its node); every optional field goes through Evaluate,
// which yields an empty string instead of blocking until the timeout.
func scrapeOffer(ctx context.Context, url, category string) (database.CreatePossibilityParams, error) {
	ctx, cancel := context.WithTimeout(ctx, offerTimeout)
	defer cancel()

	var jobTitle string
	var companyName string
	var description string
	var location string
	var body string
	var stack string
	var published string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Text("h1", &jobTitle, chromedp.ByQuery),
		chromedp.Text("h2", &companyName, chromedp.ByQuery),
		chromedp.Text("body", &body, chromedp.ByQuery),
		// The location is the line printed right below the job title
		// ("al. 29 Listopada 20, Krakow"). The surrounding markup is generated
		// class names, so the rendered text is the stable anchor. Evaluate is
		// used rather than Text because a missing node must yield an empty
		// string - a waiting Text selector would stall the whole crawl.
		chromedp.Evaluate(`
				(() => {
					const h1 = document.querySelector('h1');
					if (!h1) return '';
					const title = (h1.innerText || '').trim();
					const lines = document.body.innerText.split('\n').map(l => l.trim()).filter(Boolean);
					const i = lines.indexOf(title);
					return i >= 0 && i + 1 < lines.length ? lines[i + 1] : '';
				})()
			`,
			&location),
		chromedp.Evaluate(`
				(() => {
					const r = document.evaluate(
						"//h3[normalize-space()='Job description']/following-sibling::div",
						document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null
					);
					return r.singleNodeValue ? (r.singleNodeValue.innerText || '').trim() : '';
				})()
			`,
			&description),
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
		chromedp.Evaluate(`
				(() => {
					const m = document.body.innerText.match(/(?:Published|Opublikowano):\s*(\d{2})\.(\d{2})\.(\d{4})/);
					return m ? m[3] + '-' + m[2] + '-' + m[1] : '';
				})()
			`,
			&published),
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
		PostedAt:    optDate(published),
	}

	if strings.Contains(body, "Remote") {
		offer.IsRemote = true
	}

	return offer, nil
}

// optDate parses the ISO date pulled out of the offer page's "Published:"
// line. Offers without a readable date are stored as NULL and sort last.
func optDate(s string) pgtype.Date {
	day, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return pgtype.Date{}
	}

	return pgtype.Date{Time: day, Valid: true}
}

func optText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
