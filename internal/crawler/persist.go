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

	idx, err := op.offerStore.CreatePossibility(ctx, c)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("%s exists, skipping ...\n", c.Url)
		return
	}
	if err != nil {
		log.Printf("Error creating possibility: %v\n", err)
		return
	}
	log.Printf("Created possibility: %d\n", idx.ID)
}
