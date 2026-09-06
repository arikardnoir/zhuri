# Contributing

zhuri is a small, solo-maintained project. Contributions are welcome, but
don't expect an enterprise-style process here - just the basics that keep
the codebase consistent.

Found a security issue instead of a regular bug? Don't open an issue for
that - see [SECURITY.md](SECURITY.md).

## Before you write code

For anything bigger than a one-line fix, open an issue first describing
what you want to change and why. Saves both of us the awkwardness of a
rejected PR after the fact. Typos, doc fixes, and small obvious bugs can
just go straight to a PR.

## Building and testing

```
make build   # compiles ./zhuri
make check   # gofmt -l, go vet, go test - all three must pass
make bench   # go test -bench=. -benchmem
```

`make check` is exactly what CI runs, so if it passes locally it'll pass
there too. There's no separate linter config beyond `gofmt` and `go vet` -
keep it that simple.

## Code style

Nothing exotic, just:

- Run `gofmt` before committing (`make check` catches it if you forget).
- No comments that just restate what the code does. A comment should exist
  only when the *why* isn't obvious from the code itself - a workaround, a
  non-obvious invariant, a subtle edge case.
- Match the existing structure: `internal/detect`, `internal/fix`,
  `internal/report`, `internal/walk` each own one concern. If your change
  doesn't fit cleanly into one of those, that's worth a sentence in the
  issue/PR description explaining why a new package makes sense.
- Tests are table-driven where that fits naturally (see `*_test.go` in any
  `internal/` package for the pattern). Add fixtures to `testdata/` rather
  than inlining large byte literals when the input needs specific raw
  bytes (see how the existing Windows-1252/UTF-16 fixtures were built).
- No new dependencies unless there's genuinely no reasonable way to do it
  with the standard library plus `golang.org/x/text`. This is a small CLI
  tool, not a place to accumulate a dependency tree.

## Submitting a PR

- Keep it focused - one change, one PR. A PR that fixes a bug and
  refactors an unrelated function is two PRs.
- Update tests and, if the behavior visible to users changed, the README.
- CI (build, vet, fmt, `go test -race`) and CodeQL both have to pass
  before a merge. Branch protection enforces this, it's not optional.
- Be clear in the PR description about *why*, not just *what* - the diff
  already shows what changed.

That's it. If something here is unclear, open an issue and ask - the
first version of any guide like this is usually wrong about something.
