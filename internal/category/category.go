package category

// Category is one job-offer category: how it is stored in the possibilities
// table (Name), how it appears in URLs (Slug) and where the crawler scrapes
// its offers from (ScrapeURL).
type Category struct {
	Slug      string
	Name      string
	ScrapeURL string
}

type NoFluffCategory struct {
	Category
	SearchTerm string
}
