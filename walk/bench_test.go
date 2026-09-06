package walk

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// buildTree creates n small files spread across a handful of subdirectories,
// approximating a real config/ folder that gets walked recursively.
func buildTree(b *testing.B, n int) string {
	b.Helper()
	dir := b.TempDir()
	for i := 0; i < n; i++ {
		sub := filepath.Join(dir, fmt.Sprintf("group%d", i%10))
		if err := os.MkdirAll(sub, 0755); err != nil {
			b.Fatal(err)
		}
		path := filepath.Join(sub, fmt.Sprintf("file%d.yaml", i))
		if err := os.WriteFile(path, []byte("key: value\n"), 0644); err != nil {
			b.Fatal(err)
		}
	}
	return dir
}

func BenchmarkCollectRecursive(b *testing.B) {
	dir := buildTree(b, 2000)
	opts := Options{Recursive: true, Extensions: map[string]bool{"yaml": true}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files, _, err := Collect([]string{dir}, opts)
		if err != nil {
			b.Fatal(err)
		}
		if len(files) != 2000 {
			b.Fatalf("len(files) = %d, want 2000", len(files))
		}
	}
}
