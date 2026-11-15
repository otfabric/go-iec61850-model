// SPDX-License-Identifier: MIT

package official

import "testing"

// Benchmarks of the embedded catalog (make bench). Each checks its
// result and reports allocations.

// BenchmarkCatalogColdLoad measures what the first call of Catalog
// costs: decompressing and decoding the embedded namespaces and
// building the catalog. Later calls return the same catalog.
func BenchmarkCatalogColdLoad(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(catalogJSON)))
	for b.Loop() {
		c, err := load(catalogJSON)
		if err != nil {
			b.Fatal(err)
		}
		if got := len(c.IDs()); got != 28 {
			b.Fatalf("%d namespaces, want 28", got)
		}
	}
}

// BenchmarkProfile measures building a profile from the loaded catalog:
// what the first call of Open costs for a name, without the load.
func BenchmarkProfile(b *testing.B) {
	c, err := Catalog()
	if err != nil {
		b.Fatal(err)
	}
	for _, name := range []string{"default", "extended"} {
		def, ok := Lookup(name)
		if !ok {
			b.Fatalf("no profile %s", name)
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				p, err := c.Profile(def.Namespaces...)
				if err != nil {
					b.Fatal(err)
				}
				if got := len(p.Namespaces()); got != len(def.Namespaces) {
					b.Fatalf("%d namespaces, want %d", got, len(def.Namespaces))
				}
			}
		})
	}
}
