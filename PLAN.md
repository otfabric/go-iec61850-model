# Plan: go-iec61850-model implementation

Build a standalone, pure-Go, transport-independent IEC 61850 NSD model library per `REQUIREMENTS.md`. The repo is currently greenfield (only `README.md` + `REQUIREMENTS.md`). The core strategy is to **decouple engineering from legal/source blockers**: build the entire library and CI against *synthetic* test fixtures, while M0 source-discovery runs in parallel. The one hard integration constraint is that the `reference` package must stay byte-compatible with the existing `go-iec61850.ParseRef` format `LD/LN.DO.DA[FC]` (fields `Ref.LD`, `Ref.LN`, `Ref.Path []string`, `Ref.FC`), and `go-iec61850-model` must never import `go-iec61850`.

## Phase 0 — Scaffolding & source/license gate (M0)

1. Initialize `go.mod` (Go 1.24+), `LICENSE` (MIT), `.gitignore` (exclude all IEC archives/XML/XSD/scratch dirs), the repo folder skeleton from REQUIREMENTS §4, and empty package `doc.go` stubs.
2. *Parallel:* Investigate official NSD availability (7-2, 7-3, 7-4, 7-410, 7-420) and the current 7-7 XSD; verify URLs, package formats, manifest shape, EULA. Explicitly check whether a 7-410:2026 NSD exists.
3. Define `sources/index.json` (official URLs/discovery metadata only) and `sources/nsd.lock.json` (permitted provenance metadata only) schemas.
4. Author `docs/licensing.md` capturing what generated output may be committed/redistributed.
5. Build **synthetic NSD fixtures** under `testdata/` (small, independently authored XML/ZIP) — unblocks all later phases without real IEC files.

## Phase 1 — Importer & foundation types (M1)

*(Core engineering can start after step 1; validation uses step 5 fixtures.)*

6. Implement `model/` typed IR: `NamespaceID`, `Namespace`, `LogicalNodeClass`, `DataObject`, `CommonDataClass`, `DataAttribute`, `Enumeration`, presence conditions, functional constraints, `provenance.go`, and sentinel + typed errors in `errors.go` (REQUIREMENTS §5.4).
7. Implement `nsd/` importer: `Parse`, `LoadZIP`, `Validate`, `ReadManifest` with secure XML (size/depth bounds, no external entities) and secure ZIP (traversal, duplicate-entry, decompression-bomb guards). Report diagnostics with XML path + namespace.

## Phase 2 — Catalog, profiles, resolution (M2)

*(Depends on Phase 1.)*

8. `catalog/`: immutable multi-version index (`New`, `LogicalNode`, `DataObject`, `CDC`, `ListLogicalNodes`, `Search`), explicit-profile selection (no "latest wins"; ambiguity → `ErrAmbiguousReference`), thread-safe reads, and `Diff`.
9. `resolver/`: recursive `Resolve` / `ResolveAttribute` / `ResolveCDC` / `ResolveInheritance` (e.g. `MMXU→Hz→MV→mag→FLOAT32`), cycle/conflict detection, provenance retained.
10. `reference/`: `Parse` / `Validate` / `Normalize` / `Format` matching the `go-iec61850` ref format exactly; distinguish syntax validity vs model validity vs device presence.

## Phase 3 — Semantic & ontology (M3)

*(Depends on Phase 2.)*

11. `semantic/`: typed quantity/phase/unit descriptors (`Describe`, `FindMeasurements`, `Match`, `NormalizeUnit`), auditable mapping rules, override hooks, confidence/ambiguity markers; separate unit *hints* from device values.
12. `ontology/`: small deterministic concept graph (`BuildGraph`, `Relations`, `FindConcepts`, `ExportJSON`), no RDF/OWL deps; keep adapter seam for a future `go-ot-ontology`.

## Phase 4 — Generator, CLI, docs (M4)

*(Depends on Phase 3.)*

