package tools_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/bootstrap"
	"tally/internal/mcpserver"
	"tally/internal/store"
	"tally/internal/tools"
)

// testConfirmSecret is the confirmation-token secret every test session is
// configured with, exposed so tests can craft their own tokens (e.g. an
// already-expired one) to exercise confirm.Verify's failure paths through
// the tools rather than the confirm package directly.
const testConfirmSecret = "test-confirmation-secret"

// newTestSession spins up a fresh SQLite-backed server (in a temp dir) with
// every tool registered, and returns a connected MCP client session talking
// to it over real streamable HTTP. Each call gets its own empty ledger.
func newTestSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "tally-test.db")
	cfg := &bootstrap.Config{
		MCPToken:           "unused",
		ConfirmationSecret: testConfirmSecret,
		DBPath:             dbPath,
	}

	db, err := bootstrap.InitDataStore(cfg)
	if err != nil {
		t.Fatalf("InitDataStore failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	server := mcpserver.New("tally-mcp-test", "0.0.0")
	tools.RegisterAll(server, tools.Deps{DB: db, Q: store.New(db), ConfirmSecret: cfg.ConfirmationSecret})

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

// craftConfirmationToken builds a confirmation token in the same wire format
// as internal/confirm.Issue, but with an arbitrary expiresAt -- letting
// tests exercise the "already expired" rejection path, which Issue's fixed
// 15-minute TTL can't produce directly.
func craftConfirmationToken(t *testing.T, secret, action, id, revision string, expiresAt int64) string {
	t.Helper()

	type payload struct {
		Action    string `json:"action"`
		ID        string `json:"id"`
		Revision  string `json:"revision"`
		ExpiresAt int64  `json:"expires_at"`
	}

	body, err := json.Marshal(payload{Action: action, ID: id, Revision: revision, ExpiresAt: expiresAt})
	if err != nil {
		t.Fatalf("marshal token payload: %v", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := mac.Sum(nil)

	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig)
}
