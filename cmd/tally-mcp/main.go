package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"tally/internal/authn"
	"tally/internal/bootstrap"
	"tally/internal/mcpserver"
	"tally/internal/tools"
)

const (
	serverName    = "tally-mcp"
	serverVersion = "0.1.0"
)

func main() {
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally-mcp: configuration error: %s\n", err)
		os.Exit(1)
	}

	ezCfg := bootstrap.BuildEzbookkeepingConfig(cfg)

	if err := bootstrap.InitDataStore(ezCfg); err != nil {
		fmt.Fprintf(os.Stderr, "tally-mcp: failed to initialize data store: %s\n", err)
		os.Exit(1)
	}

	uid, err := bootstrap.EnsureSingleUser(cfg.DefaultCurrency)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally-mcp: failed to bootstrap single user: %s\n", err)
		os.Exit(1)
	}

	mux := buildMux(cfg, uid)

	fmt.Printf("tally-mcp: ready (uid=%d), listening on %s\n", uid, cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}

// buildMux assembles the HTTP routes: an authenticated /mcp endpoint and an
// unauthenticated /healthz check. Split out from main so it can be exercised
// directly with httptest, without binding a real network port.
func buildMux(cfg *bootstrap.Config, uid int64) *http.ServeMux {
	server := mcpserver.New(serverName, serverVersion)
	tools.RegisterAll(server, tools.Deps{
		UID:             uid,
		DefaultCurrency: cfg.DefaultCurrency,
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", authn.Middleware(cfg.MCPToken, mcpserver.HTTPHandler(server)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return mux
}
