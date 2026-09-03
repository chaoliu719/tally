package widgets

import (
	"strings"
	"testing"
)

func TestTimelineHTMLRendered(t *testing.T) {
	html, ok := HTML("timeline")
	if !ok {
		t.Fatal("timeline widget not found")
	}
	if strings.Contains(html, BundlePlaceholder) {
		t.Error("bundle placeholder not substituted")
	}
	if !strings.Contains(html, "globalThis.ExtApps={") {
		t.Error("export{} not rewritten to globalThis.ExtApps assignment")
	}
	if strings.Contains(html, "export{") {
		t.Error("leftover ES-module export{} in browser-served bundle")
	}
	if !strings.Contains(html, "App:") {
		t.Error("App export missing from globalThis.ExtApps")
	}
}

func TestUnknownWidget(t *testing.T) {
	if _, ok := HTML("nope"); ok {
		t.Error("expected unknown widget to report not found")
	}
}

func TestTimelineWidgetBehaviours(t *testing.T) {
	html, _ := HTML("timeline")
	for _, want := range []string{
		"callServerTool",       // widget fetches its own pages
		"newest_first",         // ... in descending order
		"search_transactions",  // ... from this tool
		"list_categories",      // one-time id->name lookup for row labels
		"list_sources",         // one-time id->name lookup for row labels
		"上一页",                  // pager control
		"页尾",                   // jump-to-last-page control
		"这个账本还没有交易",            // empty state
		"BroadcastChannel",     // supersession guard
		"onhostcontextchanged", // live theme follow
		":root.dark",           // dark palette
		"autoResize",           // height tracks content
	} {
		if !strings.Contains(html, want) {
			t.Errorf("timeline widget missing %q", want)
		}
	}
}

func TestTimelineScriptIsScopeIsolated(t *testing.T) {
	html, _ := HTML("timeline")
	// claude.ai splices the widget into a shared scope via document.write; a
	// leaked top-level declaration (notably `io`) breaks the whole write.
	if !strings.Contains(html, "(async () => {") {
		t.Error("widget script is not wrapped in an IIFE")
	}
	if strings.Contains(html, "\nconst io ") || strings.Contains(html, "\nconst io=") {
		t.Error("widget declares `io` — collides with the claude.ai apps sandbox")
	}
}

func TestPreviewHTMLUsesShim(t *testing.T) {
	html, ok := PreviewHTML("timeline")
	if !ok {
		t.Fatal("preview timeline not found")
	}
	if strings.Contains(html, BundlePlaceholder) {
		t.Error("placeholder not substituted in preview")
	}
	if !strings.Contains(html, "previewShim") && !strings.Contains(html, "callServerTool(a){console.log") {
		t.Error("preview not wired to the fake host shim")
	}
	if strings.Contains(html, "class eI extends") {
		t.Error("preview must not embed the real ext-apps runtime")
	}
}
