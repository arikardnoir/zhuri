// Package fix turns detected non-UTF-8 bytes into valid UTF-8 and writes the
// result back to disk safely.
package fix

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"golang.org/x/text/encoding"
)

// Transcode decodes data using enc and confirms the result is valid UTF-8.
// It never returns invalid UTF-8: if the decoded bytes still fail
// validation, it returns an error instead so the caller can refuse to write.
func Transcode(data []byte, enc encoding.Encoding) ([]byte, error) {
	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("falha ao transcodificar: %w", err)
	}
	if !utf8.Valid(out) {
		return nil, fmt.Errorf("resultado continua a não ser UTF-8 válido depois de transcodificar")
	}
	return out, nil
}

// WriteAtomic writes data to path without ever leaving a half-written file
// in its place: it writes to a temporary file in the same directory, sets
// its permissions to match perm, and renames it over the original. If
// backup is true, the original bytes are saved to path+".bak" first.
func WriteAtomic(path string, data []byte, perm os.FileMode, original []byte, backup bool) error {
	dir := filepath.Dir(path)

	if backup {
		if err := os.WriteFile(path+".bak", original, perm); err != nil {
			return fmt.Errorf("falha ao criar backup: %w", err)
		}
	}

	tmp, err := os.CreateTemp(dir, ".zhuri-tmp-*")
	if err != nil {
		return fmt.Errorf("falha ao criar ficheiro temporário: %w", err)
	}
	tmpPath := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("falha ao escrever ficheiro temporário: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("falha ao fechar ficheiro temporário: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("falha ao ajustar permissões: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("falha ao substituir ficheiro original: %w", err)
	}

	ok = true
	return nil
}
