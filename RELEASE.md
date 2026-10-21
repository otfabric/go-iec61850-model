# go-iec61850-model Releases

## Unreleased

**Previous release:** none

## Summary

First implementation of the model core and its semantic layer (milestones
M0–M3 of [REQUIREMENTS.md](REQUIREMENTS.md)): a secure NSD importer, a typed
model, a multi-release catalog with explicit profiles, reference parsing
compatible with go-iec61850, type resolution with provenance, rule-based
semantic descriptions, a concept graph, and the `nsdctl` CLI with a
deterministic generator behind a license gate. Verified on synthetic
fixtures only; no official namespace release is claimed as supported yet.

## Changes

### Added

- **Package `model`** — namespaces and namespace IDs, logical node classes,
  data objects, CDCs, data attributes, constructed attributes, enumerations,
  basic types, functional constraints, presence conditions, provenance,
  diagnostics, logical node groups, and the module's sentinel and typed errors.
- **Package `nsd`** — `Parse`, `ParseNamespace`, `LoadZIP`, `ReadZIP`,
  `LoadFile`, `LoadDir`, `ReadManifest`, `Validate`, `ApplyDoc`. Reads NS and
  NSDoc documents, recognizes ServiceNS and ApplicableServiceNS, reports
  unsupported constructs as diagnostics with XML paths, and enforces size,
  depth, token, entry, ratio and archive-name safety bounds.
- **Package `catalog`** — `New`, lookups with ambiguity errors, `Profile` with
  dependency closure, inheritance-aware `DataObjects` and `DataObject`,
  listings, `Search`, `Diff`.
- **Package `reference`** — `Parse`, `ParseStrict`, `ParseClass`,
  `ParseLNName`, `Normalize`, `FromMMS`, `Ref.ToMMS`; same format and
  validation rules as `go-iec61850` `ParseRef`.
- **Package `resolver`** — `Resolve`, `ResolveString`, `ResolveClass`,
  `ResolvePath`, `ResolveAttribute`, `ResolveCDC`, `ResolveInheritance`,
  `Leaves`; parameterized CDCs, multi-instance data objects, FC checks and
  cycle detection.
- **Package `semantic`** — `Describe`, `FindMeasurements`, `Match`,
  `NormalizeUnit`, `DeviceUnit`, `Mapper` with `DefaultRules` (MMXU, MMXN,
  MMTR) and overrides. Descriptors carry quantity, phase, voltage reference,
  energy direction, value component, unit hint, rule, source and confidence.
- **Package `ontology`** — `BuildGraph`, `Concept`, `Concepts`,
  `FindConcepts`, `Relations`, `Incoming`, `Walk`, `ExportJSON`.
- **Errors** — `ErrNoMapping`, `ErrUnknownUnit`, `ErrInvalidRule`,
  `ErrUnknownConcept`.
- **`nsdctl`** (`cmd/nsdctl`) — `inspect`, `validate`, `dependencies`,
  `diff`, `model show|list|search|describe|measurements|leaves`,
  `generate go|docs|json` (`--check`, `--force`, `--publish`,
  `--include-doc-text`), `sources check`, `version`; text and JSON output;
  exit codes 0–5. Released as a binary by the release workflow.
- **Source registry** — `sources/index.json` (replaces the planned YAML) and
  the lock format with monitor states and redistribution decisions.

### Changed

- **Dependencies** — `github.com/spf13/cobra` for `cmd/nsdctl`; the library
  packages still use the standard library only.

### Package coverage

- **Tracked packages** — `sources/index.json` lists the 14 code component
  packages with tiers (required: 7-2, 7-3, 7-4, 7-410, 7-420; recommended:
  90-9, 90-14; optional: 90-6, 90-3, 90-4, 90-11, 90-21, 8-1, and the 7-7
  schema).
- **`nsdctl sources check --packages DIR`** reports per package whether it is
  present and carries its release; **`nsdctl sources lock`** pins digests and
  statuses; **`nsdctl schema`** lists XSD constructs the importer does not
  model.
- **Importer** — ZIP archives are recognized by content (official names such
  as `…2007B6.light` have no `.zip`); light/full is taken from the package
  name; XSD files are inventoried (`Package.Schemas`, `nsd.ParseSchema`,
  `nsd.Supported`); a directory of packages keeps one manifest per archive
  (`Package.Manifests`).
- **Dependencies** — a later release of the same version and revision now
  satisfies a dependency that names an earlier release.
- **Conflicts** — the ontology graph and the generators keep working when
  extension namespaces define the same name; conflicts are listed
  (`Graph.Conflicts`, generated headers). Generator output format 2.

### Official packages

- **`nsdctl sources fetch`** and Makefile targets `nsd-fetch`, `nsd-lock`,
  `nsd-check`, `nsd-generate`, `nsd-generate-check`, `nsd-clean`; packages are
  downloaded from `assets.iec.ch`, verified against pinned digests, and kept
  out of version control.
- **Workflow `nsd.yml`** — fetch, validate, integration test, deterministic
  generation, compile check; no caches or artifacts; weekly tracking issue.
- **Importer** — abstract classes inside `LNClasses`, CDC variants, per-FC
  attribute copies, extension classes, `Manifest.xml`, namespace copyright,
  changes and NSD version, FC applicable services, value bounds, service
  namespaces, deduplication of identical nested copies.
- **Lock** — the 14 packages are pinned (`sources/nsd.lock.json`), all
  `local-only`.
- **Generator** — `NOTICE.md` with IEC attribution, notice and license; output
  format 3.

### nsdctl

- **Binaries** for Linux (amd64, arm64, armv7), macOS (amd64, arm64) and
  Windows (amd64, arm64) on every release, with a checksum file.
- **`nsdctl version`** prints version, tag, commit, build date, Go version and
  platform, like the other otfabric CLIs; `--json` supported.
- **`model show`** lists every variant of a CDC, the namespace of each data
  object, and the alternatives when an attribute's FC depends on the variant.
- **`schema`** reports gaps only for the NSD schema, not for the package
  metadata schemas.

### Generated names package

- **Package `names`** — committed Go constants for every logical node class
  (`LN_MMXU`), data object (`MMXU_Hz`), CDC (`CDC_WYE`) and enumeration
  (`BehaviourModeKind_on`, with `String`) of the 12 NSD namespaces, generated
  from the official packages; `names/NOTICE.md` attributes them to IEC with the
  license and copyright notices. Regenerated with `make nsd-publish`; CI checks
  it against the pinned packages.
- **Lock** — the 12 NSD namespaces are `permitted` (decision recorded in
  docs/licensing.md); 8-1 and 7-7 stay `local-only`.
- **Generator** — output format 4: names defined by several namespaces get one
  set of constants (union of data objects, merged literals), and the Go output
  comes with a generated test.
- **Fix** — namespaces loaded from a directory of archives record the digest of
  their archive, which the license gate compares with the lock.
