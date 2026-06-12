package misc

import (
	"slices"
	"strings"
)

func Rune(s string) rune {
	return []rune(s)[0]
}

func RuneInAlphabet(r rune) bool {
	return RuneInLowerAlphabet(r) || RuneInUpperAlphabet(r)
}

func RuneInLowerAlphabet(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func RuneInUpperAlphabet(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func SortByAlphabet(x []string) {
	slices.SortFunc(x, func(a, b string) int {
		if pa, pb := CompareStringAlphabet(a), CompareStringAlphabet(b); pa != pb {
			return pa - pb
		}
		return strings.Compare(a, b)
	})
}

func CompareStringAlphabet(s string) int {
	if len(s) != 1 {
		return len(s) + 2
	}
	if RuneInAlphabet(Rune(s)) {
		return len(s) + 1
	}
	return len(s) + 3
}
