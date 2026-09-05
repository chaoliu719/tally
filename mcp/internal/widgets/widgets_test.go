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
		"list_categories",      // fallback id->name lookup for an older server
		"list_sources",         // fallback id->name lookup for an older server
		"ingestInlineLookups",  // categories/sources ride along in the tool result
		"drainAll",             // background: pull the whole ledger for local paging/filtering
		"descendantIdsOf",      // local include-descendants for the category filter
		"matchesFilters",       // filtering is local, no search_transactions params
		"上一页",                  // pager control
		"页尾",                   // jump-to-last-page control
		"这个账本还没有交易",            // empty state
		"没有符合当前条件的交易",          // filtered-empty state, distinct from empty ledger
		"BroadcastChannel",     // supersession guard
		"onhostcontextchanged", // live theme follow
		":root.dark",           // dark palette
		"autoResize",           // height tracks content
	} {
		if !strings.Contains(html, want) {
			t.Errorf("timeline widget missing %q", want)
		}
	}
	// The filter bar applies live on every input change -- there is no
	// "应用" / confirm button to click (change timeline-widget-local-filtering).
	for _, absent := range []string{"filterApply", "button.primary"} {
		if strings.Contains(html, absent) {
			t.Errorf("timeline widget still references removed apply button: %q", absent)
		}
	}
	if !strings.Contains(html, `el.addEventListener("change", commitFilters)`) {
		t.Error("filter inputs are not wired to live re-filtering")
	}
	// The filter bar shows/hides via an explicit .open class, not the
	// [hidden] attribute (which .filterbar's own display rule defeats in
	// some browsers).
	if !strings.Contains(html, ".filterbar.open") || !strings.Contains(html, `classList.toggle("open")`) {
		t.Error("filter bar is not toggled via the .open class")
	}
	if !strings.Contains(html, `aria-expanded`) {
		t.Error("filter toggle does not track aria-expanded")
	}
	// Theme only follows an explicit light/dark host context; a context
	// without a theme (e.g. on a display-mode change) must not reset it.
	if !strings.Contains(html, `if (theme !== "light" && theme !== "dark") return;`) {
		t.Error("applyTheme does not guard against a themeless host context")
	}
	// The "全屏" button hides once the panel is in fullscreen.
	if !strings.Contains(html, "syncDisplayMode") {
		t.Error("expand button visibility is not synced to the display mode")
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
