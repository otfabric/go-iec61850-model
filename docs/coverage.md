# Coverage

Coverage is reported in four stages, never merged into one claim:

| Stage | Meaning | Where |
| --- | --- | --- |
| Published | The standard edition exists | [nsd-sources.md](nsd-sources.md) |
| Available | An official NSD package for it can be obtained | `sources/nsd.lock.json` status `nsd-available` |
| Parsed | The importer reads it; definitions imported, unsupported constructs reported | `nsdctl inspect`, `nsdctl validate --strict` |
| Validated | A profile with it has no errors and its data objects resolve | `coverage.md` from `nsdctl generate docs`; lock status `nsd-validated` / `adopted` |

## Current state

No official NSD package has been parsed yet (milestone M0 is open), so this
module claims support for no namespace release. The table in
[nsd-sources.md](nsd-sources.md) is the authoritative status.

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
