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
// The embedded data (catalog_gen.json, catalog_gen.go) is generated from
// the packages pinned in sources/nsd.lock.json with "make nsd-publish";
// do not edit it. It is derived from IEC code components and published
// under the IEC Code Components license: see NOTICE.md in this directory
// for the attribution, license and copyright notices.
//
// # Profiles
//
// Definitions keep their namespace. Two profiles are embedded:
//
//   - default: IEC 61850-7-2, 7-3, 7-4, 7-410 and 7-420, the preferred
//     sources for current models. Lookups are unambiguous except FSCH,
//     which IEC 61850-7-410 redefines (as a deprecated class).
//   - extended: the default profile plus the technical reports 90-3,
//     90-4, 90-6, 90-9, 90-11, 90-14 and 90-21. IEC TR 61850-90-9 defines
//     DBAT, DSTO, SBAT and storage base classes that IEC 61850-7-420 took
//     over; such names are ambiguous unqualified and are looked up with
//     their namespace (catalog LogicalNodeIn, resolver In).
//
// Nothing is merged or given precedence implicitly; catalog Overlaps
// compares definitions that share a name.
package official
