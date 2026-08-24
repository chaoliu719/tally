package tools

import "strings"

// escapeLikeKeyword escapes the three characters that are special in a SQL
// LIKE pattern -- \ (the character SearchTransactions' query uses as its
// ESCAPE character), % and _ -- so a user-supplied keyword is always matched
// as a literal substring, never interpreted as a LIKE wildcard expression.
// \ must be escaped first: escaping % and _ afterwards leaves the
// backslashes those replacements just introduced alone, but escaping \
// second would double-escape them (see design.md's "LIKE 通配符转义"
// decision).
func escapeLikeKeyword(keyword string) string {
	keyword = strings.ReplaceAll(keyword, `\`, `\\`)
	keyword = strings.ReplaceAll(keyword, `%`, `\%`)
	keyword = strings.ReplaceAll(keyword, `_`, `\_`)
	return keyword
}
