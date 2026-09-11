package cleaner

import (
	"errors"
	"testing"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestFixJsonFields(t *testing.T) {
	store := &fakeStore{rows: []database.ListPossibilitiesWithJsonFieldsRow{
		{
			ID: 1,
			Location: text(`{"places":[{"country":{"code":"POL","name":"Poland"},"city":"Wrocław","street":"",` +
				`"postalCode":"","geoLocation":{"latitude":51.1,"longitude":17.0},"url":"x"}],"fullyRemote":false}`),
			Tech: text(`{"values":[{"value":"business-analyst","type":"category"},` +
				`{"value":"C++","type":"requirement"},{"value":"Java","type":"requirement"}]}`),
		},
		{
			ID:       2,
			Location: text("Kraków"),
			Tech:     text(`{"values":[{"value":"data","type":"category"}]}`),
		},
		{
			ID:       3,
			Location: text(`{"places":[{"city":"Remote"},{"province":"lower-silesian","provinceOnly":true}]}`),
			Tech:     pgtype.Text{},
		},
		{
			ID:       4,
			Location: text(`{"places":[{"city":`),
			Tech:     text(`{"values":[{"value":"Go","type":"requirement"}]}`),
		},
	}}

	fixed, err := NewCleaner(store).FixJsonFields(t.Context())
	if err != nil {
		t.Fatalf("FixJsonFields() error = %v", err)
	}

	if fixed != 3 {
		t.Errorf("FixJsonFields() fixed %d possibilities, want 3", fixed)
	}

	want := map[int64]database.UpdatePossibilityLocationTechParams{
		1: {ID: 1, Location: text("Wrocław"), Tech: text("C++, Java")},
		2: {ID: 2, Location: text("Kraków"), Tech: pgtype.Text{}},
		3: {ID: 3, Location: text("Remote"), Tech: pgtype.Text{}},
	}

	for id, w := range want {
		if got := store.updated[id]; got != w {
			t.Errorf("possibility %d updated to %+v, want %+v", id, got, w)
		}
	}

	if got, ok := store.updated[4]; ok {
		t.Errorf("possibility 4 with broken JSON was updated to %+v, want it left alone", got)
	}
}

func TestFixJsonFieldsReportsStoreErrors(t *testing.T) {
	storeErr := errors.New("connection lost")
	row := database.ListPossibilitiesWithJsonFieldsRow{ID: 1, Location: text(`{"places":[]}`)}

	tests := []struct {
		name  string
		store *fakeStore
	}{
		{name: "listing fails", store: &fakeStore{listErr: storeErr}},
		{name: "updating fails", store: &fakeStore{rows: []database.ListPossibilitiesWithJsonFieldsRow{row}, updateErr: storeErr}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed, err := NewCleaner(tt.store).FixJsonFields(t.Context())
			if !errors.Is(err, storeErr) {
				t.Errorf("FixJsonFields() error = %v, want %v", err, storeErr)
			}
			if fixed != 0 {
				t.Errorf("FixJsonFields() fixed %d possibilities, want 0", fixed)
			}
		})
	}
}
