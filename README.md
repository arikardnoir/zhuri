# zhuri

An encoding fixer for config files. It doesn't format YAML, doesn't validate schemas - it detects bytes that aren't UTF-8, figures out where they actually came from, and converts.

## The problem

If you've hit this before, you'll recognize it instantly:

```
err: yaml: invalid trailing UTF-8 octet
```

Everyone's first reaction is to open the file, stare at the indentation, count spaces, question their own sanity. But the YAML is fine. What actually happened is that someone - usually Excel, or an editor with the wrong locale, or an `scp` run from Windows without a second thought - saved the file as Windows-1252 instead of UTF-8. `descrição` became `descri\xe7\xe3o`. `região` became `regi\xe3o`. Every accented character in that text is one or more bytes that, read as UTF-8, simply don't make sense, and your config tool's parser dies halfway through.

`yamllint` won't help you here - it just tells you the file is invalid, which you already knew. zhuri reads the raw bytes, figures out that it's Windows-1252 (or ISO-8859-1, or UTF-16 with a BOM), shows you exactly where it broke, and - if you ask it to - fixes it.

## Usage

The tool itself talks back in Portuguese (that's the language it was originally built for), so the output below is real, unedited terminal output:

```
$ zhuri config/app.yaml
app.yaml
  não é UTF-8 válido - detetado Windows-1252 (confiança 95%)
  L1  offset 6   descri\xe7\xe3o: "Servi\xe7o de entrega"
                       ^^^^^^^^         ^^^^
  L3  offset 49   regi\xe3o: sudeste
                      ^^^^
  2 linhas afetadas → usa --write para corrigir
```

("not valid UTF-8 - detected Windows-1252 (95% confidence)", "2 lines affected → use --write to fix".)

With no flags, zhuri never writes anything - it's always a preview. You only pass `--write` once you trust what you saw:

```
$ zhuri --write config/app.yaml
app.yaml
  não é UTF-8 válido - detetado Windows-1252 (confiança 95%)
  L1  offset 6   descri\xe7\xe3o: "Servi\xe7o de entrega"
                       ^^^^^^^^         ^^^^
  L3  offset 49   regi\xe3o: sudeste
                      ^^^^
  2 linhas afetadas → corrigido para UTF-8

$ cat config/app.yaml
descrição: "Serviço de entrega"
chave: valor
região: sudeste
```

Running zhuri again on a file that's already fixed does nothing - it checks UTF-8 validity first, and if everything's already fine, it leaves the file alone. That's deliberate: I want to be able to drop this into a pre-commit hook without worrying it'll go around rewriting files on every `git commit`.

The real pain shows up when you have a whole `config/` directory full of these, some fine, some not. That's what `-r` is for:

```
$ zhuri -r ./config/
```

## Installation

```
git clone <this-repository>
cd zhuri
make build
sudo make install
```

This builds a static binary (`CGO_ENABLED=0`) and copies it to `/usr/local/bin/zhuri`. Static means no libc dependency - the same binary runs on Debian, Alpine, Fedora, whatever, without you having to think about glibc versions.

Without root, install into a prefix of your own:

```
make install PREFIX=$HOME/.local
```

(make sure `$HOME/.local/bin` is on your `PATH`.) To uninstall, `make uninstall` with the same `PREFIX` you used.

If you just want the binary without installing it anywhere, `make build` leaves it at `./zhuri`. And if you have Go installed and prefer the more direct route, `go install ./cmd/zhuri` also works from the repository root.

## Flags

```
  -w, --write        apply the fixes (preview-only by default)
      --from <enc>   force the source encoding (windows-1252, iso-8859-1, utf-16, ...)
  -r, --recursive    walk directories recursively
      --backup       create a .bak file before writing
      --strip-bom    strip a UTF-8 BOM when writing
      --ext <list>   extensions to process (default: yaml,yml,json,xml,csv,env,properties,toml,ini,txt)
      --max-size <MB> per-file size limit in MB (default: 50)
      --quiet        only print files that have a problem
      --no-color     turn off colors
  -v, --version
  -h, --help
```

`--from` exists for when auto-detection gets it wrong - and it will, sometimes. Windows-1252 and ISO-8859-1 share almost every byte above 0xA0, so a purely Latin file with nothing in the 0x80–0x9F range is inherently ambiguous; zhuri assumes Windows-1252 because it's by far the more common case for files saved on Windows, but if you know it isn't, force it with `--from iso-8859-1`.

Colors turn themselves off when stdout isn't a terminal, or when `NO_COLOR` is set - you'll never see ANSI codes leaking into a pipe or a CI log.

Exit codes: `0` nothing to do or everything fixed, `1` problems remain unfixed (usually because you ran without `--write`), `2` execution error (path doesn't exist, invalid flag, etc.).

## Safety

This reads and rewrites other people's config files, so I was careful about a handful of things I consider non-negotiable:

- Never follows symlinks - not as a direct argument, not inside a directory walked with `-r`. They're reported as skipped, not followed.
- Special files (pipes, sockets, devices) are skipped; only regular files are touched.
- There's a size limit (50 MB by default, adjustable with `--max-size`) checked before reading the whole file into memory.
- Detects binaries (a null byte, or a high density of control characters) and refuses to touch them - transcoding a binary doesn't fix anything, it destroys it.
- Writes are atomic: it writes to a temporary file in the same directory and only then `rename`s it over the original. A crash halfway through never leaves the file half-written.
- Original file permissions are preserved.
- One bad file never aborts the whole batch - the error gets reported and zhuri moves on to the next one.

## Limitations

Encoding detection is heuristic, not magic. When there's no byte in the 0x80–0x9F range, Windows-1252 and ISO-8859-1 decode identically, so they're indistinguishable from the bytes alone - zhuri reports ISO-8859-1 in that case (there's no evidence either way, so it doesn't claim more certainty than it has), and only reports Windows-1252 when a byte in that range backs it up. Either way, use `--from` if you know the file's actual encoding better than the guess.

It also doesn't try to guess Shift-JIS, GBK, or anything outside the Latin family - the target is Western European Latin-script text (Portuguese, Spanish, French, German, and the like) saved on Windows. If your problem is something else, this probably isn't the right tool.

And if a file was genuinely truncated (the end is missing, it's not just the wrong encoding), zhuri detects the incomplete UTF-8 sequence at the end and cuts it off to leave the rest of the file valid - but the content that was cut off is still gone. There's no recovering data that was never actually written.

## Pre-commit hook / CI

Locally, to stop a broken accent from ever entering a commit again:

```bash
#!/bin/sh
# .git/hooks/pre-commit
zhuri -r config/ || {
  echo "há ficheiros com encoding errado em config/ - corre 'zhuri -r --write config/'"
  exit 1
}
```

In CI, the same command works unchanged: without `--write`, zhuri only checks, and exit code `1` fails the step when it finds something to fix.

## Tests

```
go test ./...
go test ./... -bench=. -benchmem
```

or, with the Makefile: `make check` (vet + gofmt + tests) and `make bench`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security issues go through
[SECURITY.md](SECURITY.md) instead, not a public issue.

## License

MIT. See [LICENSE](LICENSE).
