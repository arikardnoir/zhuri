// Package detect identifies whether a file's bytes are valid UTF-8 and, when
// they are not, guesses the most likely source encoding. The guess is biased
// towards Windows-1252 and ISO-8859-1, the encodings that actually show up
// in the wild for Western European Latin-script text - Portuguese, Spanish,
// French, German, and friends - saved on Windows.
package detect

import (
	"bytes"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// BOM identifies a byte-order mark found at the start of a file.
type BOM int

const (
	BOMNone BOM = iota
	BOMUTF8
	BOMUTF16LE
	BOMUTF16BE
)

func (b BOM) String() string {
	switch b {
	case BOMUTF8:
		return "UTF-8 BOM"
	case BOMUTF16LE:
		return "UTF-16LE BOM"
	case BOMUTF16BE:
		return "UTF-16BE BOM"
	default:
		return "sem BOM"
	}
}

// Kind classifies the outcome of Detect.
type Kind int

const (
	// KindAlreadyValid means the bytes are already valid UTF-8 (a BOM may
	// still be present and strippable).
	KindAlreadyValid Kind = iota
	// KindBinary means the content looks like binary data and must not be
	// touched.
	KindBinary
	// KindTruncatedTail means the file is otherwise valid UTF-8 but ends
	// with an incomplete multi-byte sequence (the classic "trailing octet"
	// error caused by a file being cut short).
	KindTruncatedTail
	// KindForeignEncoding means the bytes decode more plausibly under a
	// non-UTF-8 encoding (typically Windows-1252 or UTF-16).
	KindForeignEncoding
)

// Result describes what Detect found about a chunk of bytes.
type Result struct {
	Kind       Kind
	Name       string
	Confidence float64
	Encoding   encoding.Encoding

	BOM    BOM
	BOMLen int

	// TruncateAt is only set for KindTruncatedTail: the byte offset at
	// which the incomplete trailing sequence begins. Everything before it
	// is already valid UTF-8.
	TruncateAt int
}

// DetectBOM reports the byte-order mark at the very start of data, if any,
// and how many bytes it occupies.
func DetectBOM(data []byte) (BOM, int) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return BOMUTF8, 3
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return BOMUTF16LE, 2
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return BOMUTF16BE, 2
	default:
		return BOMNone, 0
	}
}

// IsValidUTF8 reports whether data is well-formed UTF-8.
func IsValidUTF8(data []byte) bool {
	return utf8.Valid(data)
}

// IsBinary reports whether data looks like binary content rather than text.
// A NUL byte anywhere in the sample is treated as a certain signal; beyond
// that, a high density of control characters is used as a heuristic.
func IsBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	const maxSample = 8000
	if len(sample) > maxSample {
		sample = sample[:maxSample]
	}
	if bytes.IndexByte(sample, 0x00) >= 0 {
		return true
	}
	control := 0
	for _, b := range sample {
		switch b {
		case '\n', '\r', '\t':
			continue
		}
		if b < 0x20 || b == 0x7F {
			control++
		}
	}
	return float64(control)/float64(len(sample)) > 0.3
}

// TruncatedTail reports whether data is valid UTF-8 right up until an
// incomplete multi-byte sequence at the very end. The returned offset is
// where that incomplete sequence starts.
func TruncatedTail(data []byte) (bool, int) {
	i := 0
	for i < len(data) {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			// FullRune tells us whether more bytes could still complete a
			// valid rune (truncation) or whether this is a hard error
			// regardless of how much more data followed (real corruption).
			if !utf8.FullRune(data[i:]) {
				return true, i
			}
			return false, 0
		}
		i += size
	}
	return false, 0
}

