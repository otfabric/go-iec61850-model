# PROGRESS.md — `otfabric/go-iec61850-model`

## Evaluation of REQUIREMENTS.md and PLAN.md

**Date:** 2026-10-21

The requirements are sound and the plan's core idea — build everything on
synthetic fixtures while source discovery and the license review run in
parallel — is the right one. The first slice follows the plan's recommended
option B: M0 scaffolding plus M1 (importer, model) and M2 (catalog, profiles,
references, resolution), leaving semantic mapping, ontology, CLI, generator and
monitor for later milestones.

Points the plan left open or that needed adjustment:

1. **Errors live in one package.** REQUIREMENTS §4 lists `model/errors.go` and
   `reference/errors.go`. Separate sentinels per package would make
   `errors.Is` depend on which package produced an error, so every sentinel
   and typed error is in `model`, and the other packages wrap them. Added
   sentinels beyond §5.4: `ErrLimitExceeded`, `ErrUnsafeArchive`,
   `ErrUnknownNamespace`, `ErrUnknownCDC`, `ErrUnknownType`, `ErrCycle`.
2. **`catalog` does not import `nsd`.** REQUIREMENTS §8.1 calls
   `catalog.New(packages...)` with `*nsd.Package` values. `catalog.New` takes
   a `catalog.Source` interface instead, which `*nsd.Package` implements, so
   the example works unchanged and the catalog stays independent of the
   importer.
3. **Model validation of references is in `resolver`.** §5.3 puts `Validate`
   in `reference`, but model validation needs a profile and type resolution;
   `reference` validates syntax and `resolver` validates against the model,
   which also avoids an import cycle. The three levels of validity of §6.4
   map to `reference`, `resolver`, and SCL/discovery.
4. **Multi-instance data objects.** go-iec61850's own reference vectors use
   `GGIO1.Ind1.stVal`, where `Ind1` is an instance of the `Omulti` data object
   `Ind`. The plan did not mention this; the resolver supports it.
5. **Parameterized CDCs.** ENS/ENC `stVal` take their type from the data
   object's underlying type. Not in the plan; required to resolve `Beh.stVal`.
6. **Inheritance on catalogs too.** Inheritance-aware lookups work on both
   `Catalog` and `Profile`; they fail with ambiguity errors on a catalog with
   several releases rather than being profile-only.
7. **IECManifest.xml is kept generically.** Its format is not verified yet, so
   `nsd.Manifest` preserves every element and attribute in order instead of
   guessing field names. Model it once an official manifest has been seen.
8. **XSD validation (open decision 4).** Decided: standard library only.
   The importer validates what it models and reports everything else; no XSD
   engine. Revisit only if official packages show constructs this misses.
9. **Default profile.** "Latest compatible, released, available and validated"
   needs the lock file. Until then `DefaultProfile` succeeds only when there is
   exactly one release per standard.
10. **NSD XML namespace.** Only `http://www.iec.ch/61850/2016/NSD` is accepted
    by default; `WithSchemaNamespaces` widens it after a structural check. The
    attribute set modeled for each element follows IEC 61850-7-7 as understood
    without the official schema at hand; unknown attributes are reported, not
    dropped, so the first official import will show any gap (M1 exit
    criterion).

Open decisions 1–3 of REQUIREMENTS §16 (official URLs, redistribution, 7-410
NSD) still need someone with IEC access; they block real data, not code.

## Phase M0 — Scaffolding and source/license gate

**Status:** Scaffolding complete; source discovery and license review open

| Item | State |
| --- | --- |
| `go.mod` (Go 1.24), MIT `LICENSE`, `.gitignore` excluding IEC file types | Done |
| `Makefile`, `.golangci.yml`, `codecov.yml` aligned with sibling repos | Done |
| `.github/workflows/ci.yml`, `release.yml` on the shared otfabric workflows | Done |
| README, ERRORS, SECURITY, CONTRIBUTING, RELEASE, docs/ (architecture, model, namespaces, reference, nsd-sources, licensing) | Done |
| `sources/index.json`, `sources/nsd.lock.json` skeletons | Done (unverified) |
| Synthetic fixtures (`testdata/nsd`) | Done |
| Verify official NSD availability, URLs, EULA; 7-410:2026 NSD | **Open** — needs IEC access |
| Licensing decision for generated output | **Open** |

