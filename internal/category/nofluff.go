package category

const noFluffScrapeParams = "?salaryCurrency=PLN&salaryPeriod=month&region=pl&language=pl-PL&page=1"

const noFluffMainUrl = "https://nofluffjobs.com/api/search/posting"

var NoFluffAll = []NoFluffCategory{
	{Slug: "go", Name: "Go", ScrapeURL: noFluffMainUrl + noFluffScrapeParams, SearchTerm: "go"},
	{Slug: "golang", Name: "Go", ScrapeURL: noFluffMainUrl + noFluffScrapeParams, SearchTerm: "golang"},
	{Slug: "php", Name: "PHP", ScrapeURL: noFluffMainUrl + noFluffScrapeParams, SearchTerm: "php"},
	{Slug: "architect", Name: "Software Architect", ScrapeURL: noFluffMainUrl + noFluffScrapeParams, SearchTerm: "architecture"},
	{Slug: "ai-ml", Name: "AI/ML", ScrapeURL: noFluffMainUrl + noFluffScrapeParams, SearchTerm: "artificial-intelligence"},
	{Slug: "c", Name: "C/C++", ScrapeURL: noFluffMainUrl + noFluffScrapeParams, SearchTerm: "c"},
	{Slug: "c++", Name: "C/C++", ScrapeURL: noFluffMainUrl + noFluffScrapeParams, SearchTerm: "c++"},
}

// NoFluffBySlug returns the category addressed by a URL slug.
func NoFluffBySlug(slug string) (NoFluffCategory, bool) {
	for _, c := range NoFluffAll {
		if c.Slug == slug {
			return c, true
		}
	}

	return NoFluffCategory{}, false
}

//https://nofluffjobs.com/pl/job/senior-data-platform-engineer-python-or-go-link-group-warszawa-1

// https://nofluffjobs.com/api/posting/senior-data-platform-engineer-python-or-go-link-group-warszawa-1
