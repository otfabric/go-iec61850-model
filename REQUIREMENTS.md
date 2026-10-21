# OTFabric — IEC 61850 Model Library

**Repository proposal:** `github.com/otfabric/go-iec61850-model`  
**Artifact:** Development requirements and implementation plan  
**Status:** Proposed  
**Plan date:** 2026-10-21

## 1. Executive summary

Build a standalone, pure-Go, transport-independent library that imports the official IEC 61850 Namespace Definition (NSD) models and exposes a typed, edition-aware model catalog for reuse throughout OTFabric.

The initial library must support:

1. **NSD Importer** — read and validate official machine-readable namespace packages.
2. **Model Catalog** — search Logical Node (LN) classes, Data Objects (DOs), Common Data Classes (CDCs), attributes, enumerations and namespace metadata.
3. **Type Resolver** — resolve DO → CDC → DA → constructed/primitive type, including nested definitions and inheritance.
4. **Reference Validator** — parse and validate class-level and instance-level IEC 61850 references.
5. **Semantic Mapping** — describe engineering measurements (quantity, phase, reference, units) while distinguishing inferred from device-specific semantics.
6. **Ontology Helpers** — expose concepts and relationships in a form reusable by other libraries.

**Scope:** Information models and metadata, not MMS, GOOSE, Sampled Values, transport protocols, an SCL runtime, or a device simulator. Those systems may depend on this library, not vice versa.

### Required standards coverage

- **IEC 61850-7-2:** Abstract Communication Service Interface (ACSI), supporting types and service-related definitions as applicable to the NSD model.
- **IEC 61850-7-3:** Common Data Classes (CDCs) and data-attribute definitions.
- **IEC 61850-7-4:** General Logical Node classes and Data Objects.
- **IEC 61850-7-410:** Additional generation / hydropower-related Logical Node models.
- **IEC 61850-7-420:** Distributed energy resources and distribution-automation models, including inverter-related models.
- **IEC 61850-7-7:** The NSD machine-readable format and XSD used for importer validation.

Prefer the **latest compatible official, published and obtainable NSD packages** over the older PDF editions already purchased. Do not assume that the publication date of an IEC specification and the release date of its NSD package are identical.

### Editions to investigate and track

| IEC part | Locally purchased edition | Preferred publication baseline (verify at implementation time) | Role |
| --- | --- | --- | --- |
| 7-2 | 2010 | Ed. 2.1 (2010 + AMD1:2020) | Abstract model definitions |
| 7-3 | 2010 | Ed. 2.1 (2010 + AMD1:2020) | CDCs and attributes |
| 7-4 | 2010 | Ed. 2.1 (2010 + AMD1:2020) | General LNs |
| 7-410 | 2012 | 2026 Ed. 3.0, **if corresponding NSD is available** | Hydropower/generation extensions |
| 7-420 | 2009 | 2021 Ed. 2.0 | DER extensions |
| 7-7 | Not purchased | Current official NSD schema, including amendments/corrigenda as applicable | XML/XSD schema |

The project must separately report **published standard**, **available NSD**, **successfully parsed NSD**, and **validated compatible profile**. In particular, do not claim 7-410:2026 namespace support until an official corresponding NSD is available and tested.

## 2. Source-of-truth and licensing policy

### 2.1 Allowed upstream sources

**Only official IEC-published artifacts may serve as normative generation input:**

- IEC NSD Light archives (preferred where sufficient).
- IEC NSD Full archives (optional, subject to explicit access and license conditions).
- Official IEC manifests and package-level metadata (`IECManifest.xml` where applicable).
- Official IEC 61850-7-7 schemas (XSD).
- Official IEC notices and licensing information attached to those Code Components.

Purchased PDF standards may be consulted manually to understand modeling intent, but **must not form a dependency of the generator, be committed, or be republished**. Third-party mirrors, GitHub dumps, vendor mappings, and unofficial NSD copies are not authoritative normative inputs.

Useful official starting points:

