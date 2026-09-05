// Package report renders the broken lines of a non-UTF-8 file for the
// terminal: which line, which byte offset, and exactly which bytes were
// invalid.
package report

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// LineIssue describes one line of a file that contains invalid UTF-8 bytes.
type LineIssue struct {
	LineNum  int    // 1-based line number
	Offset   int    // byte offset of the first invalid byte, within the whole file
	Rendered string // the line, with invalid bytes shown as \xNN
	Marker   string // caret line, aligned under Rendered, pointing at the \xNN runs
	Count    int    // number of invalid byte-runs on this line
}

// FindLineIssues scans data line by line and reports every line that
// contains invalid UTF-8, along with a terminal-safe rendering of it.
func FindLineIssues(data []byte) []LineIssue {
	var issues []LineIssue
	lineNum := 0
	start := 0
	for start <= len(data) {
		lineNum++
		end := start
		for end < len(data) && data[end] != '\n' {
			end++
		}
		line := data[start:end]
		if issue, ok := analyzeLine(line, lineNum, start); ok {
			issues = append(issues, issue)
		}
		if end == len(data) {
			break
		}
		start = end + 1
	}
	return issues
}

func analyzeLine(line []byte, lineNum, startOffset int) (LineIssue, bool) {
	if utf8.Valid(line) {
		return LineIssue{}, false
	}

	var content, marker strings.Builder
	firstOffset := -1
	count := 0
	i := 0
	for i < len(line) {
		r, size := utf8.DecodeRune(line[i:])
		if r == utf8.RuneError && size == 1 {
			if firstOffset == -1 {
				firstOffset = startOffset + i
			}
			count++
			seg := fmt.Sprintf(`\x%02x`, line[i])
			content.WriteString(seg)
			marker.WriteString(strings.Repeat("^", len(seg)))
			i++
			continue
		}
		content.WriteString(string(r))
		marker.WriteString(" ")
		i += size
	}

	return LineIssue{
		LineNum:  lineNum,
		Offset:   firstOffset,
		Rendered: content.String(),
		Marker:   strings.TrimRight(marker.String(), " "),
		Count:    count,
	}, true
}

// ansi codes used for the (optional) coloured report.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiCyan   = "\x1b[36m"
)

func colorize(s, code string, enabled bool) string {
	if !enabled || s == "" {
		return s
	}
	return code + s + ansiReset
}

// ShouldColor decides whether ANSI colour codes should be emitted for w,
// honouring --no-color and the NO_COLOR convention (https://no-color.org).
func ShouldColor(w io.Writer, noColorFlag bool) bool {
	if noColorFlag {
		return false
	}
	if _, present := os.LookupEnv("NO_COLOR"); present {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// PrintReport prints the full report for one file: its name, a status line,
// every broken line with its marker, and a footer line.
func PrintReport(w io.Writer, color bool, filename, statusLine string, issues []LineIssue, footerLine string) {
	fmt.Fprintln(w, colorize(filename, ansiBold, color))
	if statusLine != "" {
		fmt.Fprintln(w, "  "+colorize(statusLine, ansiYellow, color))
	}
	for _, issue := range issues {
		prefix := "  L" + strconv.Itoa(issue.LineNum) + "  offset " + strconv.Itoa(issue.Offset) + "   "
		fmt.Fprintln(w, prefix+issue.Rendered)
		fmt.Fprintln(w, strings.Repeat(" ", len(prefix))+colorize(issue.Marker, ansiRed, color))
	}
	if footerLine != "" {
		fmt.Fprintln(w, "  "+colorize(footerLine, ansiCyan, color))
	}
}

// LineCountLabel formats "N linha(s) afetada(s)" with correct pluralization.
func LineCountLabel(n int) string {
	return strconv.Itoa(n) + " " + pluralize(n, "linha afetada", "linhas afetadas")
}
