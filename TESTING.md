# Testing

How the tests of go-iec61850-model are organised and run. The library and
`nsdctl` (the nested module `cmd/nsdctl`) are tested separately; every `make`
target below covers both unless it says otherwise.

## Tiers

| Tier | Command | Needs | Runs in CI |
| --- | --- | --- | --- |
| Unit tests | `make test` | nothing: synthetic fixtures in [testdata/nsd](testdata/nsd) | `ci.yml`, one run per module |
| Race detector | `make test-race` (part of `make check`) | nothing | `ci.yml` |
| Coverage gates | `make coverage-check coverage-pkg-check` (part of `make check`) | nothing | `ci.yml`: 90 % in total (`coverage-min`) and per package (`package-coverage`) |
| Fuzz targets | `make fuzz` (`FUZZTIME`, default 15s per target) | nothing | `ci.yml`: `fuzz-time: 10s` for both modules |
| Benchmarks | `make bench` | nothing | no |
| Golden generator output | `TestGolden` (unit tier); `make golden` rewrites it | nothing | `ci.yml`, with the unit tests |
| Official packages | `make nsd-fetch nsd-check official` | the IEC packages, downloaded into `.nsd-cache` (network) | `nsd.yml` |
| Committed output | `make nsd-publish-check` | the same | `nsd.yml` |
| Local doc set | `make test-docs` | `official/docs_gen.json.gz`, extracted from IEC PDFs you hold | no |

[CONTRIBUTING.md](CONTRIBUTING.md#when-ci-runs) says when each workflow runs.

Tests that need a prerequisite skip without it and say why:

| Test | Skips unless | Fails instead of skipping with |
| --- | --- | --- |
| `TestOfficialPackages` (`cmd/nsdctl`) | `NSD_DIR` names a directory with the official packages (`make official` sets it) | `make official`, which requires the directory |
| the tests of `official/docs_real_test.go` | a local doc set is embedded (`-tags iecdocs`) or named by `IEC61850_MODEL_DOCS` | `REQUIRE_IEC_DOCS=1` (`make test-docs` sets it) |

No test downloads anything, and no test reads an IEC file from the
repository: there is none ([docs/licensing.md](docs/licensing.md)).

## Coverage

`COVERAGE_MIN` and `COVER_PKG_MIN` in the [Makefile](Makefile) are 90. The
only package left out of the per-package gate is `internal/fixtures`, a helper
that loads the synthetic fixtures and is imported by tests only; `codecov.yml`
leaves it out for the same reason. The gates are never lowered to make a
change pass ([AGENTS.md](AGENTS.md)).

## Limits and allocation budgets

Every limit of [SECURITY.md](SECURITY.md#limits) has a test at its boundary
(one below, at, one above) and a test of what a refused call allocated. An
error alone does not show that a limit works: the call may have built what
the limit should prevent before it returned.

- `nsd/budget_test.go`: the limits of a document, and the cost of an import
  as a function of its size. `checkLinear` imports a document of n and of 2n
  elements and requires that the larger one costs less than three times the
  smaller, and that both stay below a number of bytes per byte of document.
- `nsd/archive_limits_test.go`, `nsd/manifest_limits_test.go`: the limits of
  archives and manifests. The entry limit is measured in allocations
  (`testing.AllocsPerRun`), the others in bytes (`runtime.ReadMemStats`
  around one call).
- `resolver/limits_test.go`, `resolver/limits_internal_test.go`: the
  expansion and depth limits of `Leaves`, and the FC search of
  `ResolvePath`, which examines each CDC once per lookup: measured in CDC
  lookups through a `Lookup` that counts them, and checked against its
  definition for every data object of the embedded catalog.
- `cmd/nsdctl/check_test.go`: the limits of an SCL file.

The budgets are about twice what the code costs with Go 1.27 on arm64, with
and without the race detector. A budget that fails after a change of the Go
release is a reason to measure again, not to raise it unseen: the test output
(`go test -v`) prints what each shape cost.

`resolver/race_test.go` and `norace_test.go` exist for two tests over every
class of the embedded catalog: expanding them takes 15 s under the race
detector and searching every FC of their data objects 12 s, so that run
takes the largest class only, and the run without it all of them.

## Fuzz targets

`make fuzz` finds the targets with `go test -list '^Fuzz'` in both modules,
so a new target runs without being listed anywhere. Each asserts invariants,
not only that the call returns:

| Target | Invariants |
| --- | --- |
| `nsd.FuzzParse` | a document or an error, never both; the error matches `ErrInvalidNSD`, `ErrUnsupportedSchema` or `ErrLimitExceeded`; at most 10 000 findings of bounded size; an accepted namespace validates and its literal values are 32-bit; allocation budget |
| `nsd.FuzzReadZIP` | a package or an error of a documented category; namespaces ordered and distinct, with the archive digest; at most 100 000 findings |
| `nsd.FuzzReadManifest`, `nsd.FuzzParseSchema` | result or documented error; the fixed bounds of a manifest; sorted, distinct schema names; allocation budget |
| `nsd.FuzzReadDocSets` | a set that reads encodes and reads back with the same keys |
| `reference.FuzzParse`, `FuzzParseClass`, `FuzzNormalize`, `FuzzFromMMS`, `FuzzParseLNName` | a parsed reference prints to a string that parses to the same reference; every error is a `*model.ReferenceError` |
| `model.FuzzParseNamespaceID` | a parsed ID prints to a string that parses; every error matches `ErrUnknownNamespace` |
| `presence.FuzzReadInstance` | an instance that reads evaluates; every error matches `ErrInvalidInstance` |
| `resolver.FuzzLeaves` | an expansion returns at most 100 000 leaves, each a path of the class that ends at an attribute, or a documented error |
| `generator.FuzzGenerateGo` | generation succeeds for any names, and the generated files declare what the model has and nothing else |
| `nsdctl` `FuzzReadSCL` | an instance or an error that matches `ErrInvalidInstance`; an instance has at most 100 000 names from its templates |

`testdata/fuzz/` is ignored by Git. When a target fails, the input it writes
there reproduces the failure (`go test -run 'FuzzX/<name>'`); turn it into a
named case of a table test, or into a seed (`f.Add`), in the change that
fixes it, and delete the file.

## Ownership

Values the packages hand out are the caller's. `official/ownership_test.go`
and `ontology/ownership_test.go` change what `official.Docs`, `PublicDocs`
and an `ontology.Graph` return and read again; under the race detector they
do so from several goroutines while profiles are built.

## Lifecycle contract tests: not applicable

The lifecycle matrix of the organisation's standards (closing twice, closing
during a read or a write, cancellation, late responses, callbacks, queues,
reconnect, shutdown) does not apply to this module, and no test is missing
for it. No type of the library has a `Close`, a connection, a handshake, a
goroutine, a queue or a reconnect: every call is synchronous, runs on the
caller's goroutine and returns when it is done. `ontology.Graph.Walk` calls
its function on the caller's goroutine, without a lock held. The only shared
state is what `official` builds once and keeps; the concurrent readers of it
are tested under the race detector (`TestConcurrentReads` in `catalog`,
`TestConcurrentDescribe` in `semantic`, `TestDocumentedProfilesRace` and
`TestDocSetsConcurrentOwners` in `official`).

`nsdctl` is a command that runs once. Its only network use is `sources
fetch`, one HTTPS request at a time under the context of the command; an
interrupt ends it with exit code 130 (`TestExitInterrupted`).
