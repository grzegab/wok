package cleaner

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"strings"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/nofluff"
	"github.com/jackc/pgx/v5/pgtype"
)

// FixJsonFields rewrites the locations and technologies nofluff offers used to
// be stored with as raw JSON into the comma separated lists the crawler stores
// now, reporting how many possibilities it fixed. A possibility that cannot be
// fixed is logged and left as it is, so it does not hold up the rest.
func (c *Cleaner) FixJsonFields(ctx context.Context) (int, error) {
	rows, err := c.store.ListPossibilitiesWithJsonFields(ctx)
	if err != nil {
		return 0, fmt.Errorf("could not list possibilities: %w", err)
	}

	fixed := 0
	for _, r := range rows {
		location, err := fromJson(r.Location, nofluff.Location.Cities)
		if err != nil {
			log.Printf("Skipping possibility %d, could not decode its location: %v\n", r.ID, err)
			continue
		}

		tech, err := fromJson(r.Tech, nofluff.Tiles.Requirements)
		if err != nil {
			log.Printf("Skipping possibility %d, could not decode its tech: %v\n", r.ID, err)
			continue
		}

		err = c.store.UpdatePossibilityLocationTech(ctx, database.UpdatePossibilityLocationTechParams{
			ID:       r.ID,
			Location: location,
			Tech:     tech,
		})
		if err != nil {
			return fixed, fmt.Errorf("could not update possibility %d: %w", r.ID, err)
		}
		fixed++
	}

	return fixed, nil
}

// fromJson turns a field stored as raw JSON into the list it stands for.
// Fields that already hold a list are returned untouched.
func fromJson[T any](field pgtype.Text, list func(T) string) (pgtype.Text, error) {
	if !strings.HasPrefix(field.String, "{") {
		return field, nil
	}

	var v T
	if err := json.Unmarshal([]byte(field.String), &v); err != nil {
		return field, err
	}

	s := list(v)
	return pgtype.Text{String: s, Valid: s != ""}, nil
}