## Phase M1 — Importer and foundation types

**Status:** Complete on synthetic fixtures; official-package validation open

| File | Purpose |
| --- | --- |
| `model/namespace.go` | `NamespaceID` (parse, format, satisfy, compare), `Namespace`, basic types, FCs, trigger options, presence conditions, abbreviations, `Doc` |
| `model/logical_node.go`, `model/cdc.go`, `model/types.go` | LN classes, data objects, CDCs, attributes, constructed types, enumerations, type kinds, FCs, presence, LN groups |
| `model/provenance.go` | Provenance, `Diagnostic`, `Diagnostics` |
| `model/errors.go` | Sentinels and typed errors |
| `nsd/parser.go`, `nsd/raw.go`, `nsd/normalize.go` | Bounded read, token pre-scan, raw decode with catch-alls, normalization with XML-path diagnostics |
| `nsd/validate.go` | Duplicate definitions, members, literals; self references |
| `nsd/nsdoc.go` | NSDoc texts, `ApplyDoc` |
| `nsd/manifest.go` | Generic manifest reader |
| `nsd/package.go` | `LoadZIP`, `ReadZIP`, `LoadFile`, `LoadDir`; archive safety; provenance; documentation |

Exit criterion "all obtainable official NSDs parse, or gaps are reported"
waits for official packages.

## Phase M2 — Catalog, profiles, resolution

**Status:** Complete on synthetic fixtures

| File | Purpose |
| --- | --- |
| `catalog/view.go` | Shared name index and lookups with ambiguity errors; listings |
| `catalog/catalog.go` | `New`, `Namespace`, `Releases`, `Standards`, `DefaultProfile` |
| `catalog/profile.go` | `Profile` with dependency closure, conflict and reference checks, inheritance |
| `catalog/search.go` | `Search` with kind, name, CDC, type, FC, standard and group filters |
| `catalog/diff.go` | `Diff` of two sources |
| `reference/ref.go`, `reference/class.go` | Instance and class references, LN names, MMS form, `Normalize` |
| `resolver/resolver.go` | Path resolution, parameterized types, multi-instance DOs, FC checks, `Leaves`, cycle detection |

Tested: MMXU → Hz → MV → mag → FLOAT32; PNV → phsA → cVal → mag → f;
inherited `Beh.stVal` → BehaviourModeKind; extension-namespace class
inheriting across namespaces; release-to-release diff; ambiguity without a
profile; cycles; FC mismatches.

### Test summary

`make check` passes: gofmt (Go 1.24 and local), vet, staticcheck,
golangci-lint, govulncheck, race tests. Coverage: model 98.5%, nsd 96.7%,
catalog 98.1%, reference 99.5%, resolver 96.1%. Fuzz targets:
`nsd.FuzzParse`, `reference.FuzzParse`.

## Phase M3 — Semantic mapping and ontology helpers

**Status:** Complete on synthetic fixtures

**Date:** 2026-10-21

| File | Purpose |
| --- | --- |
| `semantic/quantity.go` | Quantities (labels, units, `Accepts`), phases, voltage references, energy directions, value components, confidence, basis |
| `semantic/units.go` | `Unit`, `NormalizeUnit`, `DeviceUnit` (through SIUnitKind and MultiplierKind of the loaded model) |
| `semantic/rules.go` | `Rule`, `DefaultRules` (MMXU, MMXN, MMTR, wildcard Hz), `Mapper`, overrides |
| `semantic/describe.go` | `Describe`, `DescribeResolution`, `FindMeasurements`, `Match`, `Criteria` |
| `ontology/graph.go` | Concepts, relations, `BuildGraph`, `FindConcepts`, `Relations`, `Incoming`, `Walk`, `ExportJSON` |

### Design decisions

