package detect

import (
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// namedEncodings maps the identifiers accepted by --from to their
// golang.org/x/text encoding. Several aliases point at the same encoding.
var namedEncodings = map[string]encoding.Encoding{
	"windows-1252": charmap.Windows1252,
	"cp1252":       charmap.Windows1252,
	"win-1252":     charmap.Windows1252,
	"1252":         charmap.Windows1252,
	"iso-8859-1":   charmap.ISO8859_1,
	"latin1":       charmap.ISO8859_1,
	"latin-1":      charmap.ISO8859_1,
	"8859-1":       charmap.ISO8859_1,
	"iso-8859-15":  charmap.ISO8859_15,
	"latin9":       charmap.ISO8859_15,
	"utf-16":       unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	"utf16":        unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	"utf-16le":     unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	"utf-16be":     unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
	"utf-8":        encoding.Nop,
	"utf8":         encoding.Nop,
}

// Lookup resolves a user-supplied encoding name (as passed to --from) to its
// encoding.Encoding implementation.
func Lookup(name string) (encoding.Encoding, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if enc, ok := namedEncodings[key]; ok {
		return enc, nil
	}
	return nil, fmt.Errorf("encoding desconhecido: %q (tenta windows-1252, iso-8859-1, utf-16, utf-16le, utf-16be)", name)
}
