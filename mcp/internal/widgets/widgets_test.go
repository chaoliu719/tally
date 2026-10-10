package widgets

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestEveryWidgetRenders(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatal("no widgets registered")
	}
	for _, name := range names {
		html, ok := HTML(name)
		if !ok {
			t.Fatalf("%s: registered name has no rendered HTML", name)
		}
		if strings.Contains(html, BundlePlaceholder) {
			t.Errorf("%s: bundle placeholder not substituted", name)
		}
		if !strings.Contains(html, "globalThis.ExtApps={") {
			t.Errorf("%s: export{} not rewritten to globalThis.ExtApps assignment", name)
		}
		if strings.Contains(html, "export{") {
			t.Errorf("%s: leftover ES-module export{} in browser-served bundle", name)
		}
		if !strings.Contains(html, "globalThis.Tally =") {
			t.Errorf("%s: shared runtime not spliced in", name)
		}
		if !strings.Contains(html, "<!doctype html>") {
			t.Errorf("%s: not a complete HTML document", name)
		}
	}
}

func TestEveryWidgetSelfContained(t *testing.T) {
	for _, name := range Names() {
		html, _ := HTML(name)
		for _, marker := range []string{
			`src="http`, `src='http`, `href="http`, `href='http`,
			"<link ", "@import", "//esm.sh", "//unpkg", "//cdn",
		} {
			if strings.Contains(html, marker) {
				t.Errorf("%s: references external resource %q", name, marker)
			}
		}
	}
}

func TestEveryWidgetScopeIsolated(t *testing.T) {
	for _, name := range Names() {
		html, _ := HTML(name)
		if !strings.Contains(html, "(async () => {") && !strings.Contains(html, "(function") {
			t.Errorf("%s: widget script is not wrapped in an IIFE", name)
		}
		if strings.Contains(html, "\nconst io ") || strings.Contains(html, "\nconst io=") {
			t.Errorf("%s: declares `io` — collides with the claude.ai apps sandbox", name)
		}
	}
}

func TestEveryWidgetPreviewUsesShim(t *testing.T) {
	for _, name := range Names() {
		html, ok := PreviewHTML(name)
		if !ok {
			t.Fatalf("%s: no preview HTML", name)
		}
		if strings.Contains(html, BundlePlaceholder) {
			t.Errorf("%s: placeholder not substituted in preview", name)
		}
		if !strings.Contains(html, "TallyPreviewHost") {
			t.Errorf("%s: preview not wired to the fake host shim", name)
		}
		if strings.Contains(html, "class eI extends") {
			t.Errorf("%s: preview must not embed the real ext-apps runtime", name)
		}
	}
}

func TestUnknownWidget(t *testing.T) {
	if _, ok := HTML("nope"); ok {
		t.Error("expected unknown widget to report not found")
	}
	if _, ok := PreviewHTML("nope"); ok {
		t.Error("expected unknown preview to report not found")
	}
}

func TestRuntimeScopeIsolated(t *testing.T) {
	if !strings.HasPrefix(strings.TrimSpace(runtimeJS), "globalThis.Tally = (function") {
		t.Error("runtime.js does not open with the namespace-IIFE assignment")
	}
	for _, line := range strings.Split(runtimeJS, "\n") {
		if strings.HasPrefix(line, "const ") || strings.HasPrefix(line, "let ") || strings.HasPrefix(line, "var ") {
			t.Errorf("runtime.js leaks a top-level binding: %q", line)
		}
	}
}

