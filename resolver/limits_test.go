// SPDX-License-Identifier: MIT

package resolver_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/reference"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// nested returns a namespace with class L, whose data object D has an
// attribute a of a constructed type T1: depth types of fanout members
// each, the members of the last one basic.
func nested(fanout, depth int) string {
	var b strings.Builder
	b.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="F" version="2026"><BasicTypes><BasicType name="B"/></BasicTypes><ConstructedAttributes>`)
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, `<ConstructedAttribute name="T%d">`, i)
		for j := range fanout {
			if i == depth {
				fmt.Fprintf(&b, `<SubDataAttribute name="m%d" type="B" typeKind="BASIC"/>`, j)
			} else {
				fmt.Fprintf(&b, `<SubDataAttribute name="m%d" type="T%d" typeKind="CONSTRUCTED"/>`, j, i+1)
			}
		}
		b.WriteString(`</ConstructedAttribute>`)
	}
	b.WriteString(`</ConstructedAttributes><CDCs><CDC name="C"><DataAttribute name="a" fc="ST" type="T1" typeKind="CONSTRUCTED"/></CDC></CDCs>` +
		`<LNClasses><LNClass name="L"><DataObject name="D" type="C"/></LNClass></LNClasses></NS>`)
	return b.String()
}

func nestedResolver(t *testing.T, fanout, depth int) *resolver.Resolver {
	t.Helper()
	ns, _, err := nsd.ParseNamespace(strings.NewReader(nested(fanout, depth)))
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
	return resolver.New(p)
}

// Leaves visits at most 100 000 elements. The depth limit does not
// bound an expansion: a few types of a few members each multiply.
func TestLeavesExpansionLimit(t *testing.T) {
	// Six types of eight members: 262 144 leaves from a document of 4 KiB.
	r := nestedResolver(t, 8, 6)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	leaves, err := r.Leaves("L", nil, "")
	runtime.ReadMemStats(&after)
	var re *model.ResolutionError
	if leaves != nil || !errors.Is(err, model.ErrLimitExceeded) || !errors.As(err, &re) ||
		!strings.HasPrefix(re.Path, "L.D.a.m") || re.Reason != "expansion of more than 100000 elements" {
		t.Fatalf("%d leaves, error = %v, want ErrLimitExceeded", len(leaves), err)
	}
	// An expansion to the limit allocates about 1.2 GiB here, most of it
	// released while it runs; the whole expansion would take three times
	// as much.
	if got := after.TotalAlloc - before.TotalAlloc; got > 2<<30 {
		t.Errorf("a refused expansion allocated %d MiB, budget 2048 MiB", got>>20)
	}

	// Two types less are within the limit: 4096 leaves.
	leaves, err = nestedResolver(t, 8, 4).Leaves("L", nil, "")
	if err != nil || len(leaves) != 4096 || leaves[4095].Path() != "L.D.a.m7.m7.m7.m7" {
		t.Errorf("four types of eight members: %d leaves, error %v", len(leaves), err)
	}
}

// Leaves follows at most DefaultMaxDepth elements: the class, the data
// object, the attribute and the members of nested types.
func TestLeavesDepthLimit(t *testing.T) {
	// A path through d nested types of one member each has d+3 elements.
	leaves, err := nestedResolver(t, 1, resolver.DefaultMaxDepth-3).Leaves("L", nil, "")
	if err != nil || len(leaves) != 1 || len(leaves[0].Steps) != resolver.DefaultMaxDepth {
		t.Fatalf("a path of %d elements: %d leaves, error %v", resolver.DefaultMaxDepth, len(leaves), err)
	}
	for _, depth := range []int{resolver.DefaultMaxDepth - 2, 40, 60} {
		leaves, err := nestedResolver(t, 1, depth).Leaves("L", nil, "")
		var re *model.ResolutionError
		if leaves != nil || !errors.Is(err, model.ErrLimitExceeded) || !errors.As(err, &re) || re.Reason != "expansion deeper than 32 elements" {
			t.Errorf("a path of %d elements: error = %v, want ErrLimitExceeded", depth+3, err)
		}
	}
	// Three members per type: the limit on the depth is reached first,
	// after 30 types, long before the limit on the elements.
	if _, err := nestedResolver(t, 3, 40).Leaves("L", nil, ""); !errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "deeper than 32") {
		t.Errorf("40 types of three members: error = %v, want the depth limit", err)
	}
}

// cdcChain returns a namespace with class L, whose data object D is of
// CDC C1: depth CDCs, each with fanout sub data objects of the next one,
// the last with one data attribute of FC ST.
func cdcChain(fanout, depth int) string {
	var b strings.Builder
	b.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="F" version="2026"><BasicTypes><BasicType name="B"/></BasicTypes><CDCs>`)
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, `<CDC name="C%d">`, i)
		if i == depth {
			b.WriteString(`<DataAttribute name="a" fc="ST" type="B" typeKind="BASIC"/>`)
		} else {
			for j := range fanout {
				fmt.Fprintf(&b, `<SubDataObject name="s%d" type="C%d"/>`, j, i+1)
			}
		}
		b.WriteString(`</CDC>`)
	}
	b.WriteString(`</CDCs><LNClasses><LNClass name="L"><DataObject name="D" type="C1"/></LNClass></LNClasses></NS>`)
	return b.String()
}

