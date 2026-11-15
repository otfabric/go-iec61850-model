// SPDX-License-Identifier: MIT

package resolver

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// The expansion limit at its boundary, with a limit small enough to
// count by hand. MMXU.Hz of the fixtures is an MV: the data object and
// its seven data attributes, of which mag (a constructed type with two
// members), units (two members) and range are structures or leaves.
func TestExpansionLimitBoundary(t *testing.T) {
	r := New(fixtures.Profile(t))
	if r.maxExpansion != 100000 || r.In(fixtures.ID74).maxExpansion != 100000 {
		t.Fatalf("limit %d, want 100000 for New and In", r.maxExpansion)
	}
	all, err := r.Leaves("MMXU", []string{"Hz"}, "")
	if err != nil {
		t.Fatal(err)
	}
	// Count what the expansion visits: every element on the way to a
	// leaf, once.
	visited := map[string]bool{}
	for _, l := range all {
		for i := 2; i <= len(l.Steps); i++ {
			visited[joinSteps(l.Steps[:i], nil)] = true
		}
	}
	n := len(visited)
	if n <= len(all) || len(all) < 2 {
		t.Fatalf("%d leaves, %d elements: the fixture has no structure", len(all), n)
	}
	for _, tc := range []struct {
		limit int
		ok    bool
	}{{0, false}, {1, false}, {n - 1, false}, {n, true}, {n + 1, true}} {
		lr := *r
		lr.maxExpansion = tc.limit
		leaves, err := lr.Leaves("MMXU", []string{"Hz"}, "")
		switch {
		case tc.ok && (err != nil || len(leaves) != len(all)):
			t.Errorf("limit %d for %d elements: %d leaves, error %v", tc.limit, n, len(leaves), err)
		case !tc.ok && (!errors.Is(err, model.ErrLimitExceeded) || leaves != nil):
			t.Errorf("limit %d for %d elements: error = %v, want ErrLimitExceeded", tc.limit, n, err)
		}
	}
	// The limit is one budget for all data objects of a class.
	class, err := r.Leaves("MMXU", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	lr := *r
	lr.maxExpansion = n
	if _, err := lr.Leaves("MMXU", nil, ""); len(class) <= len(all) || !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("the class with the budget of one data object: error = %v, want ErrLimitExceeded", err)
	}
}

// The limit counts the structures as well as the leaves: an expansion
// that ends in types without members returns nothing and is bounded all
// the same.
func TestExpansionLimitCountsStructures(t *testing.T) {
	// Class L, data object D, attribute a of type T1: three members of
	// type T2, which has three members of type T3, which has none.
	const doc = `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="F" version="2026"><ConstructedAttributes>` +
		`<ConstructedAttribute name="T1"><SubDataAttribute name="x" type="T2" typeKind="CONSTRUCTED"/><SubDataAttribute name="y" type="T2" typeKind="CONSTRUCTED"/><SubDataAttribute name="z" type="T2" typeKind="CONSTRUCTED"/></ConstructedAttribute>` +
		`<ConstructedAttribute name="T2"><SubDataAttribute name="x" type="T3" typeKind="CONSTRUCTED"/><SubDataAttribute name="y" type="T3" typeKind="CONSTRUCTED"/><SubDataAttribute name="z" type="T3" typeKind="CONSTRUCTED"/></ConstructedAttribute>` +
		`<ConstructedAttribute name="T3"/></ConstructedAttributes>` +
		`<CDCs><CDC name="C"><DataAttribute name="a" fc="ST" type="T1" typeKind="CONSTRUCTED"/></CDC></CDCs>` +
		`<LNClasses><LNClass name="L"><DataObject name="D" type="C"/></LNClass></LNClasses></NS>`
	ns, _, err := nsd.ParseNamespace(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	c, err := catalog.New(ns)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Profile(ns.ID)
	if err != nil {
		t.Fatal(err)
	}
	// D, a, three members of T1 and nine of T2: 14 elements, no leaf.
	for _, tc := range []struct {
		path  []string
		limit int
		ok    bool
	}{
		{nil, 13, false}, {nil, 14, true}, {nil, 15, true},
		{[]string{"D"}, 13, false}, {[]string{"D"}, 14, true},
		{[]string{"D", "a"}, 12, false}, {[]string{"D", "a"}, 13, true},
		{[]string{"D", "a", "x"}, 3, false}, {[]string{"D", "a", "x"}, 4, true},
		{[]string{"D", "a", "x", "y"}, 0, false}, {[]string{"D", "a", "x", "y"}, 1, true},
	} {
		r := New(p)
		r.maxExpansion = tc.limit
		leaves, err := r.Leaves("L", tc.path, "")
		switch {
		case tc.ok && (err != nil || len(leaves) != 0):
			t.Errorf("%v with a limit of %d: %d leaves, error %v", tc.path, tc.limit, len(leaves), err)
		case !tc.ok && !errors.Is(err, model.ErrLimitExceeded):
			t.Errorf("%v with a limit of %d: error = %v, want ErrLimitExceeded", tc.path, tc.limit, err)
		}
	}
}
