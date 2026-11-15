// SPDX-License-Identifier: MIT

package nsd_test

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// Allocation and work budgets of the importer (SECURITY.md, "Limits").
// A test that only asserts the error of a limit does not show what the
// call allocated before it returned; these tests measure it.

// allocated returns the bytes one call of fn allocates.
func allocated(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// checkLinear asserts that importing build(n) allocates at most perByte
// bytes per byte of document, and that a document of twice the size
// costs less than three times as much: the cost grows with the size of
// the document, not with its square.
//
// perByte is about twice what the shape costs here (go1.27, arm64, with
// and without the race detector). The cost per byte depends on the
// shape: encoding/xml reads every document twice, once to apply the
// limits and once into the structures, and the shorter the elements,
// the more a byte costs. TestCostOfTheShortestElements pins the most
// expensive shape.
func checkLinear(t *testing.T, n, perByte int, build func(n int) []byte, load func(data []byte) error, wantErr error) {
	t.Helper()
	var cost [2]uint64
	for i, size := range []int{n, 2 * n} {
		data := build(size)
		var err error
		cost[i] = allocated(func() { err = load(data) })
		if !errors.Is(err, wantErr) {
			t.Fatalf("size %d: error = %v, want %v", size, err, wantErr)
		}
		t.Logf("size %d: %d bytes of document, %d allocated (%d per byte)", size, len(data), cost[i], cost[i]/uint64(len(data)))
		if budget := uint64(perByte * len(data)); cost[i] > budget {
			t.Errorf("size %d: a document of %d bytes allocated %d bytes (%d per byte), budget %d per byte",
				size, len(data), cost[i], cost[i]/uint64(len(data)), perByte)
		}
	}
	if cost[1] > 3*cost[0] {
		t.Errorf("twice the document allocated %d bytes, more than three times %d", cost[1], cost[0])
	}
}

func parseBytes(data []byte) error {
	_, err := nsd.Parse(bytes.NewReader(data))
	return err
}

// A long name with many children that each have a finding: the path of
// a finding quotes at most 256 bytes of a name.
func TestBudgetLongNameWithManyChildren(t *testing.T) {
	build := func(n int) []byte {
		var b strings.Builder
		b.WriteString(nsOpen + `<Enumerations><Enumeration name="` + strings.Repeat("N", 40*n) + `">`)
		for i := range n {
			fmt.Fprintf(&b, `<Literal name="l%d" literalVal="%d" extra="1"/>`, i, i)
		}
		b.WriteString(`</Enumeration></Enumerations></NS>`)
		return []byte(b.String())
	}
	checkLinear(t, 1000, 96, build, parseBytes, nil)

	doc, err := nsd.Parse(bytes.NewReader(build(3)))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range doc.Diagnostics {
		if len(d.Path) > 2*256+64 {
			t.Errorf("path of %d bytes: %.80s", len(d.Path), d.Path)
		}
	}
}

// Many attributes in an XML namespace with a long URI: the message of
// a finding quotes at most 256 bytes of the URI.
func TestBudgetAttributesInLongNamespace(t *testing.T) {
	build := func(n int) []byte {
		var b strings.Builder
		b.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" xmlns:x="urn:` + strings.Repeat("u", 40*n) + `" id="T" version="2026"`)
		for i := range n {
			fmt.Fprintf(&b, ` x:a%d="1"`, i)
		}
		b.WriteString(`/>`)
		return []byte(b.String())
	}
	checkLinear(t, 1000, 128, build, parseBytes, nil)

	doc, err := nsd.Parse(bytes.NewReader(build(20)))
	if err != nil || len(doc.Diagnostics) != 20 {
		t.Fatalf("diagnostics %v, error %v", doc.Diagnostics, err)
	}
	if m := doc.Diagnostics[0].Message; !strings.Contains(m, `uuu...:a0`) || len(m) > 256+128 {
		t.Errorf("message of %d bytes: %.120s", len(m), m)
	}
}

// Many definitions of one CDC with distinct variants: a valid document.
func TestBudgetCDCVariants(t *testing.T) {
	build := func(n int) []byte {
		var b strings.Builder
		b.WriteString(nsOpen + `<CDCs>`)
		for i := range n {
			fmt.Fprintf(&b, `<CDC name="X" variant="v%d"/>`, i)
		}
		b.WriteString(`</CDCs></NS>`)
		return []byte(b.String())
	}
	checkLinear(t, 4000, 200, build, parseBytes, nil)
}

// Many definitions of one CDC without a variant: every one is a
// finding, and a document carries at most 10 000 of them.
func TestBudgetFindingsOfValidation(t *testing.T) {
	build := func(n int) []byte {
		return []byte(nsOpen + `<CDCs>` + strings.Repeat(`<CDC name="X"/>`, n) + `</CDCs></NS>`)
	}
	checkLinear(t, 20000, 450, build, parseBytes, model.ErrLimitExceeded)
}

// One element with many attributes the importer does not model.
func TestFindingLimit(t *testing.T) {
	build := func(n int) []byte {
		var b strings.Builder
		b.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026"`)
		for i := range n {
			fmt.Fprintf(&b, ` a%d="1"`, i)
		}
		b.WriteString(`/>`)
		return []byte(b.String())
	}
	for _, tc := range []struct {
		n  int
		ok bool
	}{{0, true}, {1, true}, {9999, true}, {10000, true}, {10001, false}, {40000, false}} {
		doc, err := nsd.Parse(bytes.NewReader(build(tc.n)), nsd.WithSource("many.nsd"))
		switch {
		case tc.ok && (err != nil || len(doc.Diagnostics) != tc.n):
			t.Errorf("%d findings: error %v", tc.n, err)
		case !tc.ok && !errors.Is(err, model.ErrLimitExceeded):
			t.Errorf("%d findings: error = %v, want ErrLimitExceeded", tc.n, err)
		case !tc.ok:
			var se *model.SourceError
			if !errors.As(err, &se) || se.Source != "many.nsd" || !strings.Contains(se.Reason, "more than 10000 findings") || !strings.Contains(se.Reason, "unknown-attribute at /NS") {
				t.Errorf("%d findings: error %v", tc.n, err)
			}
		}
	}
	checkLinear(t, 20000, 240, build, parseBytes, model.ErrLimitExceeded)

	// In strict mode the findings are errors; the limit is the same.
	if _, err := nsd.Parse(bytes.NewReader(build(10001)), nsd.WithStrict()); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("strict: error = %v, want ErrLimitExceeded", err)
	}
	var ve *model.ValidationError
	if _, err := nsd.Parse(bytes.NewReader(build(10000)), nsd.WithStrict()); !errors.As(err, &ve) || len(ve.Diagnostics) != 10000 {
		t.Errorf("strict at the limit: error = %v", err)
	}
}

