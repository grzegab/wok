package cleaner

import (
	"context"
	"fmt"
	"slices"
	"strings"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/technology"
	"github.com/jackc/pgx/v5/pgtype"
)

// offerKey is what makes two offers the same one: the same company asking for
// the same technologies in one category, like an offer posted for many cities.
type offerKey struct {
	category string
	company  string
	tech     string
}

// MarkDuplicates keeps the technologies of repeated offers on the oldest one
// only; every later copy gets duplicate as its tech and a link to that offer,
// reporting how many offers it marked. Copies are marked rather than deleted,
// as the next crawl would only bring them back.
func (c *Cleaner) MarkDuplicates(ctx context.Context) (int, error) {
	rows, err := c.store.ListPossibilitiesForDuplicates(ctx)
	if err != nil {
		return 0, fmt.Errorf("could not list possibilities: %w", err)
	}

	originals := map[offerKey]int64{}
	marked := 0
	for _, r := range rows {
		key, ok := duplicateKey(r)
		if !ok {
			continue
		}

		original, seen := originals[key]
		if !seen {
			originals[key] = r.ID
			continue
		}

		err = c.store.MarkPossibilityDuplicate(ctx, database.MarkPossibilityDuplicateParams{
			ID:          r.ID,
			Tech:        pgtype.Text{String: technology.Duplicate, Valid: true},
			DuplicateOf: pgtype.Int8{Int64: original, Valid: true},
		})
		if err != nil {
			return marked, fmt.Errorf("could not mark possibility %d as duplicate of %d: %w", r.ID, original, err)
		}
		marked++
	}

	return marked, nil
}

// duplicateKey ignores the case of the company name and the order of the
// technologies. Offers with no technologies have nothing to count twice, so
// they get no key.
func duplicateKey(r database.ListPossibilitiesForDuplicatesRow) (offerKey, bool) {
	tech := technology.Normalize(r.Tech.String)
	if tech == "" || tech == technology.Duplicate {
		return offerKey{}, false
	}

	names := strings.Split(tech, ", ")
	slices.Sort(names)

	return offerKey{
		category: r.Category,
		company:  strings.ToLower(strings.TrimSpace(r.CompanyName)),
		tech:     strings.Join(names, ", "),
	}, true
}
