package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/bootstrap"
	"tally/internal/tools"
)

func futureTime() int64 {
	return time.Now().Add(time.Hour).Unix()
}

// newE2ESession spins up a fresh SQLite-backed server (in a temp dir) using
// the same buildMux wiring main() uses, and returns a connected MCP client
// session talking to it over real authenticated HTTP, along with the id of a
// freshly created default ledger. Each call gets its own empty ledger; the
// server and client are torn down via t.Cleanup.
func newE2ESession(t *testing.T) (*mcp.ClientSession, string) {
	t.Helper()

	const token = "e2e-test-token"

	dbPath := filepath.Join(t.TempDir(), "tally-e2e.db")
	cfg := &bootstrap.Config{
		MCPToken:           token,
		ConfirmationSecret: "e2e-test-confirmation-secret",
		DBPath:             dbPath,
	}

	db, err := bootstrap.InitDataStore(cfg)
	if err != nil {
		t.Fatalf("InitDataStore failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mux := buildMux(cfg, db)
	httpServer := httptest.NewServer(mux)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{token: token, base: http.DefaultTransport},
		},
	}

	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("initialize handshake failed: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	var ledgerOut tools.ManageLedgerOutput
	call(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Test Ledger"}, &ledgerOut)

	return session, ledgerOut.Ledger.ID
}

// call invokes an MCP tool and unmarshals its structured output into out. It
// fails the test if the call itself errors, at either the protocol or tool
// level; use callExpectError for scenarios that expect a tool-level error.
func call(t *testing.T, session *mcp.ClientSession, name string, args any, out any) {
	t.Helper()

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s) protocol error: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) tool error: %s", name, contentText(res))
	}
	if out != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structured content: %v", err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("unmarshal into %T: %v", out, err)
		}
	}
}

// callExpectError invokes an MCP tool expecting a tool-level error
// (IsError=true), as opposed to a protocol-level error.
func callExpectError(t *testing.T, session *mcp.ClientSession, name string, args any) {
	t.Helper()

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s) protocol error (expected a tool error instead): %v", name, err)
	}
	if !res.IsError {
		t.Fatalf("CallTool(%s) succeeded, expected a tool error", name)
	}
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
