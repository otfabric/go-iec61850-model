// SPDX-License-Identifier: MIT

package catalog_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
)

func TestNew(t *testing.T) {
	c := fixtures.Catalog(t)
	if got := len(c.Namespaces()); got != 5 {
		t.Fatalf("%d namespaces", got)
	}
	if got := c.Releases(fixtures.Standard); len(got) != 2 || got[0] != fixtures.ID74 || got[1] != fixtures.ID74R2 {
		t.Errorf("Releases = %v", got)
	}
	if got := c.Standards(); strings.Join(got, ",") != "IEC 61850-7-2,IEC 61850-7-3,IEC 61850-7-4,OTF TEST-EXT" {
		t.Errorf("Standards = %v", got)
	}
	if ns, err := c.Namespace(fixtures.ID73); err != nil || ns.ID != fixtures.ID73 {
		t.Errorf("Namespace = %v, %v", ns, err)
	}
	if _, err := c.Namespace(model.NamespaceID{Standard: "nope"}); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown namespace error = %v", err)
	}
	// The same namespace value twice is accepted; a different value with the same ID is not.
	ns := c.Namespaces()[0]
	if _, err := catalog.New(ns, ns); err != nil {
		t.Errorf("same namespace twice: %v", err)
	}
	cp := *ns
	if _, err := catalog.New(ns, &cp); !errors.Is(err, model.ErrNamespaceConflict) {
		t.Errorf("duplicate ID error = %v", err)
	}
	if _, err := catalog.New(nil); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("nil source error = %v", err)
	}
	if _, err := catalog.New(nilNamespaces{}); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("nil namespace error = %v", err)
	}
	if _, err := catalog.New(&model.Namespace{}); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("namespace without ID error = %v", err)
	}
}

type nilNamespaces struct{}

func (nilNamespaces) Namespaces() []*model.Namespace { return []*model.Namespace{nil} }

func TestCatalogLookupsAreAmbiguousAcrossReleases(t *testing.T) {
	c := fixtures.Catalog(t)
	_, err := c.LogicalNode("MMXU")
	var ae *model.AmbiguityError
	if !errors.As(err, &ae) || !errors.Is(err, model.ErrAmbiguousReference) || len(ae.Candidates) != 2 {
		t.Fatalf("LogicalNode(MMXU) error = %v", err)
	}
	if ln, err := c.LogicalNode("DTST"); err != nil || ln.Source != fixtures.IDExt {
		t.Errorf("unique class lookup failed: %v", err)
	}
	if _, err := c.LogicalNode("XXXX"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown class error = %v", err)
	}
	if cdc, err := c.CDC("MV"); err != nil || cdc.Source != fixtures.ID73 {
		t.Errorf("CDC(MV) = %v, %v", cdc, err)
	}
	if _, err := c.CDC("NOPE"); !errors.Is(err, model.ErrUnknownCDC) {
		t.Errorf("unknown CDC error = %v", err)
	}
	if _, err := c.ConstructedAttribute("Vector"); err != nil {
		t.Error(err)
	}
	if _, err := c.Enumeration("HealthKind"); err != nil {
		t.Error(err)
	}
	if _, err := c.BasicType("FLOAT32"); err != nil {
		t.Error(err)
	}
	if _, err := c.BasicType("FLOAT99"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("unknown basic type error = %v", err)
	}
	if fc, err := c.FunctionalConstraint(model.FCMX); err != nil || fc.Abbreviation != model.FCMX {
		t.Error(err)
	}
	if p, err := c.PresenceCondition("AtLeastOne"); err != nil || p.Argument == "" {
		t.Error(err)
	}
	// Inheritance-aware lookups also work on the catalog when the names are unique.
	if do, err := c.DataObject("DTST", "Beh"); err == nil || !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("DTST inherits through an ambiguous base: %v %v", do, err)
	}
}

