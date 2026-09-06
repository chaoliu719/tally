package tools

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/widgets"
)

// widgetResourceTTL is the freshness hint on a widget's ui:// resource read:
// how long a host may reuse a cached copy before refetching. Trade-off:
// larger = fewer refetches of the (~380KB) payload, but a new deploy takes
// this long to reach an already-open client.
const widgetResourceTTL = 5 * time.Minute

// registerWidgetResource wires the ui:// resource that serves a widget's
// self-contained HTML, with the shared freshness hint. name is the widget
// directory name under mcp/internal/widgets/src/.
func registerWidgetResource(s *mcp.Server, name, resourceName, description string) {
	uri := widgets.URI(name)
	s.AddResource(&mcp.Resource{
		Name:        resourceName,
		URI:         uri,
		MIMEType:    widgets.MIMEType,
		Description: description,
	}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		html, ok := widgets.HTML(name)
		if !ok {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		res := &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: widgets.MIMEType,
				Text:     html,
			}},
		}
		res.TTLMs = int(widgetResourceTTL / time.Millisecond)
		res.CacheScope = "public"
		return res, nil
	})
}

// widgetToolMeta is the _meta a launcher tool sets so a host that supports the
// apps surface renders the widget: {"ui": {"resourceUri": "ui://widgets/<name>.html"}}.
func widgetToolMeta(name string) mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"resourceUri": widgets.URI(name)}}
}
