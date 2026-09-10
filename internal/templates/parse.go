package templates

import (
	"embed"
	"html/template"

	"github.com/grzegab/wok/internal/category"
)

// funcs exposes the menu categories to every template, so the shared header
// partial renders the navigation without each handler passing it in.
var funcs = template.FuncMap{
	"categories": func() []category.Category { return category.JustJoinAll },
}

// Parse parses one embedded template area (its own directory plus the shared
// partials/) into a ready-to-execute template set.
func Parse(area embed.FS) (*template.Template, error) {
	return template.New("wok").Funcs(funcs).ParseFS(area, "*/*.gohtml")
}
