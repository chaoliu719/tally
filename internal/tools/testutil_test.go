package tools_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/bootstrap"
	"tally/internal/mcpserver"
	"tally/internal/tools"
)

// newTestSession spins up a fresh SQLite-backed server (in a temp dir) with
// every tool registered, and returns a connected MCP client session talking
// to it over real streamable HTTP. Each call gets its own empty ledger.
func newTestSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "tally-test.db")
	cfg := &bootstrap.Config{
		MCPToken:        "unused",
		DefaultCurrency: "CNY",
		DBPath:          dbPath,
	}

	ezCfg := bootstrap.BuildEzbookkeepingConfig(cfg)
	if err := bootstrap.InitDataStore(ezCfg); err != nil {
		t.Fatalf("InitDataStore failed: %v", err)
	}

	uid, err := bootstrap.EnsureSingleUser(cfg.DefaultCurrency)
	if err != nil {
		t.Fatalf("EnsureSingleUser failed: %v", err)
	}

	server := mcpserver.New("tally-mcp-test", "0.0.0")
	tools.RegisterAll(server, tools.Deps{UID: uid, DefaultCurrency: cfg.DefaultCurrency})

	httpServer := httptest.NewServer(mcpserver.HTTPHandler(server))
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: httpServer.URL}

	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	return session
}

// callTool invokes a tool and unmarshals its structured output into out. It
// fails the test if the call itself errors (protocol-level failure); use
// callToolExpectError for scenarios that expect a tool-level error.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args any, out any) *mcp.CallToolResult {
	t.Helper()

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) protocol error: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) returned a tool error: %s", name, contentText(res))
	}

	if out != nil {
		if err := unmarshalStructured(res, out); err != nil {
			t.Fatalf("CallTool(%s) failed to unmarshal structured content: %v", name, err)
		}
	}

	return res
}

// callToolExpectError invokes a tool expecting a tool-level error
// (IsError=true), as opposed to a protocol-level error.
func callToolExpectError(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) protocol error (expected a tool error instead): %v", name, err)
	}
	if !res.IsError {
		t.Fatalf("CallTool(%s) succeeded, expected a tool error", name)
	}

	return res
}

func contentText(res *mcp.CallToolResult) string {
	var out string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			out += tc.Text
		}
	}
	return out
}

// unmarshalStructured round-trips res.StructuredContent (already decoded by
// the client into a generic any) through JSON to populate the typed out.
func unmarshalStructured(res *mcp.CallToolResult, out any) error {
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
