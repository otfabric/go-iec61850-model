// SPDX-License-Identifier: MIT

// Package official embeds the IEC 61850 information model of the official
// IEC NSD packages tracked by this module, so that programs can look up
// classes, data objects, CDCs and types, resolve references and describe
// measurements without downloading or loading any IEC file.
//
//	p, err := official.Default() // IEC 61850-7-2, 7-3, 7-4, 7-410, 7-420
//	res, err := resolver.New(p).ResolveString("LD0/MMXU1.Hz.mag.f[MX]")
//	d, err := semantic.Describe(p, "MMXU.PNV.phsA")
//
// The embedded data (catalog_gen.json.gz, declared in catalog_gen.go) is
// generated from the packages pinned in sources/nsd.lock.json with "make
// nsd-publish"; catalog_gen.json holds the same data indented, for
// review, and is not embedded. Do not edit them. They are derived from
// IEC code components and published under the IEC Code Components
// license: see NOTICE.md in this directory for the attribution, license
// and copyright notices.
//
// # Profiles
//
// The catalog holds every NSD release IEC publishes: 7-2, 7-3 and 7-4
// from 2007A2 to 2007B6, 90-6 2018A5 and 2018B, and one release of each
// other part. Definitions keep their namespace. Three profiles are
// embedded:
//
//   - default: the newest releases of IEC 61850-7-2, 7-3, 7-4, 7-410 and
//     7-420, the preferred sources for current models. Lookups are
//     unambiguous except FSCH, which IEC 61850-7-410 redefines (as a
//     deprecated class).
//   - extended: the default profile plus the technical reports 90-3,
//     90-4, 90-6, 90-9, 90-11, 90-14 and 90-21. IEC TR 61850-90-9 defines
//     DBAT, DSTO, SBAT and storage base classes that IEC 61850-7-420 took
//     over; such names are ambiguous unqualified and are looked up with
//     their namespace (catalog LogicalNodeIn, resolver In).
//   - edition2: IEC 61850-7-2, 7-3 and 7-4 at 2007A3, Edition 2, for
//     devices that declare IEC 61850-7-4:2007 or 2007A.
//
// [ForNamespace] names the profile for the release a device declares in
// LLN0.NamPlt.ldNs; any other release can be selected by ID with
// Catalog().Profile. Definitions are never merged; catalog Overlaps
// compares definitions that share a name.
package official
