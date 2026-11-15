// SPDX-License-Identifier: MIT

package nsd_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// Benchmarks of the importer (make bench). Each checks its result, so
// that a call that fails is not measured as a fast one, and reports
// bytes per second and allocations.

// BenchmarkParse measures the import of one NS document: the synthetic
// 7-4 fixture, and generated documents of 100 and 1000 classes of
// twenty data objects each.
func BenchmarkParse(b *testing.B) {
	classes := func(n int) []byte {
		var s strings.Builder
		s.WriteString(nsOpen + `<LNClasses>`)
		for i := range n {
			fmt.Fprintf(&s, `<LNClass name="C%04d">`, i)
			for j := range 20 {
				fmt.Fprintf(&s, `<DataObject name="Do%d" type="MV" presCond="O"/>`, j)
			}
			s.WriteString(`</LNClass>`)
		}
		s.WriteString(`</LNClasses></NS>`)
		return []byte(s.String())
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		classes int
	}{
		{"fixture-7-4", fixtures.Read(b, fixtures.NS74), 5},
		{"classes=100", classes(100), 100},
		{"classes=1000", classes(1000), 1000},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.data)))
			for b.Loop() {
				doc, err := nsd.Parse(bytes.NewReader(tc.data))
				if err != nil {
					b.Fatal(err)
				}
				if got := len(doc.Namespace.LNClasses); got != tc.classes {
					b.Fatalf("%d classes, want %d", got, tc.classes)
				}
			}
		})
	}
}

// BenchmarkReadZIP measures the load of an archive with the three
// synthetic namespaces, their documentation and a manifest.
func BenchmarkReadZIP(b *testing.B) {
	data := buildZIP(b, fixtureEntries(b, "pkg/")...)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		p, err := nsd.ReadZIP(bytes.NewReader(data))
		if err != nil {
			b.Fatal(err)
		}
		if got := len(p.Namespaces()); got != 3 {
			b.Fatalf("%d namespaces, want 3", got)
		}
	}
}
