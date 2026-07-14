package app

import (
	"html/template"
	"log"
	"net/http"
)

type DefaultHandler struct {
	templates *template.Template
}

func NewDefaultHandler(tmpl *template.Template) *DefaultHandler {
	return &DefaultHandler{templates: tmpl}
}

func (h *DefaultHandler) Landing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.templates.ExecuteTemplate(w, "notfound", nil); err != nil {
		log.Printf("could not execute notfound template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
