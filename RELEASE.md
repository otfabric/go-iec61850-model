# go-iec61850-model Releases

## v0.10.0

**Date:** 2026-11-11
**Previous release:** v0.9.1

Presence conditions are evaluated. The new package `presence` takes an instance (the data objects of a logical node, the attributes of a data object, with the values and context the conditions read) and returns, per element, whether the condition makes it required, optional or forbidden and whether the instance is satisfied or violated, with the reason. A condition that needs a fact the instance does not give is undetermined and names the fact; nothing is guessed.

Semantic mapping for the measurement and status classes. The default rules grow from MMXU, MMXN and MMTR to the measurement, supervision, process and DER classes of IEC 61850-7-4 and 7-420, each derived from the data object name, its CDC and the standard's description; what the standard leaves open is excluded with a reason instead of guessed.

### Added

- **Package `presence`**: `Evaluate` over an element list; `New(profile)` with `LogicalNode`, `CDC`, `ConstructedAttribute`, `Check`, `Validate` and `In`, following described children (data object → CDC → sub data objects and constructed members). `Report.All`, `Violations`, `Undetermined`, `OK`, `Err`; `Result` with `Requirement`, `Verdict`, `Reason`, `Missing`, `Instances`.
- Rules for every condition the embedded catalog uses or defines (40): `M`, `O`, `F`, `na`; `Mmulti`, `Omulti`, `MmultiRange`, `OmultiRange` with range and instance-number width checks (7-2 §22.2); the sibling forms `MF`, `MO`, `OF`, `OM`, `FM`, `MmultiF`; the groups `AtLeastOne`, `AtMostOne`, `AllOrNonePerGroup`, `AllOnlyOneGroup`, `AllAtLeastOneGroup`, `MAllOrNonePerGroup`; the context forms `MOln0`, `MFln0`, `MOrootLD`, `MOlnNs`, `MOdataNs`, `MOcdcNs`, `MOsbo`, `MOenhanced`, `MOrms`, `MFsubst`, `MFscaledAV`, `MFscaledMagV`, `MFscaledAngV`; and the fact forms `MOcond`, `MFcond`, `OFcond`, `MOoperTm`, `MONamPlt`, `MORange`, `OMSynPh`, `MFsg`, `MFnsg`. `Conditions`, `Describe`, `Known`, `IsMultiInstance`, `ParseRange`.
- `presence.ReadInstance`: the JSON form of an instance, with size, name-count and depth limits.
- `presence.ValidateNamespace`: warnings for presence conditions without a rule, with a missing or malformed argument, an unexpected argument or an undeclared sibling. In the embedded catalog it reports one: IEC 61850-7-410 AEMC.VolFunRef is `Mmulti(VolSpt)`.
- `resolver.Resolver.Presence` and `ValidatePresence`: the evaluation through a resolver, honouring `In`. Resolution itself is unchanged.
- `model.ErrInvalidInstance`, `model.ErrPresenceViolation` and `*model.PresenceError`.
- `nsdctl model check`: evaluate a class instance from `--present`, a JSON instance (`--instance`) or an SCL file (`--scl` with `--ln LD/LN` or `--ln-type ID`), with `--value`, `--fact`, `--ld-ns`, `--ln-ns`, `--ctl-model`, `--root-ld` and `--all`; exit code 1 on a violation. `nsdctl validate --presence`.
- docs/presence.md.
- **Default rules** (294) for MMXU (`HzRte`), MMXN (`Imp`), MMTN, MSQI, MHAI, MHAN, MMDC, MMET, MFLW, MENV, MHYD; SIML, SARC, STMP, SPRS, SVBR, ZBAT, ZAXN and the operation counter of every supervision class; XCBR and XSWI positions and counters, YLTC and ATCC tap positions and measurements, KPMP and KFAN; the DER classes of 7-420 (DGEN, DLOD, DSTO, DBAT, DINV, SBAT, SINV and the other DER units through their abstract classes); ZBTC and SBAT of IEC TR 61850-90-3 and the measured energies of IEC TR 61850-90-6 in MMTR and MMTN.
- **Vocabulary**: 38 new quantities (temperature, pressure, flow rate, irradiance, THD, crest factor, imbalance, switch and tap position, operating state, counts, ...), DC references (pole-pole, positive-ground, negative-ground), `Sequence`, `Descriptor.Of` and `Rule.Of` (the quantity a ratio refers to), `Descriptor.Harmonic` (the array index of HMV, HWYE and HDEL values), `ComponentState` for `stVal` and `valWTr.posVal`, `Quantity.IsState`, and `Criteria.Of`/`Criteria.Sequence`.
- **Harmonics and sequences**: HWYE and HDEL take the phase from `phsAHar` … `phsCAHar`, HMV and SEQ data objects expand into their arrays and components; notes state the device dependencies (`numCyc`, `hvRef`, `seqT`).
- **Exclusions**: `Exclusion`, `DefaultExclusions` (83, each with a reason), `Mapper.WithExclusions` and `Mapper.Exclusions`; an excluded data object fails `Describe` with `ErrNoMapping` and the reason.
- **Coverage report**: `Coverage`, `Mapper.Coverage` and `Mapper.CoverageOf` report per class which data objects are mapped, excluded, unmapped or not measured; `IsMeasuredCDC`. The generated `coverage.md` and `coverage.json` gain a semantic coverage section per class.
- Units: Pa, Pa/s, K/s, m/s, m/s², m³, m³/s, l/s, W/m², kg/m³, S/m, dB, ppm, 1/s, rad/s, min, h, Ah, As, C, and the SIUnitKind spelling "Watts".
- `TestSpec_*` tests per class against the embedded official namespaces, and per-class golden files in `semantic/testdata/golden` (`make golden`).