func TestProfileSelection(t *testing.T) {
	c := fixtures.Catalog(t)
	p, err := c.Profile(fixtures.ID74)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.String(); got != "IEC 61850-7-2:2007B1, IEC 61850-7-3:2007B1, IEC 61850-7-4:2007B1" {
		t.Errorf("profile = %s", got)
	}
	if ids := p.IDs(); len(ids) != 3 {
		t.Errorf("IDs = %v", ids)
	}
	if ln, err := p.LogicalNode("MMXU"); err != nil || ln.Source != fixtures.ID74 {
		t.Errorf("MMXU in profile: %v", err)
	}
	if ds := p.Diagnostics(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}

	// A partial ID selects the only matching release.
	if _, err := c.Profile(model.NamespaceID{Standard: "OTF TEST-EXT"}); !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("extension depends on 7-4 without release; two are loaded: %v", err)
	}
	if p, err := c.Profile(model.NamespaceID{Standard: fixtures.Standard, Release: 2}); err != nil || len(p.Namespaces()) != 3 {
		t.Errorf("partial ID with release: %v", err)
	}
	var de *model.DependencyError
	_, err = c.Profile(fixtures.IDExt)
	if !errors.As(err, &de) || len(de.Candidates) != 2 || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("ambiguous dependency error = %v", err)
	}
	if _, err := c.Profile(fixtures.IDExt, fixtures.ID74R2); err != nil {
		t.Errorf("explicit release resolves the ambiguity: %v", err)
	}
	if _, err := c.Profile(model.NamespaceID{Standard: fixtures.Standard}); !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("partial ID with two releases: %v", err)
	}
	if _, err := c.Profile(fixtures.ID74, fixtures.ID74R2); !errors.Is(err, model.ErrNamespaceConflict) {
		t.Errorf("two releases of one standard: %v", err)
	}
	if _, err := c.Profile(fixtures.ID74, fixtures.ID74); err != nil {
		t.Errorf("same ID twice: %v", err)
	}
	if _, err := c.Profile(); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("empty selection: %v", err)
	}
	if _, err := c.Profile(model.NamespaceID{Standard: "nope"}); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown namespace: %v", err)
	}
	if _, err := c.DefaultProfile(); !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("DefaultProfile with two releases: %v", err)
	}
}

