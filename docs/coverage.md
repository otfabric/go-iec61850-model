# Coverage

Coverage is reported in four stages, never merged into one claim:

| Stage | Meaning | Where |
| --- | --- | --- |
| Published | The standard edition exists | [nsd-sources.md](nsd-sources.md) |
| Available | An official NSD package for it can be obtained | `sources/nsd.lock.json` status `nsd-available` |
| Parsed | The importer reads it; definitions imported, unsupported constructs reported | `nsdctl inspect`, `nsdctl validate --strict` |
| Validated | A profile with it has no errors and its data objects resolve | `coverage.md` from `nsdctl generate docs`; lock status `nsd-validated` / `adopted` |

## Current state

All 33 tracked packages are available and parsed. The 28 model namespaces
(7-2, 7-3 and 7-4 from 2007A2 to 2007B6, 7-410, 7-420 and the technical
reports) are `nsd-validated` in the lock and embedded in
[`official`](../official): validated together they give no errors, only the
upstream gaps listed in [nsd-sources.md](nsd-sources.md#what-the-official-packages-contain). The
8-1 service namespaces (2003 to 2003A4) and the 7-7 schema are
`nsd-available`: recognized, not modeled. The lock and
[nsd-sources.md](nsd-sources.md) are the authoritative status; the counts per
namespace of the extended profile are in
[docs/reference/coverage.md](reference/coverage.md).

## Producing a coverage report

```bash
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" generate docs -o ./generated-docs
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" generate json -o ./generated-json   # coverage.json
```

The report lists per namespace: classes (abstract), data objects of concrete
classes that resolve down to leaf types, CDCs, constructed types,
enumerations, mapped measurements, and the profile's errors and warnings. It
is derived from IEC files and therefore local-only; record the result as the
lock entry's status and notes instead of committing the report.

The report for the synthetic fixtures is in
[testdata/golden/docs/coverage.md](../testdata/golden/docs/coverage.md).
