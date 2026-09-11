package cleaner

import (
	"errors"
	"maps"
	"testing"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestNormalizeTech(t *testing.T) {
	store := &fakeStore{techRows: []database.ListPossibilitiesTechRow{
		{ID: 1, Tech: text("React, JavaScript, TypeScript")},
		{ID: 2, Tech: text("C++, Java, Python")},
		{ID: 3, Tech: text("go, docker")},
		{ID: 4, Tech: text(" , ")},
	}}

	changed, err := NewCleaner(store).NormalizeTech(t.Context())
	if err != nil {
		t.Fatalf("NormalizeTech() error = %v", err)
	}

	want := map[int64]pgtype.Text{
		1: text("react, typescript/javascript"),
		2: text("c++, java, python"),
		4: {},
	}

	if changed != len(want) {
		t.Errorf("NormalizeTech() changed %d possibilities, want %d", changed, len(want))
	}

	if !maps.Equal(store.techUpdated, want) {
		t.Errorf("NormalizeTech() updated %v, want %v (already normalized ones left alone)", store.techUpdated, want)
	}
}

func TestNormalizeTechReportsStoreErrors(t *testing.T) {
	storeErr := errors.New("connection lost")
	row := database.ListPossibilitiesTechRow{ID: 1, Tech: text("Go")}

	tests := []struct {
		name  string
		store *fakeStore
	}{
		{name: "listing fails", store: &fakeStore{listErr: storeErr}},
		{name: "updating fails", store: &fakeStore{techRows: []database.ListPossibilitiesTechRow{row}, updateErr: storeErr}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed, err := NewCleaner(tt.store).NormalizeTech(t.Context())
			if !errors.Is(err, storeErr) {
				t.Errorf("NormalizeTech() error = %v, want %v", err, storeErr)
			}
			if changed != 0 {
				t.Errorf("NormalizeTech() changed %d possibilities, want 0", changed)
			}
		})
	}
}
