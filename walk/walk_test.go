package walk

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = 'a'
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectNonRecursiveOnlyTopLevel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.yaml"), 10)
	writeFile(t, filepath.Join(dir, "sub", "b.yaml"), 10)

	files, _, err := Collect([]string{dir}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "a.yaml" {
		t.Errorf("files = %v, want apenas a.yaml", files)
	}
}

func TestCollectRecursiveDescendsIntoSubdirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.yaml"), 10)
	writeFile(t, filepath.Join(dir, "sub", "b.yaml"), 10)
	writeFile(t, filepath.Join(dir, "sub", "deeper", "c.yaml"), 10)

	files, _, err := Collect([]string{dir}, Options{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	names := basenames(files)
	sort.Strings(names)
	want := []string{"a.yaml", "b.yaml", "c.yaml"}
	if !equalSlices(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestCollectExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.yaml"), 10)
	writeFile(t, filepath.Join(dir, "b.png"), 10)

	files, skipped, err := Collect([]string{dir}, Options{Recursive: true, Extensions: map[string]bool{"yaml": true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "a.yaml" {
		t.Errorf("files = %v, want apenas a.yaml", files)
	}
	if len(skipped) != 1 || skipped[0].Reason != SkipExt {
		t.Errorf("skipped = %v, want b.png com SkipExt", skipped)
	}
}

func TestCollectExplicitFileIgnoresExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.png")
	writeFile(t, path, 10)

	files, skipped, err := Collect([]string{path}, Options{Extensions: map[string]bool{"yaml": true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("um ficheiro passado explicitamente deve ser incluído mesmo fora da lista de extensões, files=%v skipped=%v", files, skipped)
	}
}

func TestCollectMaxSizeSkipsLargeFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "small.yaml"), 10)
	writeFile(t, filepath.Join(dir, "big.yaml"), 1000)

	files, skipped, err := Collect([]string{dir}, Options{MaxSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "small.yaml" {
		t.Errorf("files = %v, want apenas small.yaml", files)
	}
	if len(skipped) != 1 || skipped[0].Reason != SkipTooLarge {
		t.Errorf("skipped = %v, want big.yaml com SkipTooLarge", skipped)
	}
}

func TestCollectSkipsSymlinkArgument(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.yaml")
	writeFile(t, target, 10)
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks não suportados neste ambiente: %v", err)
	}

	files, skipped, err := Collect([]string{link}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("files = %v, want nenhum (symlink deve ser ignorado)", files)
	}
	if len(skipped) != 1 || skipped[0].Reason != SkipSymlink {
		t.Errorf("skipped = %v, want SkipSymlink", skipped)
	}
}

func TestCollectDoesNotFollowSymlinkedDirRecursive(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	writeFile(t, filepath.Join(realDir, "x.yaml"), 10)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlinks não suportados neste ambiente: %v", err)
	}

	files, skipped, err := Collect([]string{dir}, Options{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	names := basenames(files)
	if len(names) != 1 || names[0] != "x.yaml" {
		t.Errorf("files = %v, want apenas real/x.yaml (via caminho real, não o link)", files)
	}
	foundSymlinkSkip := false
	for _, s := range skipped {
		if s.Reason == SkipSymlink {
			foundSymlinkSkip = true
		}
	}
	if !foundSymlinkSkip {
		t.Error("esperava reportar o symlink 'link' como ignorado")
	}
}

func TestCollectSkipsSpecialFiles(t *testing.T) {
	if _, err := exec.LookPath("mkfifo"); err != nil {
		t.Skip("mkfifo não disponível")
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := exec.Command("mkfifo", fifo).Run(); err != nil {
		t.Skipf("não foi possível criar fifo: %v", err)
	}

	files, skipped, err := Collect([]string{dir}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("files = %v, want nenhum", files)
	}
	if len(skipped) != 1 || skipped[0].Reason != SkipSpecial {
		t.Errorf("skipped = %v, want SkipSpecial", skipped)
	}
}

func basenames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
