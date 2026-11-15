// SPDX-License-Identifier: MIT

package resolver_test

import (
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// Benchmarks of the resolver on the embedded default profile (make
// bench). Each checks its result and reports allocations.

func benchResolver(b *testing.B) *resolver.Resolver {
	b.Helper()
	p, err := official.Default()
	if err != nil {
		b.Fatal(err)
	}
	return resolver.New(p)
}

// BenchmarkResolvePath measures the resolution of one path: a short
// one, and one through a sub data object and two constructed types.
func BenchmarkResolvePath(b *testing.B) {
	r := benchResolver(b)
	for _, tc := range []struct {
		name, class string
		path        []string
		fc          model.FunctionalConstraint
		typ         string
	}{
		{"MMXU.Hz.mag.f", "MMXU", []string{"Hz", "mag", "f"}, model.FCMX, "FLOAT32"},
		{"MMXU.PhV.phsA.cVal.mag.f", "MMXU", []string{"PhV", "phsA", "cVal", "mag", "f"}, model.FCMX, "FLOAT32"},
		{"XCBR.Pos.stVal", "XCBR", []string{"Pos", "stVal"}, model.FCST, "Dbpos"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				res, err := r.ResolvePath(tc.class, tc.path, tc.fc)
				if err != nil {
					b.Fatal(err)
				}
				if res.Type() != tc.typ || len(res.Steps) != len(tc.path)+1 {
					b.Fatalf("%s resolves to %s in %d steps", tc.name, res.Type(), len(res.Steps))
				}
			}
		})
	}
}

// BenchmarkLeaves measures the expansion of a class into its leaves:
// MMXU, and MHAI, the largest class of the embedded catalog.
func BenchmarkLeaves(b *testing.B) {
	r := benchResolver(b)
	for _, tc := range []struct {
		class string
		min   int
	}{{"MMXU", 1000}, {"MHAI", 11043}} {
		b.Run(tc.class, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				leaves, err := r.Leaves(tc.class, nil, "")
				if err != nil {
					b.Fatal(err)
				}
				if len(leaves) < tc.min {
					b.Fatalf("%s has %d leaves, want at least %d", tc.class, len(leaves), tc.min)
				}
			}
		})
	}
}
