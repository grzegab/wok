package category

// Category is one job-offer category: how it is stored in the possibilities
// table (Name), how it appears in URLs (Slug) and where the crawler scrapes
// its offers from (ScrapeURL).
type Category struct {
	Slug      string
	Name      string
	ScrapeURL string
}

// scrapeParams keeps the offer list sorted newest first, so the crawler sees
// fresh offers on every pass.
const scrapeParams = "?orderBy=DESC&sortBy=newest"

// All is the single source of truth for the categories shown in the menu and
// crawled by cmd/crawler.
var All = []Category{
	{Slug: "go", Name: "Go", ScrapeURL: "https://justjoin.it/job-offers/all-locations/go" + scrapeParams},
	{Slug: "php", Name: "PHP", ScrapeURL: "https://justjoin.it/job-offers/all-locations/php" + scrapeParams},
	{Slug: "architect", Name: "Software Architect", ScrapeURL: "https://justjoin.it/job-offers/all-locations/architecture" + scrapeParams},
	{Slug: "ai-ml", Name: "AI/ML", ScrapeURL: "https://justjoin.it/job-offers/all-locations/ai" + scrapeParams},
	{Slug: "c", Name: "C/C++", ScrapeURL: "https://justjoin.it/job-offers/all-locations/c" + scrapeParams},
}

// BySlug returns the category addressed by a URL slug.
func BySlug(slug string) (Category, bool) {
	for _, c := range All {
		if c.Slug == slug {
			return c, true
		}
	}

	return Category{}, false
}
