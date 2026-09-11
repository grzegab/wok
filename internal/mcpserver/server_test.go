package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/grzegab/wok/internal/category"
)

// connect wires the server to an in-memory client. The database is never
// touched, so the tools exercised here must be the ones that do not query it.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := New(nil).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting server: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting client: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })

	return clientSession
}

func TestServerExposesOnlyReadOnlyTools(t *testing.T) {
	want := []string{
		"get_application",
		"get_possibility",
		"list_applications",
		"list_categories",
		"list_duplicates",
		"list_possibilities",
		"tech_stats",
	}

	session := connect(t)

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}

	got := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		got = append(got, tool.Name)

		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q is not annotated as read-only", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has no input schema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %q has no output schema", tool.Name)
		}
	}

	if len(got) != len(want) {
		t.Fatalf("got tools %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("tool %d: got %q, want %q", i, got[i], name)
		}
	}
}

// The category column stores the display name, not the slug, so list_categories
// is what keeps a caller from filtering on a value that never matches a row.
func TestListCategoriesReturnsStoredNames(t *testing.T) {
	session := connect(t)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_categories",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("calling list_categories: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_categories failed: %v", result.Content)
	}

	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshalling structured content: %v", err)
	}

	var got CategoriesOutput
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decoding structured content: %v", err)
	}

	if len(got.Categories) != len(category.JustJoinAll) {
		t.Fatalf("got %d categories, want %d", len(got.Categories), len(category.JustJoinAll))
	}
	for i, want := range category.JustJoinAll {
		if got.Categories[i].Name != want.Name || got.Categories[i].Slug != want.Slug {
			t.Errorf("category %d: got %+v, want slug %q name %q", i, got.Categories[i], want.Slug, want.Name)
		}
	}
}

func TestResolveCategory(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "stored name passes through", value: "C/C++", want: "C/C++"},
		{name: "url slug maps onto the stored name", value: "c", want: "C/C++"},
		{name: "unknown value is rejected", value: "rust", wantErr: true},
		{name: "empty value is rejected", value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCategory(tt.value)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want an error", got)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveLimit(t *testing.T) {
	tests := []struct {
		name    string
		limit   int32
		want    int32
		wantErr bool
	}{
		{name: "unset falls back to the default", limit: 0, want: defaultPossibilityLimit},
		{name: "value within range passes through", limit: 10, want: 10},
		{name: "the maximum is allowed", limit: maxPossibilityLimit, want: maxPossibilityLimit},
		{name: "above the maximum is rejected", limit: maxPossibilityLimit + 1, wantErr: true},
		{name: "negative is rejected", limit: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveLimit(tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %d, want an error", got)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}
