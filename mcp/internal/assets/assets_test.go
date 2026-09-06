package assets

import "testing"

func TestBytesKnownAssets(t *testing.T) {
	cases := map[string]string{
		"icon.png":    "image/png",
		"favicon.ico": "image/x-icon",
	}
	for name, wantCT := range cases {
		data, ct, ok := Bytes(name)
		if !ok {
			t.Errorf("Bytes(%q): ok = false, want true", name)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Bytes(%q): empty data", name)
		}
		if ct != wantCT {
			t.Errorf("Bytes(%q): content-type = %q, want %q", name, ct, wantCT)
		}
	}
}

func TestBytesUnknownAsset(t *testing.T) {
	if _, _, ok := Bytes("logo.svg"); ok {
		t.Error("Bytes(\"logo.svg\"): ok = true, want false")
	}
}
