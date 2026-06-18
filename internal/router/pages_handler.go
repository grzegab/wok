package router

import (
	"html/template"
	"net/http"

	"github.com/grzegab/wok/internal/templates"
)

func landing(w http.ResponseWriter, _ *http.Request) {
	tmpl, err := template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	err = tmpl.ExecuteTemplate(w, "base", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func robotsTxt(w http.ResponseWriter, _ *http.Request) {
	data, err := templates.FS.ReadFile("other/robots.txt")
	if err != nil {
		http.Error(w, "Robots.txt not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}
