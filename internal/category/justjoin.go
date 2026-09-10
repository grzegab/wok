package category

// justJoinScrapeParams keeps the offer list sorted newest first, so the crawler sees
// fresh offers on every pass.
const justJoinScrapeParams = "?orderBy=DESC&sortBy=newest"

const justJoinMainUrl = "https://justjoin.it/job-offers/all-locations"

// JustJoinAll is the single source of truth for the categories shown in the menu and
// crawled by cmd/crawler.
var JustJoinAll = []Category{
	{Slug: "go", Name: "Go", ScrapeURL: justJoinMainUrl + "/go" + justJoinScrapeParams},
	{Slug: "php", Name: "PHP", ScrapeURL: justJoinMainUrl + "/php" + justJoinScrapeParams},
	{Slug: "architect", Name: "Software Architect", ScrapeURL: justJoinMainUrl + "/architecture" + justJoinScrapeParams},
	{Slug: "ai-ml", Name: "AI/ML", ScrapeURL: justJoinMainUrl + "/ai" + justJoinScrapeParams},
	{Slug: "c", Name: "C/C++", ScrapeURL: justJoinMainUrl + "/c" + justJoinScrapeParams},
}

// JustJoinBySlug returns the category addressed by a URL slug.
func JustJoinBySlug(slug string) (Category, bool) {
	for _, c := range JustJoinAll {
		if c.Slug == slug {
			return c, true
		}
	}

	return Category{}, false
}
