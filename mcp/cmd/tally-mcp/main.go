package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/assets"
	"tally/internal/authn"
	"tally/internal/bootstrap"
	"tally/internal/mcpserver"
	"tally/internal/oauth"
	"tally/internal/store"
	"tally/internal/tools"
	"tally/internal/widgets"
)

const (
	serverName    = "tally-mcp"
	serverVersion = "0.1.5"
	serverTitle   = "Tally"
	serverWebsite = "https://github.com/chaoliu719/tally"
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
	server := mcpserver.New(&mcp.Implementation{
		Name:       serverName,
		Version:    serverVersion,
		Title:      serverTitle,
		WebsiteURL: serverWebsite,
		Icons: []mcp.Icon{{
			Source:   cfg.PublicBaseURL + "/icon.png",
			MIMEType: "image/png",
			Sizes:    []string{"512x512"},
		}},
	})
	tools.RegisterAll(server, tools.Deps{
		DB:            db,
		Q:             store.New(db),
		ConfirmSecret: cfg.ConfirmationSecret,
	})

	oauthSrv := oauth.New(oauth.Config{
		SigningSecret: cfg.OAuthSigningSecret,
		BaseURL:       cfg.PublicBaseURL,
		StaticToken:   cfg.MCPToken,
	})

	mux := http.NewServeMux()
	oauthSrv.Routes(mux)
	mux.Handle("/mcp", authn.Middleware(cfg.MCPToken, oauthSrv, mcpserver.HTTPHandler(server)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Brand assets: the server icon advertised in the initialize response and
	// the site favicon. Public, like /healthz and /.well-known/* -- hosts and
	// browsers fetch these with a plain unauthenticated GET.
	mux.HandleFunc("/icon.png", serveAsset("icon.png"))
	mux.HandleFunc("/favicon.ico", serveAsset("favicon.ico"))

	// Opt-in dev aid: serve widget HTML wired to a fake host so it can be
	// styled in a plain browser tab. Off unless TALLY_DEV_WIDGET_PREVIEW=1;
	// never enabled in the production deployment.
	if os.Getenv("TALLY_DEV_WIDGET_PREVIEW") == "1" {
		mux.HandleFunc("/widget-preview/", func(w http.ResponseWriter, r *http.Request) {
			name := strings.TrimPrefix(r.URL.Path, "/widget-preview/")
			html, ok := widgets.PreviewHTML(name)
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(html))
		})
	}

	return mux
}

// serveAsset returns a handler that writes the named embedded brand asset with
// its Content-Type and a day-long cache hint.
func serveAsset(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, contentType, ok := assets.Bytes(name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(data)
	}
}
