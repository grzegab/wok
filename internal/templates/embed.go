package templates

import "embed"

// FS holds every server-rendered template. Layout/auth partials live in
// partials/, and each system area has its own directory (offers/, stats/,
// and later cv/, scraped/). When a directory gets its first *.gohtml file,
// add its glob both here and to the template.ParseFS call in cmd/web/main.go.

//go:embed offers/*.gohtml partials/*.gohtml
var Offers embed.FS

//go:embed defaults/*.gohtml partials/*.gohtml
var Defaults embed.FS

//go:embed auth/*.gohtml partials/*.gohtml
var Auth embed.FS

//go:embed stats/*.gohtml partials/*.gohtml
var Stats embed.FS

//go:embed assets/*
var Assets embed.FS
