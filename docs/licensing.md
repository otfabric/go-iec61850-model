# Licensing of IEC source material

This repository is MIT-licensed. The license covers the code written for it:
API contracts, importer, catalog, resolver, tests and tools. It grants nothing
for IEC standards or for the NSD files you load with the library.

## What the repository contains

- No official IEC NSD, NSDoc or XSD files, and no ZIP packages of them.
- No text from IEC standards or NSDoc files: no titles, descriptions, tables or
  presence-condition explanations.
- Small, **independently authored** test fixtures in [testdata/nsd](../testdata/nsd).
  They follow the NSD structure so the importer is exercised on realistic
  input, contain only the definitions the tests need, and use OTFabric wording
  for every text. Short identifiers that the tests need to exercise resolution
  chains (`MMXU`, `Hz`, `MV`, `mag`, `FLOAT32`) appear in them.
- OTFabric-authored labels, such as the short logical node group names returned
  by `model.LNGroup.Label`.

## What stays local

Official NSD packages are **inputs you obtain and load yourself**. Treat
everything derived from them as local-only until a licensing decision says
otherwise:

- the archives, XML and documentation texts themselves;
- catalogs, JSON exports, generated Go code and generated documentation built
  from them;
- complete lists of identifiers extracted from them.

Generating code or documentation from NSD files can create derivative material
protected by copyright. Deleting the source XML afterwards does not change
that. Do not commit such output, publish it in CI caches, Actions artifacts or
release archives, or paste it into issues.

At run time the library holds what you loaded in memory, including NSDoc texts
(`model.Doc.Title`, `model.Doc.Description`). Showing those texts to the users
of your own installation is a matter of your IEC license; publishing them is
not covered by this repository's license.

## Status of the review

The IEC Code Components license and its attribution requirements have **not
been reviewed** yet (milestone M0 in [REQUIREMENTS.md](../REQUIREMENTS.md)).
Until that review is documented here, with the applicable notices:

- `nsdctl generate` marks its output local-only and adds a `LOCAL-ONLY.md`
  notice; `--publish` is refused unless the lock file records
  `"redistribution": "permitted"` for every source namespace (see
  [generation.md](generation.md));
- no IEC-derived catalog ships with this module;
- the CI never downloads official packages.

## Purchased standards

PDF editions of the standards may be consulted to understand modeling intent.
They are never an input of the code, never committed and never quoted at
length in code comments or documentation.
