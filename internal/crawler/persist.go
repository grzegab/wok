package crawler

import (
	"context"
	"errors"
	"log"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5"
)

type OfferStore interface {
	CheckPossibility(ctx context.Context, url string) (string, error)
	CreatePossibility(ctx context.Context, arg database.CreatePossibilityParams) (database.Possibility, error)
}

type OfferPersist struct {
	offerStore OfferStore
}

func NewOfferPersist(of OfferStore) *OfferPersist {
	return &OfferPersist{
		offerStore: of,
	}
}

func (op *OfferPersist) Persist(ch <-chan database.CreatePossibilityParams) {
	for c := range ch {
		op.persistOne(c)
	}
}

func (op *OfferPersist) persistOne(c database.CreatePossibilityParams) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// check if exists -> url
	_, err := op.offerStore.CheckPossibility(ctx, c.Url)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		idx, err := op.offerStore.CreatePossibility(ctx, c)
		if err != nil {
			log.Printf("Error creating possibility: %v\n", err)
			return
		}
		log.Printf("Created possibility: %d\n", idx.ID)
	case err != nil:
		log.Printf("Error checking possibility: %v\n", err)
	default:
		log.Printf("%s exists, skipping ...\n", c.Url)
	}
}
