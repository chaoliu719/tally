// Package widgets serves the HTML for tally-mcp's MCP Apps (interactive UI
// widgets rendered inline by hosts that support the apps surface, e.g.
// claude.ai). Each widget is a single self-contained HTML file under this
// directory with a /*__EXT_APPS_BUNDLE__*/ placeholder; at load time the
// placeholder is replaced with the vendored ext-apps browser runtime rewritten
// to expose globalThis.ExtApps, because the iframe CSP blocks module imports.
//
// This is a deliberate, scoped exception to "tally-mcp is a pure fact core
// that serves no frontend UI" -- see openspec change add-transaction-timeline-
// widget. Widgets are read-only and reach ledger data only through the
// server's own tools via callServerTool.
package widgets

import (
	"fmt"
	"regexp"
	"strings"

	_ "embed"
)

// MIMEType is the mime type a host uses to recognize a resource as an
// interactive MCP App widget rather than plain HTML source.
const MIMEType = "text/html;profile=mcp-app"

// BundlePlaceholder is the token each widget HTML file contains where the
// ext-apps browser runtime is spliced in.
const BundlePlaceholder = "/*__EXT_APPS_BUNDLE__*/"

//go:embed vendor/ext-apps-app-with-deps.js
var extAppsBundle string

//go:embed timeline.html
var timelineHTML string

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

// rendered holds each widget's HTML with the bundle already spliced in,
// built once at package init.
var rendered = func() map[string]string {
	bundle := browserBundle()
	splice := func(html string) string {
		return strings.Replace(html, BundlePlaceholder, bundle, 1)
	}
	return map[string]string{
		"timeline": splice(timelineHTML),
	}
}()

// HTML returns the fully rendered HTML for the named widget. The second
// return is false if no widget by that name exists.
func HTML(name string) (string, bool) {
	html, ok := rendered[name]
	return html, ok
}

// URI returns the ui:// resource URI for the named widget.
func URI(name string) string {
	return fmt.Sprintf("ui://widgets/%s.html", name)
}
