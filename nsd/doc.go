// SPDX-License-Identifier: MIT

// Package nsd imports IEC 61850 namespace definition (NSD) files, the
// machine-readable form of IEC 61850-7-2, -7-3, -7-4 and their domain
// extensions defined by IEC 61850-7-7.
//
// [Parse] reads one XML document: a namespace definition (NS), a
// documentation file (NSDoc), or a service namespace, which is
// recognized but not modeled yet. [LoadZIP], [ReadZIP], [LoadFile] and
// [LoadDir] read whole packages and apply the documentation texts to
// the namespaces they belong to. [ReadManifest] reads an
// IECManifest.xml package manifest.
//
// # Input safety
//
// Inputs are untrusted. Every document (namespace, documentation,
// schema or manifest) is bounded in size, nesting depth and token
// count, and DOCTYPE declarations are rejected, which also rules out
// entity expansion. Archives are bounded in entry count, total
// uncompressed size and compression ratio; absolute names, path
// traversal, backslashes, duplicate entries (compared case-insensitively)
// and non-regular entries are rejected. Archives nested inside archives
// are opened one level deep. Nothing is written to disk.
//
// Besides the limits the options set there are fixed ones: a document
// carries at most 10 000 findings and a package 100 000, a manifest has
// at most 10 000 elements, and a finding quotes at most 256 bytes of a
// name. Exceeding a limit fails with [model.ErrLimitExceeded].
// SECURITY.md of the module lists every limit with the memory it means.
//
// # Diagnostics
//
// The importer does not drop data silently. Elements and attributes it
// does not model are reported as warnings with their XML path; missing
// required attributes, invalid values (a literal value is a 32-bit
// integer), names with control characters and duplicate definitions are
// errors. A document with errors is rejected with a
// [*model.ValidationError] listing every finding; [WithStrict] makes
// warnings fatal too.
//
// Validation is structural and uses the standard library only: the
// importer checks the elements and attributes it models and the rules
// in [Validate], but it does not run an XSD validator.
//
// # Source material
//
// NSD files and their documentation texts are IEC content. This
// package reads them from files or readers supplied by the caller and
// never downloads them; see docs/licensing.md for what may be
// redistributed.
package nsd