1. **Explicit rules only.** A descriptor exists only when a rule names the
   class (or a base class) and data object and the CDC matches the model.
   Nothing is inferred from name fragments ("Tot", "Wh"); REQUIREMENTS §6.5
   asks to avoid inference where names are overloaded.
2. **Auditable output.** Every descriptor carries rule ID, source, basis
   (rule or override) and confidence; overrides must name their source.
3. **Unit hints vs device units.** `UnitHint` comes from the rule. Device units
   are read with `DeviceUnit` from the model's SIUnitKind/MultiplierKind
   enumerations, so no unit codes are hard-coded in the library.
4. **Phase from structure.** WYE/DEL sub data objects give the phase; DEL
   implies line-line. MMXU.PhV is mapped line-ground and PNV line-neutral, as
   in the REQUIREMENTS §6.5 table.
5. **Semantic vocabulary in `semantic` for now** (open decision 6). Quantity
   and phase names are protocol-neutral strings, and ontology IDs of those
   kinds (`quantity:voltage`) are meant to stay stable when they move to a
   shared module.
6. **Graph built from a profile.** Concept IDs are names, which are unique
   only within a profile; a profile with conflicting definitions fails with
   `ErrNamespaceConflict` rather than producing an ambiguous graph.
7. **Fixtures extended** with MMTR and CDC BCR to cover energy and counters.

New sentinels: `ErrNoMapping`, `ErrUnknownUnit`, `ErrInvalidRule`,
`ErrUnknownConcept`.

### Test summary

`make check` passes. Coverage: semantic 96.5%, ontology 98.0%; total 96.5%.
Examples document `Describe`, `FindMeasurements`, `Match`, `DeviceUnit`,
overrides, graph queries and traversal.

## Phase M4 — Generator, CLI and docs

**Status:** Complete on synthetic fixtures

**Date:** 2026-10-21

| File | Purpose |
| --- | --- |
| `cmd/nsdctl/main.go` | Root command, global flags, source/profile loading, exit codes, version |
| `cmd/nsdctl/cmd_packages.go` | `inspect`, `validate`, `dependencies`, `diff` |
| `cmd/nsdctl/cmd_model.go` | `model show|list|search|describe|measurements|leaves` |
| `cmd/nsdctl/cmd_generate.go` | `generate go|docs|json`, `sources check` |
| `internal/generator/` | Go, Markdown and JSON generation; headers with provenance; `Write`/`Check`; `Policy` |
| `internal/sources/` | Registry and lock types, validation, digest verification |
| `testdata/golden/` | Golden output for the synthetic fixtures |

### Design decisions

1. **cobra, as in sclparse and sunspecctl**, in the same module (REQUIREMENTS
   §4 places `cmd/nsdctl` here). Only the command imports it.
2. **Exit codes** extend sclparse's (0 ok, 1 usage, 2 parse, 3 validation)
   with 4 (differences / out of date) and 5 (license policy).
3. **License gate.** Output is local-only by default with a notice file;
   `--publish` requires a lock entry with `redistribution: permitted` and a
   matching digest for every source namespace.
4. **No device structs.** The Go output declares names and enumeration types,
   per REQUIREMENTS §7.
5. **Doc texts opt-in.** NSDoc texts are excluded from generated output unless
   `--include-doc-text`; JSON exports drop local paths.
6. **Registry in JSON** instead of YAML: no YAML dependency for the library.
7. **`sources check` is offline.** It validates the registry and lock and
   verifies local digests; discovery of new releases needs verified official
   endpoints and belongs to the monitor (M5).
8. **Golden output** under `testdata/golden` makes generator output a tested
   contract across versions; `.gitattributes` keeps fixtures byte-exact so
   digests match on every platform.

### Test summary

`make check` passes. Coverage: cmd/nsdctl 93.5%, generator 92.2%, sources
96.1%; total about 94.5%. The generated Go file is type-checked in tests.

## Package coverage (all tracked NSD packages)

**Date:** 2026-10-21

