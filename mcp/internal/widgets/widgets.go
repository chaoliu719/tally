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

// previewShim is a stand-in for globalThis.ExtApps used by PreviewHTML: it
// lets a widget be opened in an ordinary browser tab (no host, no iframe
// bridge) for styling work. It fires ontoolresult from a ?payload= query
// param and logs the widget->host calls instead of performing them.
const previewShim = `globalThis.ExtApps={App:class{
  constructor(){this._h={}}
  set ontoolresult(f){this._h.result=f} get ontoolresult(){return this._h.result}
  set ontoolinput(f){this._h.input=f} get ontoolinput(){return this._h.input}
  set onhostcontextchanged(f){this._h.ctx=f} get onhostcontextchanged(){return this._h.ctx}
  async connect(){
    const q=new URLSearchParams(location.search);
    const p=q.get("payload");
    if(p&&this._h.input)this._h.input({arguments:{ledger_id:q.get("ledger_id")||"1"}});
    if(p&&this._h.result)this._h.result({content:[{type:"text",text:"preview\n\n"+p}]});
  }
  getHostContext(){return{theme:new URLSearchParams(location.search).get("theme")||"light",availableDisplayModes:[]}}
  async callServerTool(a){console.log("callServerTool",a);return{structuredContent:{transactions:[],next_cursor:""}}}
  sendMessage(m){console.log("sendMessage",m)}
  updateModelContext(m){console.log("updateModelContext",m)}
  requestDisplayMode(m){console.log("requestDisplayMode",m)}
  openLink(l){console.log("openLink",l)}
}};`

// PreviewHTML returns the named widget wired to previewShim instead of the
// real ext-apps runtime, for opening in a plain browser tab.
func PreviewHTML(name string) (string, bool) {
	html, ok := map[string]string{"timeline": timelineHTML}[name]
	if !ok {
		return "", false
	}
	return strings.Replace(html, BundlePlaceholder, previewShim, 1), true
}

// URI returns the ui:// resource URI for the named widget.
func URI(name string) string {
	return fmt.Sprintf("ui://widgets/%s.html", name)
}
