package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

// copyFixture copies a testdata fixture into dir under the given name and
// returns its full path.
func copyFixture(t *testing.T, dir, fixture, as string) string {
	t.Helper()
	data, err := os.ReadFile(fixturePath(fixture))
	if err != nil {
		t.Fatalf("ler fixture %s: %v", fixture, err)
	}
	dst := filepath.Join(dir, as)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func runZhuri(args ...string) (stdout, stderr string, code int) {
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

func TestPreviewModeDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "windows1252.yaml", "app.yaml")
	before, _ := os.ReadFile(path)

	stdout, _, code := runZhuri(path)

	if code != exitProblems {
		t.Errorf("code = %d, want %d", code, exitProblems)
	}
	if !strings.Contains(stdout, "Windows-1252") {
		t.Errorf("stdout devia mencionar Windows-1252, got: %s", stdout)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("modo de pré-visualização não deveria alterar o ficheiro")
	}
}

func TestWriteModeFixesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "windows1252.yaml", "app.yaml")

	stdout, _, code := runZhuri("--write", path)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stdout=%s", code, exitOK, stdout)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "descrição: \"Serviço de entrega\"\nchave: valor\nregião: sudeste\n"
	if string(got) != want {
		t.Errorf("conteúdo = %q, want %q", got, want)
	}

	// Segunda passagem: nada deve mudar.
	stdout2, _, code2 := runZhuri("--write", path)
	if code2 != exitOK {
		t.Errorf("segunda passagem code = %d, want %d", code2, exitOK)
	}
	if !strings.Contains(stdout2, "já é UTF-8 válido") {
		t.Errorf("segunda passagem deveria reportar já-válido, got: %s", stdout2)
	}
	got2, _ := os.ReadFile(path)
	if string(got2) != want {
		t.Error("segunda passagem não deveria alterar o ficheiro (idempotência)")
	}
}

func TestFromOverrideISO88591(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "iso8859_1.yaml", "app.yaml")

	_, _, code := runZhuri("--write", "--from", "iso-8859-1", path)
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "nome: café com leite\n"
	if string(got) != want {
		t.Errorf("conteúdo = %q, want %q", got, want)
	}
}

func TestFromInvalidEncodingIsExecError(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "windows1252.yaml", "app.yaml")

	_, stderr, code := runZhuri("--from", "encoding-que-nao-existe", path)
	if code != exitError {
		t.Errorf("code = %d, want %d", code, exitError)
	}
	if stderr == "" {
		t.Error("esperava mensagem de erro em stderr")
	}
}

func TestUTF8BOMDefaultKeepsBOM(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "utf8_bom.txt", "app.txt")
	before, _ := os.ReadFile(path)

	_, _, code := runZhuri("--write", path)
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("sem --strip-bom, o BOM UTF-8 não deveria ser tocado")
	}
	if !bytes.HasPrefix(after, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("BOM UTF-8 deveria continuar presente")
	}
}

func TestUTF8BOMStripped(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "utf8_bom.txt", "app.txt")

	_, _, code := runZhuri("--write", "--strip-bom", path)
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	after, _ := os.ReadFile(path)
	if bytes.HasPrefix(after, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("BOM UTF-8 deveria ter sido removido")
	}
}

func TestUTF16BOMConverted(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "utf16le_bom.txt", "app.txt")

	_, _, code := runZhuri("--write", path)
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "café da manhã\n"
	if string(got) != want {
		t.Errorf("conteúdo = %q, want %q", got, want)
	}
}

func TestBinaryFileIsSkippedUntouched(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "binary.bin", "app.bin")
	before, _ := os.ReadFile(path)

	stdout, _, code := runZhuri("--write", "--ext", "bin", path)
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "binário") {
		t.Errorf("stdout devia mencionar 'binário', got: %s", stdout)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("ficheiro binário nunca deve ser alterado")
	}
}

