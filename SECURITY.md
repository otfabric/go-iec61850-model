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

- **Documents** are bounded in size (`WithMaxDocumentSize`, default 32 MiB),
  element depth (`WithMaxDepth`, 64) and token count (`WithMaxTokens`).
  DOCTYPE declarations are rejected, which rules out entity expansion and
  external entities.
- **Archives** are bounded in entry count (`WithMaxArchiveEntries`), total
  uncompressed size (`WithMaxArchiveSize`, 256 MiB) and compression ratio
  (`WithMaxCompressionRatio`); declared sizes are checked but not trusted, the
  actual decompressed size is enforced while reading. Absolute names, `..`,
  backslashes, drive letters, NUL bytes, duplicate names (case-insensitive),
  symlinks and other non-regular entries are rejected. Archives inside
  archives are opened one level deep.
- **Nothing is written to disk** and nothing is fetched from the network.

Lower the bounds when you accept input from parties you do not trust, and
treat every string of a loaded model (names, documentation texts) as untrusted
before you put it into HTML, SQL or shell commands.

### Only Official Sources

Model definitions decide how references and values are interpreted. Load NSD
packages only from official IEC sources and record their SHA-256 digests (see
[docs/nsd-sources.md](docs/nsd-sources.md)); `model.Provenance` carries the
digest of every loaded document and archive.

### Dependencies

The module uses the Go standard library only. `make check` runs `govulncheck`;
vulnerabilities reported in the standard library are fixed by building with a
patched Go release.
