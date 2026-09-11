package cleaner

import (
	"context"

	database "github.com/grzegab/wok/internal/database/generated"
)

type PossibilityStore interface {
	ListPossibilitiesWithJsonFields(ctx context.Context) ([]database.ListPossibilitiesWithJsonFieldsRow, error)
	UpdatePossibilityLocationTech(ctx context.Context, arg database.UpdatePossibilityLocationTechParams) error
	ListPossibilitiesTech(ctx context.Context) ([]database.ListPossibilitiesTechRow, error)
	UpdatePossibilityTech(ctx context.Context, arg database.UpdatePossibilityTechParams) error
	ListPossibilitiesForDuplicates(ctx context.Context) ([]database.ListPossibilitiesForDuplicatesRow, error)
	MarkPossibilityDuplicate(ctx context.Context, arg database.MarkPossibilityDuplicateParams) error
}

type Cleaner struct {
	store PossibilityStore
}

func NewCleaner(store PossibilityStore) *Cleaner {
	return &Cleaner{store: store}
}