// countingLookup counts the CDC lookups of a resolver and refuses every
// lookup after the first limit.
type countingLookup struct {
	resolver.Lookup
	lookups, limit int
}

var errLookupLimit = errors.New("lookup limit of the test reached")

func (l *countingLookup) CDCVariantsFrom(from model.NamespaceID, name string) ([]*model.CommonDataClass, error) {
	l.lookups++
	if l.lookups > l.limit {
		return nil, errLookupLimit
	}
	return l.Lookup.CDCVariantsFrom(from, name)
}

// A path that ends at a data object and is asked with an FC must have an
// attribute with that FC in its CDC or in a CDC below it. Each CDC is
// examined once per lookup, however many sub data objects refer to it:
// the data object and every CDC of the chain take one CDC lookup each.
func TestFCSearchExaminesEachCDCOnce(t *testing.T) {
	for _, tc := range []struct {
		fanout, depth int
		fc            model.FunctionalConstraint
	}{
		{4, 1, model.FCST}, {4, 1, model.FCMX},
		{4, 2, model.FCST}, {4, 2, model.FCMX},
		{4, 3, model.FCMX},
		{4, 30, model.FCST}, {4, 30, model.FCMX}, {4, 30, model.FCCO},
		{1, 30, model.FCMX},
		{16, 200, model.FCMX},
	} {
		ns, _, err := nsd.ParseNamespace(strings.NewReader(cdcChain(tc.fanout, tc.depth)))
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
		l := &countingLookup{Lookup: p, limit: 10000}
		res, err := resolver.New(l).ResolvePath("L", []string{"D"}, tc.fc)
		name := fmt.Sprintf("%d levels of %d sub data objects, FC %s", tc.depth, tc.fanout, tc.fc)
		var re *model.ResolutionError
		switch {
		case tc.fc == model.FCST && (err != nil || res.Path() != "L.D"):
			t.Errorf("%s: error = %v, want L.D", name, err)
		case tc.fc != model.FCST && (res != nil || !errors.Is(err, model.ErrUnknownAttribute) || !errors.As(err, &re) ||
			re.Reason != "no data attribute with functional constraint "+string(tc.fc)):
			t.Errorf("%s: error = %v, want ErrUnknownAttribute", name, err)
		}
		if l.lookups > tc.depth+1 {
			t.Errorf("%s: %d CDC lookups, want at most %d (each CDC once)", name, l.lookups, tc.depth+1)
		}
	}
}

// Every class of the embedded catalog is far below the limit.
func TestLeavesOfTheEmbeddedCatalog(t *testing.T) {
	p, err := official.Extended()
	if err != nil {
		t.Fatal(err)
	}
	r := resolver.New(p)
	most, name := 0, ""
	for _, ln := range p.ListLogicalNodes() {
		// Expanding every class takes 15 s under the race detector: there
		// the largest one stands for all. The run without it covers every
		// class.
		if raceDetector && ln.Name != "MHAI" {
			continue
		}
		leaves, err := r.In(ln.Source).Leaves(ln.Name, nil, "")
		if errors.Is(err, model.ErrLimitExceeded) {
			t.Errorf("%s (%s): %v", ln.Name, ln.Source, err)
		}
		if len(leaves) > most {
			most, name = len(leaves), ln.Name
		}
	}
	// SECURITY.md and the comment of Leaves quote this figure.
	if most != 11043 || name != "MHAI" {
		t.Errorf("the largest class is %s with %d leaves; the documents say MHAI with 11043", name, most)
	}
}

