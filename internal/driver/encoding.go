package driver

import (
	"strings"
	"unicode/utf8"
)

// decodeSource normalises a source file to UTF-8.  A leading byte order mark
// is removed and files that are not valid UTF-8 are decoded as Windows-1251,
// the encoding used by the Russian distribution of PascalABC.NET.
func decodeSource(src []byte) []byte {
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		src = src[3:]
	}
	if utf8.Valid(src) {
		return src
	}
	var b strings.Builder
	b.Grow(len(src) * 2)
	for _, c := range src {
		if c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(cp1251[c-0x80])
		}
	}
	return []byte(b.String())
}
