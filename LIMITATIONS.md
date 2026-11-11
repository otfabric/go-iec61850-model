# Limitations

What go-iec61850-model does not do, and what has not been verified. Nothing in
this repository is certified: no conformance test has been run, and the
coverage below is a statement about the tracked NSD packages, not about any
device or engineering tool.

## Scope

- **No runtime.** The module is the information model only: it has no MMS,
  GOOSE, Sampled Values or SCL runtime, and reads no device.
- **Device instances.** `reference` and `resolver` validate against the
  namespaces. Whether a reference exists on a particular device is a matter of
  its SCL or online discovery, which this module does not perform.
- **Service namespaces are recognized, not modelled.** IEC 61850-8-1
  (`ServiceNS`) is identified on import and contributes nothing to a catalog,
  and the 7-7 package is inventoried as a schema
  ([docs/nsd-sources.md](docs/nsd-sources.md#coverage)).
- **No XSD validation.** The importer uses the standard library only. It
  validates what it models and reports the rest, so it does not prove that a
  document is valid against the NSD schema.
- **One NSD XML namespace by default.** Another one fails with
  `model.ErrUnsupportedSchema` unless the caller accepts it with
  `nsd.WithSchemaNamespaces`.
- **Semantic mapping is partial.** The default rules cover MMXU, MMXN and MMTR
  plus a frequency rule; a data object with no rule returns `ErrNoMapping`
  ([docs/semantic.md](docs/semantic.md)). There are no default rules for the
  7-420 or 7-410 classes.
- **No update monitor.** Nothing discovers new IEC releases: `nsdctl sources
  check` contacts no server, and the weekly `nsd.yml` run only detects packages
  that changed or disappeared at their pinned locations.

## References and resolution

- **Name rules are opt-in.** `reference.Parse` accepts names outside the
  character set and start rules of IEC 61850-7-2 §22.2, as devices use them;
  `Ref.ValidateNames` checks them.
- **Array bounds.** An array index (`phsAHar(3)`) is accepted on array
  elements only; its upper bound (`maxPts`) is device configuration and is not
  checked.
- **Control services follow 8-1.** `SBO`, `SBOw`, `Oper` and `Cancel` (FC CO)
  are the IEC 61850-8-1 mapping. Which of them a device has depends on its
  `ctlModel`, which the model does not know.
- **Presence conditions are not evaluated.** Conditional forms (`MOcond`,
  `AtLeastOne`, ...) are carried verbatim; equal instance-number widths
  (7-2 §22.2) are not checked.

## Model gaps from upstream

- Not every data object resolves to leaf types: paths through a type that no
  loaded namespace defines fail, and the importer does not invent one. The
  affected references, and the per-namespace counts, are in
  [docs/nsd-sources.md](docs/nsd-sources.md#what-the-official-packages-contain).
- No Edition 1. IEC publishes no NSD for IEC 61850-7-4:2003 (Edition 1), so
  devices that declare it in `LLN0.NamPlt.ldNs` cannot be checked against
  official definitions; `official.ForNamespace` reports `ErrUnknownNamespace`
  for them. Edition 2 (2007A) and Edition 2.1 (2007B) are embedded.
- Overlapping definitions stay ambiguous. A name that two namespaces of a
  profile define (`DBAT` in 7-420 and 90-9, for example) fails with
  `model.ErrAmbiguousSymbol` until it is looked up with its namespace.

## IEC content

- The repository ships no NSD packages and no text from the standards. The
  committed output has no documentation texts; texts are available only from
  NSDocs the caller loads, a doc set the caller supplies, or a build with the
  `iecdocs` tag over a locally generated doc set
  ([docs/licensing.md](docs/licensing.md#doc-sets)).
- The doc set extractor (`internal/tools/docextract`) reads the PDF tables
  heuristically. Coverage differs per standard (it is reported in
  `.docs-work/report.txt`): abstract classes, sub data attributes and
  extension namespaces have fewer texts, and a text the PDF layout splits
  oddly can be cut short. Texts are never invented: an element without a
  confident match stays empty.
- `catalog.Query.Text` is a case-insensitive substring match of all terms,
  not a ranked full-text search.
- Regular CI runs on independently authored synthetic fixtures. The official
  packages are exercised only by `nsd.yml` and locally (`make official`).

## Not verified

- Resolution results against an independent IEC 61850 implementation or an
  engineering tool: there is no interoperability suite in this repository.
- The package releases not pinned in `sources/nsd.lock.json`.
