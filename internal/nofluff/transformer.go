package nofluff

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/technology"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const postingUrl = "https://nofluffjobs.com/api/posting/"
const jobUrl = "https://nofluffjobs.com/pl/job/"
const postingTimeout = 30 * time.Second

type ExistingOffers interface {
	ExistingPossibilityUrl(ctx context.Context, url string) (string, error)
}

type OfferTransformer struct {
	existingOffers ExistingOffers
	client         *http.Client
	OffersDb       chan database.CreatePossibilityParams
}

func NewOfferTransformer(existingOffers ExistingOffers) *OfferTransformer {
	return &OfferTransformer{
		existingOffers: existingOffers,
		client:         &http.Client{Timeout: postingTimeout},
		OffersDb:       make(chan database.CreatePossibilityParams, 150),
	}
}

func (ot *OfferTransformer) Transform(ctx context.Context, buffer <-chan RawOffer) {
	defer close(ot.OffersDb)

	for ro := range buffer {
		url, err := ot.existingOffers.ExistingPossibilityUrl(ctx, offerUrl(ro.Url))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("could not check nofluff offer %s: %v", ro.Url, err)
			continue
		}
		if url != "" {
			continue
		}

		offer, err := ot.FillOffer(ctx, ro, postingUrl+ro.Url)
		if err != nil {
			log.Printf("could not fetch nofluff offer %s: %v", ro.Url, err)
			continue
		}
		possibility, err := ot.ConvertOffer(offer)
		if err != nil {
			log.Printf("could not convert nofluff offer %s: %v", ro.Url, err)
			continue
		}

		ot.OffersDb <- possibility
	}
}

func (ot *OfferTransformer) FillOffer(ctx context.Context, offer RawOffer, url string) (RawOffer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return offer, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ot.client.Do(req)
	if err != nil {
		return offer, fmt.Errorf("could not send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return offer, fmt.Errorf("posting answered with %s", resp.Status)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return offer, fmt.Errorf("could not read response body: %w", err)
	}
	offer.Description = string(raw)

	return offer, nil
}

func (ot *OfferTransformer) ConvertOffer(offer RawOffer) (database.CreatePossibilityParams, error) {
	return database.CreatePossibilityParams{
		Category:    offer.Category,
		Url:         offerUrl(offer.Url),
		CompanyName: offer.CompanyName,
		JobTitle:    offer.JobTitle,
		Location:    optText(offer.Location.Cities()),
		IsRemote:    offer.Location.FullyRemote,
		Tech:        optText(technology.Normalize(offer.Tech.Requirements())),
		Description: optText(offer.Description),
		PostedAt:    epochMillisDate(offer.PostedAt),
		Source:      optText(offer.Source),
	}, nil
}

func offerUrl(slug string) string {
	return jobUrl + slug
}

// Cities lists the distinct cities an offer is open in, comma separated.
func (l Location) Cities() string {
	var names []string
	for _, p := range l.Places {
		if p.City != "" && !slices.Contains(names, p.City) {
			names = append(names, p.City)
		}
	}

	return strings.Join(names, ", ")
}

// Requirements lists the technologies an offer requires, comma separated like
// the other sources store them. The category tile is not a technology.
func (t Tiles) Requirements() string {
	var techs []string
	for _, v := range t.Values {
		if v.Type == "requirement" {
			techs = append(techs, v.Value)
		}
	}

	return strings.Join(techs, ", ")
}

// epochMillisDate converts the "posted" field, a Unix epoch in milliseconds,
// to a date. Offers without a posted time are stored as NULL.
func epochMillisDate(ms int) pgtype.Date {
	if ms <= 0 {
		return pgtype.Date{}
	}

	return pgtype.Date{Time: time.UnixMilli(int64(ms)).UTC(), Valid: true}
}

func optText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}

	return pgtype.Text{String: s, Valid: true}
}
