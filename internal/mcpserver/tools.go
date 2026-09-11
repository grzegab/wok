package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/grzegab/wok/internal/category"
	"github.com/grzegab/wok/internal/config"
	database "github.com/grzegab/wok/internal/database/generated"
)

const (
	defaultPossibilityLimit = 50
	maxPossibilityLimit     = 200
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

type TechStatsInput struct {
	Category string `json:"category" jsonschema:"category name or slug, as returned by list_categories"`
}

type TechStatsOutput struct {
	Category     string     `json:"category"`
	Technologies []TechStat `json:"technologies" jsonschema:"technologies ordered by how many offers mention them, most frequent first"`
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

func (t *toolset) techStats(ctx context.Context, _ *mcp.CallToolRequest, in TechStatsInput) (*mcp.CallToolResult, TechStatsOutput, error) {
	name, err := resolveCategory(in.Category)
	if err != nil {
		return nil, TechStatsOutput{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	rows, err := t.db.StatsPossibilityByCategory(ctx, name)
	if err != nil {
		return nil, TechStatsOutput{}, fmt.Errorf("counting technologies for %q: %w", name, err)
	}

	technologies := make([]TechStat, 0, len(rows))
	for _, row := range rows {
		technologies = append(technologies, TechStat{Technology: row.Technology, Count: row.Count})
	}

	return nil, TechStatsOutput{Category: name, Technologies: technologies}, nil
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
