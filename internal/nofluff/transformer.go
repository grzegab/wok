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
	"strconv"
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
		Location:    optText(string(offer.Location)),
		Tech:        optText(string(offer.Tech)),
		Description: optText(offer.Description),
		PostedAt:    optDate(strconv.Itoa(offer.PostedAt)),
		Source:      optText(offer.Source),
	}, nil
}

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
