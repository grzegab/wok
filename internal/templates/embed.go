package templates

import "embed"

//go:embed base.gohtml partials/* other/*
var FS embed.FS

//go:embed assets/*
var FSa embed.FS
