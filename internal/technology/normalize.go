package technology

import (
	"slices"
	"strings"
)

// Duplicate is stored as the tech of an offer that repeats another one, so its
// technologies are counted once and the copies show up in stats under this name.
const Duplicate = "duplicate"

// aliases groups technologies that are counted as one under a shared name.
var aliases = map[string]string{
	"javascript":       "typescript/javascript",
	"typescript":       "typescript/javascript",
	"k8s":              "kubernetes (k8s)",
	"kubernetes":       "kubernetes (k8s)",
	"kubernetes apis":  "kubernetes (k8s)",
	"grpc api":         "grpc",
	"protocol buffers": "grpc",
	"jvm":              "java",
	"rag":              "ai",
	"llm":              "ai",
	"llms":             "ai",
	"api integrations": "api",
	"rest api":         "api",
	"rest apis":        "api",
	"rest":             "api",
	"open api":         "api",
	"openapi":          "api",
	"go language":      "go",
	"golang":           "go",
	"postgres":         "postgresql",
	"psql":             "postgresql",
	"pgsql":            "postgresql",
}

// Normalize brings a comma separated technology list to the form it is stored
// in: lowercase names, aliases grouped under one name, each listed once.
func Normalize(list string) string {
	var names []string
	for name := range strings.SplitSeq(list, ",") {
		name = canonical(strings.ToLower(strings.TrimSpace(name)))
		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	return strings.Join(names, ", ")
}

// canonical returns the name a lowercase technology is counted under. Every
// name containing the word "ai" (ai integrations, generative ai) counts as ai.
func canonical(name string) string {
	if alias, ok := aliases[name]; ok {
		return alias
	}

	if slices.Contains(strings.Fields(name), "ai") {
		return "ai"
	}

	return name
}
