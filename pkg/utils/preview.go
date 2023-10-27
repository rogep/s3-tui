package utils

import (
	"unicode/utf8"
)

func isBinaryFile(preview []byte) bool {
	return !utf8.Valid(preview)
}

func ParsePreview(preview []byte) []byte {
	bin := isBinaryFile(preview)

	if bin {
		return []byte("Cannot preview binary file")
	}
	return preview
}
