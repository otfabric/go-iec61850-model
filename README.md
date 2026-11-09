# go-iec61850-model

[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/otfabric/go-iec61850-model.svg)](https://pkg.go.dev/github.com/otfabric/go-iec61850-model)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/otfabric/go-iec61850-model/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/go-iec61850-model/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/otfabric/go-iec61850-model/graph/badge.svg)](https://codecov.io/gh/otfabric/go-iec61850-model)

A pure-Go, transport-independent library for the **IEC 61850 information
model**: it imports the official machine-readable namespace definitions (NSD,
IEC 61850-7-7) and gives you a typed, edition-aware catalog of logical node
classes, data objects, CDCs, data attributes, enumerations, functional
constraints and presence conditions.

Use it to look up what a logical node such as `MMXU` holds, resolve
`LD0/MMXU1.Hz.mag.f[MX]` down to `FLOAT32`, validate references, compare
namespace releases, or explain the model in a UI. It contains no MMS, GOOSE,
Sampled Values or SCL runtime; those depend on this library, not the other way
round.

> **IEC content.** The repository ships no NSD files and no text from the
> standards. It commits what is generated from the 28 official NSD namespaces
> (identifiers, types and structure; no documentation texts), under the IEC
> Code Components license with the attribution in `NOTICE.md`: the embedded
> catalog [`official`](official), the constants [`names`](names) and the
> [model reference](docs/reference/index.md). You can also load officially
> obtained NSD packages at run time. See [docs/licensing.md](docs/licensing.md).

---

## Table of contents

- [Status](#status)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Packages](#packages)
- [Normative model, device instance, semantic inference](#normative-model-device-instance-semantic-inference)
- [Namespaces, releases and profiles](#namespaces-releases-and-profiles)
- [Install nsdctl](#install-nsdctl)
- [Command-line tool](#command-line-tool)
- [Official packages](#official-packages)
- [Errors](#errors)
- [Input safety](#input-safety)
- [Documentation](#documentation)
- [Development](#development)
- [License](#license)

---

## Status

Pre-release (milestones M0–M4 of [REQUIREMENTS.md](REQUIREMENTS.md); see
[PROGRESS.md](PROGRESS.md)).

| Area | State |
| --- | --- |
| NSD importer (`nsd`) | Implemented; imports all 33 tracked official packages, every NSD release IEC publishes (0 validation errors) |
| Catalog, profiles, search, diff (`catalog`) | Implemented |
| Reference parsing (`reference`) | Implemented; compatible with `go-iec61850` `ParseRef` |
| Type resolution (`resolver`) | Implemented |
| Semantic mapping (`semantic`) | Implemented: MMXU, MMXN, MMTR rules, overrides, unit handling |
| Ontology helpers (`ontology`) | Implemented: concept graph, traversal, JSON export |
| Embedded catalog (`official`) | Committed: every published NSD release (28 namespaces: 7-2, 7-3 and 7-4 from 2007A2 to 2007B6, 90-6 2018A and 2018B, one release of each other part) with the profiles `default`, `extended` and `edition2`, with IEC attribution; gzip-compressed, about 0.3 MB in a binary |
| Generated names (`names`) | Committed: typed constants for every class, data object, CDC and enumeration of the default profile, with IEC attribution |
| Model reference ([docs/reference](docs/reference/index.md)) | Committed: every class, CDC and type of the extended profile |
| `nsdctl` CLI, generator | Implemented: inspect, validate, dependencies, model, catalog (profiles, overlaps, select), diff, generate (go, docs, json, catalog) with license gate, sources fetch/check/lock, schema, version, shell completion; binaries for Linux, macOS and Windows |
| NSD update monitor | Planned (M5) |

**Standards coverage.** The importer targets the NSD format of IEC 61850-7-7
and is tested against the 33 tracked official packages, every NSD release IEC
publishes: 7-2, 7-3, 7-4 (2007B6), 7-410 (2024A4) and 7-420 (2019A4)
(required), 90-9 and 90-14 (recommended), 90-6, 90-3, 90-4, 90-11, 90-21 and
8-1 (optional), the earlier releases of 7-2, 7-3 and 7-4 (2007A2 to 2007B5,
Edition 2 and 2.1), 90-6 (2018A) and 8-1 (2003 to 2003A3), and the 7-7 schema.
They load and validate together; in the newest releases 93% of the concrete
data objects resolve to leaf types, the rest hit upstream gaps. IEC publishes
no NSD for Edition 1 (2003) of 7-4.
[docs/nsd-sources.md](docs/nsd-sources.md) tracks what is published,
available, parsed and validated.

## Installation

```bash
go get github.com/otfabric/go-iec61850-model
```

Requires Go 1.26 or later. No CGO, no network access at run time. The library
packages use the standard library only. `cmd/nsdctl`, a nested module, adds
[cobra](https://github.com/spf13/cobra); it is not a dependency of the library.

## Quick start

The embedded official catalog needs no files:

```go
p, err := official.Default() // IEC 61850-7-2, 7-3, 7-4, 7-410 and 7-420
if err != nil {
	log.Fatal(err)
}
dos, _ := p.DataObjects("MMXU") // what MMXU holds, inherited data objects included
res, _ := resolver.New(p).ResolveString("LD0/MMXU1.Hz.mag.f[MX]")
fmt.Println(len(dos), res.Type()) // FLOAT32
```

The `extended` profile adds the technical reports. A name that two of its
namespaces define is never merged; look it up with its namespace:

```go
x, _ := official.Extended()
_, err = x.LogicalNode("DBAT") // errors.Is(err, model.ErrAmbiguousSymbol): 7-420 and 90-9
dbat, _ := x.LogicalNodeIn(model.NamespaceID{Standard: "IEC 61850-7-420"}, "DBAT")
```

Or load officially obtained packages yourself:

```go
package main

import (
	"fmt"
	"log"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/reference"
	"github.com/otfabric/go-iec61850-model/resolver"
)

func main() {
	// Load officially obtained NSD packages (ZIP archives, directories or files).
	var sources []catalog.Source
	for _, path := range []string{"/data/nsd/7-2.zip", "/data/nsd/7-3.zip", "/data/nsd/7-4.zip"} {
		pkg, err := nsd.LoadZIP(path)
		if err != nil {
			log.Fatal(err)
		}
		sources = append(sources, pkg)
	}
	c, err := catalog.New(sources...)
	if err != nil {
		log.Fatal(err)
	}

	// Select releases explicitly; dependencies are added when unambiguous.
	id, _ := model.ParseNamespaceID("IEC 61850-7-4:2007B5")
	p, err := c.Profile(id)
	if err != nil {
		log.Fatal(err)
	}

	// What does MMXU hold, including inherited data objects?
	dos, _ := p.DataObjects("MMXU")
	for _, do := range dos {
		fmt.Printf("%-8s %-4s %s\n", do.Name, do.Type, do.Presence)
	}

	// Resolve a device reference down to its basic type.
	ref, _ := reference.Parse("LD0/MMXU1.Hz.mag.f[MX]")
	res, err := resolver.New(p).Resolve(ref)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.DataObjectPath(), res.AttributePath(), res.Type()) // Hz mag.f FLOAT32
}
```

Describe what a value means:

```go
d, _ := semantic.Describe(p, "MMXU.PNV.phsA.cVal.mag.f")
fmt.Println(d.Quantity, d.Phase, d.Reference, d.UnitHint) // voltage A line-neutral V
```

Runnable examples against the synthetic fixtures are in the `example_test.go`
files of [catalog](catalog/example_test.go), [resolver](resolver/example_test.go),
[semantic](semantic/example_test.go) and [ontology](ontology/example_test.go).

## Packages

| Package | Purpose | Main API |
| --- | --- | --- |
| [`model`](model) | Typed model: namespaces, LN classes, DOs, CDCs, DAs, enumerations, FCs, presence conditions, provenance, diagnostics, errors | `Namespace`, `NamespaceID`, `LogicalNodeClass`, `CommonDataClass`, `Diagnostic`, sentinels |
| [`nsd`](nsd) | Secure importer for NSD XML, NSDoc and ZIP packages | `Parse`, `LoadZIP`, `ReadZIP`, `LoadFile`, `LoadDir`, `ReadManifest`, `Validate`, `ApplyDoc` |
| [`catalog`](catalog) | Immutable multi-release index, explicit profiles, namespace-qualified lookups, overlap reports, search, diff | `New`, `Catalog.Profile`, `LogicalNode`, `LogicalNodeIn`, `DataObjects`, `Overlaps`, `Search`, `Diff` |
| [`reference`](reference) | Instance and class references, LN name parts, MMS form | `Parse`, `ParseClass`, `ParseLNName`, `Normalize`, `FromMMS` |
| [`resolver`](resolver) | Path and type resolution with provenance | `New`, `Resolve`, `ResolveString`, `ResolvePath`, `Leaves` |
| [`semantic`](semantic) | Quantity, phase, reference, component and unit hints from explicit rules | `Describe`, `FindMeasurements`, `Match`, `NormalizeUnit`, `DeviceUnit`, `Mapper` |
| [`official`](official) | Generated: the official namespaces, embedded, with the named profiles | `Default`, `Extended`, `Open`, `Catalog`, `Profiles` |
| [`names`](names) | Generated from the default profile: `LN_*`, `CDC_*`, `<Class>_<DataObject>` constants and one type per enumeration | `names.MMXU_Hz`, `names.BehaviourModeKind_on` |
| [`ontology`](ontology) | Typed concept graph with deterministic traversal and JSON export | `BuildGraph`, `Relations`, `Incoming`, `FindConcepts`, `Walk`, `ExportJSON` |

The import direction is `model` ← `nsd`, `catalog` ← `resolver` (which also
uses `reference`) ← `semantic` ← `ontology`. The module never imports `go-iec61850`, `go-mms` or any
transport stack; see [docs/architecture.md](docs/architecture.md).

## Normative model, device instance, semantic inference

The library keeps three things apart:

1. **Normative model** — what a namespace release defines: `MMXU` has an
   optional data object `Hz` of CDC `MV`. This module.
2. **Device instance** — what a particular IED implements: `LD0/MMXU1.Hz`
   exists on this device. SCL ([`go-iec61850/scl`](https://github.com/otfabric/go-iec61850))
   or discovery. A successful resolution here does not mean a device has the
   object.
3. **Semantic inference** — what a value means in engineering terms
   (frequency, expected in Hz). The `semantic` package, from explicit rules,
   always marked as OTFabric interpretation with rule, source and confidence,
   and never changing normative meaning. Its unit hints are not the units a
   device reports; `semantic.DeviceUnit` reads those.

## Namespaces, releases and profiles

A `catalog.Catalog` may hold several releases of the same namespace. A lookup
of a name that two loaded namespaces define fails with
`model.ErrAmbiguousSymbol` (`ErrAmbiguousReference`); the catalog never picks
one. A `catalog.Profile` selects at most one release per standard and adds,
for each dependency it does not select, the newest loaded release that
satisfies it (releases of one version and revision only add corrections).
`official.ForNamespace` names the embedded profile for the release a device
declares in `LLN0.NamPlt.ldNs`.

Namespace identity is part of a definition's identity. IEC TR 61850-90-9 and
IEC 61850-7-420 both define `DBAT`, `DSTO` and `SBAT`, differently; the default
profile therefore leaves 90-9 out, and the extended profile keeps both
definitions, reachable with `LogicalNodeIn` and compared by `Overlaps` (`nsdctl
catalog overlaps`). See [docs/namespaces.md](docs/namespaces.md).

## Install nsdctl

```bash
curl -fsSL https://get.otfabric.io/nsdctl | sh
```

```powershell
irm https://get.otfabric.io/nsdctl.ps1 | iex
```

Linux (amd64, arm64, armv7), macOS (Intel, Apple silicon) and Windows (amd64, arm64). The installer verifies the release's SHA-256 checksum, installs per user without `sudo` (`~/.local/bin`, Windows `%LOCALAPPDATA%\OTFabric\bin`), and upgrades in place when run again. Pin a version with `OTF_VERSION=x.y.z`; for CI, containers and device images use `OTF_UNMANAGED_INSTALL=<dir>`; all options: [otfabric/install](https://github.com/otfabric/install#options). `OTF_VERSION` is the version of go-iec61850-model, which ships `nsdctl`. For offline machines, the archives and checksums are on [Releases](https://github.com/otfabric/go-iec61850-model/releases).

### Shell completion

Commands and the flags with a value set complete (`--profile`, `--ns`, `--left`, `--right`, `--kind`, `--fc`, `--class`, `--tiers`), and so do NSD sources, lock and registry files and output directories.

```bash
# bash (needs the bash-completion package)
mkdir -p ~/.local/share/bash-completion/completions
nsdctl completion bash > ~/.local/share/bash-completion/completions/nsdctl

# zsh: a directory on your fpath, then restart the shell
mkdir -p ~/.zsh/completions && nsdctl completion zsh > ~/.zsh/completions/_nsdctl
# and once in ~/.zshrc:  fpath=(~/.zsh/completions $fpath); autoload -U compinit && compinit

# fish
mkdir -p ~/.config/fish/completions && nsdctl completion fish > ~/.config/fish/completions/nsdctl.fish
```

```powershell
# PowerShell: add this line to your profile ($PROFILE)
nsdctl completion powershell | Out-String | Invoke-Expression
```

## Command-line tool

`nsdctl` answers integration questions straight from the official IEC models
embedded in it — no download, no setup:

```console
$ nsdctl model describe 'LD0/MMXU1.PhV.phsA.cVal.mag.f[MX]'
MMXU.PhV.phsA.cVal.mag.f
quantity     voltage
phase        A
reference    line-ground
component    magnitude
unit hint    V
mapping      rule mmxu.phv, rule, high confidence (otfabric-default)
```

[docs/use-cases.md](docs/use-cases.md) walks through real tasks with real
output: what a power meter (`MMXU`) exposes and in which units, what a tag
from an IED means, decoding `SIUnit = 38` and `multiplier = 3` into kW, the
data objects of a battery system (`DBAT`, `DSTO`), a circuit breaker's
position, protection settings per setting group, devices on the older IEC TR
61850-90-9 storage model, and building a tag list with `--json`.

```bash
nsdctl sources fetch                         # official packages into .nsd-cache/, digests verified
nsdctl sources check --packages .nsd-cache   # coverage of the 14 tracked packages
nsdctl validate .nsd-cache
nsdctl -s .nsd-cache model show MMXU
nsdctl -s .nsd-cache model describe 'LD0/MMXU1.PNV.phsA.cVal.mag.f[MX]'
nsdctl model show DBAT                                 # embedded catalog, default profile
nsdctl -p extended model show DBAT --ns 90-9            # the 90-9 definition
nsdctl catalog overlaps --left IEC_61850-7-420 --right IEC_TR_61850-90-9
nsdctl diff --exit-code ./old/ ./new/
nsdctl -s .nsd-cache generate go -o .generated/go --package iec61850names
nsdctl schema .nsd-cache/IEC_61850-7-7.2017.NSD.2017A.full.zip
nsdctl version
```

Without `--source`, the `model` and `catalog` commands use the embedded
official catalog. Text or `--json` output; exit codes follow the organisation
table: 0 ok, 1 findings (differences, validation errors), 2 usage, 3 invalid
input, 4 download failed, 9 refused by the license policy, 10 output or internal
error, 130 interrupted ([cmd/nsdctl/docs/exitcodes.md](cmd/nsdctl/docs/exitcodes.md)).
Generated output is local-only unless the lock file permits publishing it.
[cmd/nsdctl/docs/cli.md](cmd/nsdctl/docs/cli.md) walks through every command with real output from
the official packages; [docs/generation.md](docs/generation.md) covers the
generator.

## Errors

Every package returns errors that wrap the sentinels of `model`
(`ErrInvalidNSD`, `ErrUnknownLogicalNode`, `ErrAmbiguousReference`, ...), with
typed errors (`*ResolutionError`, `*AmbiguityError`, `*DependencyError`,
`*SourceError`, `*ValidationError`, `*ReferenceError`) for detail. See
[ERRORS.md](ERRORS.md).

## Input safety

NSD input is treated as untrusted: documents are bounded in size, depth and
token count, DTDs are rejected, archives are checked for traversal, duplicate
and non-regular entries, entry count, total size and compression ratio. Nothing
is written to disk. See [SECURITY.md](SECURITY.md).

## Documentation

| Document | Content |
| --- | --- |
| [docs/architecture.md](docs/architecture.md) | Package boundaries and data flow |
| [docs/model.md](docs/model.md) | LN, DO, CDC, DA, FC, enumerations, presence conditions |
| [docs/namespaces.md](docs/namespaces.md) | Namespace IDs, releases, dependencies and profiles |
| [docs/reference.md](docs/reference.md) | Reference syntax, parsing, resolution |
| [docs/semantic.md](docs/semantic.md) | Quantities, phases, units, rules and overrides |
| [docs/ontology.md](docs/ontology.md) | Concept graph, relations, export |
| [docs/use-cases.md](docs/use-cases.md) | Real tasks solved with `nsdctl`, with real output |
| [cmd/nsdctl/docs/cli.md](cmd/nsdctl/docs/cli.md) | `nsdctl` commands and flags; [exit codes](cmd/nsdctl/docs/exitcodes.md) |
| [docs/generation.md](docs/generation.md) | Generated outputs, determinism and the license gate |
| [docs/coverage.md](docs/coverage.md) | Published, available, parsed and validated coverage |
| [docs/nsd-sources.md](docs/nsd-sources.md) | Official sources, acquisition, provenance, coverage |
| [docs/licensing.md](docs/licensing.md) | IEC content and redistribution limits |
| [docs/reference](docs/reference/index.md) | Generated model reference of the official namespaces |
| [ERRORS.md](ERRORS.md) | Error taxonomy |
| [LIMITATIONS.md](LIMITATIONS.md) | Scope not implemented, upstream gaps, what has not been verified |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Workflow, checks and source policy |
| [REQUIREMENTS.md](REQUIREMENTS.md), [PLAN.md](PLAN.md), [PROGRESS.md](PROGRESS.md) | Scope, plan and progress |

## Official packages

The generated output is the core of the library and is committed, under the
IEC Code Components license with the attribution in each `NOTICE.md` (see
[docs/licensing.md](docs/licensing.md#decision)):

| Directory | Content | Profile |
| --- | --- | --- |
| [`official/`](official) | Embedded catalog of every release, profiles `default`, `extended` and `edition2` | every release |
| [`names/`](names) | Typed constants for classes, data objects, CDCs and enumerations | default |
| [`docs/reference/`](docs/reference/index.md) | Model reference | extended |

```go
import "github.com/otfabric/go-iec61850-model/names"

ref := string(names.MMXU_PhV) + ".phsA.cVal.mag.f" // "MMXU.PhV.phsA.cVal.mag.f"
mode := names.BehaviourModeKind(1)                 // mode.String() == "on"
```

The official packages themselves are temporary inputs, downloaded from `assets.iec.ch`,
verified against the digests pinned in `sources/nsd.lock.json`, and never
committed:

```bash
make nsd-fetch nsd-check official   # download into .nsd-cache/, validate, integration test
make nsd-generate                   # Go, docs and JSON into .generated/ (git-ignored)
make nsd-publish                    # regenerate the committed names/, official/ and docs/reference/
make generate-check                 # fetch, then fail if the committed output is stale (make generate regenerates it)
```

The [IEC NSD packages](.github/workflows/nsd.yml) workflow runs the same steps
in CI without caches or artifacts.

## Development

```bash
make check   # format, tidy, vet, staticcheck, golangci-lint, govulncheck, race tests, coverage gate
make test    # unit tests
make fuzz    # fuzz the NSD parser and the reference parser
make build   # bin/nsdctl (a nested module: cmd/nsdctl)
```

Run `make` for all targets.

## License

MIT — see [LICENSE](LICENSE). The license covers the code of this repository.
The generated output in [`names/`](names), [`official/`](official) and
[`docs/reference/`](docs/reference/index.md) is derived from IEC code
components and published under the IEC Code Components license, with the
attribution and notices in the `NOTICE.md` of each directory. Nothing here grants rights to
the IEC standards or to NSD files you load; see
[docs/licensing.md](docs/licensing.md).
