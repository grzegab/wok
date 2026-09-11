package nofluff

import (
	"context"
	"errors"
	"fmt"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

const postingUrl = "https://nofluffjobs.com/api/posting/"
const jobUrl = "https://nofluffjobs.com/pl/job/"

type ExistingOffers interface {
	ExistingPossibilityUrl(ctx context.Context, url string) (string, error)
}

type OfferTransformer struct {
	existingOffers ExistingOffers
	OffersDb       chan database.CreatePossibilityParams
}

func NewOfferTransformer(existingOffers ExistingOffers) *OfferTransformer {
	return &OfferTransformer{
		existingOffers: existingOffers,
		OffersDb:       make(chan database.CreatePossibilityParams, 150),
	}
}

func (ot *OfferTransformer) Transform(ctx context.Context, buffer <-chan RawOffer) {
	defer close(ot.OffersDb)

	for ro := range buffer {
		mainUrl := jobUrl + ro.Url
		apiUrl := postingUrl + ro.Url
		url, err := ot.existingOffers.ExistingPossibilityUrl(ctx, mainUrl)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			// skip error, our loos but wheel keeps spinning
			continue
		}
		if url != "" {
			// already stored, nothing to do
			continue
		}

		offer, err := ot.FillOffer(ctx, ro, apiUrl)
		if err != nil {
			// skip error, our loos but wheel keeps spinning
			continue
		}
		possibility, err := ot.ConvertOffer(offer)
		if err != nil {
			// skip error, our loos but wheel keeps spinning
			continue
		}

		ot.OffersDb <- possibility
	}
}

func (ot *OfferTransformer) FillOffer(ctx context.Context, offer RawOffer, url string) (RawOffer, error) {
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	if err != nil {
		ctx.Done()
		return offer, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ctx.Done()
		return offer, fmt.Errorf("could not send request: %w", err)
	}

	defer resp.Body.Close()

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
		Url:         offer.Url,
		CompanyName: offer.CompanyName,
		JobTitle:    offer.JobTitle,
		Location:    optText(cities(offer.Location)),
		Tech:        optText(requirements(offer.Tech)),
		Description: optText(offer.Description),
		PostedAt:    epochMillisDate(offer.PostedAt),
		Source:      optText(offer.Source),
	}, nil
}

// cities lists the distinct cities an offer is open in, comma separated.
func cities(l Location) string {
	var names []string
	for _, p := range l.Places {
		if p.City != "" && !slices.Contains(names, p.City) {
			names = append(names, p.City)
		}
	}

	return strings.Join(names, ", ")
}

// requirements lists the technologies an offer requires, comma separated like
// the other sources store them. The category tile is not a technology.
func requirements(t Tiles) string {
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
