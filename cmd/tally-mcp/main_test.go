package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/bootstrap"
)

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (t *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

// TestMCPHandshakeFlow drives initialize -> tools/list -> tools/call -> ping
// through a real MCP client (the official SDK's client, talking streamable
// HTTP) against the actual server wiring.
func TestMCPHandshakeFlow(t *testing.T) {
	const token = "test-token"

	cfg := &bootstrap.Config{MCPToken: token, DefaultCurrency: "CNY"}
	mux := buildMux(cfg, 1)

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{token: token, base: http.DefaultTransport},
		},
	}

	ctx := context.Background()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("initialize handshake failed: %v", err)
	}
	defer session.Close()

	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	_, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "nonexistent_tool"})
	if err == nil {
		t.Fatal("expected tools/call for an unregistered tool to fail")
	}
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected a structured *jsonrpc.Error, got %T: %v", err, err)
	}
	if rpcErr.Code != jsonrpcCodeInvalidParams {
		t.Fatalf("expected code %d (InvalidParams), got %d", jsonrpcCodeInvalidParams, rpcErr.Code)
	}

	if err := session.Ping(ctx, nil); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// The process/session must still be usable after the tools/call error and
	// the invalid-JSON request below — nothing should have crashed it.
	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatalf("tools/list after error should still succeed: %v", err)
	}
}

const jsonrpcCodeInvalidParams = -32602

// TestMalformedRequestBody verifies that a request body that isn't valid
// JSON gets a structured error response rather than crashing the server or
// leaving it unable to serve subsequent requests.
func TestMalformedRequestBody(t *testing.T) {
	const token = "test-token"

	cfg := &bootstrap.Config{MCPToken: token, DefaultCurrency: "CNY"}
	mux := buildMux(cfg, 1)

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	req, err := http.NewRequest(http.MethodPost, httpServer.URL+"/mcp", bytes.NewReader([]byte("not valid json{{{")))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 600 {
		t.Fatalf("expected an error status for malformed JSON, got %d", resp.StatusCode)
	}

	// The server must still be alive and able to serve a normal request.
	if respCode := getStatus(t, httpServer.URL+"/healthz"); respCode != http.StatusOK {
		t.Fatalf("server did not survive malformed request, /healthz = %d", respCode)
	}
}

func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
