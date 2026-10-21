// SPDX-License-Identifier: MIT

package catalog_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
)

func attr(name, typ string, fc model.FunctionalConstraint, presence string) model.DataAttribute {
	return model.DataAttribute{Name: name, Type: typ, FC: fc, Presence: model.PresenceCondition{Name: presence}}
}

// typeNamespaces adds CDCs, constructed attributes, basic types and class
// hierarchies that differ between two namespaces depending on one base.
func typeNamespaces() []catalog.Source {
	base := &model.Namespace{
		ID:         idBase,
		BasicTypes: []model.BasicType{{Name: "FLOAT32"}, {Name: "INT64"}},
		CDCs:       []model.CommonDataClass{{Name: "ENS"}},
		AbstractLNClasses: []model.LogicalNodeClass{
			{Name: "DomainLN", Abstract: true, DataObjects: []model.DataObject{do("Beh", "ENS", "M")}},
		},
	}
	std := &model.Namespace{
		ID: idStd, Dependencies: []model.NamespaceID{idBase},
		CDCs: []model.CommonDataClass{
			{Name: "BCR", DataAttributes: []model.DataAttribute{attr("actVal", "INT64", model.FCST, "M")}},
			{Name: "SPG", Variant: "SP", DataAttributes: []model.DataAttribute{attr("setVal", "BOOLEAN", model.FCSP, "M")}},
			{Name: "SPG", Variant: "SG", DataAttributes: []model.DataAttribute{attr("setVal", "BOOLEAN", model.FCSG, "M")}},
		},
		ConstructedAttributes: []model.ConstructedAttribute{
			{Name: "Point", SubDataAttributes: []model.DataAttribute{attr("x", "FLOAT32", "", "M"), attr("y", "FLOAT32", "", "M")}},
		},
		LNClasses: []model.LogicalNodeClass{
			{Name: "CHN", Base: "DomainLN"},
			{Name: "BROK", Base: "DomainLN"},
		},
	}
	tr := &model.Namespace{
		ID: idTR, Dependencies: []model.NamespaceID{idBase},
		CDCs: []model.CommonDataClass{
			{Name: "BCR", DataAttributes: []model.DataAttribute{attr("actVal", "INT64", model.FCST, "M"), attr("frVal", "INT64", model.FCST, "O")}},
		},
		ConstructedAttributes: []model.ConstructedAttribute{
			{Name: "Point", SubDataAttributes: []model.DataAttribute{attr("x", "FLOAT32", "", "M"), attr("y", "FLOAT32", "", "M")}},
		},
		AbstractLNClasses: []model.LogicalNodeClass{
			{Name: "TrLN", Abstract: true, Base: "DomainLN"},
		},
		LNClasses: []model.LogicalNodeClass{
			{Name: "CHN", Base: "TrLN"},
			{Name: "BROK", Base: "NoSuchLN"},
		},
	}
	for _, ns := range []*model.Namespace{base, std, tr} {
		stamp(ns)
		for i := range ns.ConstructedAttributes {
			ns.ConstructedAttributes[i].Source = ns.ID
		}
		for i := range ns.BasicTypes {
			ns.BasicTypes[i].Source = ns.ID
		}
	}
	return []catalog.Source{base, std, tr}
}

func typeProfile(t *testing.T) *catalog.Profile {
	t.Helper()
	c, err := catalog.New(typeNamespaces()...)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Profile(idStd, idTR)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestQualifiedTypeLookups(t *testing.T) {
	p := typeProfile(t)
	unknown := model.NamespaceID{Standard: "T 61850-7-99"}

	if c, err := p.CDCIn(idTR, "BCR"); err != nil || c.Source != idTR || len(c.DataAttributes) != 2 {
		t.Errorf("CDCIn(TR, BCR) = %v, %v", c, err)
	}
	if vs, err := p.CDCVariantsIn(idStd, "SPG"); err != nil || len(vs) != 2 {
		t.Errorf("CDCVariantsIn(std, SPG) = %v, %v", vs, err)
	}
	if _, err := p.CDCIn(idTR, "SPG"); !errors.Is(err, model.ErrUnknownCDC) {
		t.Errorf("CDCIn(TR, SPG) error = %v", err)
	}
	if _, err := p.CDCVariantsIn(unknown, "BCR"); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("CDCVariantsIn(unknown) error = %v", err)
	}
	if ca, err := p.ConstructedAttributeIn(idStd, "Point"); err != nil || ca.Source != idStd {
		t.Errorf("ConstructedAttributeIn(std, Point) = %v, %v", ca, err)
	}
	if _, err := p.ConstructedAttributeIn(idBase, "Point"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("ConstructedAttributeIn(base, Point) error = %v", err)
	}
	if _, err := p.EnumerationIn(unknown, "Kind"); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("EnumerationIn(unknown) error = %v", err)
	}
	for name, err := range map[string]error{
		"InheritanceIn": func() error { _, err := p.InheritanceIn(unknown, "CHN"); return err }(),
		"DataObjectsIn": func() error { _, err := p.DataObjectsIn(unknown, "CHN"); return err }(),
		"DataObjectIn":  func() error { _, err := p.DataObjectIn(unknown, "CHN", "Beh"); return err }(),
	} {
		if !errors.Is(err, model.ErrUnknownNamespace) {
			t.Errorf("%s(unknown) error = %v", name, err)
		}
	}
	// A base class that is not defined anywhere.
	if _, err := p.InheritanceIn(idTR, "BROK"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("InheritanceIn(TR, BROK) error = %v", err)
	}
	if chain, err := p.InheritanceIn(idTR, "CHN"); err != nil || len(chain) != 3 || chain[1].Name != "TrLN" {
		t.Errorf("InheritanceIn(TR, CHN) = %v, %v", chain, err)
	}

	// Unqualified lookups of names both namespaces define are ambiguous;
	// unknown names are reported as such.
	if _, err := p.ConstructedAttribute("Point"); !errors.Is(err, model.ErrAmbiguousSymbol) {
		t.Errorf("ConstructedAttribute(Point) error = %v", err)
	}
	if _, err := p.ConstructedAttribute("Nope"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("ConstructedAttribute(Nope) error = %v", err)
	}
	if _, err := p.BasicType("FLOAT99"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("BasicType(FLOAT99) error = %v", err)
	}
	if _, err := p.FunctionalConstraint("ZZ"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("FunctionalConstraint(ZZ) error = %v", err)
	}
	if _, err := p.PresenceCondition("Nope"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("PresenceCondition(Nope) error = %v", err)
	}
}

