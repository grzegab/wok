package nofluff

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"github.com/grzegab/wok/internal/category"
	"log"
	"math/rand/v2"
	"net/http"
	"time"
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
		body, err := json.Marshal(searchRequest{RawSearch: c.SearchTerm})
		if err != nil {
			return
		}

		// To set custom headers, use [NewRequest] and DefaultClient.Do.
		req, err := http.NewRequest("POST", c.ScrapeURL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}

		defer resp.Body.Close()

		dec := jsontext.NewDecoder(resp.Body)
		dec.ReadToken()

		for dec.PeekKind() != '}' {
			key, _ := dec.ReadToken()
			if key.String() == "postings" {
				dec.ReadToken() // '['
				for dec.PeekKind() != ']' {
					var o RawOffer
					json.UnmarshalDecode(dec, &o)
					o.Category = c.Name

					select {
					case of.RawOfferBuff <- o:
					case <-ctx.Done():
						return
					}
				}
				dec.ReadToken() // ']'
			} else {
				dec.SkipValue()
			}
		}
	}
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
