// Package mcpserver wires up the MCP protocol server and its HTTP transport.
package mcpserver

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// New creates an MCP server with the given implementation name/version.
// Tools are registered onto it separately (see internal/tools), keeping this
// package limited to transport/protocol concerns.
func New(name, version string) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)
}

// HTTPHandler returns an http.Handler that serves the MCP streamable-HTTP
// transport for s. It is meant to be mounted at a single path (e.g. /mcp) on
// a net/http.ServeMux.
func HTTPHandler(s *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s
	}, nil)
}
