package fix

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

func TestTranscodeWindows1252(t *testing.T) {
	data := []byte{'c', 'a', 'f', 0xE9} // "café" em Windows-1252
	out, err := Transcode(data, charmap.Windows1252)
	if err != nil {
		t.Fatalf("Transcode falhou: %v", err)
	}
	if string(out) != "café" {
		t.Errorf("out = %q, want %q", out, "café")
	}
	if !utf8.Valid(out) {
		t.Error("resultado deveria ser UTF-8 válido")
	}
}

func TestTranscodeInvalidUTF16(t *testing.T) {
	// Byte length ímpar sem BOM: o decoder UTF-16 não consegue produzir um
	// resultado válido a partir disto.
	data := []byte{0x00, 0x61, 0x00}
	enc := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM)
	_, err := Transcode(data, enc)
	if err == nil {
		t.Error("esperava erro ao transcodificar UTF-16 sem BOM esperado")
	}
}

func TestWriteAtomicBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	original := []byte("descri\xe7\xe3o: valor\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	fixed := []byte("descrição: valor\n")
	if err := WriteAtomic(path, fixed, 0644, original, false); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(fixed) {
		t.Errorf("conteúdo = %q, want %q", got, fixed)
	}

	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error("não devia existir .bak quando backup=false")
	}

	// Nenhum ficheiro temporário deve sobrar no diretório.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "app.yaml" {
			t.Errorf("ficheiro inesperado sobrou no diretório: %s", e.Name())
		}
	}
}

func TestWriteAtomicBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	original := []byte("regi\xe3o: sudeste\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	fixed := []byte("região: sudeste\n")
	if err := WriteAtomic(path, fixed, 0644, original, true); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("ler backup: %v", err)
	}
	if string(backup) != string(original) {
		t.Errorf("backup = %q, want conteúdo original %q", backup, original)
	}
}

func TestWriteAtomicPreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	original := []byte("a\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(path, []byte("b\n"), 0600, original, false); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissões = %v, want 0600", info.Mode().Perm())
	}
}
