package misc

import "strings"

func JoinNewLine(str ...string) string {
	return strings.Join(str, "\n")
}

func CompareString(a, b string, caseInsensitive ...bool) bool {
	if HasTrueOption(caseInsensitive) {
		return strings.EqualFold(a, b)
	}
	return a == b
}