// Detect inspects data and reports its most likely encoding situation.
func Detect(data []byte) Result {
	if len(data) == 0 {
		return Result{Kind: KindAlreadyValid, Name: "utf-8"}
	}

	if bom, n := DetectBOM(data); bom != BOMNone {
		switch bom {
		case BOMUTF8:
			payload := data[n:]
			if utf8.Valid(payload) {
				return Result{Kind: KindAlreadyValid, Name: "utf-8", BOM: bom, BOMLen: n, Confidence: 1}
			}
			inner := detectPayload(payload)
			inner.BOM = bom
			inner.BOMLen = n
			if inner.Kind == KindTruncatedTail {
				inner.TruncateAt += n
			}
			return inner
		case BOMUTF16LE:
			return Result{
				Kind:       KindForeignEncoding,
				Name:       "UTF-16LE",
				Confidence: 1,
				Encoding:   unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM),
				BOM:        bom,
				BOMLen:     n,
			}
		case BOMUTF16BE:
			return Result{
				Kind:       KindForeignEncoding,
				Name:       "UTF-16BE",
				Confidence: 1,
				Encoding:   unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM),
				BOM:        bom,
				BOMLen:     n,
			}
		}
	}

	if utf8.Valid(data) {
		return Result{Kind: KindAlreadyValid, Name: "utf-8", Confidence: 1}
	}

	if IsBinary(data) {
		return Result{Kind: KindBinary, Name: "binário", Confidence: 1}
	}

	return detectPayload(data)
}

// detectPayload runs the non-UTF-8 heuristic against data that has no BOM
// (or already had it stripped by the caller).
func detectPayload(data []byte) Result {
	if truncated, at := TruncatedTail(data); truncated {
		return Result{Kind: KindTruncatedTail, Name: "UTF-8 truncado", Confidence: 1, TruncateAt: at}
	}

	has8x := false
	var highBytes, accented int
	for _, b := range data {
		if b >= 0x80 && b <= 0x9F {
			has8x = true
		}
		if b >= 0xA0 {
			highBytes++
			if isLatinAccentByte(b) {
				accented++
			}
		}
	}

	ratio := 0.0
	if highBytes > 0 {
		ratio = float64(accented) / float64(highBytes)
	}

	// Windows-1252 and ISO-8859-1 decode identically for every byte >= 0xA0;
	// the only place they differ is 0x80-0x9F, which Windows-1252 uses for
	// printable punctuation (curly quotes, em dash, etc.) and ISO-8859-1
	// leaves as C1 control codes that essentially never appear in real text.
	// So a byte in that range is real evidence for Windows-1252 specifically;
	// without it, the two are indistinguishable from the bytes alone, and
	// guessing "Windows-1252" anyway would overstate what we actually know.
	// ISO-8859-1 is reported instead in that case, at lower confidence - the
	// decoded output is byte-for-byte the same regardless of which of the
	// two gets picked here, so this only changes what gets reported, not
	// what a --write actually produces.
	if has8x {
		confidence := 0.9 + 0.09*ratio
		if confidence > 0.99 {
			confidence = 0.99
		}
		return Result{
			Kind:       KindForeignEncoding,
			Name:       "Windows-1252",
			Confidence: confidence,
			Encoding:   charmap.Windows1252,
		}
	}

	confidence := 0.75 + 0.2*ratio
	if confidence > 0.99 {
		confidence = 0.99
	}
	return Result{
		Kind:       KindForeignEncoding,
		Name:       "ISO-8859-1",
		Confidence: confidence,
		Encoding:   charmap.ISO8859_1,
	}
}

// isLatinAccentByte reports whether b, interpreted as a Latin-1 /
// Windows-1252 code point, is an accented Latin letter. 0xC0-0xFF is
// accented letters for every Western European language that shares this
// codepage - Portuguese, Spanish, French, Italian, German, the Scandinavian
// languages, and more - with exactly two exceptions sitting in the middle
// of that range: 0xD7 (multiplication sign) and 0xF7 (division sign), which
// are symbols, not letters.
func isLatinAccentByte(b byte) bool {
	return b >= 0xC0 && b != 0xD7 && b != 0xF7
}
