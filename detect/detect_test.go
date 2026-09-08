package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatalf("ler fixture %s: %v", name, err)
	}
	return data
}

func TestIsValidUTF8(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"vazio", nil, true},
		{"ascii", []byte("hello world"), true},
		{"utf8 valido com acentos", []byte("descrição, não, região"), true},
		{"byte solto invalido", []byte{0x64, 0xe7, 0x6f}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidUTF8(tc.data); got != tc.want {
				t.Errorf("IsValidUTF8(%v) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

func TestDetectBOM(t *testing.T) {
	cases := []struct {
		name    string
		data    []byte
		wantBOM BOM
		wantLen int
	}{
		{"sem bom", []byte("hello"), BOMNone, 0},
		{"utf8 bom", []byte{0xEF, 0xBB, 0xBF, 'a'}, BOMUTF8, 3},
		{"utf16le bom", []byte{0xFF, 0xFE, 'a', 0}, BOMUTF16LE, 2},
		{"utf16be bom", []byte{0xFE, 0xFF, 0, 'a'}, BOMUTF16BE, 2},
		{"vazio", nil, BOMNone, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bom, n := DetectBOM(tc.data)
			if bom != tc.wantBOM || n != tc.wantLen {
				t.Errorf("DetectBOM = (%v, %d), want (%v, %d)", bom, n, tc.wantBOM, tc.wantLen)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"texto normal", []byte("descrição do serviço\n"), false},
		{"vazio", nil, false},
		{"tem byte nulo", []byte("abc\x00def"), true},
		{"denso em bytes de controlo", func() []byte {
			b := make([]byte, 100)
			for i := range b {
				b[i] = byte(i % 10)
			}
			return b
		}(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBinary(tc.data); got != tc.want {
				t.Errorf("IsBinary = %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("fixture binaria", func(t *testing.T) {
		data := readFixture(t, "binary.bin")
		if !IsBinary(data) {
			t.Error("esperava que binary.bin fosse detetado como binário")
		}
	})
}

func TestTruncatedTail(t *testing.T) {
	cases := []struct {
		name          string
		data          []byte
		wantTruncated bool
		wantOffset    int
	}{
		{"utf8 completo e valido", []byte("café\n"), false, 0},
		{"lead byte de 2 bytes sem continuacao", []byte{'a', 0xC3}, true, 1},
		{"lead byte de 3 bytes com apenas 1 continuacao", []byte{'a', 0xE3, 0x80}, true, 1},
		{"byte de continuacao orfao (corrupcao real, nao truncamento)", []byte{'a', 0x80, 'b'}, false, 0},
		{"lead byte 2 bytes seguido de byte invalido (corrupcao real)", []byte{'a', 0xC3, 0x20}, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTrunc, gotOffset := TruncatedTail(tc.data)
			if gotTrunc != tc.wantTruncated || (gotTrunc && gotOffset != tc.wantOffset) {
				t.Errorf("TruncatedTail(%v) = (%v, %d), want (%v, %d)", tc.data, gotTrunc, gotOffset, tc.wantTruncated, tc.wantOffset)
			}
		})
	}

	t.Run("fixture truncated.yaml", func(t *testing.T) {
		data := readFixture(t, "truncated.yaml")
		trunc, offset := TruncatedTail(data)
		if !trunc {
			t.Fatal("esperava truncamento detetado")
		}
		if offset != len(data)-1 {
			t.Errorf("offset = %d, want %d", offset, len(data)-1)
		}
	})
}

func TestDetectFixtures(t *testing.T) {
	cases := []struct {
		fixture  string
		wantKind Kind
	}{
		{"valid_utf8.yaml", KindAlreadyValid},
		{"ascii.yaml", KindAlreadyValid},
		{"windows1252.yaml", KindForeignEncoding},
		{"french.yaml", KindForeignEncoding},
		{"spanish.yaml", KindForeignEncoding},
		{"german.yaml", KindForeignEncoding},
		{"utf16le_bom.txt", KindForeignEncoding},
		{"utf8_bom.txt", KindAlreadyValid},
		{"truncated.yaml", KindTruncatedTail},
		{"binary.bin", KindBinary},
		{"empty.yaml", KindAlreadyValid},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			data := readFixture(t, tc.fixture)
			result := Detect(data)
			if result.Kind != tc.wantKind {
				t.Errorf("Detect(%s).Kind = %v, want %v", tc.fixture, result.Kind, tc.wantKind)
			}
		})
	}
}

func TestDetectAmbiguousFixtureDefaultsToISO88591(t *testing.T) {
	// windows1252.yaml has no byte in 0x80-0x9F, so Windows-1252 and
	// ISO-8859-1 are indistinguishable from its bytes alone (the fixture
	// name reflects how it was produced, not what Detect can prove about
	// it). ISO-8859-1 is the honest answer here, at reduced confidence -
	// see the comment above the has8x branch in detectPayload.
	data := readFixture(t, "windows1252.yaml")
	result := Detect(data)
	if result.Kind != KindForeignEncoding {
		t.Fatalf("Kind = %v, want KindForeignEncoding", result.Kind)
	}
	if result.Name != "ISO-8859-1" {
		t.Errorf("Name = %q, want ISO-8859-1", result.Name)
	}
	if result.Confidence < 0.9 {
		t.Errorf("Confidence = %v, want >= 0.9 (every high byte in this fixture is a recognized accent)", result.Confidence)
	}
	if result.Confidence >= 0.9+0.09 {
		t.Errorf("Confidence = %v, should stay below the has8x confidence floor", result.Confidence)
	}
	if result.Encoding == nil {
		t.Fatal("Encoding não deveria ser nil")
	}
}

func TestIsLatinAccentByte(t *testing.T) {
	cases := []struct {
		name string
		b    byte
		want bool
	}{
		{"ASCII letter", 'A', false},
		{"below the accented range", 0xBF, false}, // ¿ - punctuation, not a letter
		{"multiplication sign", 0xD7, false},
		{"division sign", 0xF7, false},
		{"ç - Portuguese/French", 0xE7, true},
		{"ñ - Spanish", 0xF1, true},
		{"Ñ - Spanish", 0xD1, true},
		{"ö - German", 0xF6, true},
		{"ß - German", 0xDF, true},
		{"ù - French", 0xF9, true},
		{"å - Scandinavian", 0xE5, true},
		{"ÿ - top of the range", 0xFF, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLatinAccentByte(tc.b); got != tc.want {
				t.Errorf("isLatinAccentByte(%#02x) = %v, want %v", tc.b, got, tc.want)
			}
		})
	}
}

func TestDetectOtherLatinLanguages(t *testing.T) {
	// Windows-1252/ISO-8859-1 aren't Portuguese-specific: every Western
	// European language sharing this codepage should get the same
	// high-confidence treatment once its accented letters are recognized.
	for _, fixture := range []string{"french.yaml", "spanish.yaml", "german.yaml"} {
		t.Run(fixture, func(t *testing.T) {
			data := readFixture(t, fixture)
			result := Detect(data)
			if result.Kind != KindForeignEncoding {
				t.Fatalf("Kind = %v, want KindForeignEncoding", result.Kind)
			}
			if result.Confidence < 0.9 {
				t.Errorf("Confidence = %v, want >= 0.9 (every high byte in %s is a recognized accent)", result.Confidence, fixture)
			}
			decoded, err := result.Encoding.NewDecoder().Bytes(data)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !IsValidUTF8(decoded) {
				t.Errorf("decoded output for %s is not valid UTF-8", fixture)
			}
		})
	}
}

func TestDetectConfidentWindows1252(t *testing.T) {
	// A byte in 0x80-0x9F is real evidence for Windows-1252 specifically:
	// ISO-8859-1 leaves that range as C1 control codes, which essentially
	// never show up in real text. 0x97 here is an em dash typed on Windows.
	data := readFixture(t, "windows1252_em_dash.yaml")
	result := Detect(data)
	if result.Kind != KindForeignEncoding {
		t.Fatalf("Kind = %v, want KindForeignEncoding", result.Kind)
	}
	if result.Name != "Windows-1252" {
		t.Errorf("Name = %q, want Windows-1252", result.Name)
	}
	if result.Confidence < 0.9 {
		t.Errorf("Confidence = %v, want >= 0.9 (0x80-0x9F byte present)", result.Confidence)
	}
	if result.Encoding == nil {
		t.Fatal("Encoding não deveria ser nil")
	}
}

func TestDetectUTF8BOMWithForeignPayload(t *testing.T) {
	// A UTF-8 BOM followed by Windows-1252 bytes: the offset carried in a
	// truncated-tail result must be relative to the whole file, not just
	// the payload after the BOM.
	payload := readFixture(t, "truncated.yaml")
	data := append([]byte{0xEF, 0xBB, 0xBF}, payload...)
	result := Detect(data)
	if result.Kind != KindTruncatedTail {
		t.Fatalf("Kind = %v, want KindTruncatedTail", result.Kind)
	}
	want := len(data) - 1
	if result.TruncateAt != want {
		t.Errorf("TruncateAt = %d, want %d (offset must include BOM length)", result.TruncateAt, want)
	}
}

func TestDetectEmpty(t *testing.T) {
	result := Detect(nil)
	if result.Kind != KindAlreadyValid {
		t.Errorf("Detect(nil).Kind = %v, want KindAlreadyValid", result.Kind)
	}
}

func TestDecodeUTF16LEFixture(t *testing.T) {
	data := readFixture(t, "utf16le_bom.txt")
	result := Detect(data)
	if result.Kind != KindForeignEncoding || result.Name != "UTF-16LE" {
		t.Fatalf("Detect(utf16le_bom.txt) = %+v", result)
	}
	out, err := result.Encoding.NewDecoder().Bytes(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := "café da manhã\n"
	if string(out) != want {
		t.Errorf("decoded = %q, want %q", out, want)
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"windows-1252", false},
		{"Windows-1252", false},
		{"iso-8859-1", false},
		{"latin1", false},
		{"utf-16", false},
		{"utf-16le", false},
		{"utf-16be", false},
		{"utf-8", false},
		{"nao-existe-2000", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Lookup(tc.name)
			if (err != nil) != tc.wantErr {
				t.Errorf("Lookup(%q) err = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestLookupISO88591DecodesFixture(t *testing.T) {
	enc, err := Lookup("iso-8859-1")
	if err != nil {
		t.Fatal(err)
	}
	data := readFixture(t, "iso8859_1.yaml")
	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		t.Fatal(err)
	}
	want := "nome: café com leite\n"
	if string(out) != want {
		t.Errorf("decoded = %q, want %q", out, want)
	}
}