func TestScopedTypeLookups(t *testing.T) {
	p := typeProfile(t)

	// A definition's own namespace wins over the rest of the set.
	if vs, err := p.CDCVariantsFrom(idTR, "BCR"); err != nil || len(vs) != 1 || vs[0].Source != idTR {
		t.Errorf("CDCVariantsFrom(TR, BCR) = %v, %v", vs, err)
	}
	if ca, err := p.ConstructedAttributeFrom(idStd, "Point"); err != nil || ca.Source != idStd {
		t.Errorf("ConstructedAttributeFrom(std, Point) = %v, %v", ca, err)
	}
	// Dependencies are searched next.
	if b, err := p.BasicTypeFrom(idStd, "FLOAT32"); err != nil || b.Source != idBase {
		t.Errorf("BasicTypeFrom(std, FLOAT32) = %v, %v", b, err)
	}
	if vs, err := p.CDCVariantsFrom(idStd, "ENS"); err != nil || vs[0].Source != idBase {
		t.Errorf("CDCVariantsFrom(std, ENS) = %v, %v", vs, err)
	}
	// Outside the scope, the lookup falls back to the whole set, where
	// the name is ambiguous.
	for name, err := range map[string]error{
		"CDCVariantsFrom":          func() error { _, err := p.CDCVariantsFrom(idBase, "BCR"); return err }(),
		"ConstructedAttributeFrom": func() error { _, err := p.ConstructedAttributeFrom(idBase, "Point"); return err }(),
	} {
		if !errors.Is(err, model.ErrAmbiguousSymbol) {
			t.Errorf("%s(base) error = %v", name, err)
		}
	}
	if _, err := p.EnumerationFrom(idStd, "Nope"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("EnumerationFrom(std, Nope) error = %v", err)
	}
	if _, err := p.BasicTypeFrom(idStd, "FLOAT99"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("BasicTypeFrom(std, FLOAT99) error = %v", err)
	}
}

func TestOverlapsOfTypes(t *testing.T) {
	p := typeProfile(t)
	got, err := p.Overlaps(idStd, idTR)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]catalog.Overlap{}
	for _, o := range got {
		byName[string(o.Kind)+" "+o.Name] = o
	}
	if o := byName["cdc BCR"]; o.Class != catalog.OverlapCompatible || !slices.Equal(o.OnlyRight, []string{"frVal"}) {
		t.Errorf("BCR overlap = %+v", o)
	}
	if o := byName["ca Point"]; o.Class != catalog.OverlapIdentical {
		t.Errorf("Point overlap = %+v", o)
	}
	if o := byName["ln CHN"]; o.Class != catalog.OverlapConflicting || len(o.Differences) != 1 || !strings.HasPrefix(o.Differences[0], "inheritance") {
		t.Errorf("CHN overlap = %+v", o)
	}
	if o := byName["ln BROK"]; o.Class != catalog.OverlapConflicting || len(o.Differences) != 1 || !strings.HasPrefix(o.Differences[0], "cannot resolve") {
		t.Errorf("BROK overlap = %+v", o)
	}
	if _, err := p.Overlaps(model.NamespaceID{Standard: "T 61850-7-99"}, idTR); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown left: %v", err)
	}
	if _, err := p.Overlaps(idStd, model.NamespaceID{Standard: "T 61850-7-99"}); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown right: %v", err)
	}
}

func TestResolveNamespaceAcrossReleases(t *testing.T) {
	second := idStd
	second.Release = 5
	c, err := catalog.New(&model.Namespace{ID: idStd}, &model.Namespace{ID: second})
	if err != nil {
		t.Fatal(err)
	}
	var ae *model.AmbiguityError
	if _, err := c.ResolveNamespace(model.NamespaceID{Standard: idStd.Standard}); !errors.As(err, &ae) || len(ae.Candidates) != 2 {
		t.Errorf("partial ID over two releases: %v", err)
	}
	if got, err := c.ResolveNamespace(second); err != nil || got != second {
		t.Errorf("exact ID = %v, %v", got, err)
	}
	if got := c.Scope(model.NamespaceID{Standard: "T 61850-7-99"}); len(got) != 0 {
		t.Errorf("Scope of an unknown namespace = %v", got)
	}
}
