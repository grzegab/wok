package technology

import (
	"slices"
	"strings"
)

// aliases groups technologies that are counted as one under a shared name.
var aliases = map[string]string{
	"javascript": "typescript/javascript",
	"typescript": "typescript/javascript",
}

// Normalize brings a comma separated technology list to the form it is stored
// in: lowercase names, aliases grouped under one name, each listed once.
func Normalize(list string) string {
	var names []string
	for name := range strings.SplitSeq(list, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if alias, ok := aliases[name]; ok {
			name = alias
		}

		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	return strings.Join(names, ", ")
}
