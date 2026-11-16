// Package tools implements the MCP tool handlers backed by tally's own
// SQLite-based store.
package tools

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/store"
)

// ezbookkeeping generated ids (account, category, transaction, ...) as
// snowflake-style int64s that routinely exceed 2^53; tally's own ids are
// plain SQLite auto-increment integers and stay well under that. The wire
// convention is kept unchanged regardless -- every id crossing the tool
// boundary is a JSON string, not a JSON number -- so no MCP client needs to
// know which storage layer is behind the server at any given time.
func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", s, err)
	}
	return id, nil
}

// Deps holds everything a tool handler needs to talk to tally's store.
type Deps struct {
	DB *sql.DB
	Q  *store.Queries

	// ConfirmSecret signs and verifies the confirmation tokens used by
	// destructive operations' preview -> apply flow (see internal/confirm).
	ConfirmSecret string
}

// registrations holds one entry per tool file's registration function. Each
// tool file calls register() from an init() func, so adding a new tool is
// just "add a file, call register()" — this file never needs to change.
var registrations []func(*mcp.Server, Deps)

func register(fn func(*mcp.Server, Deps)) {
	registrations = append(registrations, fn)
}

// RegisterAll registers every known tool onto s.
func RegisterAll(s *mcp.Server, deps Deps) {
	for _, fn := range registrations {
		fn(s, deps)
	}
}

// addTool wraps mcp.AddTool with an input schema normalized so slice and
// pointer fields emit a single JSON Schema type (e.g. "array" or "object")
// rather than ["null", "array"] / ["null", "object"]. Downstream LLM
// adapters (such as Vertex AI / Gemini functionDeclarations) expand multi-type
// arrays into any_of while leaving sibling keywords like items/properties/
// description at the same level, which Vertex AI rejects with HTTP 400.
func addTool[In, Out any](s *mcp.Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	if t.InputSchema == nil {
		schema, err := jsonschema.For[In](nil)
		if err != nil {
			panic(fmt.Sprintf("addTool: tool %q input schema: %v", t.Name, err))
		}
		stripNullTypes(schema)
		t.InputSchema = schema
	}
	mcp.AddTool(s, t, h)
}

func stripNullTypes(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Types) > 0 {
		filtered := s.Types[:0:0]
		for _, typ := range s.Types {
			if typ != "null" {
				filtered = append(filtered, typ)
			}
		}
		switch len(filtered) {
		case 1:
			s.Type = filtered[0]
			s.Types = nil
		default:
			s.Types = filtered
		}
	}
	for _, prop := range s.Properties {
		stripNullTypes(prop)
	}
	stripNullTypes(s.Items)
	for _, item := range s.ItemsArray {
		stripNullTypes(item)
	}
	for _, item := range s.PrefixItems {
		stripNullTypes(item)
	}
	stripNullTypes(s.AdditionalProperties)
	for _, def := range s.Defs {
		stripNullTypes(def)
	}
	for _, def := range s.Definitions {
		stripNullTypes(def)
	}
}
