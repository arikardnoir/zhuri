package main

import (
	"fmt"
	"io"
	"os"

	"zhuri/internal/detect"
	"zhuri/internal/fix"
	"zhuri/internal/report"
)

// fileOutcome is what happened when zhuri looked at one file.
type fileOutcome struct {
	skipped    string // non-empty if the file was left untouched for a benign reason
	hadIssue   bool   // true if there was something worth reporting
	failed     bool   // true if the content could not be turned into valid UTF-8
	statusLine string
	issues     []report.LineIssue
}

func processFile(path string, opt options) (fileOutcome, error) {
	f, err := os.Open(path)
	if err != nil {
		return fileOutcome{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fileOutcome{}, err
	}
	if !info.Mode().IsRegular() {
		return fileOutcome{skipped: "ficheiro especial"}, nil
	}
	if opt.maxSize > 0 && info.Size() > opt.maxSize {
		return fileOutcome{skipped: fmt.Sprintf("excede o limite de tamanho (%d bytes)", info.Size())}, nil
	}

	limit := opt.maxSize
	if limit <= 0 {
		limit = info.Size() + 1
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return fileOutcome{}, err
	}
	if opt.maxSize > 0 && int64(len(data)) > opt.maxSize {
		return fileOutcome{skipped: "excede o limite de tamanho"}, nil
	}

	if len(data) == 0 {
		return fileOutcome{}, nil
	}

	var result detect.Result
	if opt.from != "" {
		enc, err := detect.Lookup(opt.from)
		if err != nil {
			return fileOutcome{}, err
		}
		bom, bomLen := detect.DetectBOM(data)
		result = detect.Result{Kind: detect.KindForeignEncoding, Name: opt.from, Confidence: 1, Encoding: enc, BOM: bom, BOMLen: bomLen}
	} else {
		result = detect.Detect(data)
	}

	if result.Kind == detect.KindBinary {
		return fileOutcome{skipped: "binário"}, nil
	}

	newData, statusLine, issues, failed := resolve(data, result, opt.stripBOM)

	if newData == nil && !failed {
		return fileOutcome{}, nil
	}

	outcome := fileOutcome{
		hadIssue:   true,
		failed:     failed,
		statusLine: statusLine,
		issues:     issues,
	}

	if failed {
		return outcome, nil
	}

	if opt.write {
		if err := fix.WriteAtomic(path, newData, info.Mode().Perm(), data, opt.backup); err != nil {
			return fileOutcome{}, err
		}
	}

	return outcome, nil
}

// resolve computes what the fixed content of data should look like given
// its detection result. It returns nil newData (and failed=false) when
// there is nothing to change.
func resolve(data []byte, result detect.Result, stripBOM bool) (newData []byte, statusLine string, issues []report.LineIssue, failed bool) {
	switch result.Kind {
	case detect.KindAlreadyValid:
		if result.BOM == detect.BOMUTF8 && stripBOM {
			return data[result.BOMLen:], "BOM UTF-8 removido", nil, false
		}
		return nil, "", nil, false

	case detect.KindTruncatedTail:
		issues = report.FindLineIssues(data)
		statusLine = fmt.Sprintf("sequência UTF-8 truncada no fim do ficheiro (offset %d)", result.TruncateAt)
		return data[:result.TruncateAt], statusLine, issues, false

	case detect.KindForeignEncoding:
		statusLine = fmt.Sprintf("não é UTF-8 válido - detetado %s (confiança %d%%)", result.Name, int(result.Confidence*100))

		isUTF16 := result.BOM == detect.BOMUTF16LE || result.BOM == detect.BOMUTF16BE
		if !isUTF16 {
			issues = report.FindLineIssues(data)
		}

		payload := data
		var prefix []byte
		if result.BOM == detect.BOMUTF8 {
			prefix = data[:result.BOMLen]
			payload = data[result.BOMLen:]
		}

		decoded, err := fix.Transcode(payload, result.Encoding)
		if err != nil {
			return nil, statusLine, issues, true
		}

		if len(prefix) > 0 && !stripBOM {
			out := make([]byte, 0, len(prefix)+len(decoded))
			out = append(out, prefix...)
			out = append(out, decoded...)
			return out, statusLine, issues, false
		}
		return decoded, statusLine, issues, false

	default:
		return nil, "", nil, false
	}
}
