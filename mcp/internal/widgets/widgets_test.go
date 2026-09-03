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