13. `internal/generator/`: deterministic (`gofmt`) Go/JSON/doc output with provenance headers, `--check` mode, license-policy gate.
14. `cmd/nsdctl/`: `inspect`, `validate`, `dependencies`, `model show|list|search`, `diff`, `generate go|docs`, `sources check` — text + JSON output, nonzero exit on error.
15. Write the REQUIREMENTS §12 `docs/*` set and `examples/`.

## Phase 5 — CI, monitor, integration (M5)

*(CI can start early; monitor depends on `nsdctl sources check`.)*

16. `.github/workflows/ci.yml`: `go test ./... -race`, `go vet`, fmt, lint, deterministic generator check — zero IEC downloads.
17. `.github/workflows/nsd-monitor.yml`: weekly `nsdctl sources check` comparing official metadata to `nsd.lock.json`, dedup-aware issue creation, monitor states (`publication-announced` … `adopted`).
18. Provide a thin `go-iec61850` adapter example (consumed there, not here) proving dependency direction.

## Relevant files

- `REQUIREMENTS.md` — authoritative spec; §4 layout, §5 API sketch, §5.4 errors, §6 component specs, §13 milestones, §16 open decisions.
- `go.mod`, `.gitignore` — module init + IEC-source exclusion (new).
- `model/errors.go` — sentinel + typed errors `*ResolutionError` / `*ValidationError` / `*SourceError`.
- `reference/parser.go` — must mirror `go-iec61850` `LD/LN.DO.DA[FC]` (`LD`, `LN`, `Path []string`, `FC`).
- `sources/index.json`, `sources/nsd.lock.json` — permitted metadata only.
- `testdata/` — synthetic fixtures only; never real IEC content.

## Verification

1. `go build ./... && go vet ./... && go test ./... -race` all green using synthetic fixtures.
2. ZIP/XML safety unit tests: path traversal, duplicate entries, decompression bomb, oversized/deeply-nested input rejected.
3. Resolver tests for nested chains (`MMXU→Hz→MV→mag→FLOAT32`, `PNV→phsA→cVal→mag→f`) + ambiguity/cycle errors.
4. `reference.Parse` round-trip equals `go-iec61850` ParseRef semantics for shared fixtures.
5. Determinism: generator produces byte-identical output across runs for a locked input; `nsdctl generate --check` passes.
6. Confirm no IEC-derived/large artifacts tracked by Git; `ci.yml` run requires no network/credentials.

## Decisions

- Synthetic fixtures are the primary dev/test input; real NSD integration tests are local/optional and never enter Git/CI.
- Single Go module, multiple packages (no per-part submodules).
- `semantic/` starts IEC-specific with an adapter seam toward a future `go-ot-ontology`.

## Open decisions to resolve (REQUIREMENTS §16)

1. Which exact official IEC NSD package URLs/schema revisions are accessible and permitted for scripted retrieval? (Blocks real data, **not** synthetic-fixture dev.)
2. What generated output may legally be committed/distributed under the IEC Code Components EULA? Is attribution required in generated code? (Default: local-only.)
3. Can a compatible 7-410:2026 NSD be obtained, or should the initial hydro profile pin an older official NSD?
4. XSD validation approach — **recommended:** pure stdlib `encoding/xml` + manual structural validation (no CGO/deps) vs a vetted XSD adapter.
5. Canonical object-reference parser — match `go-iec61850.ParseRef` format exactly; share semantics without cyclic imports or breaking API changes.
6. Semantic quantities — **recommended:** start IEC-specific in `semantic/` with an adapter seam to a future `go-ot-ontology`, rather than hosting them there from day one.

## Further considerations

1. **First-handoff scope:** A) full M0–M5 end-to-end skeleton, or B) M0–M2 (importer + catalog + resolver) as a solid, well-tested core first. *Recommend B.*
2. **XSD validation:** pure stdlib (recommended) vs XSD adapter — see open decision 4.
3. **Progress tracking:** consider committing a per-milestone checklist matching the existing `go-iec61850` repo's `PLAN.md`/`PROGRESS.md` style.
