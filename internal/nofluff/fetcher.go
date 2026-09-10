package nofluff

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/grzegab/wok/internal/category"
)

const frequency = 4 * time.Hour
const maxFrequencyJitter = 1 * time.Hour

type RawOffer struct {
	Url         string         `json:"url"`
	CompanyName string         `json:"name"`
	JobTitle    string         `json:"title"`
	Location    jsontext.Value `json:"location"`
	IsRemote    bool           `json:"fullyRemote"`
	MainTech    jsontext.Value `json:"technology"`
	Tech        jsontext.Value `json:"tiles"`
	Source      string         `json:"source"`
	PostedAt    int            `json:"posted"`
	Category    string
	Description string
}

type Fetcher struct {
	categories   []category.NoFluffCategory
	RawOfferBuff chan RawOffer
}

// NewNoFluffFetcher gives all URLs for categories to fetch.
func NewNoFluffFetcher(categories []category.NoFluffCategory) *Fetcher {
	return &Fetcher{
		categories:   categories,
		RawOfferBuff: make(chan RawOffer, 150),
	}
}

func (of *Fetcher) FetchOffers(ctx context.Context) {
	wait := frequency + rand.N(maxFrequencyJitter)

	for {
		of.fetchAll(ctx)
		log.Printf("Checking nofluff again in %s\n", wait)

		if !pause(ctx, wait) {
			close(of.RawOfferBuff)
			return
		}
	}
}

type searchRequest struct {
	RawSearch string `json:"rawSearch"`
}

func (of *Fetcher) fetchAll(ctx context.Context) {
	for _, c := range of.categories {
		of.fetchCategory(ctx, c)

		if ctx.Err() != nil {
			return
		}
	}
}

// fetchCategory walks a category's listing page by page. The listing reports
// how many pages it has, so that count normally ends the walk; a page that
// carries no offers ends it too, in case the count is missing or the listing
// shrinks while it is being read.
func (of *Fetcher) fetchCategory(ctx context.Context, c category.NoFluffCategory) {
	for page := 1; ; page++ {
		offers, totalPages, err := of.fetchPage(ctx, c, page)
		if err != nil {
			log.Printf("Stopping %s at page %d: %v\n", c.SearchTerm, page, err)
			return
		}

		log.Printf("Fetched %d %s offers from page %d\n", offers, c.SearchTerm, page)

		if offers == 0 || (totalPages > 0 && page >= totalPages) {
			return
		}
	}
}

// fetchPage streams one page of a category's listing into RawOfferBuff and
// reports how many offers it carried along with the listing's page count.
func (of *Fetcher) fetchPage(ctx context.Context, c category.NoFluffCategory, page int) (int, int, error) {
	requestUrl, err := pageUrl(c.ScrapeURL, page)
	if err != nil {
		return 0, 0, err
	}

	body, err := json.Marshal(searchRequest{RawSearch: c.SearchTerm})
	if err != nil {
		return 0, 0, fmt.Errorf("could not encode search: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, bytes.NewReader(body))
	if err != nil {
		return 0, 0, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("could not send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("listing answered with %s", resp.Status)
	}

	return of.readListing(ctx, resp.Body, c.Name)
}

// readListing decodes the listing as it arrives rather than buffering it, so
// offers reach the transformer while the rest of the page is still coming in.
func (of *Fetcher) readListing(ctx context.Context, body io.Reader, categoryName string) (int, int, error) {
	var offers, totalPages int

	dec := jsontext.NewDecoder(body)
	if _, err := dec.ReadToken(); err != nil { // '{'
		return offers, totalPages, fmt.Errorf("could not read listing: %w", err)
	}

	for dec.PeekKind() != '}' {
		key, err := dec.ReadToken()
		if err != nil {
			return offers, totalPages, fmt.Errorf("could not read listing: %w", err)
		}

		// The token is only valid until the decoder moves on, so the name is
		// copied out before the value below is read.
		name := key.String()

		switch name {
		case "postings":
			offers, err = of.readPostings(ctx, dec, categoryName)
		case "totalPages":
			err = json.UnmarshalDecode(dec, &totalPages)
		default:
			err = dec.SkipValue()
		}

		if err != nil {
			return offers, totalPages, fmt.Errorf("could not read %s: %w", name, err)
		}
	}

	return offers, totalPages, nil
}

// readPostings hands every offer of the "postings" array to the transformer,
// returning how many made it through.
func (of *Fetcher) readPostings(ctx context.Context, dec *jsontext.Decoder, categoryName string) (int, error) {
	offers := 0

	if _, err := dec.ReadToken(); err != nil { // '['
		return offers, err
	}

	for dec.PeekKind() != ']' {
		var o RawOffer
		if err := json.UnmarshalDecode(dec, &o); err != nil {
			return offers, err
		}
		o.Category = categoryName

		// A blocking send would keep the fetcher here once the buffer fills
		// up, so shutdown would stall behind a full channel.
		select {
		case of.RawOfferBuff <- o:
			offers++
		case <-ctx.Done():
			return offers, ctx.Err()
		}
	}

	if _, err := dec.ReadToken(); err != nil { // ']'
		return offers, err
	}

	return offers, nil
}

// pageUrl points a listing URL at one page of its results.
func pageUrl(listingUrl string, page int) (string, error) {
	u, err := url.Parse(listingUrl)
	if err != nil {
		return "", fmt.Errorf("could not parse listing url: %w", err)
	}

	query := u.Query()
	query.Set("page", strconv.Itoa(page))
	u.RawQuery = query.Encode()

	return u.String(), nil
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
