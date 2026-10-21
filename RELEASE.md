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
