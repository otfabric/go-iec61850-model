// SPDX-License-Identifier: MIT

package official_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/resolver"
	"github.com/otfabric/go-iec61850-model/semantic"
)

var (
	ns7420 = model.NamespaceID{Standard: "IEC 61850-7-420"}
	ns909  = model.NamespaceID{Standard: "(Tr)IEC 61850-90-9"}
)

func TestEmbeddedCatalog(t *testing.T) {
	c, err := official.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(c.Namespaces()); got != 12 || len(official.Namespaces()) != 12 {
		t.Fatalf("%d embedded namespaces, want 12", got)
	}
	for _, ns := range c.Namespaces() {
		if ns.Copyright.LicenseURI != "www.iec.ch/CCv1" || ns.Provenance.SHA256 == "" {
			t.Errorf("%s lacks its license or provenance", ns.ID)
		}
	}
	names := []string{}
	for _, p := range official.Profiles() {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "default,extended" {
		t.Errorf("profiles %v", names)
	}
	if _, ok := official.Lookup("nope"); ok {
		t.Error("unknown profile found")
	}
	if _, err := official.Open("nope"); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown profile: %v", err)
	}
}

// The default profile holds IEC 61850-7-2, 7-3, 7-4, 7-410 and 7-420,
// without IEC TR 61850-90-9, and its storage classes are unambiguous.
func TestDefaultProfile(t *testing.T) {
	p, err := official.Default()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, id := range p.IDs() {
		ids = append(ids, id.String())
	}
	if strings.Join(ids, ", ") != "IEC 61850-7-2:2007B6, IEC 61850-7-3:2007B6, IEC 61850-7-4:2007B6, IEC 61850-7-410:2024A4, IEC 61850-7-420:2019A4" {
		t.Fatalf("default profile %s", ids)
	}
	for _, name := range []string{"DBAT", "DSTO", "SBAT", "MMXU", "MMTR", "DINV"} {
		ln, err := p.LogicalNode(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if strings.HasPrefix(name, "D") && name != "DINV" || name == "SBAT" {
			if ln.Source.Standard != "IEC 61850-7-420" {
				t.Errorf("%s comes from %s, want IEC 61850-7-420", name, ln.Source)
			}
		}
	}
	// FSCH is the one name the default profile defines twice: IEC
	// 61850-7-410 redefines it as a deprecated class.
	conflicts := 0
	for _, d := range p.Diagnostics() {
		if d.Code == "conflicting-definition" {
			conflicts++
			if !strings.Contains(d.Message, `"FSCH"`) {
				t.Errorf("unexpected overlap in the default profile: %s", d.Message)
			}
		}
	}
	if conflicts != 1 {
		t.Errorf("%d overlaps in the default profile, want FSCH only", conflicts)
	}
	if again, _ := official.Default(); again != p {
		t.Error("profiles are built once and shared")
	}
	r := resolver.New(p)
	for _, ref := range []string{"LD0/MMXU1.Hz.mag.f[MX]", "MMXU.PhV.phsA.cVal.mag.f", "DBAT.BatSt.stVal"} {
		if _, err := r.ResolveString(ref); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
	if d, err := semantic.Describe(p, "MMXU.TotW"); err != nil || d.Quantity != semantic.QuantityActivePower {
		t.Errorf("describe: %+v %v", d, err)
	}
}

// The extended profile preserves both definitions: unqualified lookups
// are ambiguous, qualified ones return the requested definition, and
// nothing is merged.
func TestExtendedProfile(t *testing.T) {
	p, err := official.Extended()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"DBAT", "DSTO", "SBAT"} {
		_, err := p.LogicalNode(name)
		var ae *model.AmbiguityError
		if !errors.As(err, &ae) || !errors.Is(err, model.ErrAmbiguousReference) {
			t.Errorf("%s unqualified: %v, want an ambiguity error", name, err)
		}
		a, errA := p.LogicalNodeIn(ns7420, name)
		b, errB := p.LogicalNodeIn(ns909, name)
		if errA != nil || errB != nil || a == b || a.Source.Standard != "IEC 61850-7-420" || b.Source.Standard != "(Tr)IEC 61850-90-9" {
			t.Errorf("%s qualified: %v %v", name, errA, errB)
		}
	}
	// Each definition keeps its own structure: references inside a
	// definition resolve in its own namespace first.
	dosA, errA := p.DataObjectsIn(ns7420, "DBAT")
	dosB, errB := p.DataObjectsIn(ns909, "DBAT")
	if errA != nil || errB != nil || len(dosA) == len(dosB) {
		t.Fatalf("DBAT data objects: %d (7-420) %v, %d (90-9) %v", len(dosA), errA, len(dosB), errB)
	}
	chainA, _ := p.InheritanceIn(ns7420, "DBAT")
	chainB, _ := p.InheritanceIn(ns909, "DBAT")
	if chainA[1].Source.Standard != "IEC 61850-7-420" || chainB[1].Source.Standard != "(Tr)IEC 61850-90-9" {
		t.Errorf("base classes resolve in their own namespace: %s, %s", chainA[1].Source, chainB[1].Source)
	}
	if _, err := resolver.New(p).ResolveString("DBAT.BatSt.stVal"); !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("unqualified resolution: %v", err)
	}
	if res, err := resolver.New(p).In(ns7420).ResolveString("LD0/DBAT1.BatSt.stVal[ST]"); err != nil || res.LNClass.Source.Standard != "IEC 61850-7-420" {
		t.Errorf("qualified resolution: %v", err)
	}
	if _, err := semantic.Match(p, semantic.Criteria{Quantity: semantic.QuantityFrequency}); err != nil {
		t.Errorf("matching across ambiguous classes: %v", err)
	}
}

// The overlap report classifies the definitions 90-9 and 7-420 share.
func TestOverlapReport(t *testing.T) {
	p, err := official.Extended()
	if err != nil {
		t.Fatal(err)
	}
	overlaps, err := p.Overlaps(ns7420, ns909)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]catalog.Overlap{}
	for _, o := range overlaps {
		if o.Kind == catalog.KindLogicalNode {
			byName[o.Name] = o
		}
	}
	for _, name := range []string{"DBAT", "DSTO", "SBAT"} {
		o, ok := byName[name]
		if !ok {
			t.Errorf("%s missing from the overlap report", name)
			continue
		}
		if o.Class == catalog.OverlapIdentical {
			t.Errorf("%s classified identical; the definitions differ", name)
		}
	}
	all, err := p.Overlaps(model.NamespaceID{}, model.NamespaceID{})
	if err != nil || len(all) < len(overlaps) {
		t.Errorf("all overlaps: %d %v", len(all), err)
	}
}
