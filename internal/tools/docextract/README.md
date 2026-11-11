# docextract

Maintainer tool that builds a **local** documentation doc set
(`nsd.DocSet`, JSON form) from IEC 61850 PDFs the maintainer is licensed to
use. It is Python, outside the Go build, and no Go package depends on it.

The output contains text extracted from the standards. It is written to
`official/docs_gen.json.gz`, which `.gitignore` excludes: **never commit it,
attach it to a release or copy text from it into the repository** (see
[docs/licensing.md](../../../docs/licensing.md)).

## Use

```sh
make docs-extract SPECS_DIR=/path/to/IEC61850   # writes official/docs_gen.json.gz
make test-docs                                  # tests against it (-tags iecdocs)
make build-docs                                 # binaries with it embedded
```

Its texts take precedence over the public OT Fabric texts (`texts/`), which
fill every key it leaves out. Or, without a rebuild, point a program at the
file:

```sh
IEC61850_MODEL_DOCS=$PWD/official/docs_gen.json.gz nsdctl ...
```

Requirements: `python3` with PyMuPDF (`pymupdf`) and poppler's `pdftotext`.
Inputs, found by name in `SPECS_DIR`: `IEC 61850-7-2-2010.pdf`,
`IEC 61850-7-3-2010.pdf`, `IEC 61850-7-4-2010.pdf`,
`IEC 61850-7-410-2012.pdf`, `IEC 61850-7-420-2009.pdf`. Page dumps are cached
in `.docs-work/` (git-ignored); `.docs-work/report.txt` lists every key with
its confidence (high: exact table row; medium: CDC-agnostic text, subclass
majority or alias; low: generic data object semantics) and source.

## Method

The elements and their keys come from the committed catalog
(`official/catalog_gen.json`), so every key names an element of an embedded
namespace (`LN:MMXU`, `DO:MMXU.TotW`, `CDC:MV`, `DA:MV.mag`, `FC:ST`, ...).
The texts come from the tables of the PDFs (`find_tables`) and from the layout
text: LN class tables and headings, 7-3 Table 64 and the CDC headings, 7-3
clause 6, 7-2 Tables 2 and 20, 7-3 Table 1 (through an OTFabric alias table of
the Edition 2.1 condition names) and the abbreviation clauses.

The PDFs are Edition 2 (2010/2012) and 7-420 Edition 1 (2009), the NSDs
Edition 2.1. Elements new in 2.1 (abstract classes, renamed 7-410 classes,
most of 7-420 Edition 2, enumerations) have no text, and a text can be stale
where the meaning changed. `make test-docs` enforces per-namespace coverage
floors and that every key matches an element.
