package mcpserver

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	database "github.com/grzegab/wok/internal/database/generated"
)

const dateLayout = "2006-01-02"

// The sqlc models keep nullable columns as pgtype values, whose Go structure
// ({Valid, ...}) does not match the JSON they marshal to. The MCP SDK infers a
// tool's output schema from the Go type, so returning them directly would
// advertise a schema the payload never satisfies. These DTOs are the tool-facing
// shape instead. Nullable fields must be pointers: a *time.Time infers as a
// plain (non-nullable) string, so timestamps are only used where the column is
// NOT NULL and nullable dates are carried as *string.
type Application struct {
	ID              int64     `json:"id"`
	CompanyName     string    `json:"company_name"`
	ApplicationDate string    `json:"application_date" jsonschema:"date the CV was sent, YYYY-MM-DD"`
	Status          string    `json:"status" jsonschema:"one of: applied, progress, failed, abandon"`
	HadChat         bool      `json:"had_chat"`
	StartingPrice   *int32    `json:"starting_price" jsonschema:"bottom of the salary range offered by the company"`
	EndingPrice     *int32    `json:"ending_price" jsonschema:"top of the salary range offered by the company"`
	MyPrice         *int32    `json:"my_price" jsonschema:"rate asked for by the applicant"`
	Outcome         *string   `json:"outcome"`
	OfferLink       *string   `json:"offer_link"`
	FixesAfterChat  *string   `json:"fixes_after_chat"`
	Stack           *string   `json:"stack"`
	AdditionalInfo  *string   `json:"additional_info"`
	ThingsToFix     *string   `json:"things_to_fix"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PossibilityListItem struct {
	ID          int64     `json:"id"`
	Url         string    `json:"url"`
	Category    string    `json:"category"`
	CompanyName string    `json:"company_name"`
	JobTitle    string    `json:"job_title"`
	Location    *string   `json:"location"`
	IsRemote    bool      `json:"is_remote"`
	Tech        *string   `json:"tech" jsonschema:"comma-separated technology list"`
	Source      *string   `json:"source" jsonschema:"job board the offer was scraped from"`
	PostedAt    *string   `json:"posted_at" jsonschema:"date the offer was published, YYYY-MM-DD"`
	CreatedAt   time.Time `json:"created_at" jsonschema:"time the offer was scraped"`
}

// Possibility is a full offer including the description, which the list tool omits.
type Possibility struct {
	PossibilityListItem
	Description *string `json:"description"`
}

type DuplicateOffer struct {
	ID       int64   `json:"id"`
	Url      string  `json:"url"`
	JobTitle string  `json:"job_title"`
	Location *string `json:"location"`
	Source   *string `json:"source" jsonschema:"job board the offer was scraped from"`
}

// DuplicateGroup is one offer the cleaner kept together with the copies it
// found of it: the same company asking for the same technologies.
type DuplicateGroup struct {
	CompanyName string           `json:"company_name"`
	Tech        *string          `json:"tech" jsonschema:"comma-separated technology list of the kept offer, the copies had the same one"`
	Original    DuplicateOffer   `json:"original" jsonschema:"offer that keeps its technologies"`
	Duplicates  []DuplicateOffer `json:"duplicates" jsonschema:"copies whose tech was replaced with duplicate"`
}

type TechStat struct {
	Technology    string  `json:"technology"`
	Count         int64   `json:"count" jsonschema:"offers mentioning it in the last days"`
	Share         float64 `json:"share" jsonschema:"percent of the period's offers mentioning it"`
	PreviousCount int64   `json:"previous_count" jsonschema:"offers mentioning it in the period before"`
	PreviousShare float64 `json:"previous_share" jsonschema:"percent of the previous period's offers mentioning it"`
}

type Category struct {
	Slug string `json:"slug" jsonschema:"identifier used in the app URLs"`
	Name string `json:"name" jsonschema:"value stored in the possibilities.category column"`
}

func toApplication(a database.Application) Application {
	return Application{
		ID:              a.ID,
		CompanyName:     a.CompanyName,
		ApplicationDate: dateString(a.ApplicationDate),
		Status:          string(a.Status),
		HadChat:         a.HadChat,
		StartingPrice:   int4Ptr(a.StartingPrice),
		EndingPrice:     int4Ptr(a.EndingPrice),
		MyPrice:         int4Ptr(a.MyPrice),
		Outcome:         textPtr(a.Outcome),
		OfferLink:       textPtr(a.OfferLink),
		FixesAfterChat:  textPtr(a.FixesAfterChat),
		Stack:           textPtr(a.Stack),
		AdditionalInfo:  textPtr(a.AdditionalInfo),
		ThingsToFix:     textPtr(a.ThingsToFix),
		CreatedAt:       a.CreatedAt.Time,
		UpdatedAt:       a.UpdatedAt.Time,
	}
}

func toPossibilityListItem(p database.ListPossibilitiesByCategoryPagedRow) PossibilityListItem {
	return PossibilityListItem{
		ID:          p.ID,
		Url:         p.Url,
		Category:    p.Category,
		CompanyName: p.CompanyName,
		JobTitle:    p.JobTitle,
		Location:    textPtr(p.Location),
		IsRemote:    p.IsRemote,
		Tech:        textPtr(p.Tech),
		Source:      textPtr(p.Source),
		PostedAt:    datePtr(p.PostedAt),
		CreatedAt:   p.CreatedAt.Time,
	}
}

func toPossibility(p database.Possibility) Possibility {
	return Possibility{
		PossibilityListItem: PossibilityListItem{
			ID:          p.ID,
			Url:         p.Url,
			Category:    p.Category,
			CompanyName: p.CompanyName,
			JobTitle:    p.JobTitle,
			Location:    textPtr(p.Location),
			IsRemote:    p.IsRemote,
			Tech:        textPtr(p.Tech),
			Source:      textPtr(p.Source),
			PostedAt:    datePtr(p.PostedAt),
			CreatedAt:   p.CreatedAt.Time,
		},
		Description: textPtr(p.Description),
	}
}

// toDuplicateGroups folds the rows, ordered by the kept offer, into one group
// per kept offer.
func toDuplicateGroups(rows []database.ListDuplicatesByCategoryRow) []DuplicateGroup {
	groups := make([]DuplicateGroup, 0)
	for _, r := range rows {
		if len(groups) == 0 || groups[len(groups)-1].Original.ID != r.OriginalID {
			groups = append(groups, DuplicateGroup{
				CompanyName: r.CompanyName,
				Tech:        textPtr(r.OriginalTech),
				Original: DuplicateOffer{
					ID:       r.OriginalID,
					Url:      r.OriginalUrl,
					JobTitle: r.OriginalJobTitle,
					Location: textPtr(r.OriginalLocation),
					Source:   textPtr(r.OriginalSource),
				},
			})
		}

		group := &groups[len(groups)-1]
		group.Duplicates = append(group.Duplicates, DuplicateOffer{
			ID:       r.ID,
			Url:      r.Url,
			JobTitle: r.JobTitle,
			Location: textPtr(r.Location),
			Source:   textPtr(r.Source),
		})
	}

	return groups
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}

	return &t.String
}

func int4Ptr(i pgtype.Int4) *int32 {
	if !i.Valid {
		return nil
	}

	return &i.Int32
}

func dateString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}

	return d.Time.Format(dateLayout)
}

func datePtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}

	s := d.Time.Format(dateLayout)

	return &s
}