// The findings of the two passes over a namespace, structure and
// validation, share the limit of a document.
func TestFindingLimitOverBothPasses(t *testing.T) {
	build := func(attrs, cdcs int) []byte {
		var b strings.Builder
		b.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026"`)
		for i := range attrs {
			fmt.Fprintf(&b, ` a%d="1"`, i)
		}
		b.WriteString(`><CDCs>` + strings.Repeat(`<CDC name="X" variant="v"/>`, cdcs+1) + `</CDCs></NS>`)
		return []byte(b.String())
	}
	var ve *model.ValidationError
	if _, err := nsd.Parse(bytes.NewReader(build(9990, 10))); !errors.As(err, &ve) || len(ve.Diagnostics) != 10000 {
		t.Errorf("10000 findings over both passes: error = %v", err)
	}
	if _, err := nsd.Parse(bytes.NewReader(build(9990, 11))); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("10001 findings over both passes: error = %v, want ErrLimitExceeded", err)
	}
	if _, err := nsd.Parse(bytes.NewReader(build(10000, 5))); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("validation after a full first pass: error = %v, want ErrLimitExceeded", err)
	}
}

// Validate itself has no limit: it works on a namespace the caller
// holds, and every finding has a bounded size.
func TestValidateKeepsEveryFinding(t *testing.T) {
	ns := &model.Namespace{CDCs: make([]model.CommonDataClass, 12001)}
	for i := range ns.CDCs {
		ns.CDCs[i].Name = "X"
	}
	// 12 000 duplicates and 12 001 definitions without a variant.
	if ds := nsd.Validate(ns); len(ds) != 24001 {
		t.Errorf("%d findings, want 24001", len(ds))
	}
	long := strings.Repeat("L", 5000)
	ns = &model.Namespace{Enumerations: []model.Enumeration{{Name: "E", Literals: []model.Literal{{Name: long, Value: 1}, {Name: "b", Value: 1}, {Name: long, Value: 2}}}}}
	ds := nsd.Validate(ns)
	if len(ds) != 2 {
		t.Fatalf("findings %v", ds)
	}
	for _, d := range ds {
		if len(d.Path)+len(d.Message) > 2*256+128 {
			t.Errorf("finding of %d bytes: %.100s", len(d.Path)+len(d.Message), d.Message)
		}
	}
}

// The most expensive document per byte is one of the shortest elements
// that decode into the largest structure: unknown children of a CDC,
// four bytes each. SECURITY.md quotes the figure.
func TestCostOfTheShortestElements(t *testing.T) {
	build := func(n int) []byte {
		return []byte(nsOpen + `<CDCs><CDC name="C">` + strings.Repeat(`<a/>`, n) + `</CDC></CDCs></NS>`)
	}
	// About 870 bytes per byte here: the budget is twice that.
	checkLinear(t, 30000, 1800, build, parseBytes, model.ErrLimitExceeded)
}

