package crawler

import (
	"context"
	"log"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
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

	saved, err := op.offerStore.CreatePossibility(ctx, c)
	if err != nil {
		log.Printf("Error saving possibility: %v\n", err)
		return
	}
	log.Printf("Saved possibility: %d (%s)\n", saved.ID, c.Url)
}