func TestTruncatedTailFixTruncatesToLastValidByte(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "truncated.yaml", "app.yaml")
	original, _ := os.ReadFile(path)

	stdout, _, code := runZhuri(path)
	if code != exitProblems {
		t.Errorf("code = %d, want %d", code, exitProblems)
	}
	if !strings.Contains(stdout, "truncad") {
		t.Errorf("stdout devia mencionar truncamento, got: %s", stdout)
	}

	_, _, code2 := runZhuri("--write", path)
	if code2 != exitOK {
		t.Fatalf("code = %d, want %d", code2, exitOK)
	}
	fixed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed) != len(original)-1 {
		t.Errorf("len(fixed) = %d, want %d", len(fixed), len(original)-1)
	}
	if !bytes.Equal(fixed, original[:len(original)-1]) {
		t.Error("conteúdo corrigido deveria ser exatamente o prefixo válido")
	}
}

func TestEmptyFileNoProblem(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "empty.yaml", "app.yaml")

	_, _, code := runZhuri(path)
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
}

func TestMaxSizeSkipsBigFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.yaml")
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), 2000), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runZhuri("--max-size", "0", path)
	_ = stdout
	if code != exitOK {
		t.Errorf("--max-size 0 (ilimitado) code = %d, want %d", code, exitOK)
	}

	// max-size é em MB no CLI; força um limite efetivo bem pequeno usando
	// um ficheiro maior que 1 MB não é prático num teste, por isso
	// verificamos o caminho de "excede o limite" diretamente via processFile.
	out, err := processFile(path, options{maxSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if out.skipped == "" {
		t.Error("esperava que o ficheiro fosse ignorado por exceder o limite de tamanho")
	}
}

func TestBackupCreatesBakFile(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "windows1252.yaml", "app.yaml")
	original, _ := os.ReadFile(path)

	_, _, code := runZhuri("--write", "--backup", path)
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("ler .bak: %v", err)
	}
	if !bytes.Equal(backup, original) {
		t.Error(".bak deveria conter o conteúdo original")
	}
}

func TestNoArgsIsExecError(t *testing.T) {
	_, _, code := runZhuri()
	if code != exitError {
		t.Errorf("code = %d, want %d", code, exitError)
	}
}

func TestVersionFlag(t *testing.T) {
	stdout, _, code := runZhuri("--version")
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "zhuri") {
		t.Errorf("stdout = %q, want conter 'zhuri'", stdout)
	}
}

func TestHelpFlag(t *testing.T) {
	_, stderr, code := runZhuri("--help")
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stderr, "zhuri") {
		t.Errorf("stderr da ajuda deveria mencionar zhuri, got: %s", stderr)
	}
}

func TestUnknownFlagIsExecError(t *testing.T) {
	_, _, code := runZhuri("--isto-nao-existe")
	if code != exitError {
		t.Errorf("code = %d, want %d", code, exitError)
	}
}

func TestRecursiveDirectoryScan(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir, "windows1252.yaml", "a.yaml")
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, sub, "windows1252.yaml", "b.yaml")

	stdout, _, code := runZhuri("-r", dir)
	if code != exitProblems {
		t.Errorf("code = %d, want %d", code, exitProblems)
	}
	if !strings.Contains(stdout, "a.yaml") || !strings.Contains(stdout, "b.yaml") {
		t.Errorf("stdout devia mencionar ambos os ficheiros, got: %s", stdout)
	}
}

func TestSymlinkArgumentIsSkipped(t *testing.T) {
	dir := t.TempDir()
	target := copyFixture(t, dir, "windows1252.yaml", "real.yaml")
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks não suportados neste ambiente: %v", err)
	}

	stdout, _, code := runZhuri(link)
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "symlink") {
		t.Errorf("stdout devia mencionar symlink, got: %s", stdout)
	}
}

func TestQuietSuppressesCleanOutput(t *testing.T) {
	dir := t.TempDir()
	path := copyFixture(t, dir, "valid_utf8.yaml", "app.yaml")

	stdout, _, code := runZhuri("--quiet", path)
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("--quiet não deveria imprimir nada para um ficheiro sem problemas, got: %q", stdout)
	}
}
