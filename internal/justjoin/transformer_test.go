package justjoin

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestOptDate(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		valid bool
		want  string
	}{
		{"iso date", "2026-08-31", true, "2026-08-31"},
		{"padded", " 2026-01-05 ", true, "2026-01-05"},
		{"empty means unknown", "", false, ""},
		{"site format is not accepted", "31.08.2026", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optDate(tt.in)

			if got.Valid != tt.valid {
				t.Fatalf("optDate(%q).Valid = %v, want %v", tt.in, got.Valid, tt.valid)
			}
			if tt.valid && got.Time.Format("2006-01-02") != tt.want {
				t.Fatalf("optDate(%q) = %s, want %s", tt.in, got.Time.Format("2006-01-02"), tt.want)
			}
		})
	}
}

type stubKnownOffers struct {
	known []string
	err   error
}

func (s stubKnownOffers) ExistingPossibilityUrls(context.Context, []string) ([]string, error) {
	return s.known, s.err
}

func TestUnseen(t *testing.T) {
	all := []string{"a", "b", "c"}

	tests := []struct {
		name  string
		store stubKnownOffers
		urls  []string
		want  []string
	}{
		{"keeps the order of the new offers", stubKnownOffers{known: []string{"b"}}, all, []string{"a", "c"}},
		{"drops the whole batch when nothing is new", stubKnownOffers{known: all}, all, []string{}},
		{"keeps everything when nothing is stored", stubKnownOffers{}, all, all},
		{"keeps everything when the lookup fails", stubKnownOffers{err: errors.New("down")}, all, all},
		{"empty batch needs no lookup", stubKnownOffers{}, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ot := NewOfferTransformer(tt.store)

			got := ot.unseen(context.Background(), FetchedOffers{Category: "Go", Urls: tt.urls})

			if !slices.Equal(got, tt.want) {
				t.Fatalf("unseen() = %v, want %v", got, tt.want)
			}
		})
	}
}
