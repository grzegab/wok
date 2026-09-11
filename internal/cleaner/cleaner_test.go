package cleaner

import (
	"context"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeStore struct {
	rows        []database.ListPossibilitiesWithJsonFieldsRow
	techRows    []database.ListPossibilitiesTechRow
	listErr     error
	updateErr   error
	updated     map[int64]database.UpdatePossibilityLocationTechParams
	techUpdated map[int64]pgtype.Text
}

func (s *fakeStore) ListPossibilitiesWithJsonFields(context.Context) ([]database.ListPossibilitiesWithJsonFieldsRow, error) {
	return s.rows, s.listErr
}

func (s *fakeStore) UpdatePossibilityLocationTech(_ context.Context, arg database.UpdatePossibilityLocationTechParams) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if s.updated == nil {
		s.updated = map[int64]database.UpdatePossibilityLocationTechParams{}
	}
	s.updated[arg.ID] = arg

	return nil
}

func (s *fakeStore) ListPossibilitiesTech(context.Context) ([]database.ListPossibilitiesTechRow, error) {
	return s.techRows, s.listErr
}

func (s *fakeStore) UpdatePossibilityTech(_ context.Context, arg database.UpdatePossibilityTechParams) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if s.techUpdated == nil {
		s.techUpdated = map[int64]pgtype.Text{}
	}
	s.techUpdated[arg.ID] = arg.Tech

	return nil
}

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}
