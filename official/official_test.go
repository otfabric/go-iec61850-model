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
	// Every published release: 7-2, 7-3 and 7-4 2007A2 to 2007B6, 90-6
	// 2018A5 and 2018B, and one release of each other part.
	if got := len(c.Namespaces()); got != 28 || len(official.Namespaces()) != 28 {
		t.Fatalf("%d embedded namespaces, want 28", got)
	}
	if got := c.Releases("IEC 61850-7-4"); len(got) != 6 || got[0].String() != "IEC 61850-7-4:2007A2" || got[5].String() != "IEC 61850-7-4:2007B6" {
		t.Errorf("7-4 releases = %v", got)
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
	if strings.Join(names, ",") != "default,extended,edition2" {
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

func TestForNamespace(t *testing.T) {
	for _, tc := range []struct {
		declared, profile string
		revision          string
	}{
		{"IEC 61850-7-4:2007B", "default", "B"},
		{"IEC 61850-7-4:2007B4", "default", "B"},
		{"IEC 61850-7-420:2019A", "default", "A"},
		{"(Tr)IEC 61850-90-9:2018A", "extended", "A"},
		{"IEC TR 61850-90-9:2018A3", "extended", "A"},
	} {
		p, err := official.ForNamespace(tc.declared)
		if err != nil || p.Name != tc.profile {
			t.Errorf("ForNamespace(%s) = %s, %v; want %s", tc.declared, p.Name, err, tc.profile)
			continue
		}
		want, _ := model.ParseNamespaceID(tc.declared)
		ok := false
		for _, id := range p.Namespaces {
			ok = ok || (id.Version == want.Version && id.Revision == tc.revision && strings.HasSuffix(id.Standard, want.Standard[len(want.Standard)-7:]))
		}
		if !ok {
			t.Errorf("ForNamespace(%s): profile %s holds no %s release", tc.declared, p.Name, tc.revision)
		}
	}
	// Without a revision, the oldest embedded revision of the version.
	p, err := official.ForNamespace("IEC 61850-7-4:2007")
	if err != nil {
		t.Fatal(err)
	}
	oldest := ""
	for _, q := range official.Profiles() {
		for _, id := range q.Namespaces {
			if id.Standard == "IEC 61850-7-4" && (oldest == "" || id.Revision < oldest) {
				oldest = id.Revision
			}
		}
	}
	for _, id := range p.Namespaces {
		if id.Standard == "IEC 61850-7-4" && id.Revision != oldest {
			t.Errorf("ForNamespace(IEC 61850-7-4:2007) = %s with %s, want revision %s", p.Name, id, oldest)
		}
	}
	for _, s := range []string{"IEC 61850-7-4:2003", "IEC 61850-7-4:2007B99", "IEC 61850-7-4:2007Z", "IEC 61850-7-4", "IEC 61850-7-4:x", "OTF 1:2020A"} {
		if _, err := official.ForNamespace(s); !errors.Is(err, model.ErrUnknownNamespace) {
			t.Errorf("ForNamespace(%s) error = %v", s, err)
		}
	}
}

// The edition2 profile holds Edition 2 of the core parts, 2007A3, for
// devices that declare IEC 61850-7-4:2007 or 2007A.
func TestEdition2Profile(t *testing.T) {
	p, err := official.Open("edition2")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, id := range p.IDs() {
		ids = append(ids, id.String())
	}
	if strings.Join(ids, ", ") != "IEC 61850-7-2:2007A3, IEC 61850-7-3:2007A3, IEC 61850-7-4:2007A3" {
		t.Errorf("edition2 = %v", ids)
	}
	// Edition 2.1 added data objects to MMXU and logical nodes such as XFUS.
	ed2, err := p.DataObjects("MMXU")
	if err != nil {
		t.Fatal(err)
	}
	d, err := official.Default()
	if err != nil {
		t.Fatal(err)
	}
	ed21, err := d.DataObjects("MMXU")
	if err != nil || len(ed2) >= len(ed21) {
		t.Errorf("MMXU: %d data objects in Edition 2, %d in 2.1 (%v)", len(ed2), len(ed21), err)
	}
	if _, err := p.LogicalNode("XFUS"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("XFUS in Edition 2: %v", err)
	}
	for declared, want := range map[string]string{
		"IEC 61850-7-4:2007": "edition2", "IEC 61850-7-4:2007A": "edition2", "IEC 61850-7-4:2007A2": "edition2",
		"IEC 61850-7-4:2007B": "default", "IEC 61850-7-3:2007B5": "default",
	} {
		if got, err := official.ForNamespace(declared); err != nil || got.Name != want {
			t.Errorf("ForNamespace(%s) = %s, %v; want %s", declared, got.Name, err, want)
		}
	}
}
