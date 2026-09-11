package cleaner

import (
	"errors"
	"maps"
	"testing"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/technology"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMarkDuplicates(t *testing.T) {
	store := &fakeStore{duplicateRows: []database.ListPossibilitiesForDuplicatesRow{
		{ID: 1, Category: "Go", CompanyName: "EPAM", Tech: text("go, docker")},
		{ID: 2, Category: "Go", CompanyName: "EPAM", Tech: text("go, docker")},
		{ID: 3, Category: "Go", CompanyName: " epam ", Tech: text("docker, go")},
		{ID: 4, Category: "Go", CompanyName: "EPAM", Tech: text("go, kubernetes (k8s)")},
		{ID: 5, Category: "Go", CompanyName: "Allegro", Tech: text("go, docker")},
		{ID: 6, Category: "DevOps", CompanyName: "EPAM", Tech: text("go, docker")},
		{ID: 7, Category: "Go", CompanyName: "EPAM", Tech: text("Golang, Docker")},
		{ID: 8, Category: "Go", CompanyName: "Nobody", Tech: text("")},
		{ID: 9, Category: "Go", CompanyName: "Nobody", Tech: text("")},
		{ID: 10, Category: "Go", CompanyName: "Orphan", Tech: text(technology.Duplicate)},
		{ID: 11, Category: "Go", CompanyName: "Orphan", Tech: text(technology.Duplicate)},
	}}

	marked, err := NewCleaner(store).MarkDuplicates(t.Context())
	if err != nil {
		t.Fatalf("MarkDuplicates() error = %v", err)
	}

	duplicate := func(id, original int64) database.MarkPossibilityDuplicateParams {
		return database.MarkPossibilityDuplicateParams{
			ID:          id,
			Tech:        text(technology.Duplicate),
			DuplicateOf: pgtype.Int8{Int64: original, Valid: true},
		}
	}
	want := map[int64]database.MarkPossibilityDuplicateParams{
		2: duplicate(2, 1),
		3: duplicate(3, 1),
		7: duplicate(7, 1),
	}

	if marked != len(want) {
		t.Errorf("MarkDuplicates() marked %d possibilities, want %d", marked, len(want))
	}

	if !maps.Equal(store.marked, want) {
		t.Errorf("MarkDuplicates() marked %v, want %v (the oldest offer, other companies, "+
			"other stacks, other categories and offers without tech left alone)", store.marked, want)
	}
}

func TestMarkDuplicatesReportsStoreErrors(t *testing.T) {
	storeErr := errors.New("connection lost")
	rows := []database.ListPossibilitiesForDuplicatesRow{
		{ID: 1, Category: "Go", CompanyName: "EPAM", Tech: text("go")},
		{ID: 2, Category: "Go", CompanyName: "EPAM", Tech: text("go")},
	}

	tests := []struct {
		name  string
		store *fakeStore
	}{
		{name: "listing fails", store: &fakeStore{listErr: storeErr}},
		{name: "marking fails", store: &fakeStore{duplicateRows: rows, updateErr: storeErr}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marked, err := NewCleaner(tt.store).MarkDuplicates(t.Context())
			if !errors.Is(err, storeErr) {
				t.Errorf("MarkDuplicates() error = %v, want %v", err, storeErr)
			}
			if marked != 0 {
				t.Errorf("MarkDuplicates() marked %d possibilities, want 0", marked)
			}
		})
	}
}