### Changed

- The public text of `AllAtLeastOneGroup` (`texts/en/pc.tsv`) now says what the evaluator checks: every group complete or absent, at least one group present.
- `nsdctl validate` documents its exit code 1 for errors (it said 3).
- Generator output format 6: the semantic coverage section and the new rules change `coverage.md`, `coverage.json`, `logical-nodes.md` and `ontology.json`. The committed `docs/reference` is regenerated when `nsdctl` is re-pinned.
- `Quantity.DefaultUnit` is the zero unit for states, counts, rotational speed, vibration and imbalance.

### Tests

- `TestSpec_7_3_Table1_*` and `TestSpec_7_2_22_2_MultiInstance` per condition family, on synthetic element lists; a coverage test that every condition name of the embedded catalog has a rule and every fact rule is undetermined without its fact; evaluation of every class of the default and extended profiles; `FuzzReadInstance`; `FuzzReadSCL` and synthetic SCL fixtures for `nsdctl model check`.

---

## v0.9.1

**Date:** 2026-11-11
**Previous release:** v0.9.0

### Changed

- `nsdctl` is built on library v0.9.0. The v0.9.0 binaries were built on a pre-release commit with the same library code.

---

## v0.9.0

**Date:** 2026-11-11
**Previous release:** v0.8.2

Spec review against IEC 61850-7-2:2010, 7-3:2010, 7-4:2010 and 8-1:2011. The importer and the embedded catalog match the NSD files element for element; the fixes are in the reference parser and the resolver. Titles and descriptions now surface in the API, from public texts written by OT Fabric for every element of every embedded namespace, with an optional local doc set on top; the repository ships no IEC text.

### Fixed

- **Control references resolve** (8-1 §20.3, Tables 80/83/85/88/90): `SBO`, `SBOw`, `Oper` and `Cancel` with FC CO are built from the CDC service parameters (`ctlVal`, `operTm`, `origin`, `ctlNum`, `T`, `Test`, `Check`; Cancel has no Check), so `XCBR.Pos.Oper.ctlVal[CO]` and `Pos[CO]` resolve and `Leaves` lists the control leaves; a CDC without a ctlVal parameter (the deprecated 7-410 TAG) gets no ctlVal member (MDL-01).
- **Array elements** `name(n)` (7-2 §6.1.2.3) are parsed per component and resolved; a malformed index or an index on a non-array is rejected; `ToMMS` rejects indexed references (MDL-02).
- **Length limits** (7-2 Table 4, 8-1 §8.1.3.2.2): `ParseStrict` measures without the `[FC]` suffix and enforces LD ≤ 64 and path ≤ 64; `Validate` enforces LD ≤ 64; `ToMMS`/`FromMMS` enforce the 64-character MMS identifier (MDL-03, MDL-04, MDL-05).
- **FC syntax** (7-2 Table 20, 8-1 Figure 4): an FC is two upper-case letters; the constants are split into ACSI and SCSM sets with `IsACSI`/`IsSCSM`, plus `FCGS` and `FCXX` (MDL-06).
- **Multi-instance ranges**: instance numbers outside `OmultiRange`/`MmultiRange` are rejected (MDL-07).
- **NSDoc**: presCondArgs texts are applied to the new `PresenceCondition.ArgsText` (MDL-08); the NSDoc of the exact release wins, and a matching NSDoc that applies nothing raises the warning `doc-ids-unmatched` (MDL-09).
- `ResolveString("MMXU1.Hz")` without an LD fails with a hint to add `LD/` (MDL-10).

