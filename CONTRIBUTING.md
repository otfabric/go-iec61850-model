# Contributing to go-iec61850-model

Thank you for your interest in contributing to `go-iec61850-model`! This
document covers the guidelines for contributing.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```sh
   git clone git@github.com:<your-username>/go-iec61850-model.git
   cd go-iec61850-model
   ```
3. Create a feature branch:
   ```sh
   git checkout -b feature/my-change
   ```

## Development Setup

### Prerequisites

- Go 1.24+ (see [go.mod](go.mod)); the code must build and pass on Go 1.24, so
  do not use newer language or standard-library features
- [staticcheck](https://staticcheck.dev/), [golangci-lint](https://golangci-lint.run/)
  and [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) for
  the checks
- No IEC files: all tests run on the synthetic fixtures in
  [testdata/nsd](testdata/nsd)

### Build & Test

```sh
make check   # format, tidy, vet, lint, vulnerability scan, race tests, coverage gate (90%)
make test    # unit tests
make fuzz    # fuzz targets, FUZZTIME each (default 15s)
```

Run `make` (or `make help`) for the full list of targets.

## Source Policy

This is the rule that matters most in this repository:

- **Never commit official IEC material**: NSD or NSDoc files, XSD schemas,
  ZIP packages, PDFs, or text, tables or descriptions copied from them. The
  `.gitignore` blocks the usual file types; review your diff anyway.
- **Test fixtures are independently authored.** Files in `testdata/` are
  written by hand, contain only what a test needs, and use our own wording for
  every title and description. Short identifiers needed to exercise the model
  (`MMXU`, `Hz`, `MV`, `mag`) are fine; reproducing whole class tables is not.
- **Do not paste IEC text into code comments, docs or issues.** Describe
  behavior in your own words and refer to the clause number.
- Official packages are fetched with `make nsd-fetch` into the git-ignored
  `.nsd-cache/` (or a directory outside the repository) and tested with `make
  official`; generated output goes to the git-ignored `.generated/`. Neither may
  enter Git, CI caches, logs or artifacts.

See [docs/licensing.md](docs/licensing.md).

## Making Changes

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Start every Go file with `// SPDX-License-Identifier: MIT`
- Standard library only in the library packages; discuss any new dependency
  first
- Every exported identifier has a doc comment that tells a caller something
  useful; comments end with a period (enforced by the linter)
- Deterministic output: sort map keys before producing lists, keep document
  order where the model has one

### Formatting

`make fmt` formats with the `gofmt` of the Go version in `go.mod`, which is
the version CI uses. `make fmt-check` (part of `make check`) fails if either
that `gofmt` or your local one would change a file. If they disagree, split
long lines yourself.

### Generator output

`internal/generator` output is a contract: `TestGolden` compares it with
[testdata/golden](testdata/golden). After an intentional change, bump
`generator.FormatVersion`, run `make golden` and review the diff. Never commit
other output generated from official IEC files.

### The committed generated output

[`names/`](names), [`official/`](official) (except the hand-written
`official.go`, `doc.go` and `official_test.go`) and
[`docs/reference/`](docs/reference/index.md) are generated from the official
packages; never edit them by hand. After a change of the lock (`make nsd-lock`) or of the Go generator
output, run `make nsd-fetch nsd-publish` and commit the result together with
the change. The `nsd.yml` workflow fails when it is out of date. The
downloaded packages themselves are never committed.

### Errors

Return the sentinels and typed errors of package `model` so callers can use
`errors.Is` / `errors.As` across packages, and wrap with `%w` or
`errors.Join`. Never degrade an unknown element to a generic type. See
[ERRORS.md](ERRORS.md).

### Commit Messages

```
component: short description

Optional longer explanation of the change, why it was made,
and any relevant context.
```

### Pull Requests

1. Run `make check` and make sure it passes
2. Add tests for new behavior, using synthetic fixtures
3. Update the documentation (README, ERRORS.md, docs/) when behavior changes
4. Add an entry to [RELEASE.md](RELEASE.md) under the unreleased version

## Reporting Issues

Report bugs with a minimal reproduction. Do not attach official IEC files to
issues; describe the construct (element, attributes, XML path) instead. For
security issues see [SECURITY.md](SECURITY.md).
