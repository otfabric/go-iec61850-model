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
package the project covers. Names and release labels come from the IEC
catalog extract the project was given; none has been checked against an
official listing yet (`"verified": false`), because the IEC listing pages are
not publicly retrievable. Release labels such as `2007B6`, `2019A4` and
`2024A4` are namespace version, revision and release, not publication dates.

| Tier | Part | Package | Adds |
| --- | --- | --- | --- |
| required | 7-2 | `IEC_61850-7-2.NSD.2007B6.light` | Abstract services, basic types, FCs, presence conditions |
| required | 7-3 | `IEC_61850-7-3.NSD.2007B6.Light` | CDCs, constructed attributes, enumerations |
| required | 7-4 | `IEC_61850-7-4.NSD.2007B6.Light` | Logical node classes and data objects |
| required | 7-410 | `IEC_61850-7-410.NSD.2024A4.Light.zip` | Hydropower and generation |
| required | 7-420 | `IEC_61850-7-420.NSD.2019A4.Light.zip` | DER, inverters, storage |
| recommended | 90-9 | `IEC_TR_61850-90-9.NSD.2018A3.Light.zip` | Electrical energy storage, batteries |
| recommended | 90-14 | `IEC_TR_61850-90-14.NSD.2020A3.Light.zip` | Power converters, FACTS, HVDC |
| optional | 90-6 | `IEC_61850-90-6.NSD.2018B.light.zip` | Distribution automation, FLISR |
| optional | 90-3 | `IEC_TR_61850-90-3.NSD.2015B.Light.zip` | Condition monitoring, asset health |
| optional | 90-4 | `IEC_TR_61850-90-4.NSD.2018A.Light.zip` | Network engineering, bridges, clocks |
| optional | 90-11 | `IEC_TR_61850-90-11.NSD.2019A2.light.zip` | Logic modeling |
| optional | 90-21 | `IEC_TR_61850-90-21.NSD.2022A2.Light` | Travelling-wave fault location |
| optional | 8-1 | `IEC_61850-8-1.NSD.2003A4.Light` | MMS mapping definitions and FCs |
| optional (schema) | 7-7 | `IEC_61850-7-7.2017.NSD.2017A.full.zip` | NSD XML schema (2017B6 is announced in the 2025 corrigendum) |

Open points to verify: whether a newer 7-4 namespace (a `2025A3` label
appears in public listings) should replace `2007B6`, and the official 7-7
`2017B6` package.

## Workflow with the downloaded packages

Keep the packages in a directory outside the repository, e.g. `~/nsd`.
File names may keep or drop `.zip` and differ in case; packages without an
extension are recognized as ZIP archives by content.

```bash
nsdctl sources check --packages ~/nsd                 # every tier; --tiers required for the core five
nsdctl validate ~/nsd                                  # all namespaces with their dependencies
nsdctl schema ~/nsd/IEC_61850-7-7.2017.NSD.2017A.full.zip   # schema constructs the importer does not model
nsdctl sources lock --packages ~/nsd                   # review the lock entries
nsdctl sources lock --packages ~/nsd --write           # pin digests and statuses into sources/nsd.lock.json
make official NSD_DIR=~/nsd                            # integration test against the real packages
```

`sources check` reports, per package, `ok`, `missing`, `release-mismatch`
(the package carries another namespace release than its name says),
`invalid` (it cannot be loaded) or `no-schema`. Loading the whole directory
works as one source: each archive keeps its own manifest, and later releases
satisfy dependencies that name an earlier release of the same version and
revision.

## Coverage

The importer accepts NSD documents in the XML namespace
`http://www.iec.ch/61850/2016/NSD`. If an official package declares another
namespace, the import fails with `model.ErrUnsupportedSchema`; after checking
that the structure is compatible, accept it with
`nsd.WithSchemaNamespaces(...)` and report it so support can be added.

## Lock file

[`sources/nsd.lock.json`](../sources/nsd.lock.json) records, per adopted
namespace release, only metadata that may be committed: standard and
publication, namespace ID, official source location, retrieval time, SHA-256,
package kind, dependencies and validation status. It is empty until the first
official package has been validated.

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