### Added

- **Doc sets** (`nsd.DocSet`): texts keyed by model path (`LN:MMXU`, `DO:MMXU.TotW`, `CDC:MV`, `DA:MV.mag`, `FC:ST`, …), so one set covers every release; JSON `otfabric-docset/1` (optionally gzip) or NSDoc-style XML; `ApplyDocSet` fills empty fields only, and `nsd.WithFallbackDocs` lets your own NSDoc win.
- **Public texts** (`texts/`, MIT): 10,042 titles and descriptions in OT Fabric's own wording, every key of every embedded namespace except 14 unused abbreviation terms whose meaning is not established (text `-`) (`official/texts_gen.json.gz`, generated by `internal/tools/textgen`; `make texts`, `make texts-check` in `make check`). Logical node, CDC, attribute, type and condition texts are hand-written; data object descriptions are composed from 1,152 term expansions (`TotW` → "Total active power"), with overrides in `texts/en/do.tsv`; enumeration texts are derived from the names. A `?` third column flags an entry for review; none are flagged.
- **Documented profiles** in `official`: `DefaultDocumented`, `ExtendedDocumented`, `OpenDocumented`, `OpenWithDocs`, `DocumentedCatalog`, `CatalogWithDocs`, `DocLanguages`, `Docs`, `PublicDocs`, `DocsSource`, `PublicDocsSource`, `ErrNoDocs`. English texts are always available; a local doc set (`IEC61850_MODEL_DOCS`, or `official/docs_gen.json.gz` embedded with `-tags iecdocs`) takes precedence where it has a text. `Default()` and `Extended()` are unchanged.
- **Descriptions in results**: `model.Doc.Text/Long/HasText/Clear`, `resolver.Resolution.Doc/Description`, `Step.Doc`, `semantic.Descriptor.Title/Description`, and text search with `catalog.Query.Text`.
- `internal/tools/docextract`: a maintainer tool that builds a doc set from licensed spec PDFs (`make docs-extract SPECS_DIR=…`); `make build-docs`, `make test-docs`.

### Changed

- Rejected now: lower-case FCs, malformed `(…)` components, LD names over 64 in `Validate`, identifiers over 64 and indexed references in `ToMMS`/`FromMMS`.
- `resolver.Step` gains `Index` and `Doc`, `catalog.Result` gains `Title` and `Description`: positional struct literals and `==` comparisons need updating.
- docs/model.md documents the doc keys, the presence-condition name mapping and the Edition 2 → 2.1 deltas (MDL-11, MDL-12).

### Tests

- Spec tests named after their clause in reference, resolver and model (`TestSpec_8_1_20_3_ControlServiceParameters`, `TestSpec_7_2_Table4_ObjectReferenceLength`, …), doc-set tests with OTFabric-worded fixtures, a coverage test that every catalog element has a public text, an originality check of the public texts against a local doc set, `FuzzReadDocSets`, and real-doc-set checks that run only when a doc set is present (`REQUIRE_IEC_DOCS=1` makes the skip a failure).

---

## v0.8.2

**Date:** 2026-11-09
**Previous release:** v0.8.1

### Changed

- `nsdctl` is built on library v0.8.1. [docs/reference/index.md](docs/reference/index.md) now explains the publication stage codes of its Stage column (`IS`, `TR`, `draftTR`, not declared) in a "Stage" section.