func TestRuntimePresentInEveryRenderedWidget(t *testing.T) {
	for _, name := range Names() {
		html, _ := HTML(name)
		if !strings.Contains(html, "globalThis.Tally =") || !strings.Contains(html, "extractFirstPagePayload") {
			t.Errorf("%s: shared runtime helpers not present in rendered HTML", name)
		}
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
		"drainAll",             // background: pull the whole scope for local paging/filtering
		"descendantIdsOf",      // local include-descendants for the category filter
		"matchesFilters",       // filtering is local, no search_transactions params
		"上一页",                  // pager control
		"页尾",                   // jump-to-last-page control
		"这个账本还没有交易",            // empty state: the ledger itself has nothing
		"该范围内没有交易",             // empty state: the scope has nothing, distinct from the two others
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

// TestTimelineWidgetScope pins the scope handling added by
// scope-timeline-widget-by-range: the panel shows the scope it was opened
// with, repeats the scope's filters on every background page, swaps the
// dropdowns the scope fixed for read-only tags, and keeps the date inputs
// inside the scope. The behaviour itself is exercised in a real browser via
// the preview harness; these guard that the wiring is not lost.
func TestTimelineWidgetScope(t *testing.T) {
	html, _ := HTML("timeline")
	for _, want := range []string{
		"scopeArgs",                              // background pages repeat the scope's filters verbatim
		"...scopeArgs()",                         // ... on every search_transactions call
		"scope.start_time",                       // dates are taken from the result, never re-derived
		"scope.include_descendants",              // the category scope's descendants flag rides along too
		"renderScope",                            // scope bar, read-only tags, date bounds
		`setBound(input, "min"`,                  // date inputs cannot go before the scope...
		`setBound(input, "max"`,                  // ... or past it
		"--scopebar-h",                           // sticky day labels stack under the scope bar
		"catTagField",                            // read-only stand-in for the fixed category dropdown
		"srcTagField",                            // read-only stand-in for the fixed source dropdown
		"（含子分类）",                                 // a category scope includes its descendants
		"范围：",                                    // scope bar text
		"payload.scope",                          // older servers send none: handled as "whole ledger"
		`[hidden] { display: none !important; }`, // el.hidden must really hide (pager on empty scope)
	} {
		if !strings.Contains(html, want) {
			t.Errorf("timeline widget missing %q", want)
		}
	}
	// The account-wide total is gone from the header: only the scope's count.
	if strings.Contains(html, "账本总数") {
		t.Error("timeline widget still refers to a ledger-wide total")
	}
	// The scope is never part of the user's local filters, so "重置" cannot widen it.
	if strings.Contains(html, "filters.scope") {
		t.Error("scope must not be folded into the user-editable filters")
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
	if !strings.Contains(html, "TallyPreviewHost") && !strings.Contains(html, `console.log("callServerTool"`) {
		t.Error("preview not wired to the fake host shim")
	}
	if strings.Contains(html, "class eI extends") {
		t.Error("preview must not embed the real ext-apps runtime")
	}
	// The timeline supplies its own demo fixture.
	if !strings.Contains(html, "TallyPreviewFixture") {
		t.Error("timeline preview is missing its demo fixture")
	}
}

const placeholderWidget = `<!doctype html>
<main id="main"></main>
<script type="module">
/*__EXT_APPS_BUNDLE__*/
(async () => { await Tally.mountWidget({ name: "x", version: "1" }); })();
</script>`

func TestRegistryWalksMultipleWidgets(t *testing.T) {
	fsys := fstest.MapFS{
		"src/a/index.html": {Data: []byte(placeholderWidget)},
		"src/b/index.html": {Data: []byte(placeholderWidget)},
		"src/b/preview.js": {Data: []byte(`globalThis.TallyPreviewFixture = { marker: "b-fixture" };`)},
	}
	ws, err := buildRegistry(fsys, "globalThis.ExtApps={App:1};")
	if err != nil {
		t.Fatalf("buildRegistry: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		if _, ok := ws.rendered[name]; !ok {
			t.Errorf("widget %q not rendered", name)
		}
		if _, ok := ws.preview[name]; !ok {
			t.Errorf("widget %q has no preview", name)
		}
		if strings.Contains(ws.rendered[name], BundlePlaceholder) {
			t.Errorf("widget %q placeholder not substituted", name)
		}
	}
	if strings.Contains(ws.preview["a"], "b-fixture") {
		t.Error("widget b's fixture leaked into widget a's preview")
	}
	if !strings.Contains(ws.preview["b"], "b-fixture") {
		t.Error("widget b's preview is missing its fixture")
	}
}

func TestRegistryRejectsMissingPlaceholder(t *testing.T) {
	for _, body := range []string{
		"<!doctype html><main></main>",                                  // zero
		"<!doctype html>/*__EXT_APPS_BUNDLE__*//*__EXT_APPS_BUNDLE__*/", // two
	} {
		fsys := fstest.MapFS{"src/x/index.html": {Data: []byte(body)}}
		if _, err := buildRegistry(fsys, "x"); err == nil {
			t.Errorf("expected error for placeholder count in %q", body)
		}
	}
}
