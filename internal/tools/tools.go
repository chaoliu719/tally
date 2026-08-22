// Package tools implements the MCP tool handlers backed by ezbookkeeping services.
package tools

import (
	"fmt"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ezbookkeeping generates ids (account, category, transaction, ...) as
// snowflake-style int64s that routinely exceed 2^53. MCP clients decode JSON
// numbers into float64 by default, silently losing precision for values that
// large. So every id crossing the tool boundary is a JSON string on the
// wire, not a JSON number — formatID/parseID are the only place that
// conversion happens.
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

// Deps holds everything a tool handler needs to call into ezbookkeeping's
// service layer for tally's single user.
type Deps struct {
	UID             int64
	DefaultCurrency string
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