// fcByDefinition is what the FC search of ResolvePath answers: whether
// the CDC (any of its variants), or the CDC of one of its sub data
// objects at any depth, has a data attribute with the FC, or the FC is
// CO and a variant has service parameters (is controllable). It follows
// every sub data object as often as it occurs, which the nesting of the
// embedded catalog allows.
func fcByDefinition(l resolver.Lookup, from model.NamespaceID, name string, fc model.FunctionalConstraint, depth int) (bool, error) {
	if depth > resolver.DefaultMaxDepth {
		return false, errors.New("CDCs nested deeper than the embedded catalog nests them")
	}
	variants, err := l.CDCVariantsFrom(from, name)
	if err != nil {
		return false, err
	}
	for _, c := range variants {
		for i := range c.DataAttributes {
			if c.DataAttributes[i].FC == fc {
				return true, nil
			}
		}
		if fc == model.FCCO && len(c.ServiceParameters) > 0 {
			return true, nil
		}
	}
	for _, s := range variants[0].SubDataObjects {
		if ok, err := fcByDefinition(l, variants[0].Source, s.Type, fc, depth+1); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// For every data object and sub data object of the embedded catalog and
// every functional constraint, a path asked with the FC resolves exactly
// when the definition says so: 103 813 of the 399 819 combinations, as
// before each CDC was examined once per lookup.
func TestFCSearchOfTheEmbeddedCatalog(t *testing.T) {
	p, err := official.Extended()
	if err != nil {
		t.Fatal(err)
	}
	fcs := []model.FunctionalConstraint{
		model.FCST, model.FCMX, model.FCSP, model.FCSV, model.FCCF, model.FCDC, model.FCSG, model.FCSE, model.FCSR, model.FCOR, model.FCBL, model.FCEX,
		model.FCXX, model.FCCO, model.FCUS, model.FCMS, model.FCRP, model.FCBR, model.FCLG, model.FCGO, model.FCGS,
	}
	r := resolver.New(p)
	combinations, resolved := 0, 0
	for _, ln := range p.ListLogicalNodes() {
		// Every class takes 12 s under the race detector: there the
		// largest one stands for all, as in TestLeavesOfTheEmbeddedCatalog.
		if raceDetector && ln.Name != "MHAI" {
			continue
		}
		rl := r.In(ln.Source)
		dos, err := p.DataObjectsIn(ln.Source, ln.Name)
		if err != nil {
			t.Fatal(err)
		}
		for _, ido := range dos {
			do, err := rl.ResolvePath(ln.Name, []string{ido.Name}, "")
			if errors.Is(err, model.ErrUnknownCDC) {
				continue // HeatRtePrc and CRtePrc of DCCT (7-420): a CDC the profile does not define
			}
			if err != nil {
				t.Fatalf("%s.%s: %v", ln.Name, ido.Name, err)
			}
			paths := [][]string{{ido.Name}}
			for _, sdo := range do.CDC.SubDataObjects {
				paths = append(paths, []string{ido.Name, sdo.Name})
			}
			for _, path := range paths {
				base, err := rl.ResolvePath(ln.Name, path, "")
				if err != nil {
					t.Fatalf("%s.%v: %v", ln.Name, path, err)
				}
				for _, fc := range fcs {
					want, err := fcByDefinition(p, base.CDC.Source, base.CDC.Name, fc, 0)
					if err != nil {
						t.Fatalf("%s [%s]: %v", base.Path(), fc, err)
					}
					combinations++
					if want {
						resolved++
					}
					res, err := rl.ResolvePath(ln.Name, path, fc)
					switch {
					case want && (err != nil || res.Path() != base.Path()):
						t.Errorf("%s [%s]: error = %v, want the path", base.Path(), fc, err)
					case !want && !errors.Is(err, model.ErrUnknownAttribute):
						t.Errorf("%s [%s]: error = %v, want ErrUnknownAttribute", base.Path(), fc, err)
					}
				}
			}
		}
	}
	if !raceDetector && (combinations != 399819 || resolved != 103813) {
		t.Errorf("%d of %d combinations resolve, want 103813 of 399819", resolved, combinations)
	}
}

// The zero and nil Resolution have no steps: every accessor returns its
// zero result.
func TestResolutionZeroValue(t *testing.T) {
	var none *resolver.Resolution
	for name, r := range map[string]*resolver.Resolution{"zero": {}, "nil": none} {
		if l := r.Leaf(); l.Name != "" || l.Kind != "" || l.Type != "" || l.DeclaredIn != "" || len(l.Variants) != 0 {
			t.Errorf("%s: leaf %+v", name, l)
		}
		if r.Path() != "" || r.DataObjectPath() != "" || r.AttributePath() != "" || r.Type() != "" || r.TypeKind() != "" || r.FC() != "" ||
			r.IsAttribute() || r.Description() != "" || r.Doc() != (model.Doc{}) {
			t.Errorf("%s: accessors of a resolution without steps are not empty", name)
		}
		if ref := r.ClassRef(); ref.LNClass != "" || len(ref.Path) != 0 || ref.FC != "" {
			t.Errorf("%s: class reference %+v, want %+v", name, ref, reference.ClassRef{})
		}
	}
}
