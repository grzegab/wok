package cleaner

import (
	"context"
	"fmt"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/technology"
	"github.com/jackc/pgx/v5/pgtype"
)

// NormalizeTech rewrites stored technology lists into the normalized form the
// crawlers store now, reporting how many possibilities it changed. Lists that
// are already normalized are not written again.
func (c *Cleaner) NormalizeTech(ctx context.Context) (int, error) {
	rows, err := c.store.ListPossibilitiesTech(ctx)
	if err != nil {
		return 0, fmt.Errorf("could not list possibilities: %w", err)
	}

	changed := 0
	for _, r := range rows {
		normalized := technology.Normalize(r.Tech.String)
		if normalized == r.Tech.String {
			continue
		}

		err = c.store.UpdatePossibilityTech(ctx, database.UpdatePossibilityTechParams{
			ID:   r.ID,
			Tech: pgtype.Text{String: normalized, Valid: normalized != ""},
		})
		if err != nil {
			return changed, fmt.Errorf("could not update possibility %d: %w", r.ID, err)
		}
		changed++
	}

	return changed, nil
}
