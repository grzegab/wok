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
			name: "groups k8s under kubernetes (k8s)",
			list: "Docker, K8s",
			want: "docker, kubernetes (k8s)",
		},
		{
			name: "groups kubernetes under kubernetes (k8s)",
			list: "Kubernetes, Helm",
			want: "kubernetes (k8s), helm",
		},
		{
			name: "lists kubernetes once when an offer names it both ways",
			list: "Kubernetes, k8s, Kubernetes (K8s)",
			want: "kubernetes (k8s)",
		},
		{
			name: "groups kubernetes apis under kubernetes (k8s)",
			list: "Kubernetes APIs, Docker",
			want: "kubernetes (k8s), docker",
		},
		{
			name: "groups grpc api and protocol buffers under grpc",
			list: "gRPC API, Protocol Buffers, gRPC",
			want: "grpc",
		},
		{
			name: "groups jvm under java",
			list: "JVM, Java, Kotlin",
			want: "java, kotlin",
		},
		{
			name: "groups ai variations, rag and llm under ai",
			list: "AI, AI integrations, AI pipelines, Generative AI, RAG, LLM, LLMs",
			want: "ai",
		},
		{
			name: "does not group names that only contain the letters ai",
			list: "Airflow, OpenAI API",
			want: "airflow, openai api",
		},
		{
			name: "groups rest, open api and api integrations under api",
			list: "REST API, REST APIs, REST, Open API, OpenAPI, API integrations, API",
			want: "api",
		},
		{
			name: "groups golang and go language under go",
			list: "Golang, Go language, Go",
			want: "go",
		},
		{
			name: "groups postgres, psql and pgsql under postgresql",
			list: "Postgres, psql, PgSQL, PostgreSQL",
			want: "postgresql",
		},
		{
			name: "drops duplicates that differ only in case",
			list: "Docker, docker, DOCKER",
			want: "docker",
		},
		{
			name: "copes with missing spaces and empty items",
			list: "Docker,,Kubernetes , ",
			want: "docker, kubernetes (k8s)",
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
