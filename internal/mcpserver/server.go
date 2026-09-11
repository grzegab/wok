package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	database "github.com/grzegab/wok/internal/database/generated"
)

const (
	serverName    = "wok-db"
	serverVersion = "0.1.0"
)

// New builds the MCP server exposing the WOK database. Every tool is read-only:
// the sqlc mutations are deliberately left unregistered, and cmd/mcp additionally
// opens the connection in a read-only session.
func New(db *database.Queries) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)
	tools := newToolset(db)

	mcp.AddTool(server, readOnlyTool(
		"list_categories",
		"List the job categories WOK tracks. Returns the slug used in the app URLs and the name stored in the offers table; every other tool expects one of these.",
	), tools.listCategories)

	mcp.AddTool(server, readOnlyTool(
		"list_applications",
		"List every job application the user has sent, newest first. Includes company, date the CV was sent, salary range, the rate asked for, current status and free-text notes.",
	), tools.listApplications)

	mcp.AddTool(server, readOnlyTool(
		"get_application",
		"Get a single job application by id, with all of its notes.",
	), tools.getApplication)

	mcp.AddTool(server, readOnlyTool(
		"list_possibilities",
		"List scraped job offers in one category, newest first. Descriptions are omitted; use get_possibility for the full text of a single offer.",
	), tools.listPossibilities)

	mcp.AddTool(server, readOnlyTool(
		"get_possibility",
		"Get a single scraped job offer by id, including its full description.",
	), tools.getPossibility)

	mcp.AddTool(server, readOnlyTool(
		"tech_stats",
		"Count how many scraped offers in one category mention each technology, most frequent first. Use it to answer what a category's market asks for.",
	), tools.techStats)

	return server
}

func readOnlyTool(name, description string) *mcp.Tool {
	closedWorld := false

	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &closedWorld,
		},
	}
}
