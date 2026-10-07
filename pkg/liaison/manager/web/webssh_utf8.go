package web

import (
	"strings"
	"unicode/utf8"
)

// webSSHDecodeOutput retains incomplete UTF-8 bytes until the next SSH read.
// JSON encoding otherwise replaces each partial byte with a replacement rune.
func webSSHDecodeOutput(pending []byte, final bool) (string, []byte) {
	end := 0
	for end < len(pending) {
		if !final && !utf8.FullRune(pending[end:]) {
			break
		}
		_, size := utf8.DecodeRune(pending[end:])
		end += size
	}
	return strings.ToValidUTF8(string(pending[:end]), "\uFFFD"), pending[end:]
}
