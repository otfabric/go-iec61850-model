// SPDX-License-Identifier: MIT

// Package catalog indexes parsed namespaces for lookup, search and
// comparison.
//
// A [Catalog] holds any number of namespaces, including several
// releases of the same standard, and never prefers one release over
// another: a lookup whose name more than one loaded namespace defines
// fails with a [*model.AmbiguityError] instead of guessing.
//
// A [Profile] is an explicit, consistent selection: at most one release
// per standard, with every declared dependency satisfied. Lookups on a
// profile are unambiguous as long as its namespaces do not define the
// same name twice; [Profile.Diagnostics] reports such conflicts and
// type references the profile cannot resolve.
//
//	c, err := catalog.New(pkg72, pkg73, pkg74)
//	p, err := c.Profile(model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 5})
//	ln, err := p.LogicalNode("MMXU")
//	dos, err := p.DataObjects("MMXU") // including inherited data objects
//
// [Diff] compares two sources (namespaces, packages, profiles) and
// reports added, removed and changed definitions.
//
// Catalogs and profiles are immutable and safe for concurrent use. The
// definitions they return are shared and must not be modified.
package catalog
