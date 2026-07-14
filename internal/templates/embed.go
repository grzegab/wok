package templates

import "embed"

// FS holds every server-rendered template. Layout/auth partials live in
// partials/, and each system area has its own directory (offers/, and later
// cv/, stats/, scraped/). When a directory gets its first *.gohtml file, add
// its glob both here and to the template.ParseFS call in cmd/web/main.go.
//
//go:embed base.gohtml */*.gohtml other/*
var FS embed.FS

//go:embed assets/*
var FSa embed.FS