func TestProfileDependencyErrors(t *testing.T) {
	a := &model.Namespace{ID: model.NamespaceID{Standard: "A", Version: "1"}, Dependencies: []model.NamespaceID{{Standard: "B", Version: "2"}}}
	b1 := &model.Namespace{ID: model.NamespaceID{Standard: "B", Version: "1"}}
	c, err := catalog.New(a, b1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Profile(a.ID); !errors.Is(err, model.ErrMissingDependency) {
		t.Errorf("missing dependency error = %v", err)
	}
	if _, err := c.Profile(a.ID, b1.ID); !errors.Is(err, model.ErrNamespaceConflict) {
		t.Errorf("incompatible selection error = %v", err)
	}
	if p, err := c.DefaultProfile(); err == nil {
		t.Errorf("default profile must fail on missing dependency: %v", p)
	}
	c2, _ := catalog.New(b1)
	if p, err := c2.DefaultProfile(); err != nil || len(p.Namespaces()) != 1 {
		t.Errorf("DefaultProfile = %v, %v", p, err)
	}
}

func TestProfileDiagnostics(t *testing.T) {
	a := &model.Namespace{
		ID:                    model.NamespaceID{Standard: "A", Version: "1"},
		BasicTypes:            []model.BasicType{{Name: "B"}},
		FunctionalConstraints: []model.FunctionalConstraintDef{{Abbreviation: "ST"}},
		PresenceConditions:    []model.PresenceConditionDef{{Name: "M"}},
		Enumerations:          []model.Enumeration{{Name: "E", InheritedFrom: "Missing"}},
		CDCs: []model.CommonDataClass{{
			Name:           "X",
			SubDataObjects: []model.DataObject{{Name: "s", Type: "NoCDC"}},
			DataAttributes: []model.DataAttribute{{Name: "a", Type: "NoType", FC: "ZZ", Presence: model.PresenceCondition{Name: "Q"}}},
		}},
		ConstructedAttributes: []model.ConstructedAttribute{{Name: "C", SubDataAttributes: []model.DataAttribute{{Name: "m", Type: "NoType"}}}},
		LNClasses: []model.LogicalNodeClass{{
			Name: "ABCD", Base: "NoBase",
			DataObjects: []model.DataObject{{Name: "D", Type: "X", UnderlyingType: "NoEnum"}},
		}},
	}
	b := &model.Namespace{ID: model.NamespaceID{Standard: "B", Version: "1"}, CDCs: []model.CommonDataClass{{Name: "X"}}, BasicTypes: []model.BasicType{{Name: "B"}}}
	for i := range a.CDCs {
		a.CDCs[i].Source = a.ID
	}
	b.CDCs[0].Source = b.ID
	c, err := catalog.New(a, b)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Profile(a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	ds := p.Diagnostics()
	for _, want := range []string{
		`CDC "X" is defined by`, `basic type "B" is defined by`,
		`base class "NoBase"`, `underlying type "NoEnum"`, `CDC "NoCDC"`, `type "NoType"`,
		`functional constraint "ZZ"`, `presence condition "Q"`, `parent enumeration "Missing"`,
	} {
		found := false
		for _, d := range ds {
			if strings.Contains(d.Message, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing diagnostic %q in %v", want, ds)
		}
	}
	if _, err := p.CDC("X"); !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("conflicting CDC lookup error = %v", err)
	}
}

func TestInheritance(t *testing.T) {
	p := fixtures.Profile(t)
	chain, err := p.Inheritance("DTST")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, ln := range chain {
		names = append(names, ln.Name)
	}
	if strings.Join(names, ">") != "DTST>StatisticsLN>DomainLN" {
		t.Errorf("chain = %v", names)
	}
	dos, err := p.DataObjects("MMXU")
	if err != nil {
		t.Fatal(err)
	}
	if len(dos) != 13 || dos[0].Name != "Beh" || dos[0].DeclaredIn.Name != "DomainLN" || dos[3].Name != "ClcExp" || dos[4].Name != "TotW" {
		t.Errorf("MMXU data objects: %d, first %s", len(dos), dos[0].Name)
	}
	do, err := p.DataObject("MMXU", "Health")
	if err != nil || do.DeclaredIn.Name != "DomainLN" || do.Type != "ENS" {
		t.Errorf("inherited DataObject = %+v, %v", do, err)
	}
	if _, err := p.DataObject("MMXU", "Nope"); !errors.Is(err, model.ErrUnknownDataObject) {
		t.Errorf("unknown DO error = %v", err)
	}
	if _, err := p.DataObject("XXXX", "Hz"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown class error = %v", err)
	}
	if _, err := p.DataObjects("XXXX"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown class error = %v", err)
	}
}

func TestInheritanceCycleAndOverride(t *testing.T) {
	ns := &model.Namespace{
		ID: model.NamespaceID{Standard: "C", Version: "1"},
		AbstractLNClasses: []model.LogicalNodeClass{
			{Name: "A", Base: "B", Abstract: true},
			{Name: "B", Base: "A", Abstract: true},
			{Name: "Root", Abstract: true, DataObjects: []model.DataObject{{Name: "X", Type: "T1"}, {Name: "Y", Type: "T1"}}},
		},
		LNClasses: []model.LogicalNodeClass{
			{Name: "CYCL", Base: "A"},
			{Name: "OVER", Base: "Root", DataObjects: []model.DataObject{{Name: "X", Type: "T2"}, {Name: "Z", Type: "T1"}}},
		},
	}
	c, err := catalog.New(ns)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Inheritance("CYCL"); !errors.Is(err, model.ErrCycle) {
		t.Errorf("cycle error = %v", err)
	}
	dos, err := c.DataObjects("OVER")
	if err != nil {
		t.Fatal(err)
	}
	if len(dos) != 3 || dos[0].Name != "X" || dos[0].Type != "T2" || dos[0].DeclaredIn.Name != "OVER" || dos[2].Name != "Z" {
		t.Errorf("override order: %+v", dos)
	}
}

func TestListings(t *testing.T) {
	p := fixtures.Profile(t)
	lns := p.ListLogicalNodes()
	var names []string
	for _, ln := range lns {
		names = append(names, ln.Name)
	}
	if strings.Join(names, ",") != "DTST,GGIO,LLN0,LPHD,MMTR,MMXU" {
		t.Errorf("ListLogicalNodes = %v", names)
	}
	if got := p.ListAbstractLogicalNodes(); len(got) != 2 || got[0].Name != "DomainLN" {
		t.Errorf("ListAbstractLogicalNodes = %v", got)
	}
	if got := p.ListCDCs(); len(got) != 9 || got[0].Name != "BCR" {
		t.Errorf("ListCDCs = %d", len(got))
	}
	if got := p.ListConstructedAttributes(); len(got) != 3 || got[0].Name != "AnalogueValue" {
		t.Errorf("ListConstructedAttributes = %d", len(got))
	}
	if got := p.ListEnumerations(); len(got) != 5 || got[0].Name != "BehaviourModeKind" {
		t.Errorf("ListEnumerations = %d", len(got))
	}
	if got := p.ListBasicTypes(); len(got) != 10 || got[0].Name != "BOOLEAN" {
		t.Errorf("ListBasicTypes = %d", len(got))
	}
	all := fixtures.Catalog(t).ListLogicalNodes()
	if len(all) != 11 || all[9].Name != "MMXU" || all[9].Source != fixtures.ID74 || all[10].Source != fixtures.ID74R2 {
		t.Errorf("catalog listing must keep both releases in ID order: %d", len(all))
	}
}

func TestConcurrentReads(t *testing.T) {
	c := fixtures.Catalog(t)
	p := fixtures.Profile(t)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = c.LogicalNode("MMXU")
				_, _ = p.DataObjects("MMXU")
				_ = p.Search(catalog.Query{Name: "ph"})
				_ = c.ListLogicalNodes()
			}
		}()
	}
	wg.Wait()
}

