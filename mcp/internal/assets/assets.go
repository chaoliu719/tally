// Package assets embeds tally-mcp's static brand files (the server icon and
// favicon) so they can be served from fixed, unauthenticated HTTP routes.
// Regenerate the files from the source illustration with generate.py.
package assets

import _ "embed"

//go:embed icon.png
var iconPNG []byte

//go:embed favicon.ico
var faviconICO []byte

// Bytes returns the embedded asset for name along with the Content-Type to
// serve it with. ok is false for an unknown name.
func Bytes(name string) (data []byte, contentType string, ok bool) {
	switch name {
	case "icon.png":
		return iconPNG, "image/png", true
	case "favicon.ico":
		return faviconICO, "image/x-icon", true
	default:
		return nil, "", false
	}
}
