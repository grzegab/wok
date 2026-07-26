package crawler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

const category = "Go"

type OfferTransformer struct {
	Offers chan database.CreatePossibilityParams
}

func NewOfferTransformer() *OfferTransformer {
	return &OfferTransformer{
		Offers: make(chan database.CreatePossibilityParams),
	}
}

func (ot *OfferTransformer) Transform(ctx context.Context, ch <-chan []string) {
	ctx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	for urls := range ch {
		for i, url := range urls {
			fmt.Printf("[%d/%d] Scraping: %s\n", i+1, len(urls), url)
			// Be polite to the server with a short pause between requests
			baseSecond := 65 * time.Second
			randSecond := time.Second * time.Duration(time.Now().UnixNano()%60)
			log.Println("Sleeping set for ", baseSecond+randSecond)

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
				chromedp.InnerHTML(
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
				fmt.Printf("Failed to scrape %s: %v\n", url, err)
				continue
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

			ot.Offers <- offer

			time.Sleep(baseSecond + randSecond)
		}
	}
}

func optText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