func TestExtensionClasses(t *testing.T) {
	base := model.NamespaceID{Standard: "B", Version: "1"}
	ext := model.NamespaceID{Standard: "E", Version: "1"}
	orphanNS := model.NamespaceID{Standard: "O", Version: "1"}
	nsB := &model.Namespace{ID: base,
		CDCs:              []model.CommonDataClass{{Name: "MV", Source: base}},
		AbstractLNClasses: []model.LogicalNodeClass{{Name: "Root", Abstract: true, Source: base, DataObjects: []model.DataObject{{Name: "Beh", Type: "MV"}}}},
		LNClasses:         []model.LogicalNodeClass{{Name: "MMXU", Base: "Root", Source: base, DataObjects: []model.DataObject{{Name: "Hz", Type: "MV"}}}},
	}
	nsE := &model.Namespace{ID: ext, Dependencies: []model.NamespaceID{{Standard: "B", Version: "1"}},
		LNClasses: []model.LogicalNodeClass{{Name: "MMXU", IsExtension: true, Source: ext, DataObjects: []model.DataObject{{Name: "Added", Type: "MV"}}}},
	}
	nsO := &model.Namespace{ID: orphanNS,
		LNClasses: []model.LogicalNodeClass{{Name: "ZZZZ", IsExtension: true, Source: orphanNS, DataObjects: []model.DataObject{{Name: "X", Type: "MV"}}}},
	}
	c, err := catalog.New(nsB, nsE, nsO)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Profile(ext, orphanNS)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := p.LogicalNode("MMXU")
	if err != nil || ln.Source != base {
		t.Fatalf("base definition: %v %v", ln, err)
	}
	if exts := p.Extensions("MMXU"); len(exts) != 1 || exts[0].Source != ext {
		t.Errorf("extensions %v", exts)
	}
	dos, err := p.DataObjects("MMXU")
	if err != nil || len(dos) != 3 || dos[2].Name != "Added" || dos[2].DeclaredIn.Source != ext {
		t.Errorf("merged data objects %+v %v", dos, err)
	}
	if do, err := p.DataObject("MMXU", "Added"); err != nil || do.DeclaredIn.Source != ext {
		t.Errorf("extension data object lookup: %v", err)
	}
	var names []string
	for _, l := range p.ListLogicalNodes() {
		names = append(names, l.Name+"@"+l.Source.Standard)
	}
	if strings.Join(names, ",") != "MMXU@B,ZZZZ@O" {
		t.Errorf("listing hides merged extensions, keeps orphans: %v", names)
	}
	if orphan, err := p.LogicalNode("ZZZZ"); err != nil || !orphan.IsExtension {
		t.Errorf("orphan extension: %v", err)
	}
	if dos, err := p.DataObjects("ZZZZ"); err != nil || len(dos) != 1 {
		t.Errorf("orphan data objects: %v %v", dos, err)
	}
	found := false
	for _, d := range p.Diagnostics() {
		if strings.Contains(d.Message, "extension class without a base class") && d.Path == "ZZZZ" {
			found = true
		}
		if d.Code == "conflicting-definition" {
			t.Errorf("extensions are not conflicts: %v", d)
		}
	}
	if !found {
		t.Errorf("orphan extension not reported: %v", p.Diagnostics())
	}
	res := p.Search(catalog.Query{Name: "Added", Kinds: []catalog.Kind{catalog.KindDataObject}})
	if len(res) != 1 || !res[0].Extension {
		t.Errorf("search marks extension data objects: %+v", res)
	}
	changes := catalog.Diff(nsB, nsE)
	if len(changes) == 0 || !strings.Contains(fmt.Sprint(changes), "MMXU (extension in E)") {
		t.Errorf("diff keys extensions separately: %v", changes)
	}
}

