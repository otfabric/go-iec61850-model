# go-iec61850-model

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
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

> **IEC content is not included.** The library ships no NSD files and no text
> from the standards. You load officially obtained NSD packages at run time.
> The tests run on small, independently authored fixtures. See
> [docs/licensing.md](docs/licensing.md).

---

## Table of contents

- [Status](#status)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Packages](#packages)
- [Normative model, device instance, semantic inference](#normative-model-device-instance-semantic-inference)
- [Namespaces, releases and profiles](#namespaces-releases-and-profiles)
- [Command-line tool](#command-line-tool)
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
| NSD importer (`nsd`) | Implemented; verified on synthetic fixtures only |
| Catalog, profiles, search, diff (`catalog`) | Implemented |
| Reference parsing (`reference`) | Implemented; compatible with `go-iec61850` `ParseRef` |
| Type resolution (`resolver`) | Implemented |
| Semantic mapping (`semantic`) | Implemented: MMXU, MMXN, MMTR rules, overrides, unit handling |
| Ontology helpers (`ontology`) | Implemented: concept graph, traversal, JSON export |
| `nsdctl` CLI, generator | Implemented: inspect, validate, dependencies, model, diff, generate (go, docs, json) with license gate, sources check |
| NSD update monitor | Planned (M5) |

**Standards coverage.** The importer targets the NSD format of IEC 61850-7-7.
The project tracks 14 code component packages: 7-2, 7-3, 7-4, 7-410 and
7-420 (required), 90-9 and 90-14 (recommended), 90-6, 90-3, 90-4, 90-11,
90-21 and 8-1 (optional), and the 7-7 schema; `nsdctl sources check
--packages DIR` shows which of them are present and loadable. No official NSD package has
been parsed and validated yet, so this module claims support for **no**
specific namespace release. [docs/nsd-sources.md](docs/nsd-sources.md) tracks
what is published, available, parsed and validated.

## Installation

```bash
go get github.com/otfabric/go-iec61850-model
```

Requires Go 1.24 or later. No CGO, no network access at run time. The library
packages use the standard library only; `cmd/nsdctl` adds
[cobra](https://github.com/spf13/cobra).

## Quick start

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
| [`catalog`](catalog) | Immutable multi-release index, explicit profiles, search, diff | `New`, `Catalog.Profile`, `LogicalNode`, `DataObjects`, `Search`, `Diff` |
| [`reference`](reference) | Instance and class references, LN name parts, MMS form | `Parse`, `ParseClass`, `ParseLNName`, `Normalize`, `FromMMS` |
| [`resolver`](resolver) | Path and type resolution with provenance | `New`, `Resolve`, `ResolveString`, `ResolvePath`, `Leaves` |
| [`semantic`](semantic) | Quantity, phase, reference, component and unit hints from explicit rules | `Describe`, `FindMeasurements`, `Match`, `NormalizeUnit`, `DeviceUnit`, `Mapper` |
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

A `catalog.Catalog` may hold several releases of the same namespace. It never
picks the "latest": a lookup of a name that two loaded namespaces define fails
with `model.ErrAmbiguousReference`. A `catalog.Profile` selects at most one
release per standard and adds dependencies when exactly one loaded release
satisfies them. See [docs/namespaces.md](docs/namespaces.md).

## Command-line tool

```bash
go install github.com/otfabric/go-iec61850-model/cmd/nsdctl@latest

nsdctl inspect ./nsd/IEC_61850-7-4.zip
nsdctl -s ./nsd validate
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model show MMXU
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model describe 'LD0/MMXU1.PNV.phsA.cVal.mag.f[MX]'
nsdctl diff --exit-code ./old/ ./new/
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" generate go -o ./generated --package iec61850names
nsdctl sources check --packages ./nsd    # coverage of the 14 tracked packages
nsdctl schema ./nsd/IEC_61850-7-7.2017.NSD.2017A.full.zip
```

Text or `--json` output; exit codes 0 ok, 1 usage, 2 source error,
3 validation, 4 differences, 5 refused by the license policy. Generated
output is local-only unless the lock file permits publishing it. See
[docs/cli.md](docs/cli.md) and [docs/generation.md](docs/generation.md).

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
| [docs/cli.md](docs/cli.md) | `nsdctl` commands, flags and exit codes |
| [docs/generation.md](docs/generation.md) | Generated outputs, determinism and the license gate |
| [docs/coverage.md](docs/coverage.md) | Published, available, parsed and validated coverage |
| [docs/nsd-sources.md](docs/nsd-sources.md) | Official sources, acquisition, provenance, coverage |
| [docs/licensing.md](docs/licensing.md) | IEC content and redistribution limits |
| [ERRORS.md](ERRORS.md) | Error taxonomy |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Workflow, checks and source policy |
| [REQUIREMENTS.md](REQUIREMENTS.md), [PLAN.md](PLAN.md), [PROGRESS.md](PROGRESS.md) | Scope, plan and progress |

## Development

```bash
make check   # format, tidy, vet, staticcheck, golangci-lint, govulncheck, race tests, coverage gate
make test    # unit tests
make fuzz    # fuzz the NSD parser and the reference parser
```

Run `make` for all targets.

## License

MIT — see [LICENSE](LICENSE). The license covers the code of this repository.
It grants nothing for IEC standards or NSD files you load with it; see
[docs/licensing.md](docs/licensing.md).
