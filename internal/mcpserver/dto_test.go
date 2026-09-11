package mcpserver

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	database "github.com/grzegab/wok/internal/database/generated"
)

func TestToDuplicateGroupsGroupsCopiesUnderTheKeptOffer(t *testing.T) {
	rows := []database.ListDuplicatesByCategoryRow{
		{OriginalID: 1, CompanyName: "EPAM", OriginalJobTitle: "Go Developer", OriginalTech: pgtype.Text{String: "go", Valid: true}, ID: 4, JobTitle: "Go Developer"},
		{OriginalID: 1, CompanyName: "EPAM", OriginalJobTitle: "Go Developer", OriginalTech: pgtype.Text{String: "go", Valid: true}, ID: 7, JobTitle: "Go Developer"},
		{OriginalID: 2, CompanyName: "Allegro", OriginalJobTitle: "Backend Engineer", ID: 5, JobTitle: "Backend Engineer"},
	}

	groups := toDuplicateGroups(rows)

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}

	epam := groups[0]
	if epam.Original.ID != 1 || epam.CompanyName != "EPAM" || epam.Tech == nil || *epam.Tech != "go" {
		t.Errorf("first group = %+v, want the EPAM offer 1 keeping tech go", epam)
	}
	if len(epam.Duplicates) != 2 || epam.Duplicates[0].ID != 4 || epam.Duplicates[1].ID != 7 {
		t.Errorf("first group duplicates = %+v, want offers 4 and 7", epam.Duplicates)
	}

	allegro := groups[1]
	if allegro.Original.ID != 2 || len(allegro.Duplicates) != 1 || allegro.Duplicates[0].ID != 5 {
		t.Errorf("second group = %+v, want offer 2 with the copy 5", allegro)
	}
}

func TestToDuplicateGroupsWithoutDuplicatesIsEmptyNotNil(t *testing.T) {
	groups := toDuplicateGroups(nil)

	if groups == nil || len(groups) != 0 {
		t.Errorf("got %#v, want an empty list so the output is [] rather than null", groups)
	}
}