func TestCDCVariantsLookup(t *testing.T) {
	a := model.NamespaceID{Standard: "A", Version: "1"}
	b := model.NamespaceID{Standard: "B", Version: "1"}
	nsA := &model.Namespace{ID: a, CDCs: []model.CommonDataClass{{Name: "SPG", Variant: "SP", Source: a}, {Name: "SPG", Variant: "SG", Source: a}}}
	nsB := &model.Namespace{ID: b, CDCs: []model.CommonDataClass{{Name: "SPG", Source: b}}}
	c, _ := catalog.New(nsA)
	if cdc, err := c.CDC("SPG"); err != nil || cdc.Variant != "SP" {
		t.Errorf("first variant: %v %v", cdc, err)
	}
	if got := c.ListCDCs(); len(got) != 2 || got[1].Key() != "SPG/SP" && got[1].Key() != "SPG/SG" {
		t.Errorf("listing %v", got)
	}
	both, _ := catalog.New(nsA, nsB)
	if _, err := both.CDCVariants("SPG"); !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("same name in two namespaces: %v", err)
	}
	p, _ := both.Profile(a, b)
	conflicts := 0
	for _, d := range p.Diagnostics() {
		if d.Code == "conflicting-definition" {
			conflicts++
		}
	}
	if conflicts != 1 {
		t.Errorf("variants of one namespace are one CDC; two namespaces conflict once: %v", p.Diagnostics())
	}
	if r := both.Search(catalog.Query{Kinds: []catalog.Kind{catalog.KindCDC}}); len(r) != 3 || r[1].Variant == "" && r[0].Variant == "" {
		t.Errorf("search reports variants: %+v", r)
	}
}
