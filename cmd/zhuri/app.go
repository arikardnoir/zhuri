package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"zhuri/detect"
	"zhuri/report"
	"zhuri/walk"
)

// version is set at build time via -ldflags "-X main.version=...". A plain
// `go build`/`go run` (development, tests) keeps this fallback.
var version = "dev"

const (
	exitOK       = 0
	exitProblems = 1
	exitError    = 2
)

const defaultExtensions = "yaml,yml,json,xml,csv,env,properties,toml,ini,txt"

const defaultMaxSizeMB int64 = 50

type options struct {
	write     bool
	from      string
	recursive bool
	backup    bool
	stripBOM  bool
	quiet     bool
	noColor   bool
	color     bool // resolved value, set after parsing
	maxSize   int64
}

func usage(w io.Writer) {
	fmt.Fprint(w, `zhuri [flags] <ficheiro|pasta> [mais...]

Corrige ficheiros de configuração cujos acentos foram gravados em
Windows-1252/ISO-8859-1 em vez de UTF-8, mostrando exatamente onde estavam
os bytes partidos.

Flags:
  -w, --write        aplica as correções (por defeito só pré-visualiza)
      --from <enc>   força o encoding de origem (windows-1252, iso-8859-1, utf-16, ...)
  -r, --recursive    percorre pastas recursivamente
      --backup       cria .bak antes de escrever
      --strip-bom    remove BOM de UTF-8 ao gravar
      --ext <lista>  extensões a processar (defeito: `+defaultExtensions+`)
      --max-size <MB> limite de tamanho por ficheiro em MB (defeito: 50)
      --quiet        só imprime ficheiros com problema
      --no-color     desliga cores
  -v, --version      mostra a versão
  -h, --help         mostra esta ajuda

Códigos de saída: 0 nada a fazer / tudo corrigido, 1 problemas por corrigir, 2 erro de execução.
`)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("zhuri", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr) }

	var opt options
	var extList string
	var showVersion bool

	fs.BoolVar(&opt.write, "write", false, "aplica as correções")
	fs.BoolVar(&opt.write, "w", false, "atalho para --write")
	fs.StringVar(&opt.from, "from", "", "força o encoding de origem")
	fs.BoolVar(&opt.recursive, "recursive", false, "percorre pastas recursivamente")
	fs.BoolVar(&opt.recursive, "r", false, "atalho para --recursive")
	fs.BoolVar(&opt.backup, "backup", false, "cria .bak antes de escrever")
	fs.BoolVar(&opt.stripBOM, "strip-bom", false, "remove BOM de UTF-8 ao gravar")
	fs.StringVar(&extList, "ext", defaultExtensions, "extensões a processar")
	fs.Int64Var(&opt.maxSize, "max-size", defaultMaxSizeMB, "limite de tamanho por ficheiro em MB")
	fs.BoolVar(&opt.quiet, "quiet", false, "só imprime ficheiros com problema")
	fs.BoolVar(&opt.noColor, "no-color", false, "desliga cores")
	fs.BoolVar(&showVersion, "version", false, "mostra a versão")
	fs.BoolVar(&showVersion, "v", false, "atalho para --version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	if showVersion {
		fmt.Fprintln(stdout, "zhuri", version)
		return exitOK
	}

	paths := fs.Args()
	if len(paths) == 0 {
		usage(stderr)
		return exitError
	}

	if opt.from != "" {
		if _, err := detect.Lookup(opt.from); err != nil {
			fmt.Fprintln(stderr, "erro:", err)
			return exitError
		}
	}

	opt.color = report.ShouldColor(stdout, opt.noColor)
	opt.maxSize = opt.maxSize * 1024 * 1024

	extensions := map[string]bool{}
	for _, e := range strings.Split(extList, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		e = strings.TrimPrefix(e, ".")
		if e != "" {
			extensions[e] = true
		}
	}

	files, skipped, err := walk.Collect(paths, walk.Options{
		Recursive:  opt.recursive,
		MaxSize:    opt.maxSize,
		Extensions: extensions,
	})
	if err != nil {
		fmt.Fprintln(stderr, "erro:", err)
		return exitError
	}

	if !opt.quiet {
		for _, s := range skipped {
			fmt.Fprintf(stdout, "%s: ignorado (%s)\n", s.Path, skipReasonLabel(s))
		}
	}

	unresolvedProblem := false
	execError := false

	for _, path := range files {
		outcome, err := processFile(path, opt)
		if err != nil {
			fmt.Fprintf(stderr, "%s: erro: %v\n", path, err)
			execError = true
			continue
		}
		if outcome.skipped != "" {
			if !opt.quiet {
				fmt.Fprintf(stdout, "%s: ignorado (%s)\n", path, outcome.skipped)
			}
			continue
		}
		if !outcome.hadIssue {
			if !opt.quiet {
				fmt.Fprintf(stdout, "%s: já é UTF-8 válido\n", path)
			}
			continue
		}

		var footer string
		switch {
		case outcome.failed:
			footer = "não foi possível corrigir automaticamente - usa --from para forçar o encoding"
			unresolvedProblem = true
		case opt.write:
			footer = withCount(outcome.issues, "corrigido para UTF-8")
		default:
			footer = withCount(outcome.issues, "usa --write para corrigir")
			unresolvedProblem = true
		}

		report.PrintReport(stdout, opt.color, path, outcome.statusLine, outcome.issues, footer)
	}

	switch {
	case execError:
		return exitError
	case unresolvedProblem:
		return exitProblems
	default:
		return exitOK
	}
}

func withCount(issues []report.LineIssue, action string) string {
	if len(issues) == 0 {
		return action
	}
	return report.LineCountLabel(len(issues)) + " → " + action
}

func skipReasonLabel(s walk.Skipped) string {
	if s.Reason == walk.SkipTooLarge {
		return fmt.Sprintf("%s: %d bytes", s.Reason, s.Size)
	}
	return string(s.Reason)
}
