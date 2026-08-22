package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"tally/internal/authn"
	"tally/internal/bootstrap"
	"tally/internal/mcpserver"
	"tally/internal/store"
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

	db, err := bootstrap.InitDataStore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally-mcp: failed to initialize data store: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	mux := buildMux(cfg, db)

	fmt.Printf("tally-mcp: ready, listening on %s\n", cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}

// buildMux assembles the HTTP routes: an authenticated /mcp endpoint and an
// unauthenticated /healthz check. Split out from main so it can be exercised
// directly with httptest, without binding a real network port.
func buildMux(cfg *bootstrap.Config, db *sql.DB) *http.ServeMux {
	server := mcpserver.New(serverName, serverVersion)
	tools.RegisterAll(server, tools.Deps{
		DB: db,
		Q:  store.New(db),
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", authn.Middleware(cfg.MCPToken, mcpserver.HTTPHandler(server)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return mux
}
