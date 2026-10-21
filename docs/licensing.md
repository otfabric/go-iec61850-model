# Licensing of IEC source material

This repository is MIT-licensed, except the generated package `names/`, which
is derived from IEC code components and published under the IEC Code
Components license (see [Decision](#decision)). The MIT license covers the code
written for the repository:
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
- One IEC-derived artifact, under a recorded decision: the generated Go package
  [`names/`](../names), with the attribution and notices in `names/NOTICE.md`
  (see [Decision](#decision)).

## What stays local

Official NSD packages are **inputs you obtain and load yourself**. Treat
everything derived from them as local-only unless a recorded decision (see
[Decision](#decision)) says otherwise:

- the archives, XML and documentation texts themselves;
- catalogs, JSON exports, generated Go code and generated documentation built
  from them;
- complete lists of identifiers extracted from them.

Generating code or documentation from NSD files can create derivative material
protected by copyright. Deleting the source XML afterwards does not change
that. Apart from the `names` package, do not commit such output, publish it in
CI caches, Actions artifacts or release archives, or paste it into issues.

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

## Decision

**2026-10-21 — redistribution permitted for the generated names package.**
The project owners decided that Go code generated from the official packages
may be committed and published, under the IEC Code Components license, with
the attribution and notices it requires:

- The lock marks the 12 NSD namespaces the package is generated from as
  `"redistribution": "permitted"` (7-2, 7-3, 7-4, 7-410, 7-420 and the technical
  reports 90-3, 90-4, 90-6, 90-9, 90-11, 90-14, 90-21). The 8-1 service
  namespace and the 7-7 schema package contribute nothing to the output and stay
  `local-only`.
- The committed package [`names/`](../names) is generated with
  `nsdctl generate go --publish`, which refuses unless every source namespace is
  permitted and its digest matches the lock. Its header attributes the content to
  IEC and lists every source namespace with its digest; `names/NOTICE.md`
  carries, per namespace, the attribution, the license `www.iec.ch/CCv1` and the
  copyright notice from the NSD document, as the license requires.
- The decision covers that package only: identifiers, types and structure.
  Documentation texts (NSDoc) are not part of it, and the docs and JSON exports
  stay local-only.

The official packages themselves are still never committed or redistributed.

## Purchased standards

PDF editions of the standards may be consulted to understand modeling intent.
They are never an input of the code, never committed and never quoted at
length in code comments or documentation.
