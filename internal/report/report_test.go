package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("ler fixture %s: %v", name, err)
	}
	return data
}

func TestFindLineIssuesCleanFile(t *testing.T) {
	for _, fixture := range []string{"valid_utf8.yaml", "ascii.yaml", "empty.yaml"} {
		t.Run(fixture, func(t *testing.T) {
			issues := FindLineIssues(readFixture(t, fixture))
			if len(issues) != 0 {
				t.Errorf("FindLineIssues(%s) = %d problemas, want 0", fixture, len(issues))
			}
		})
	}
}

func TestFindLineIssuesWindows1252(t *testing.T) {
	data := readFixture(t, "windows1252.yaml")
	issues := FindLineIssues(data)
	if len(issues) != 2 {
		t.Fatalf("len(issues) = %d, want 2", len(issues))
	}

	first := issues[0]
	if first.LineNum != 1 {
		t.Errorf("issues[0].LineNum = %d, want 1", first.LineNum)
	}
	if first.Offset != 6 {
		t.Errorf("issues[0].Offset = %d, want 6", first.Offset)
	}
	wantRendered := `descri\xe7\xe3o: "Servi\xe7o de entrega"`
	if first.Rendered != wantRendered {
		t.Errorf("issues[0].Rendered = %q, want %q", first.Rendered, wantRendered)
	}
	wantMarker := strings.Repeat(" ", 6) + strings.Repeat("^", 8) + strings.Repeat(" ", 9) + strings.Repeat("^", 4)
	if first.Marker != wantMarker {
		t.Errorf("issues[0].Marker = %q, want %q", first.Marker, wantMarker)
	}
	if first.Count != 3 {
		t.Errorf("issues[0].Count = %d, want 3", first.Count)
	}

	second := issues[1]
	if second.LineNum != 3 {
		t.Errorf("issues[1].LineNum = %d, want 3", second.LineNum)
	}
	if second.Offset != 49 {
		t.Errorf("issues[1].Offset = %d, want 49", second.Offset)
	}
	wantRendered2 := `regi\xe3o: sudeste`
	if second.Rendered != wantRendered2 {
		t.Errorf("issues[1].Rendered = %q, want %q", second.Rendered, wantRendered2)
	}
	wantMarker2 := strings.Repeat(" ", 4) + strings.Repeat("^", 4)
	if second.Marker != wantMarker2 {
		t.Errorf("issues[1].Marker = %q, want %q", second.Marker, wantMarker2)
	}
}

func TestFindLineIssuesTruncated(t *testing.T) {
	data := readFixture(t, "truncated.yaml")
	issues := FindLineIssues(data)
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].LineNum != 2 {
		t.Errorf("LineNum = %d, want 2", issues[0].LineNum)
	}
	if issues[0].Offset != len(data)-1 {
		t.Errorf("Offset = %d, want %d", issues[0].Offset, len(data)-1)
	}
}

func TestPrintReportGolden(t *testing.T) {
	data := readFixture(t, "windows1252.yaml")
	issues := FindLineIssues(data)

	var buf bytes.Buffer
	footer := LineCountLabel(len(issues)) + " → corrigido para UTF-8"
	PrintReport(&buf, false, "windows1252.yaml", "não é UTF-8 válido - detetado Windows-1252 (confiança 95%)", issues, footer)

	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "report_windows1252.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != string(want) {
		t.Errorf("output não corresponde ao golden file:\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}

func TestPrintReportNoColorCodesWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	PrintReport(&buf, false, "f.yaml", "status", []LineIssue{{LineNum: 1, Offset: 0, Rendered: "x", Marker: "^"}}, "rodapé")
	if strings.Contains(buf.String(), "\x1b[") {
		t.Error("não deveria conter códigos ANSI quando color=false")
	}
}

func TestPrintReportColorCodesWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	PrintReport(&buf, true, "f.yaml", "status", []LineIssue{{LineNum: 1, Offset: 0, Rendered: "x", Marker: "^"}}, "rodapé")
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Error("esperava códigos ANSI quando color=true")
	}
}

func TestLineCountLabel(t *testing.T) {
	if got := LineCountLabel(1); got != "1 linha afetada" {
		t.Errorf("LineCountLabel(1) = %q", got)
	}
	if got := LineCountLabel(2); got != "2 linhas afetadas" {
		t.Errorf("LineCountLabel(2) = %q", got)
	}
}

func TestShouldColorRespectsNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if ShouldColor(os.Stdout, false) {
		t.Error("NO_COLOR deveria desligar as cores mesmo sem --no-color")
	}
}

func TestShouldColorRespectsFlag(t *testing.T) {
	if ShouldColor(os.Stdout, true) {
		t.Error("--no-color deveria desligar as cores")
	}
}

func TestShouldColorFalseForNonFile(t *testing.T) {
	var buf bytes.Buffer
	if ShouldColor(&buf, false) {
		t.Error("um io.Writer que não é *os.File nunca deve ter cor")
	}
}
