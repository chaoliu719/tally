// Package widgets serves the HTML for tally-mcp's MCP Apps (interactive UI
// widgets rendered inline by hosts that support the apps surface, e.g.
// claude.ai). Each widget is a directory under src/ holding an index.html
// with a single /*__EXT_APPS_BUNDLE__*/ placeholder; at load time the
// placeholder is replaced with the vendored ext-apps browser runtime
// (rewritten to expose globalThis.ExtApps, because the iframe CSP blocks
// module imports) followed by the shared frontend runtime. The directory
// name is the widget name.
//
// This is a deliberate, scoped exception to "tally-mcp is a pure fact core
// that serves no frontend UI" -- see openspec change add-transaction-timeline-
// widget. Widgets are read-only and reach ledger data only through the
// server's own tools via callServerTool.
package widgets

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// MIMEType is the mime type a host uses to recognize a resource as an
// interactive MCP App widget rather than plain HTML source.
const MIMEType = "text/html;profile=mcp-app"

// BundlePlaceholder is the token each widget index.html contains where the
// ext-apps browser runtime plus the shared frontend runtime are spliced in.
const BundlePlaceholder = "/*__EXT_APPS_BUNDLE__*/"

//go:embed vendor/ext-apps-app-with-deps.js
var extAppsBundle string

// runtimeJS is the shared frontend runtime (globalThis.Tally), spliced into
// every widget right after the ext-apps browser bundle.
//
//go:embed runtime.js
var runtimeJS string

// previewShim is the generic fake host (globalThis.ExtApps) used by
// PreviewHTML in place of the real ext-apps runtime.
//
//go:embed preview-shim.js
var previewShim string

// all:src pulls every widget directory (index.html plus sidecars like
// preview.js) into the binary. Keep src/ free of secrets and large assets.
//
//go:embed all:src
var widgetFS embed.FS

// exportRewrite matches the trailing `export{a as B,c as D};` of the ES-module
// bundle so it can be turned into a `globalThis.ExtApps={B:a,D:c}` assignment.
var exportRewrite = regexp.MustCompile(`export\{([^}]+)\};?\s*$`)

func browserBundle() string {
	return exportRewrite.ReplaceAllStringFunc(extAppsBundle, func(match string) string {
		inner := exportRewrite.FindStringSubmatch(match)[1]
		var pairs []string
		for _, part := range strings.Split(inner, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			local := part
			exported := part
			if i := strings.Index(part, " as "); i >= 0 {
				local = strings.TrimSpace(part[:i])
				exported = strings.TrimSpace(part[i+4:])
			}
			pairs = append(pairs, exported+":"+local)
		}
		return "globalThis.ExtApps={" + strings.Join(pairs, ",") + "};"
	})
}

// splicePayload is what replaces BundlePlaceholder in a rendered widget: the
// host runtime followed by the shared frontend runtime, so Tally is present
// wherever globalThis.ExtApps is.
func splicePayload(host string) string {
	return host + "\n" + runtimeJS
}

type widgetSet struct {
	rendered map[string]string // widget name -> HTML for a real host
	preview  map[string]string // widget name -> HTML wired to the fake preview host
}

// buildRegistry walks fsys (rooted so that "src/<name>/index.html" resolves)
// and renders every widget. It is pure so tests can feed a synthetic fs.FS
// with several widgets. host is the already-rewritten ext-apps browser
// bundle.
func buildRegistry(fsys fs.FS, host string) (widgetSet, error) {
	entries, err := fs.ReadDir(fsys, "src")
	if err != nil {
		return widgetSet{}, fmt.Errorf("widgets: reading src/: %w", err)
	}
	ws := widgetSet{rendered: map[string]string{}, preview: map[string]string{}}
	payload := splicePayload(host)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		indexPath := path.Join("src", name, "index.html")
		htmlBytes, err := fs.ReadFile(fsys, indexPath)
		if err != nil {
			return widgetSet{}, fmt.Errorf("widgets: %s has no index.html: %w", name, err)
		}
		html := string(htmlBytes)
		if n := strings.Count(html, BundlePlaceholder); n != 1 {
			return widgetSet{}, fmt.Errorf("widgets: %s/index.html must contain exactly one %s, found %d",
				name, BundlePlaceholder, n)
		}

		var fixture string
		if fx, err := fs.ReadFile(fsys, path.Join("src", name, "preview.js")); err == nil {
			fixture = string(fx)
		}
		previewPayload := previewShim + "\n" + fixture + "\n" + runtimeJS

		ws.rendered[name] = strings.Replace(html, BundlePlaceholder, payload, 1)
		ws.preview[name] = strings.Replace(html, BundlePlaceholder, previewPayload, 1)
	}
	if len(ws.rendered) == 0 {
		return widgetSet{}, fmt.Errorf("widgets: no widget directories found under src/")
	}
	return ws, nil
}

// registry holds every widget rendered once at package init. A malformed
// widget panics here -- the same loud failure style as before, so go test
// and startup catch it rather than shipping a raw placeholder.
var registry = func() widgetSet {
	ws, err := buildRegistry(widgetFS, browserBundle())
	if err != nil {
		panic(err)
	}
	return ws
}()

// HTML returns the fully rendered HTML for the named widget. The second
// return is false if no widget by that name exists.
func HTML(name string) (string, bool) {
	html, ok := registry.rendered[name]
	return html, ok
}

// PreviewHTML returns the named widget wired to the fake preview host instead
// of the real ext-apps runtime, for opening in a plain browser tab.
func PreviewHTML(name string) (string, bool) {
	html, ok := registry.preview[name]
	return html, ok
}

// Names returns every widget name, sorted.
func Names() []string {
	out := make([]string, 0, len(registry.rendered))
	for name := range registry.rendered {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// URI returns the ui:// resource URI for the named widget.
func URI(name string) string {
	return fmt.Sprintf("ui://widgets/%s.html", name)
}
