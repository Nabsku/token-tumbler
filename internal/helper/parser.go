package helper

import (
	"strings"
	"unicode"
)

func IsLetter(s string) bool {
	if s == "" {
		return false
	}

	return !strings.ContainsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' && r != '_'
	})
}
