package detect

import (
	"bytes"
	"testing"
)

// buildValidUTF8 builds a large buffer of already-valid UTF-8 text, the most
// common case in a real batch run: most files are already fine and the fast
// path (a single utf8.Valid scan) needs to stay cheap.
func buildValidUTF8(size int) []byte {
	line := []byte("descrição do serviço: entrega expressa na região sudeste, sem problemas\n")
	var buf bytes.Buffer
	for buf.Len() < size {
		buf.Write(line)
	}
	return buf.Bytes()
}

// buildWindows1252 builds a large buffer of Windows-1252 bytes: plain ASCII
// lines interspersed with Portuguese accented words, mirroring a real
// misencoded YAML/properties file.
func buildWindows1252(size int) []byte {
	asciiLine := []byte("key: value\n")
	accentedLine := []byte("descri\xe7\xe3o: \"Servi\xe7o de entrega\"\nregi\xe3o: sudeste\n")
	var buf bytes.Buffer
	for buf.Len() < size {
		buf.Write(asciiLine)
		buf.Write(accentedLine)
	}
	return buf.Bytes()
}

func BenchmarkIsValidUTF8(b *testing.B) {
	data := buildValidUTF8(5 * 1024 * 1024)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !IsValidUTF8(data) {
			b.Fatal("esperava UTF-8 válido")
		}
	}
}

func BenchmarkDetectAlreadyValid(b *testing.B) {
	data := buildValidUTF8(5 * 1024 * 1024)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if r := Detect(data); r.Kind != KindAlreadyValid {
			b.Fatal("esperava KindAlreadyValid")
		}
	}
}

func BenchmarkDetectWindows1252Large(b *testing.B) {
	data := buildWindows1252(8 * 1024 * 1024)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := Detect(data)
		if r.Kind != KindForeignEncoding {
			b.Fatal("esperava KindForeignEncoding")
		}
		out, err := r.Encoding.NewDecoder().Bytes(data)
		if err != nil {
			b.Fatal(err)
		}
		if len(out) == 0 {
			b.Fatal("saída vazia inesperada")
		}
	}
}