---

## v0.8.1

**Date:** 2026-11-09
**Previous release:** v0.8.0

### Changed

- `nsdctl` is built on library v0.8.0: its embedded catalog holds the 28 namespaces of every published release, with the `edition2` profile; `-p` takes any embedded release ID, e.g. `-p "IEC 61850-7-4:2007B4"`, and `catalog select` picks `edition2` for Edition 2 declarations, all without `--source`.
- `nsdctl generate docs --all-releases` lists the releases outside the documented profile on `index.md`; `make nsd-publish` uses it, so [docs/reference](docs/reference/index.md) lists the 16 earlier releases under "Other releases".
- The generated reference index explains the publication stage codes of its Stage column (`IS`, `TR`, `draftTR`, not declared) in a "Stage" section; [docs/reference](docs/reference/index.md) gets it once `nsdctl` is built on the library release that carries it. [docs/nsd-sources.md](docs/nsd-sources.md#what-the-official-packages-contain) explains the codes now.

---

## v0.8.0

**Date:** 2026-11-09
**Previous release:** v0.7.0

### Added

- The embedded catalog holds every NSD release IEC publishes: IEC 61850-7-2, 7-3 and 7-4 from 2007A2 to 2007B6 and IEC TR 61850-90-6 2018A5 besides 2018B, 28 namespaces (was 12). The registry and lock also track the 8-1 service namespace releases 2003 to 2003A3. IEC publishes no NSD for Edition 1 (2003) of 7-4.
- Profile `edition2`: IEC 61850-7-2, 7-3 and 7-4 at 2007A3, for devices that declare `IEC 61850-7-4:2007` or `2007A`. `nsdctl -p` takes any embedded release ID, e.g. `-p "IEC 61850-7-4:2007B4"`, without `--source`.
- `nsdctl catalog select NAMESPACE` names the embedded profile for the release a device declares in `LLN0.NamPlt.ldNs` (`official.ForNamespace`).
- `nsdctl generate catalog --all-releases` embeds every loaded release; `make nsd-publish` uses it.
- With `Embed` set, the generated Markdown reference (`generate docs`) lists the releases outside the documented profile in an "Other releases" table on `index.md`. The committed [docs/reference](docs/reference/index.md) documents the extended profile; it gets the table with the next patch release, whose `nsdctl` is built on this library.

The `nsdctl` binaries of this release are built on v0.7.0 and embed its catalog of 12 namespaces: `edition2`, release IDs such as `-p "IEC 61850-7-4:2007B4"` without `--source`, and `catalog select` choosing `edition2` work in the `nsdctl` built on v0.8.0, with the next patch release. With `--source` and the official packages they work already.

### Changed

- The embedded catalog of the library is generated in the gzip format of v0.7.0: about 0.33 MB in a binary for 28 namespaces, where the 12 namespaces of v0.7.0 took 3.76 MB. Loading it takes about 65 ms, once per process.
- `nsdctl dependencies` reports a dependency several releases satisfy as `ok`, with the newest release (the one a profile adds), instead of `ambiguous`.
- `nsdctl catalog overlaps` without `--profile` compares the namespaces of the extended profile instead of every loaded namespace, so earlier releases are not compared with their successors.

---

## v0.7.0

**Date:** 2026-11-09
**Previous release:** v0.6.0

### Added

- `official.ForNamespace` returns the embedded profile for the release a device declares in `LLN0.NamPlt.ldNs` or `lnNs`, e.g. `IEC 61850-7-4:2007B`. A declaration without a revision selects the oldest embedded revision of that version; `IEC 61850-7-4:2003` (Edition 1) finds none, since IEC publishes no NSD for it.
- Named profiles may pin a release: `7-4@2007A3`. A bare part selects the newest loaded release of the part instead of failing when several are loaded.
- The catalog generator can embed every loaded release, not only those of the profile (for `nsdctl generate catalog --all-releases`).
- Registry release labels without a revision, e.g. `2003` for IEC 61850-8-1:2003.

### Changed

- `nsdctl generate catalog` writes `catalog_gen.json.gz`, compact JSON compressed with gzip, and embeds only that file; the indented `catalog_gen.json` stays next to it for review, and `--check` covers both. The `official` package decompresses the embedded catalog once, on first use, and still reads plain JSON. Compressed, the catalog of v0.6.0 takes about 0.14 MB instead of 3.76 MB; the committed catalog switches to this format with the next release, whose `nsdctl` generates it. The API is unchanged.
- `catalog.Profile` adds the newest loaded release that satisfies a dependency it does not select, instead of failing with `ErrAmbiguousReference`: releases of one version and revision only add corrections. References inside definitions resolve the same way on a catalog of several releases. Lookups of a name that two loaded namespaces define stay ambiguous.

---

## v0.6.0

**Date:** 2026-11-08
**Previous release:** v0.5.0

### Changed

- No changes. v0.6.0 was released by mistake from the same commit as v0.5.0, a few seconds later; the library and `nsdctl` are identical to v0.5.0.

---

## v0.5.0

**Date:** 2026-11-08
**Previous release:** v0.4.1

### Changed

- `nsdctl` is built with the latest Go 1.26 patch release; v0.4.1 was built with go1.24.0.
- The library and `nsdctl` require Go 1.26.0 or later (was 1.24.0), the organisation floor. Library code is unchanged.

---

## v0.4.1

**Date:** 2026-11-07
**Previous release:** v0.4.0

### Changed

- The bundled command-line tools are built on v0.4.0. Library code is the same as v0.4.0.

---

## v0.4.0

**Date:** 2026-11-07
**Previous release:** v0.3.0

### Breaking changes

- **`nsdctl version`** prints one line, the format of every otfabric CLI:
  `nsdctl 0.4.0 (tag v0.4.0, commit <full sha>, built <date>, go1.27.3, darwin/arm64)`,
  instead of one field per line. `--json` writes one line with the keys
  `name`, `version`, `tag`, `commit`, `built`, `libraries`, `go`, `platform`, always
  present and in that order (was `version`, `tag`, `commit`, `buildDate`,
  `goVersion`, `platform`, indented, empty ones left out). The version has
  no leading `v`, the commit is no longer cut to 8 characters, and a build
  that was not released reports `dev` (pseudo-versions of `go build` are not
  reported as versions), the VCS revision with `-dirty`, and `unknown` for
  what is not known. `make build` injects the version and tag only when HEAD is on a tag, and the full commit.
  Library code is unchanged.

### Added

- **Shell completion** for bash, zsh, fish and PowerShell also completes
  `--source` and the source arguments of `inspect`, `validate`,
  `dependencies`, `schema` and `diff` (NSD, XSD and ZIP files), `--lock` and
  `--index` (JSON files), `--out`, `--dest` and `--packages` (directories);
  every other flag that takes a value offers no file names. Install:
  [Shell completion](README.md#shell-completion).

---

## v0.3.0

**Date:** 2026-11-07
**Previous release:** v0.2.2

### Breaking changes

- **nsdctl exit codes follow the organisation table.** Library code is unchanged.
  Scripts that test nsdctl exit codes need the new values
  ([cmd/nsdctl/docs/exitcodes.md](cmd/nsdctl/docs/exitcodes.md)):

  | Case | Old | New |
  | --- | --- | --- |
  | Usage error, unknown name, unresolvable path, refused overwrite | 1 | 2 |
  | Other failures: writing the output, unexpected errors | 1 | 10 |
  | A source cannot be read or parsed | 2 | 3 |
  | A download of `sources fetch` failed | 2 | 4 |
  | Invalid lock or registry, load errors, digest mismatch, unpinned package | 3 | 3 |
  | `validate` found errors, `dependencies` found unsatisfied dependencies | 3 | 1 |
  | Differences: `diff --exit-code`, `generate --check`, `sources check`, `schema --exit-code` | 4 | 1 |
  | The license policy refused `generate --publish` | 5 | 9 |
  | Interrupted by SIGINT or SIGTERM | – | 130 |

### Changed

- nsdctl runs under a context cancelled by SIGINT and SIGTERM, so
  `sources fetch` stops its downloads when interrupted.

---

## v0.2.2

**Date:** 2026-10-31
**Previous release:** v0.2.1

### Changed

- The bundled command-line tools are built on v0.2.1. Library code is the same as v0.2.1.

---

## v0.2.1

**Date:** 2026-10-31
**Previous release:** v0.2.0

### Changed

- Tests, CI and documents only. `make build-all` writes to `release/`. Compiled binaries are gone from git history, and tags were rewritten.

---

## v0.2.0

**Date:** 2026-10-30
**Previous release:** v0.1.1

### Changed

- **`cmd/nsdctl` is now a nested module** (`github.com/otfabric/go-iec61850-model/cmd/nsdctl`).
  The library's `go.mod` no longer requires cobra, so consumers stop inheriting
  it. Library import paths and the exported API are unchanged.
  - Release binaries are published as before.
  - Build from a checkout with `make build`, or `cd cmd/nsdctl && go build .`.
  - `go install github.com/otfabric/go-iec61850-model/cmd/nsdctl@<version>` works
    up to v0.1.1 and not after it, while the nested module's `replace` is in place.

---

## v0.1.1

**Date:** 2026-10-21
**Previous release:** v0.1.0

### Summary

nsdctl usability: shell completion, real-world use cases with real output
from the official models, and command groups that explain themselves.

### Added

- **`nsdctl completion bash|zsh|fish|powershell`** — completion scripts, as in
  the other otfabric CLIs. Flags with a fixed set of values complete to them:
  `--profile` to the profile names and embedded namespaces; `--ns`, `--left`
  and `--right` to the embedded namespaces (by ID and part, e.g. `7-420`);
  `--kind`, `--fc`, `--class` and `--tiers` to their values.
- **[docs/use-cases.md](docs/use-cases.md)** — real integration tasks solved
  with nsdctl and the embedded official models: power meter measurements and
  units (`MMXU`), explaining a reference, decoding `SIUnit`/`multiplier` and
  mode values, battery storage (`DBAT`, `DSTO`), circuit breaker position
  (`XCBR`), protection settings per setting group (`PTOC`), finding data
  objects by CDC, legacy IEC TR 61850-90-9 devices, tag lists from `--json`,
  and the same lookups in Go. A test re-runs the documented commands so the
  output stays real.

### Fixed

- `nsdctl model`, `catalog`, `sources` and `generate` printed nothing without
  a subcommand; they now print their help, and unknown subcommands fail with
  exit code 1.
- `nsdctl model list MMXU` now points to `nsdctl model show MMXU`.

---

## v0.1.0

**Date:** 2026-10-21
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

- **Package `official`** — the 12 official NSD namespaces embedded (no
  documentation texts) with the profiles `default` and `extended`:
  `official.Default()`, `official.Extended()`, `official.Open(name)`,
  `official.Notice()`.
- **Package `names`** — committed Go constants for every logical node class
  (`LN_MMXU`), data object (`MMXU_Hz`), CDC (`CDC_WYE`) and enumeration
  (`BehaviourModeKind_on`, with `String`) of the default profile, generated
  from the official packages; `names/NOTICE.md` attributes them to IEC with the
  license and copyright notices.
- **Model reference** — `docs/reference/`, generated for the extended profile.
- The three are regenerated with `make nsd-publish`; CI checks them against the
  pinned packages.
- **Namespace policy** — IEC TR 61850-90-9 is left out of the default profile
  and kept in `extended`. Definitions of the same name in two namespaces are
  never merged: unqualified lookups return `ErrAmbiguousSymbol`, qualified
  lookups (`LogicalNodeIn` and the other `…In` lookups, `Resolver.In`) select
  one, and `Overlaps` / `nsdctl catalog overlaps` compares them.
- **nsdctl** — uses the embedded catalog without `--source`; `--ns`, named
  profiles, `catalog profiles`, `catalog overlaps`, `generate catalog`,
  `notice`.
- **Lock** — the 12 NSD namespaces are `permitted` (decision recorded in
  docs/licensing.md); 8-1 and 7-7 stay `local-only`.
- **Generator** — output format 5: names defined by several namespaces are not
  merged (per-definition comments, one Go type per enumeration definition), and
  the Go output comes with a generated test.
- **Fix** — namespaces loaded from a directory of archives record the digest of
  their archive, which the license gate compares with the lock.