// MaxTokens counts the attributes of an element: each is a name and a
// value the decoder keeps.
func TestTokenLimitCountsAttributes(t *testing.T) {
	doc := func(attrs int) string {
		var b strings.Builder
		b.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026"`)
		for i := range attrs {
			fmt.Fprintf(&b, ` a%d="1"`, i)
		}
		b.WriteString(`></NS>`)
		return b.String()
	}
	// The root has xmlns, id, version and the extra attributes; with its
	// start and end tag that is 5 tokens without extra attributes.
	for _, tc := range []struct {
		attrs, limit int
		ok           bool
	}{{0, 5, true}, {0, 4, false}, {1, 6, true}, {1, 5, false}, {95, 100, true}, {96, 100, false}, {5000, 100, false}} {
		_, err := parse(t, doc(tc.attrs), nsd.WithMaxTokens(tc.limit))
		if tc.ok && err != nil || !tc.ok && !errors.Is(err, model.ErrLimitExceeded) {
			t.Errorf("%d attributes, limit %d: error = %v", tc.attrs, tc.limit, err)
		}
	}
	big := []byte(doc(200000))
	var err error
	cost := allocated(func() { _, err = nsd.Parse(bytes.NewReader(big), nsd.WithMaxTokens(1000)) })
	if !errors.Is(err, model.ErrLimitExceeded) {
		t.Fatalf("error = %v, want ErrLimitExceeded", err)
	}
	// The document is read whole, about five times its size while the
	// buffer grows, and the scan stops at the first element.
	if budget := uint64(120 * len(big)); cost > budget {
		t.Errorf("allocated %d bytes for a refused document of %d bytes, budget %d", cost, len(big), budget)
	}
}

// The limits of a document, each with what the refused call allocated.
// The input is far larger than the limit; the allocation stays near the
// limit, not near the input.
func TestDocumentLimitBudgets(t *testing.T) {
	elements := func(n int) []byte {
		return []byte(nsOpen + strings.Repeat(`<CDCs/>`, n) + `</NS>`)
	}
	nested := func(n int) []byte {
		return []byte(nsOpen + strings.Repeat(`<a>`, n) + strings.Repeat(`</a>`, n) + `</NS>`)
	}
	for _, tc := range []struct {
		name   string
		data   []byte
		opt    nsd.Option
		budget uint64
	}{
		// 8 MiB of input, a limit of 64 KiB: reading stops at the limit.
		{"size", elements(1 << 20), nsd.WithMaxDocumentSize(64 << 10), 1 << 20},
		// 200 000 levels, a limit of 64: the scan stops at level 65.
		{"depth", nested(200000), nsd.WithMaxDepth(64), 0},
		// 2 000 000 tokens, a limit of 1000.
		{"tokens", elements(1 << 20), nsd.WithMaxTokens(1000), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			cost := allocated(func() { _, err = nsd.Parse(bytes.NewReader(tc.data), tc.opt) })
			if !errors.Is(err, model.ErrLimitExceeded) {
				t.Fatalf("error = %v, want ErrLimitExceeded", err)
			}
			// Without a limit on the size the document is read whole
			// (about five times its size while the buffer grows) before
			// the scan stops.
			budget := tc.budget
			if budget == 0 {
				budget = 6 * uint64(len(tc.data))
			}
			if cost > budget {
				t.Errorf("allocated %d bytes for a refused document of %d bytes, budget %d", cost, len(tc.data), budget)
			}
		})
	}
}

// At the boundary of each document limit: one below, at, one above.
func TestDocumentLimitBoundaries(t *testing.T) {
	doc := nsOpen + `<CDCs><CDC name="X"/></CDCs></NS>` // depth 3, 6 tokens and 5 attributes
	size := int64(len(doc))
	for _, tc := range []struct {
		name string
		opt  nsd.Option
		ok   bool
	}{
		{"size-1", nsd.WithMaxDocumentSize(size - 1), false},
		{"size", nsd.WithMaxDocumentSize(size), true},
		{"size+1", nsd.WithMaxDocumentSize(size + 1), true},
		{"depth-1", nsd.WithMaxDepth(2), false},
		{"depth", nsd.WithMaxDepth(3), true},
		{"depth+1", nsd.WithMaxDepth(4), true},
		{"tokens-1", nsd.WithMaxTokens(11), false},
		{"tokens", nsd.WithMaxTokens(12), true},
		{"tokens+1", nsd.WithMaxTokens(13), true},
	} {
		_, err := parse(t, doc, tc.opt)
		if tc.ok && err != nil || !tc.ok && !errors.Is(err, model.ErrLimitExceeded) {
			t.Errorf("%s: error = %v", tc.name, err)
		}
	}
}
