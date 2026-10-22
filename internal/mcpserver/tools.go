package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/grzegab/wok/internal/category"
	"github.com/grzegab/wok/internal/config"
	database "github.com/grzegab/wok/internal/database/generated"
)

const (
	defaultPossibilityLimit = 50
	maxPossibilityLimit     = 200
	defaultTechStatsDays    = 30
	maxTechStatsDays        = 365
)

type toolset struct {
	db *database.Queries
}

func newToolset(db *database.Queries) *toolset {
	return &toolset{db: db}
}

type NoInput struct{}

type CategoriesOutput struct {
	Categories []Category `json:"categories"`
}

type ApplicationsOutput struct {
	Applications []Application `json:"applications"`
	Count        int           `json:"count"`
}

type ApplicationInput struct {
	ID int64 `json:"id" jsonschema:"id of the application, as returned by list_applications"`
}

type PossibilitiesInput struct {
	Category string `json:"category" jsonschema:"category name or slug, as returned by list_categories"`
	Limit    int32  `json:"limit,omitempty" jsonschema:"how many offers to return, 1-200, defaults to 50"`
	Offset   int32  `json:"offset,omitempty" jsonschema:"how many offers to skip, for paging through the list"`
}

type PossibilitiesOutput struct {
	Category      string                `json:"category"`
	Possibilities []PossibilityListItem `json:"possibilities"`
	Count         int                   `json:"count" jsonschema:"number of offers in this page, not the total for the category"`
}

type PossibilityInput struct {
	ID int64 `json:"id" jsonschema:"id of the offer, as returned by list_possibilities"`
}

type DuplicatesInput struct {
	Category string `json:"category" jsonschema:"category name or slug, as returned by list_categories"`
}

type DuplicatesOutput struct {
	Category string           `json:"category"`
	Groups   []DuplicateGroup `json:"groups" jsonschema:"kept offers with their copies, oldest kept offer first"`
	Count    int              `json:"count" jsonschema:"number of offers marked as duplicates in the category"`
}

type TechStatsInput struct {
	Category string `json:"category" jsonschema:"category name or slug, as returned by list_categories"`
	Days     int    `json:"days,omitempty" jsonschema:"length of the period in days, 1-365, defaults to 30; it is compared with the same-length period right before it"`
}

type TechStatsOutput struct {
	Category      string     `json:"category"`
	Days          int        `json:"days"`
	OffersCount   int64      `json:"offers_count" jsonschema:"offers scraped in the period, duplicates excluded"`
	PreviousCount int64      `json:"previous_offers_count" jsonschema:"offers scraped in the previous period, duplicates excluded"`
	Technologies  []TechStat `json:"technologies" jsonschema:"technologies ordered by how many offers in the period mention them, most frequent first"`
}

func (t *toolset) listCategories(_ context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, CategoriesOutput, error) {
	categories := make([]Category, 0, len(category.JustJoinAll))
	for _, c := range category.JustJoinAll {
		categories = append(categories, Category{Slug: c.Slug, Name: c.Name})
	}

	return nil, CategoriesOutput{Categories: categories}, nil
}

func (t *toolset) listApplications(ctx context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, ApplicationsOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	rows, err := t.db.ListApplications(ctx)
	if err != nil {
		return nil, ApplicationsOutput{}, fmt.Errorf("listing applications: %w", err)
	}

	applications := make([]Application, 0, len(rows))
	for _, row := range rows {
		applications = append(applications, toApplication(row))
	}

	return nil, ApplicationsOutput{Applications: applications, Count: len(applications)}, nil
}

func (t *toolset) getApplication(ctx context.Context, _ *mcp.CallToolRequest, in ApplicationInput) (*mcp.CallToolResult, Application, error) {
	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	row, err := t.db.GetApplication(ctx, in.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, Application{}, fmt.Errorf("no application with id %d", in.ID)
	}
	if err != nil {
		return nil, Application{}, fmt.Errorf("getting application %d: %w", in.ID, err)
	}

	return nil, toApplication(row), nil
}

