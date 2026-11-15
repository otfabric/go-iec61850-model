# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in `go-iec61850-model`, please report
it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please email **security@otfabric.io**, or open a private advisory on
GitHub (the [Security](https://github.com/otfabric/go-iec61850-model/security)
tab → **Advisories** → **Report a vulnerability**), with:

- A description of the vulnerability
- Steps to reproduce the issue, ideally a minimal program or test input
- The affected version(s)
- Any potential impact you've identified

We will acknowledge your report within 48 hours and aim to provide a fix or
mitigation within 7 days for critical issues.

## Supported Versions

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | Best effort |

## Security Considerations

### NSD Input Is Untrusted

NSD documents and archives may come from downloads, mirrors or users. The
importer treats them as untrusted:

- **Documents** are bounded in size, element depth and token count, and in
  the number of findings they may carry. DOCTYPE declarations are rejected,
  which rules out entity expansion and external entities.
- **Archives** are bounded in entry count, total uncompressed size and
  compression ratio. The entry count is applied before the directory of an
  archive is read. Declared sizes are checked but not trusted: the actual
  decompressed size is enforced while reading, and a declared compressed size
  that the archive cannot hold is rejected. Absolute names, `..`, backslashes,
  drive letters, NUL bytes, duplicate names (case-insensitive), symlinks and
  other non-regular entries are rejected. Archives inside archives are opened
  one level deep.
- **Names** with a control character are rejected, and the generator writes
  every model string into generated source as an identifier, as a quoted
  string constant or as a comment that stays on its line.
- **Nothing is written to disk** and nothing is fetched from the network.

Lower the bounds when you accept input from parties you do not trust, and
treat every string of a loaded model (names, documentation texts) as untrusted
before you put it into HTML, SQL or shell commands.

### Limits

Every limit below fails with an error that matches `model.ErrLimitExceeded`
(the instance and SCL readers also match `model.ErrInvalidInstance`), unless
the row says otherwise. A limit with an option is a default; the others are
fixed. An option that is not a
positive number is rejected before anything is read, with an error that names
the option.

| What | Limit | Option | Applies to | Test |
| --- | --- | --- | --- | --- |
| Size of one XML document: NS, NSDoc, service namespace, XSD schema, manifest | 32 MiB | `nsd.WithMaxDocumentSize` | `nsd.Parse`, `ParseNamespace`, `ParseSchema`, `ReadManifest`, and every document of an archive, a directory or a file | `TestDocumentLimitBudgets`, `TestDocumentSizeLimitForSchemasAndManifests` |
| Element nesting of a document | 64 levels | `nsd.WithMaxDepth` | the same | `TestDocumentLimitBoundaries` |
| XML tokens of a document; every attribute counts as one | 10 000 000 | `nsd.WithMaxTokens` | the same | `TestTokenLimitCountsAttributes` |
| Findings (diagnostics) of a document | 10 000 | fixed | `nsd.Parse`, `ParseNamespace` and the loaders | `TestFindingLimit` |
| Findings of a package, over all its documents | 100 000 | fixed | `nsd.ReadZIP`, `LoadZIP`, `LoadFile`, `LoadDir` | `TestPackageFindingLimit` |
| Name, attribute value or namespace URI quoted in the path or the message of a finding | 256 bytes, then cut with `...` | fixed | every finding of the importer and of `nsd.Validate` | `TestBudgetLongNameWithManyChildren` |
| Elements below the root of a manifest | 10 000 | fixed | `nsd.ReadManifest` and manifests in packages | `TestManifestElementLimit` |
| Element paths of a manifest, together | 1 MiB | fixed | the same | `TestManifestPathLimit` |
| Size of an archive file, and uncompressed size of its entries together, nested archives included | 256 MiB | `nsd.WithMaxArchiveSize` | `nsd.ReadZIP`, `LoadZIP`, `LoadFile`, `LoadDir` | `TestArchiveLimitBudgets`, `TestArchiveLimitBoundaries` |
| Entries of an archive, nested archives included. Counted before the directory is read, from the central directory headers in the archive bytes, so the headers of an archive stored inside without compression count as well | 1024 | `nsd.WithMaxArchiveEntries` | the same | `TestArchiveEntryLimitBeforeTheDirectoryIsRead` |
| Ratio of uncompressed to compressed size of an entry above 1 MiB. A declared compressed size larger than the archive is rejected (`model.ErrInvalidNSD`) | 200 | `nsd.WithMaxCompressionRatio` | the same | `TestCompressionRatioWithDeclaredSizes` |
| Archives inside archives | one level | fixed | the same | `TestReadZIPNested` |
| Doc set (JSON, gzip or NSDoc form), after decompression | 32 MiB | fixed; `nsd.ReadDocSets` and `LoadDocSets` take no options | `nsd.ReadDocSets`, `LoadDocSets`, `IEC61850_MODEL_DOCS` | `TestDocSetFormats` |
| Instance in JSON form: size, names and values of all levels together, nesting | 4 MiB, 100 000, 32 levels | fixed (`presence.MaxInstanceBytes`, `MaxInstanceNames`, `MaxDepth`) | `presence.ReadInstance` | `TestReadInstance_Errors` |
| Nesting the resolver follows | 32 elements | fixed (`resolver.DefaultMaxDepth`) | `resolver.Resolver.Leaves` | `TestLeavesDepthLimit` |
| Model elements one expansion visits, leaves and the structures above them | 100 000 | fixed | `resolver.Resolver.Leaves` | `TestLeavesExpansionLimit`, `TestExpansionLimitBoundary` |
| CDCs the FC search examines: a path that ends at a data object, asked with an FC, needs an attribute with that FC in its CDC or a CDC of its sub data objects. Not an error: the work grows with the number of CDCs below the data object | each CDC once per lookup | fixed | `resolver.Resolver.ResolvePath`, `Resolve`, `ResolveString`, `ResolveClass`, and `Leaves` with an FC | `TestFCSearchExaminesEachCDCOnce`, `TestFCSearchOfTheEmbeddedCatalog` |
| SCL file of `nsdctl model check`: size; names and values of the instance built from its type templates; nesting of the types | 64 MiB, 100 000, 32 levels | fixed | `nsdctl model check --scl` | `TestSCLLimits`, `TestSCLInstanceNameLimit` |
| Source registry and lock file: what is read of each; a longer file is not valid JSON (`model.ErrInvalidNSD`) | 4 MiB | fixed | `nsdctl sources`, `nsdctl generate --publish` | `TestLoadSizeLimit` |
| One downloaded package | 256 MiB | fixed | `nsdctl sources fetch` | `TestFetchSizeLimitBoundary` |

What the limits mean for memory, measured with Go 1.27 on arm64:

- Importing a document allocates about 40 to 900 bytes per byte of document
  in total, depending on how short its elements are, and most of it is
  released during the import. A namespace of 32 MiB in the shape of the
  official ones (classes of twenty data objects) took 2.7 s and 1.5 GiB of
  resident memory, and the returned model kept 350 MiB. The most expensive
  shape is a document of the shortest elements inside one CDC: 4 MiB of them
  took 1.5 s and 2.5 GiB before the document was refused for its findings,
  and the default limits admit five times as many (10 000 000 tokens), which
  would take about 12 GiB. The largest official namespaces are a few MiB. A
  service that accepts NSD files from others should set `WithMaxDocumentSize`,
  `WithMaxTokens` and `WithMaxArchiveSize` to what it expects, not leave the
  defaults: with a limit of 100 000 tokens the most expensive shape took 130
  MiB.
- `Leaves` allocates 6 to 25 KiB per element it visits: 91 MiB in 60 ms for
  MHAI, the largest class of the embedded catalog (11 043 leaves, 15 543
  elements), and up to 2.5 GiB in total, most of it released while it runs,
  for an expansion that reaches the limit.
- The first use of `official` decodes the embedded catalog: 33 MiB allocated
  in 40 ms, 12 MiB kept. No input is involved.

The limits are tested with what a refused call allocated, not only with its
error (`nsd/budget_test.go`, `nsd/archive_limits_test.go`,
`nsd/manifest_limits_test.go`, `resolver/limits_test.go`), and the fuzz
targets of the importer run with an allocation budget; see
[TESTING.md](TESTING.md).

### Only Official Sources

Model definitions decide how references and values are interpreted. Load NSD
packages only from official IEC sources and record their SHA-256 digests (see
[docs/nsd-sources.md](docs/nsd-sources.md)); `model.Provenance` carries the
digest of every loaded document and archive.

### Dependencies

The module uses the Go standard library only. `make check` runs `govulncheck`;
vulnerabilities reported in the standard library are fixed by building with a
patched Go release.
