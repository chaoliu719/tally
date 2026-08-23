// Package tools implements the MCP tool handlers backed by tally's own
// SQLite-based store.
package tools

import (
	"database/sql"
	"fmt"
	"strconv"

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