func (t *toolset) listPossibilities(ctx context.Context, _ *mcp.CallToolRequest, in PossibilitiesInput) (*mcp.CallToolResult, PossibilitiesOutput, error) {
	name, err := resolveCategory(in.Category)
	if err != nil {
		return nil, PossibilitiesOutput{}, err
	}

	limit, err := resolveLimit(in.Limit)
	if err != nil {
		return nil, PossibilitiesOutput{}, err
	}

	if in.Offset < 0 {
		return nil, PossibilitiesOutput{}, fmt.Errorf("offset must not be negative, got %d", in.Offset)
	}

	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	rows, err := t.db.ListPossibilitiesByCategoryPaged(ctx, database.ListPossibilitiesByCategoryPagedParams{
		Category: name,
		Limit:    limit,
		Offset:   in.Offset,
	})
	if err != nil {
		return nil, PossibilitiesOutput{}, fmt.Errorf("listing offers for %q: %w", name, err)
	}

	possibilities := make([]PossibilityListItem, 0, len(rows))
	for _, row := range rows {
		possibilities = append(possibilities, toPossibilityListItem(row))
	}

	return nil, PossibilitiesOutput{
		Category:      name,
		Possibilities: possibilities,
		Count:         len(possibilities),
	}, nil
}

func (t *toolset) getPossibility(ctx context.Context, _ *mcp.CallToolRequest, in PossibilityInput) (*mcp.CallToolResult, Possibility, error) {
	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	row, err := t.db.GetPossibility(ctx, in.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, Possibility{}, fmt.Errorf("no offer with id %d", in.ID)
	}
	if err != nil {
		return nil, Possibility{}, fmt.Errorf("getting offer %d: %w", in.ID, err)
	}

	return nil, toPossibility(row), nil
}

func (t *toolset) listDuplicates(ctx context.Context, _ *mcp.CallToolRequest, in DuplicatesInput) (*mcp.CallToolResult, DuplicatesOutput, error) {
	name, err := resolveCategory(in.Category)
	if err != nil {
		return nil, DuplicatesOutput{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	rows, err := t.db.ListDuplicatesByCategory(ctx, name)
	if err != nil {
		return nil, DuplicatesOutput{}, fmt.Errorf("listing duplicates for %q: %w", name, err)
	}

	return nil, DuplicatesOutput{Category: name, Groups: toDuplicateGroups(rows), Count: len(rows)}, nil
}

func (t *toolset) techStats(ctx context.Context, _ *mcp.CallToolRequest, in TechStatsInput) (*mcp.CallToolResult, TechStatsOutput, error) {
	name, err := resolveCategory(in.Category)
	if err != nil {
		return nil, TechStatsOutput{}, err
	}

	days := in.Days
	if days == 0 {
		days = defaultTechStatsDays
	}
	if days < 1 || days > maxTechStatsDays {
		return nil, TechStatsOutput{}, fmt.Errorf("days must be between 1 and %d, got %d", maxTechStatsDays, in.Days)
	}

	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	currentStart := time.Now().AddDate(0, 0, -days)
	rows, err := t.db.TechTrendByCategory(ctx, database.TechTrendByCategoryParams{
		Category:      name,
		CurrentStart:  pgtype.Timestamptz{Time: currentStart, Valid: true},
		PreviousStart: pgtype.Timestamptz{Time: currentStart.AddDate(0, 0, -days), Valid: true},
	})
	if err != nil {
		return nil, TechStatsOutput{}, fmt.Errorf("counting technologies for %q: %w", name, err)
	}

	out := TechStatsOutput{Category: name, Days: days, Technologies: make([]TechStat, 0, len(rows))}
	for _, row := range rows {
		out.OffersCount, out.PreviousCount = row.CurrentTotal, row.PreviousTotal
		out.Technologies = append(out.Technologies, TechStat{
			Technology:    row.Technology,
			Count:         row.CurrentCount,
			Share:         row.CurrentShare,
			PreviousCount: row.PreviousCount,
			PreviousShare: row.PreviousShare,
		})
	}

	return nil, out, nil
}

// resolveCategory maps whatever the model passed onto the value actually stored
// in possibilities.category, which is the display name ("C/C++"), not the slug.
func resolveCategory(value string) (string, error) {
	for _, c := range category.JustJoinAll {
		if c.Name == value || c.Slug == value {
			return c.Name, nil
		}
	}

	known := make([]string, 0, len(category.JustJoinAll))
	for _, c := range category.JustJoinAll {
		known = append(known, c.Name)
	}

	return "", fmt.Errorf("unknown category %q, expected one of: %s", value, strings.Join(known, ", "))
}

func resolveLimit(limit int32) (int32, error) {
	if limit == 0 {
		return defaultPossibilityLimit, nil
	}

	if limit < 0 || limit > maxPossibilityLimit {
		return 0, fmt.Errorf("limit must be between 1 and %d, got %d", maxPossibilityLimit, limit)
	}

	return limit, nil
}