The project now tracks 14 code component packages in `sources/index.json`
(5 required, 2 recommended, 6 optional, plus the 7-7 schema). The names and
release labels come from the IEC catalog extract and are marked unverified:
the official listing pages are not publicly retrievable, and public listings
mention a newer 7-4 label (`2025A3`) to check.

A full-set test builds synthetic stand-ins for all 14 packages under their
exact official names and runs `sources check`, `sources lock`, `validate`,
`model`, `schema` and `generate` over them. It found and fixed:

1. Directories of packages failed because each archive has its own
   `IECManifest.xml`; manifests are now per archive.
2. Packages named without `.zip` (`…2007B6.light`) were neither loaded from
   directories nor hashed for the lock; archives are now recognized by content.
3. A dependency naming release 5 was not satisfied by release 6; later
   releases of the same version and revision now satisfy it.
4. Name conflicts between extension namespaces made the graph and generators
   fail; they now keep the first definition and list the conflict.

`TestOfficialPackages` (`make official NSD_DIR=...`) runs the same checks
against the real packages in place.

## Official packages: download, validation and generation

**Date:** 2026-10-21

All 14 tracked packages are downloadable from `https://assets.iec.ch/public/tc57/`
(two names corrected: 7-2 is `….Light`, 90-6 is `IEC_TR_….Light`). `nsdctl
sources fetch` downloads them into `.nsd-cache/` (or outside the repository),
verified against digests pinned with `nsdctl sources lock`; Makefile targets
`nsd-fetch`, `nsd-lock`, `nsd-check`, `nsd-generate`, `nsd-generate-check`,
`nsd-clean`, `official`; workflow `.github/workflows/nsd.yml` (PRs touching the
importer, weekly, on demand) in `$RUNNER_TEMP` without caches or artifacts,
with a tracking issue for weekly failures.

The first import of real data required:

1. Abstract classes inside `LNClasses`.
2. CDC variants (`variant`), selected by FC in the resolver.
3. Per-FC attribute copies (`_setMag`), resolved by device name and FC.
4. Extension classes (`isExtension`), merged into their base class.
5. Manifest `Manifest.xml`; NS `Copyright`, `Changes`, NSD version,
   namespace type, dependency types; FC applicable services; value bounds.
6. ServiceNS / ApplicableServiceNS (`.snsd`, `.AppNS`) recognized; 8-1 counted.
7. Identical nested copies imported once.
8. Unresolved references downgraded to warnings: they are upstream gaps
   (`PhyComAddr*`, `SCR`, 7-420 enumerations, `MFsg`), listed in
   docs/nsd-sources.md.
9. `FindMeasurements` skips unmapped data objects that cannot be resolved.
10. Generated output carries `NOTICE.md` with the IEC notices and license
    (CCv1); output format 3.

Result: 12 namespaces validate together with 0 errors; 3336 of 3577 concrete
data objects resolve to leaf types; generated Go (30k lines) compiles; the
output is deterministic.

## nsdctl distribution

- Binary release for linux/amd64, linux/arm64, linux/armv7, darwin/amd64,
  darwin/arm64, windows/amd64 and windows/arm64 through the shared
  `go-binary-release.yml`, with SHA-256 checksums; `make build-all` builds the
  same targets locally.
- `nsdctl version` in the layout of the other otfabric CLIs, with `--json`.
- README install instructions per platform; docs/cli.md walkthrough with real
  output from the official packages.

## Next

- **M0** — verify official sources and the license; fill `sources/`.
- **Licensing decision** — whether IEC-derived generated output may be
  committed; then set `redistribution: permitted` in the lock and publish with
  `--publish` (NOTICE.md is ready).
- **Service namespaces** — model ServiceNS (8-1) content, which likely defines
  `PhyComAddrL2`/`PhyComAddrUDP`.
- **M3 follow-up** — extend default rules for 7-420 DER and 7-410 classes once
  their official NSDs are loaded (only with rules whose CDCs are verified).
- **M5** — `nsd-monitor.yml`: weekly discovery against verified official
  endpoints (needs M0), compare with the lock via `nsdctl sources check`,
  deduplicated issues; `docs/integration.md`; the go-iec61850 adapter.
