package router

import (
	"html/template"
	"log"
	"net/http"
)

type PagesHandler struct {
	tmpl *template.Template
}

func NewPagesHandler(tmpl *template.Template) *PagesHandler {
	return &PagesHandler{tmpl: tmpl}
}

func (h *PagesHandler) landing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.tmpl.ExecuteTemplate(w, "notfound", nil); err != nil {
		log.Printf("could not execute notfound template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
