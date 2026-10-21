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

## The license of the packages

Every official package declares the license `www.iec.ch/CCv1`, the IEC Code
Components End-user Licence Agreement (SPDX `IEC-Code-Components-EULA`), and
carries a copyright notice in its `Manifest.xml` and NS documents. As
summarized from the SPDX copy of the agreement (the iec.ch page itself refuses
automated access):

- redistributed software must retain the copyright notice, the list of
  conditions and the disclaimer;
- each code component must be attributed to IEC, naming the standard it is
  taken from;
- IEC's name may not be used to promote derived products without permission;
- the components come without warranty.

The importer reads the notice and license of every namespace
(`model.Namespace.Copyright`, `nsd.Manifest.License`), and `nsdctl generate`
writes them, with the attribution, into `NOTICE.md` next to every output.

## Status of the decision

Whether IEC-derived generated output is committed or published is a decision
for the project owners, to be recorded here, and per namespace in the lock
(`"redistribution": "permitted"`). Until then:

- `nsdctl generate` marks its output local-only and adds a `LOCAL-ONLY.md`
  notice; `--publish` is refused unless the lock file records
  `"redistribution": "permitted"` for every source namespace (see
  [generation.md](generation.md));
- no IEC-derived catalog ships with this module;
- the regular CI never downloads official packages; the separate
  `nsd.yml` workflow downloads them into `$RUNNER_TEMP`, without caches or
  artifacts, and deletes them at the end;
- downloaded packages live in the git-ignored `.nsd-cache/` or outside the
  repository (`nsdctl sources fetch` refuses anything else), generated output
  in the git-ignored `.generated/`.

## Purchased standards

PDF editions of the standards may be consulted to understand modeling intent.
They are never an input of the code, never committed and never quoted at
length in code comments or documentation.