- [IEC 61850 Code Components](https://iec61850.dvl.iec.ch/61850-resources/61850-code-components/)
- [IEC webstore](https://webstore.iec.ch/)

The implementer must verify current download URLs, package formats, version metadata, access requirements, and applicable EULA before automating retrieval. An official download may require credentials or be subject to restrictions even when discoverable publicly.

### 2.2 Intermediate-artifact rule

Original upstream archives, XML, XSD, extracted PDF text, intermediate normalized data, and temporary downloads **must never be committed to Git**.

Proposed local workflow:

```text
Official IEC packages (download or user supplied)
    ↓ ephemeral work directory
Archive verification + manifest + schema validation
    ↓
Normalized in-memory / ephemeral intermediate representation
    ↓
Generated Go sources, indexes, API docs, coverage, reports
    ↓
License-policy gate → permitted distributable output only
```

Use `.gitignore`, explicit CI artifact exclusions, scratch-directory cleanup, and log redaction. Never publish upstream content through CI caches, Actions artifacts, or generated release archives.

**Important legal constraint:** generating code or documentation from NSD files may create copyright-protected derivative material. Merely deleting the source XML does not automatically grant redistribution rights. Therefore:

- The **public MIT-licensed core** consists of OTFabric-authored API contracts, importer, resolver, mapping logic, tests, and tools.
- Full IEC-derived catalogs, verbatim identifiers, descriptions, and generated documentation are **local-only by default**, pending review of the applicable IEC Code Component license and attribution requirements.
- Any checked-in/distributed generated artifacts require a separate documented licensing decision, required notices, and provenance.
- Generated documentation must not reconstruct extensive copyright-protected standard text unless redistribution is authorized.
- Include `docs/licensing.md` that explains these limitations to consumers.

The main Go module must compile and pass its public tests without downloading IEC files.

### 2.3 Reproducibility and provenance

Record a commit-safe metadata lock file containing only permitted source metadata:

- Standard part and published edition.
- IEC namespace ID, version, revision and release.
- Official source location, retrieval timestamp, and digest (SHA-256).
- Input package kind (Light/Full) and schema version.
- Required namespace dependencies.
- Validation status and notes.

**Version lock example** (illustrative data; confirm against manifest before pinning):

```json
{
  "sources": [
    {
      "standard": "IEC 61850-7-420",
      "publication": "2021",
      "namespace": "IEC 61850-7-420",
      "version": "2019",
      "revision": "A",
      "release": 4,
      "content": "light",
      "dependencies": ["IEC 61850-7-4:2007B"]
    }
  ]
}
```

Build output must be deterministic for a given input lock and tool version. Generated reports must identify their source namespaces precisely.

## 3. Library architecture and boundaries

```text
                     Official IEC NSD packages
                               |
                        nsd (Importer)
                               |
                       model (Typed IR)
                               |
                     catalog (Versioned index)
                               |
              +----------------+----------------+
              |                |                |
          resolver          reference         semantic
              |                |                |
              +----------------+----------------+
                               |
                            ontology
                               |
                Other OTFabric Go libraries
         go-iec61850 / go-modbus / go-dnp3 /
         go-opcua / go-sunspec / simulators
```

The architecture has two intentionally distinct layers:

1. **Normative modeling:** exact IEC definitions, compatible namespace resolution, provenance, precise references, and data-type relationships.
2. **Interpretive semantics:** OTFabric-authored measurement labels, quantity/phase/reference annotations, controlled inference, and optional mappings to a future neutral ontology.

The semantic layer must never silently alter normative meaning or imply a real device implements optional Data Objects.

### Existing OTFabric integration

The existing [`otfabric/go-iec61850`](https://github.com/otfabric/go-iec61850) repository already has runtime reference parsing (`ParseRef`) and SCL parsing/generation (`sclgen`); it currently targets Go 1.24. Avoid introducing incompatible reference formats.

**Dependency direction:**

- `go-iec61850-model` **must not import** `go-iec61850`, `go-mms`, or any transport stack.
- `go-iec61850` **may import** `go-iec61850-model` through a thin adapter.
- The SCL package remains responsible for *actual device-instance configurations*, while `go-iec61850-model` owns the **normative class/type catalog**.
- Protocol-neutral ontology contracts may eventually move to a separate `go-ot-ontology` package. Keep the present IEC-specific `ontology/` package lightweight, or implement it only as an adapter to the future package.

## 4. Proposed repository layout

```text
go-iec61850-model/
├── go.mod
├── README.md
├── LICENSE
├── model/
│   ├── namespace.go
│   ├── logical_node.go
│   ├── data_object.go
│   ├── cdc.go
│   ├── attribute.go
│   ├── enumeration.go
│   ├── provenance.go
│   └── errors.go
├── nsd/
│   ├── parser.go
│   ├── manifest.go
│   ├── schema.go
│   ├── dependencies.go
│   └── validate.go
├── catalog/
│   ├── catalog.go
│   ├── index.go
│   ├── filter.go
│   ├── profile.go
│   └── diff.go
├── resolver/
│   ├── resolver.go
│   ├── inheritance.go
│   └── path.go
├── reference/
│   ├── parser.go
│   ├── validator.go
│   └── errors.go
├── semantic/
│   ├── quantity.go
│   ├── measurement.go
│   ├── electrical.go
│   ├── units.go
│   └── mapping.go
├── ontology/
│   ├── concept.go
│   ├── relation.go
│   ├── graph.go
│   └── export.go
├── cmd/nsdctl/
├── internal/generator/
├── sources/
│   ├── index.yaml          # official URLs and discovery metadata only
│   └── nsd.lock.json       # permitted provenance metadata only
├── examples/
├── testdata/               # synthetic fixtures only
├── docs/
└── .github/workflows/
    ├── ci.yml
    └── nsd-monitor.yml
```

Use a **single module** with separate packages. Do not introduce submodules for every IEC part.

## 5. Go implementation requirements

| Concern | Requirement |
| --- | --- |
| Go version | Go **1.24+** initially, aligned with `go-iec61850` |
| Portability | Pure Go, no CGO, supports Linux/macOS/Windows |
| Dependencies | Standard library for core functionality where practical |
| Network | No runtime network access required for catalog lookups |
| Concurrency | Immutable published catalogs; safe concurrent reads |
| Errors | Sentinel and structured errors, `errors.Is` / `errors.As` compatible |
| API style | Typed Go API, no pervasive `map[string]any` data model |
| Input safety | Streaming/size-bounded XML, secure ZIP handling, no path traversal |
| Serialization | JSON output first; YAML through optional adapters |
| Compatibility | Semantic versioning for Go APIs; namespace releases tracked separately |
| Determinism | Stable ordering and deterministic generation |
| Observability | Structured diagnostics with source namespace and element path |

### 5.1 Core model types (API sketch)

```go
package model

type NamespaceID struct {
    Standard string // e.g. IEC 61850-7-4
    Version  string
    Revision string
    Release  int
}

type Namespace struct {
    ID           NamespaceID
    Publication  string
    Dependencies []NamespaceID
    Nodes        []LogicalNodeClass
    CDCs         []CommonDataClass
    Enums        []Enumeration
}

type LogicalNodeClass struct {
    Name        string
    Group       string
    Description string
    DataObjects []DataObject
    Source      NamespaceID
}

type DataObject struct {
    Name        string
    CDC         string
    Presence    PresenceCondition
    Description string
}

type CommonDataClass struct {
    Name       string
    Attributes []DataAttribute
}

type DataAttribute struct {
    Name     string
    Type     string
    FC       FunctionalConstraint
    Presence PresenceCondition
    IsArray  bool
}
```

This is a **proposed normalized public contract**, not a claim that official NSD XML has these exact element names. The importer must handle the real XML schema and preserve all necessary semantics: nested and constructed attributes, enumerations, inheritance, presence conditions, CDC structure, origin, and dependencies.

### 5.2 Namespace profiles and resolver behavior

A single catalog may hold **multiple releases** of the same namespace. No global “latest wins” behavior is allowed.

Required capabilities:

- Explicit namespace selection and compatible-profile construction.
- Dependency resolution from source manifests, with clear errors for missing/incompatible dependencies.
- Separate class-level and instance-level references.
- No ambiguous lookup resolution without an explicit profile.
- Recursive type resolution from LN/DO to CDC, attribute, and primitive type.
- Retain provenance in every resolved element.
- Detect cycles, duplicate/conflicting definitions, and unsupported references.
- Edition-to-edition structural and semantic diffs.

**Default profile policy:** latest *compatible, officially released, available and validated* namespace set — not the highest publication year for every part independently.

### 5.2.1 IEC 61850-90-9 transitional namespace policy

IEC TR 61850-90-9 defines transitional electrical energy storage models, several of which were later incorporated into IEC 61850-7-420. Both namespaces define `DBAT`, `DSTO` and `SBAT`, and the definitions differ (`nsdctl catalog overlaps --left IEC_61850-7-420 --right IEC_TR_61850-90-9`).

**Decision:** drop 90-9 from the default profile, not from the library. Modern DER modeling stays straightforward, and historical devices can still be interpreted and older model definitions compared.

| Scenario | Behavior |
| --- | --- |
| Default model catalog | 7-2, 7-3, 7-4, 7-410 and 7-420 (profile `default`) |
| Historical storage models | Load 90-9 explicitly (profile `extended`) |
| Both 90-9 and 7-420 loaded | Both definitions are kept, each with its namespace |
| Lookup `DBAT` without a namespace | Ambiguity error when both define it |
| Lookup with an explicit namespace | The requested definition |
| Automatic merging of duplicates | Never |
| Source for new DER applications | 7-420 |

**Requirements:**

1. Exclude `IEC_TR_61850-90-9.NSD.2018A3` from the default catalog profile.
2. Include `IEC_61850-7-420.NSD.2019A4` as the preferred source for current DER and storage models.
3. Retain support for IEC 61850-90-9 through an optional historical/extended profile.
4. Preserve source namespace, version, revision, release and dependency information for every definition.
5. Do not silently overwrite, merge or deduplicate logical nodes with identical names from different namespaces.
6. Unqualified lookups with multiple matching definitions return `ErrAmbiguousSymbol` (an alias of `ErrAmbiguousReference`, detailed by `*AmbiguityError`).
7. Explicit namespace-qualified lookups remain available (`LogicalNodeIn(ns, name)` and the other `…In` lookups, `Resolver.In`, `nsdctl model --ns`).
8. Generate structural overlap reports, including differences in data objects, CDCs, presence conditions and inheritance (`Overlaps`, `nsdctl catalog overlaps`).
9. Validate namespace dependencies and compatibility when composing catalogs.
10. Include regression tests for overlapping definitions such as `DBAT`, `DSTO` and `SBAT`.

The profiles are declared in `sources/index.json`:

```json
"profiles": [
  { "name": "default", "parts": ["7-2", "7-3", "7-4", "7-410", "7-420"] },
  { "name": "extended", "inherits": "default", "parts": ["90-3", "90-4", "90-6", "90-9", "90-11", "90-14", "90-21"] }
]
```

Resolution rules: ambiguous names are an error, there is no implicit precedence between namespaces, and every definition keeps its namespace. References inside a definition resolve in its own namespace first, then in its dependencies.

**Design principle:** namespace identity is part of a model definition's identity. Identical logical node names do not necessarily imply identical or interchangeable definitions.

### 5.3 Functional packages and helper API

| Package | Proposed functions |
| --- | --- |
| `nsd` | `Parse`, `LoadZIP`, `Validate`, `ReadManifest` |
| `catalog` | `New`, `LogicalNode`, `DataObject`, `CDC`, `ListLogicalNodes`, `Search`, `Diff` |
| `resolver` | `Resolve`, `ResolveAttribute`, `ResolveCDC`, `ResolveInheritance` |
| `reference` | `Parse`, `Validate`, `Normalize`, `Format` |
| `semantic` | `Describe`, `FindMeasurements`, `Match`, `NormalizeUnit` |
| `ontology` | `BuildGraph`, `Relations`, `FindConcepts`, `ExportJSON` |

Functions should return `(T, error)` for failures rather than panic. Sorting and search results should be stable. Provide context-aware variants where a local operation is potentially costly; do not require `context.Context` for trivial in-memory lookups.

### 5.4 Typed errors

```go
var (
    ErrInvalidNSD         = errors.New("invalid NSD")
    ErrUnsupportedSchema  = errors.New("unsupported NSD schema")
    ErrMissingDependency  = errors.New("missing namespace dependency")
    ErrNamespaceConflict  = errors.New("namespace conflict")
    ErrUnknownLogicalNode = errors.New("unknown logical node")
    ErrUnknownDataObject  = errors.New("unknown data object")
    ErrUnknownAttribute   = errors.New("unknown data attribute")
    ErrInvalidReference   = errors.New("invalid object reference")
    ErrAmbiguousReference = errors.New("ambiguous reference")
    ErrSourceUnavailable  = errors.New("source unavailable")
)
```

Introduce typed details (e.g., `*ResolutionError`, `*ValidationError`, `*SourceError`) with the failing namespace, object path, expected type, actual type, and diagnostic context. Unknown and missing objects must **not** degrade silently to anonymous generic types.

## 6. Six component specifications

### 6.1 NSD Importer

- Parse official XML and ZIP packages, manifests, namespace declarations, schema versions, dependencies and namespaces' definitions.
- Support both NSD Light and Full where legally and technically available; avoid assuming Full is required.
- Validate input against official 7-7 schemas; report unsupported constructs rather than dropping data.
- Handle input bounds, prohibited XML constructs, nested XML depth, malicious archives, duplicate entries and decompression bombs.
- Report all diagnostics with XML path, namespace and field information.
- Operate from local files or provided readers; fetching is an optional developer-tool concern.

### 6.2 Model Catalog

- Index LNs, DOs, CDCs, DAs, enumerations, FCs, presence conditions and namespace metadata.
- Support exact lookup, filtering by standards part/domain/name/CDC, and deterministic listing.
- Expose immutable catalogs and explicit profiles.
- Compare namespace releases, reporting additions, removals, type/CDC/presence changes and dependency changes.

### 6.3 Type Resolver

- Resolve chains like `MMXU → Hz → MV → mag → FLOAT32` and nested structures such as `PNV → phsA → cVal → mag → f`.
- Support inherited classes, constructed attributes, enumeration resolution, functional constraints and presence conditions.
- Preserve provenance and fail on ambiguous or cyclical resolution.

### 6.4 Reference Validator

- Parse class references such as `MMXU.Hz`.
- Parse instance references such as `LD0/MMXU1.Hz.mag.f[MX]`, while remaining compatible with existing OTFabric object-reference semantics.
- Distinguish **syntax validity**, **validity against a normative model**, and **actual presence on a device**. The last requires device discovery or SCL data and is not guaranteed by the catalog alone.
- Normalize references without discarding information or inventing data.

### 6.5 Semantic Mapping

- Typed engineering-quantity labels (frequency, voltage, current, active/reactive/apparent power, power factor, energy, etc.).
- Typed phase/reference concepts (A/B/C, AB/BC/CA, total, line-neutral, line-ground, line-line).
- Conservative unit interpretation and scale metadata. Separate **unit hints** from an actual configured device's `units` and scale-factor values.
- Explicit, auditable mapping rules; optional user/vendor overrides with source and confidence/ambiguity markers.
- Avoid inference when names are overloaded or semantics are not guaranteed by normative definitions.

Illustrative semantic relationships:

| IEC 61850 object | Meaning |
| --- | --- |
| `MMXU.Hz` | Electrical frequency |
| `MMXU.PNV.phsA` | Phase A-to-neutral voltage |
| `MMXU.PhV.phsA` | Phase A-to-ground voltage |
| `MMXU.PPV.phsAB` | Phase A-to-B voltage |
| `MMXU.A.phsA` | Phase A current |
| `MMXU.TotW` | Total active power |

These are *standard-defined model concepts*, not guarantees that any given IED exposes all of them. Particular vendors may expose alternative objects or model their physical connections differently.

### 6.6 Ontology Helpers

- Provide a small typed concept graph: `LogicalNode contains DataObject`, `DataObject uses CDC`, `CDC contains Attribute`, `Measurement represents Quantity`.
- Support JSON export and deterministic graph traversal.
- Do **not** introduce heavy RDF/OWL/CIM dependencies in the initial release.
- Preserve a future adapter boundary to an independent `go-ot-ontology` module shared across protocols.

## 7. Code generation and distribution

The generator consumes a normalized, validated model graph and may produce:

1. Strongly typed Go identifiers and optional type-safe declarations.
2. Efficient in-memory indexes/catalog representations.
3. JSON catalogs or exports for local consumers.
4. Documentation indexes and coverage reports.
5. Change reports between namespace versions.

**Do not** generate a large standalone Go struct for each possible IED/device instance. Generate class/type metadata and typed references, leaving concrete device configurations to SCL and discovery.

Generation requirements:

- Fully deterministic files and formatting (`gofmt`).
- Generated header with source namespace IDs, digests and generator version.
- Stable naming, namespace collisions resolved explicitly.
- A check/diff mode for CI.
- Local-only by default for IEC-derived bulk catalogs until redistribution rights are established.
- Generator must fail the release-policy gate if it would publish impermissible derived content.

## 8. Example Go APIs (proposed, not implemented)

The following examples specify the intended ergonomics. Exact constructors may evolve during implementation.

### 8.1 Load locally acquired packages

```go
package main

import (
    "fmt"
    "log"

    "github.com/otfabric/go-iec61850-model/catalog"
    "github.com/otfabric/go-iec61850-model/nsd"
)

func main() {
    paths := []string{
        "/tmp/nsd/7-2.zip",
        "/tmp/nsd/7-3.zip",
        "/tmp/nsd/7-4.zip",
        "/tmp/nsd/7-410.zip",
        "/tmp/nsd/7-420.zip",
    }

    var packages []*nsd.Package
    for _, path := range paths {
        pkg, err := nsd.LoadZIP(path)
        if err != nil {
            log.Fatal(err)
        }
        packages = append(packages, pkg)
    }

    c, err := catalog.New(packages...)
    if err != nil {
        log.Fatal(err)
    }
    for _, ln := range c.ListLogicalNodes() {
        fmt.Println(ln.Name)
    }
}
```

### 8.2 Inspect Logical Node data objects

```go
ln, err := c.LogicalNode("MMXU")
if err != nil {
    return err
}
for _, object := range ln.DataObjects {
    fmt.Printf("DO=%s CDC=%s\n", object.Name, object.CDC)
}
```

This shortcut requires a selected namespace profile. If the catalog has multiple possible `MMXU` definitions, it must return `ErrAmbiguousReference` instead of guessing.

### 8.3 Resolve an IEC data attribute

```go
ref, err := reference.Parse("LD0/MMXU1.Hz.mag.f[MX]")
if err != nil {
    return err
}
resolved, err := c.Resolve(ref)
if err != nil {
    return err
}
fmt.Println(resolved.DataObject) // Hz
fmt.Println(resolved.Attribute)  // mag.f
fmt.Println(resolved.Type)       // FLOAT32
```

### 8.4 Semantic description

```go
descriptor, err := semantic.Describe(c, "MMXU.Hz")
if err != nil {
    return err
}
fmt.Println(descriptor.Quantity) // frequency
fmt.Println(descriptor.UnitHint) // Hz
```

### 8.5 Compare namespace releases

```go
changes := catalog.Diff(oldCatalog, newCatalog)
for _, change := range changes {
    fmt.Printf("%s %s %s\n", change.Kind, change.Path, change.Description)
}
```

## 9. Developer CLI — `nsdctl`

Provide a companion command-line utility for inspection, generation and maintenance:

```bash
nsdctl inspect ./source.zip
nsdctl validate ./source.zip
nsdctl dependencies ./source.zip
nsdctl model show MMXU
nsdctl model list --kind ln
nsdctl model search --cdc MV
nsdctl diff ./previous/ ./current/
nsdctl generate go --out ./generated/
nsdctl generate docs --out ./generated-docs/
nsdctl sources check
```

The CLI must provide readable text output and machine-readable JSON output for integration pipelines; errors should produce nonzero exit codes. Generation should have an explicit output directory, overwrite safety and a `--check` mode.

## 10. GitHub Actions / regular NSD updates

Create **two independent workflows**.

### 10.1 Standard `ci.yml`

Run on pushes and pull requests:

- `go test ./...`, race tests where appropriate, `go vet`, formatting, static analysis/lint.
- Synthetic-fixture schema/parser and resolver tests.
- Deterministic generator checks using synthetic input.
- Dependency/license/security scanning where practical.
- Zero reliance on downloads of official IEC copyrighted packages.

### 10.2 `nsd-monitor.yml`

Run **weekly** and on `workflow_dispatch`:

1. Query official IEC code-component publication metadata or verified official package discovery endpoints.
2. Compare available versions to `sources/nsd.lock.json`.
3. Distinguish newly published standards from newly available NSD packages.
4. Inspect manifests and digests only when access and EULA permit.
5. Check declared namespace dependencies and compatible profiles.
6. Create/update a GitHub issue describing detected changes, avoiding duplicate issues.
7. Optionally trigger a **separate controlled validation** step using legally available, ephemeral packages.
8. Require review before changing pinned profiles, publishing generated catalogs or releasing a library version.

**Expected discovery caveat:** An automatable official index for every Code Component may not be publicly available. Implement the discovery interface with an official-URL registry/fallback; never silently switch to unofficial mirrors or scrape blocked content.

Proposed workflow:

```yaml
name: IEC NSD Update Monitor

on:
  schedule:
    - cron: "17 5 * * 1"
  workflow_dispatch:

permissions:
  contents: read
  issues: write

jobs:
  check-nsd:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Check official IEC sources
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          go run ./cmd/nsdctl sources check \
            --config sources/index.yaml \
            --lock sources/nsd.lock.json \
            --create-issue
```

The workflow uses **proposed CLI functionality** that must be implemented before activation. Pin Actions by immutable SHA in the final production workflow, according to the organization's supply-chain policy.

**Monitor states:**

- `publication-announced`
- `nsd-available`
- `nsd-validated`
- `adoption-blocked` (incompatible dependencies, inaccessible package, or licensing issue)
- `adopted`

The monitor should not create noise every week when a known update has already been reported.

## 11. Testing and quality gates

| Test type | Acceptance coverage |
| --- | --- |
| XML / XSD | Valid XML; optional/unknown fields; XSD versions; malformed input |
| ZIP safety | Traversal, duplicate paths, bombs, oversized input |
| Namespace model | IDs, dependencies, presence, enums, constructed types |
| Catalog | Deterministic indexes, duplicates, thread-safe reads |
| Resolution | Nested paths, inheritance, missing/ambiguous dependencies |
| Reference | Syntax, model validation, instance-vs-class distinctions |
| Semantic | Frequency, voltage, current, power, phases, unit ambiguity |
| Diff | Add/remove/change nodes, DOs, CDCs, attributes, profiles |
| Determinism | Byte-identical output for identical locked inputs |
| Integration | Official packages in a controlled optional integration environment |
| Packaging | Public CI never needs or publishes official IEC source files |

Test fixtures in Git must be independently authored and small. Real official NSD integration tests can run locally or in a controlled, licensed execution environment but their inputs must not leak into Git history, workflow cache, logs or artifacts.

## 12. Documentation deliverables

| File | Required content |
| --- | --- |
| `README.md` | Purpose, supported standards, installation, quick start, scope and license caveat |
| `docs/architecture.md` | Module boundaries and data flow |
| `docs/nsd-sources.md` | Official sources, acquisition, validation and provenance |
| `docs/model.md` | LN, DO, CDC, DA, FC, enumerations and presence conditions |
| `docs/namespaces.md` | Versioning, revisions, releases and dependency profiles |
| `docs/reference.md` | Reference syntax, parse/validate/resolve examples |
| `docs/semantic.md` | Quantity, phase, unit interpretation and override rules |
| `docs/ontology.md` | Graph concepts, relationship API, export |
| `docs/generation.md` | Generated outputs, determinism and distribution gate |
| `docs/coverage.md` | Generated coverage: supported / parsed / resolved / validated |
| `docs/licensing.md` | IEC source and derivative-content restrictions; notices |
| `docs/integration.md` | Integration with `go-iec61850`, Modbus, DNP3, OPC UA, SCL |
| `docs/contributing.md` | Contributor workflow, tests and source policies |

Documentation must be readable for both library users and tool builders. API docs should include runnable examples and clear distinctions among **normative model**, **device instance**, and **semantic inference**.

## 13. Development milestones

### M0 — Source discovery and license gate

- Verify official NSD availability for 7-2, 7-3, 7-4, 7-410 and 7-420.
- Identify current 7-7 schema(s), package manifest format, dependencies and release versioning.
- Explicitly check whether a matching NSD exists for 7-410:2026.
- Review IEC Code Component license; document what generated code or catalogs may be redistributed.
- Define `sources/index.yaml` and `sources/nsd.lock.json`.

**Exit:** Verified sources registry + licensing policy + metadata lock design.

### M1 — Importer and foundation types

- Implement XML/ZIP parsing, manifest support, schema checks and source validation.
- Build typed IR covering LNs, DOs, CDCs, DAs, enums, presence and provenance.
- Introduce secure input bounds and structured errors.

**Exit:** All obtainable target official NSDs can be parsed, or unsupported constructs are explicitly reported.

### M2 — Catalog, dependency profiles and resolution

- Build immutable indexes and multiple-version namespace catalog.
- Implement strict dependency resolution and deterministic searches.
- Implement class/instance reference validation and nested type resolution.
- Test MMXU plus representative 7-410 and 7-420 classes.

**Exit:** Users can inspect, validate and resolve core models with explicit edition/profile selection.

### M3 — Semantic mapping and ontology helpers

- Add typed quantity, phase, reference and unit annotations.
- Add mapping rules, override hooks, provenance and uncertainty indicators.
- Add small, deterministic concept graph and JSON export.

**Exit:** IEC measurements can be described and mapped to common semantic concepts without asserting device support.

### M4 — Generator, CLI and docs

- Implement optional typed Go generation, model JSON export, diff and documentation indexes.
- Add `nsdctl`, reproducibility checks, examples and documentation.
- Enforce licensing gate on generated deliverables.

**Exit:** Useful developer SDK and CLI with reproducible results.

### M5 — CI, NSD update monitor and OTFabric integration

- Add normal public CI and weekly official NSD version monitor.
- Integrate new model package with existing `go-iec61850` through adapters; preserve existing SCL/reference behavior.
- Provide semantic mapping examples to other OTFabric libraries.
- Establish release/versioning and support policy.

**Exit:** Release-ready package with monitored coverage and zero accidental IEC source redistribution.

## 14. Definition of done for the first release

- [ ] Reusable `github.com/otfabric/go-iec61850-model` Go module builds on Go 1.24+.
- [ ] Pure-Go importer parses available official NSD packages for 7-2, 7-3, 7-4, 7-410, and 7-420, subject to access; any gaps are explicitly documented.
- [ ] Official source packages are only temporary inputs; none are committed or exposed by CI.
- [ ] Source-provenance and compatibility profiles are explicit and pinned.
- [ ] Catalog supports correct LN, DO, CDC, DA and enumeration lookups and multi-version namespaces.
- [ ] Type resolver and reference validator have typed errors and comprehensive fixtures.
- [ ] Semantic helpers include electrical quantities, phases, voltage references and conservative unit handling.
- [ ] CLI can inspect, validate, list, search, diff and generate permissible outputs.
- [ ] Go code and docs generation is deterministic and complies with the redistribution/license policy.
- [ ] Public CI passes without IEC NSD downloads or credentialed access.
- [ ] Weekly NSD update monitoring files useful issues without duplicates.
- [ ] README, architecture, sources, licensing, examples, and edition coverage docs are complete.
- [ ] Existing `go-iec61850` reference and SCL semantics are not broken.
- [ ] The project never claims support for an unavailable or unvalidated namespace release.

## 15. Forward-looking architecture

In a future phase, introduce `go-ot-ontology` to host **protocol-independent semantics** for quantities, units, phase relationships, measurement kinds, equipment, and physical connectivity. `go-iec61850-model` should provide IEC-specific adapters, rather than making Modbus, SunSpec, DNP3, BACnet or OPC UA integrations depend on an IEC model merely to describe a measurement.

This separation supports OTFabric's longer-term needs: heterogeneous device discovery, vendor register contracts, SCL-aware integration, simulation and realistic digital twins. These are **consumers of the model**, not features to place into the initial core package.

## 16. Implementation decisions to resolve during M0

1. Which exact official IEC NSD package URLs and schema revisions are accessible and permitted for scripted retrieval?
2. What output may legally be committed or distributed under the applicable IEC Code Components EULA? Is attribution required in generated code?
3. Can a compatible 7-410:2026 NSD be obtained, or should the initial hydro profile pin an older official NSD?
4. Should the parser use pure standard-library XML decoding plus manual structural validation, or a vetted XSD validation adapter for developer-time checks?
5. Which object-reference parser should be canonical, and how should existing `go-iec61850.ParseRef` be adapted without cyclic imports or breaking API changes?
6. Should a future `go-ot-ontology` host the core semantic quantities from day one, or should `semantic/` begin with internal IEC-specific contracts and migrate via adapters?

---

**Guiding principle:** *IEC sources are authoritative build inputs; OTFabric owns the parsing, type system, resolution, and semantic interpretation. Preserve provenance and namespace compatibility, minimize legal redistribution risk, and keep all runtime protocol stacks separate from the metadata library.*
