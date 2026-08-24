package tools

import "testing"

// TestEscapeLikeKeyword is a white-box unit test of the LIKE-pattern escaping
// helper (see transactions_keyword.go and design.md's "LIKE 通配符转义"
// decision). End-to-end behavior of keyword matching through the
// search_transactions MCP tool is covered separately in
// transactions_test.go (package tools_test).
func TestEscapeLikeKeyword(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain text is untouched", input: "groceries", want: "groceries"},
		{name: "single percent", input: "100%", want: `100\%`},
		{name: "single underscore", input: "order_42", want: `order\_42`},
		{name: "single backslash", input: `C:\invoices`, want: `C:\\invoices`},
		{name: "trailing backslash", input: `invoices\`, want: `invoices\\`},
		{name: "mixed percent underscore and backslash", input: `50%_off\now`, want: `50\%\_off\\now`},
		{name: "empty string", input: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeLikeKeyword(tc.input)
			if got != tc.want {
				t.Errorf("escapeLikeKeyword(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
