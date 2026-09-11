package technology

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		list string
		want string
	}{
		{
			name: "lowercases every technology",
			list: "C++, Java, Python",
			want: "c++, java, python",
		},
		{
			name: "groups javascript under typescript/javascript",
			list: "React, JavaScript",
			want: "react, typescript/javascript",
		},
		{
			name: "groups typescript under typescript/javascript",
			list: "TypeScript, Node.js",
			want: "typescript/javascript, node.js",
		},
		{
			name: "lists the group once when an offer asks for both",
			list: "TypeScript, JavaScript, Angular",
			want: "typescript/javascript, angular",
		},
		{
			name: "drops duplicates that differ only in case",
			list: "Go, golang, GO",
			want: "go, golang",
		},
		{
			name: "copes with missing spaces and empty items",
			list: "Docker,,Kubernetes , ",
			want: "docker, kubernetes",
		},
		{
			name: "an empty list stays empty",
			list: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.list); got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.list, got, tt.want)
			}
		})
	}
}
