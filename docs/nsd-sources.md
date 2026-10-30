# NSD sources

The only normative inputs of this library are **official IEC artifacts**:

- IEC NSD packages (Light, and Full where license and access allow);
- official package manifests (`IECManifest.xml`);
- the official IEC 61850-7-7 schemas;
- the notices and license terms attached to those code components.

Third-party mirrors, GitHub copies, vendor mappings and unofficial NSD files
are not authoritative. Starting points:

- [IEC 61850 Code Components](https://iec61850.dvl.iec.ch/61850-resources/61850-code-components/)
- [IEC webstore](https://webstore.iec.ch/)

Download URLs, package formats, access requirements and the EULA must be
verified before any retrieval is automated. An official download may require
credentials or be restricted even when its URL is public.

## Loading packages

The library never downloads anything. Obtain the packages, keep them outside
the repository, and load them:

```go
pkg, err := nsd.LoadZIP("/srv/nsd/IEC_61850-7-4.zip") // or LoadDir, LoadFile, ReadZIP
```

Each loaded namespace records its provenance (`model.Namespace.Provenance`):
the source (`archive.zip!entry.nsd`), the SHA-256 of the document and of the
enclosing archive, and the XML namespace of the document. A package's manifest
is available from `Package.Manifest()`.

## Tracked packages

[`sources/index.json`](../sources/index.json) lists every code component
package the project covers, with its official download location. All 14 were
verified on 2026-10-21 against `https://assets.iec.ch/public/tc57/` (the older
`www.iec.ch/public/TC57/supportdocuments/` paths refuse access). File names are
case-sensitive there. Release labels such as `2007B6` are namespace version,
revision and release, not publication dates.

| Tier | Part | Package | Adds |
| --- | --- | --- | --- |
| required | 7-2 | `IEC_61850-7-2.NSD.2007B6.Light.zip` | Abstract services, basic types, FCs, presence conditions |
| required | 7-3 | `IEC_61850-7-3.NSD.2007B6.Light.zip` | CDCs, constructed attributes, enumerations |
| required | 7-4 | `IEC_61850-7-4.NSD.2007B6.Light.zip` | Logical node classes and data objects |
| required | 7-410 | `IEC_61850-7-410.NSD.2024A4.Light.zip` | Hydropower and generation |
| required | 7-420 | `IEC_61850-7-420.NSD.2019A4.Light.zip` | DER, inverters, storage |
| recommended | 90-9 | `IEC_TR_61850-90-9.NSD.2018A3.Light.zip` | Electrical energy storage (overlaps 7-420, see below) |
| recommended | 90-14 | `IEC_TR_61850-90-14.NSD.2020A3.Light.zip` | Power converters, FACTS, HVDC |
| optional | 90-6 | `IEC_TR_61850-90-6.NSD.2018B.Light.zip` | Distribution automation, FLISR |
| optional | 90-3 | `IEC_TR_61850-90-3.NSD.2015B.Light.zip` | Condition monitoring, asset health |
| optional | 90-4 | `IEC_TR_61850-90-4.NSD.2018A.Light.zip` | Network engineering, bridges, clocks |
| optional | 90-11 | `IEC_TR_61850-90-11.NSD.2019A2.light.zip` | Logic modeling |
| optional | 90-21 | `IEC_TR_61850-90-21.NSD.2022A2.Light.zip` | Travelling-wave fault location |
| optional | 8-1 | `IEC_61850-8-1.NSD.2003A4.Light.zip` | MMS mapping (a service namespace, recognized, not modeled yet) |
| optional (schema) | 7-7 | `IEC_61850-7-7.2017.NSD.2017A.full.zip` | NSD XML schema |

The packages declare NSD format version 2017B6; a `2017B6` 7-7 package is not
published at the official location, so 7-7 stays at `2017A`.

## Downloading, pinning and generating

Downloaded packages are temporary inputs and are never committed. `nsdctl
sources fetch` downloads only from the official HTTPS locations in the
registry, refuses redirects to other hosts, bounds the size, writes
atomically, verifies every file against the SHA-256 pinned in
`sources/nsd.lock.json`, and refuses a destination inside the repository other
than the git-ignored `.nsd-cache/`.

```bash
make nsd-fetch            # download into .nsd-cache/, verified against the lock
make nsd-check            # every tracked package present and loadable; validate
make official             # integration test against the packages
make nsd-generate         # Go, docs and JSON into .generated/ (git-ignored, local-only)
make nsd-generate-check   # .generated/ is up to date
make nsd-publish          # regenerate the committed names/, official/ and docs/reference/
make nsd-clean            # remove .nsd-cache/ and .generated/
```

`NSD_CACHE`, `GENERATED_DIR`, `NSD_TIERS`, `NSD_PROFILE` (repeatable IDs) and
`GO_PACKAGE` override the defaults.

When a package is added or changes upstream on purpose:

```bash
make build                                 # nsdctl is a nested module
./bin/nsdctl sources fetch --allow-unpinned  # first download of a new package
make nsd-check official                    # review
make nsd-lock                              # pin digests and statuses (metadata only)
```

The workflow [`.github/workflows/nsd.yml`](../.github/workflows/nsd.yml) runs
the same steps on pull requests and pushes that touch the importer or the
sources, weekly, and on demand. It works in `$RUNNER_TEMP`, uses no caches and
uploads no artifacts, checks that the committed `names/`, `official/` and
`docs/reference/` match the pinned packages, deletes everything at the end, writes a metadata-only
summary, and opens (or closes) a tracking issue when the weekly run fails, for
instance because a package changed upstream. The regular CI never downloads
IEC packages.

## What the official packages contain

Findings from importing the 14 packages, reflected in the importer:

- **Manifest** `Manifest.xml` (namespace `http://www.iec.ch/CC/2017/IECManifest`)
  with copyright notice and license `www.iec.ch/CCv1`; each NS document also
  carries a `Copyright` block. Light packages ship the grammar files
  `NSD.xsd`, `IECManifest.xsd` and `IECCopyright.xsd`.
- **Abstract classes listed among concrete ones** (`AbstractLNClass` inside
  `LNClasses`).
- **Extension classes** (`isExtension="true"`): 7-420 and 90-6 add data objects
  to `MMXU`, 90-3 to `ZBAT`, 7-410 to `KPMP` and others. They declare that
  they extend the class of the same name and are applied to it; this is the
  only case in which two definitions combine. Two full definitions of a name
  (DBAT in 90-9 and 7-420) stay separate (see
  [namespaces.md](namespaces.md#namespace-identity)).
- **CDC variants**: settings CDCs such as `SPG` are defined three times with
  `variant` SP, SG and SE; data objects name only the CDC, and the FC selects
  the variant.
- **Per-FC attribute copies**: 7-410's `RST` declares `setMag`, `_setMag` and
  `__setMag` for SP, SG and SE; on a device each is `setMag`.
- **Service namespaces**: 8-1 contains a `ServiceNS` (`.snsd`) and an
  `ApplicableServiceNS` (`.AppNS`), no NS document.
- **Nested copies**: the 7-410 package contains its NSD both directly and in a
  nested copy of itself; identical copies are imported once.
- **Technical report IDs** look like `(Tr)IEC 61850-90-9` or
  `(Tr)IEC61850-90-6`.
- **Dependency types** `dependsOn`, `includes` and `extends`.

Upstream gaps, reported as warnings by `nsdctl validate`:

- **Overlapping definitions**: 90-9 (2018A3) and 7-420 (2019A4) both define
  `DBAT`, `DSTO`, `SBAT`, the storage base classes and two enumerations; 7-4 and
  7-410 both define `FSCH`; 90-14 and 7-410 both define `SFLW`. A profile with
  both namespaces is ambiguous for those names: select one with `--profile` /
  `NSD_PROFILE`, or leave 90-9 out when 7-420 2019A4 is loaded.
- **References no loaded namespace defines**: 7-4 `PhyComAddrL2`,
  `PhyComAddrUDP` (probably 8-1 service types) and `EnumDA`; 7-420 `SCR` and four
  enumerations; 7-410 presence conditions `MFsg`, `MFnsg`. Paths through them do
  not resolve; everything else does.

## Coverage

The project reports four states separately and never claims support for a
release that has not reached the last one. State on 2026-10-21, from the
official packages pinned in `sources/nsd.lock.json`:

| Namespace | Available | Parsed | Profile validates | Data objects resolving to leaf types |
| --- | --- | --- | --- | --- |
| IEC 61850-7-2:2007B6 | yes | yes | yes | n/a |
| IEC 61850-7-3:2007B6 | yes | yes | yes | n/a |
| IEC 61850-7-4:2007B6 | yes | yes | yes (8 warnings) | 1168 / 1192 |
| IEC 61850-7-410:2024A4 | yes | yes | yes (3 warnings) | 956 / 960 |
| IEC 61850-7-420:2019A4 | yes | yes | yes (6 warnings) | 429 / 536 |
| IEC TR 61850-90-9:2018A3 | yes | yes | yes (9 overlaps with 7-420) | 0 / 95 together with 7-420 |
| IEC TR 61850-90-14:2020A3 | yes | yes | yes | 273 / 278 |
| IEC TR 61850-90-6:2018B | yes | yes | yes | 155 / 155 |
| IEC TR 61850-90-3:2015B | yes | yes | yes | 222 / 228 |
| IEC TR 61850-90-4:2018A | yes | yes | yes | 109 / 109 |
| IEC TR 61850-90-11:2019A2 | yes | yes | yes | 8 / 8 |
| IEC TR 61850-90-21:2022A2 | yes | yes | yes | 16 / 16 |
| IEC 61850-8-1:2003A4 | yes | recognized (ServiceNS) | n/a | n/a |
| IEC 61850-7-7:2017A (schema) | yes | inventoried | n/a | n/a |

Counts come from `coverage.md` of `make nsd-generate` over all packages in one
profile; the shortfalls are the upstream gaps above.

The importer accepts NSD documents in the XML namespace
`http://www.iec.ch/61850/2016/NSD`. If an official package declares another
namespace, the import fails with `model.ErrUnsupportedSchema`; after checking
that the structure is compatible, accept it with
`nsd.WithSchemaNamespaces(...)` and report it so support can be added.

## Lock file

[`sources/nsd.lock.json`](../sources/nsd.lock.json) records, per adopted
namespace release, only metadata that may be committed: standard and
publication, namespace ID, official source location, retrieval time, SHA-256,
package kind, package file, dependencies, validation status and the
redistribution decision. All 14 tracked packages are pinned; the 12 NSD
namespaces the committed output is generated from are `permitted`, the 8-1
service namespace and the 7-7 schema `local-only` (see
[licensing.md](licensing.md#decision)).

Lock entry fields: `standard`, `publication`, `namespace` (canonical ID),
`content` (`light`/`full`), `source` (official HTTPS URL on an `iec.ch` host),
`retrievedAt` (RFC 3339), `sha256`, `dependencies`, `status`
(`publication-announced`, `nsd-available`, `nsd-validated`,
`adoption-blocked`, `adopted`), `redistribution` (`local-only` by default, or
`permitted` after a documented licensing decision) and `notes`.
`nsdctl sources check` validates both files; with `--packages DIR` it
compares the digests of local packages with the lock. It contacts no server:
automated discovery waits for verified official endpoints (M0) and belongs to
the weekly monitor (M5).

The registry is JSON rather than the YAML sketched in REQUIREMENTS §4, so the
module needs no YAML dependency. [`sources/index.json`](../sources/index.json)
lists the official discovery locations.
