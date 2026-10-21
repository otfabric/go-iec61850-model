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

// Two namespaces define DBAT, SBAT and an enumeration again, as IEC
// 61850-7-420 and IEC TR 61850-90-9 do. Both depend on a base namespace.
var (
	idBase = model.NamespaceID{Standard: "T 61850-7-4", Version: "2007", Revision: "B", Release: 1}
	idStd  = model.NamespaceID{Standard: "T 61850-7-420", Version: "2019", Revision: "A", Release: 4}
	idTR   = model.NamespaceID{Standard: "T TR 61850-90-9", Version: "2018", Revision: "A", Release: 3}
)

func do(name, cdc, presence string) model.DataObject {
	return model.DataObject{Name: name, Type: cdc, Presence: model.PresenceCondition{Name: presence}}
}

func overlapNamespaces() []catalog.Source {
	base := &model.Namespace{
		ID:   idBase,
		CDCs: []model.CommonDataClass{{Name: "ENS"}, {Name: "SPS"}, {Name: "MV"}},
		AbstractLNClasses: []model.LogicalNodeClass{
			{Name: "DomainLN", Abstract: true, DataObjects: []model.DataObject{do("Beh", "ENS", "M")}},
		},
	}
	std := &model.Namespace{
		ID: idStd, Dependencies: []model.NamespaceID{idBase},
		Enumerations: []model.Enumeration{{Name: "ChargeKind", Literals: []model.Literal{{Name: "Grid", Value: 1}, {Name: "PV", Value: 2}}}},
		LNClasses: []model.LogicalNodeClass{
			{Name: "DBAT", Base: "DomainLN", DataObjects: []model.DataObject{do("BatSt", "SPS", "M"), do("Vol", "MV", "O")}},
			{Name: "SBAT", Base: "DomainLN", DataObjects: []model.DataObject{do("Amp", "MV", "O"), do("Tmp", "MV", "O")}},
		},
	}
	tr := &model.Namespace{
		ID: idTR, Dependencies: []model.NamespaceID{idBase},
		Enumerations: []model.Enumeration{{Name: "ChargeKind", Literals: []model.Literal{{Name: "Grid", Value: 1}, {Name: "PV", Value: 2}}}},
		LNClasses: []model.LogicalNodeClass{
			{Name: "DBAT", Base: "DomainLN", DataObjects: []model.DataObject{do("BatSt", "SPS", "O"), do("Test", "SPS", "O")}},
			{Name: "SBAT", Base: "DomainLN", DataObjects: []model.DataObject{do("Amp", "MV", "O")}},
			{Name: "XTRA", Base: "DomainLN"},
		},
	}
	for _, ns := range []*model.Namespace{base, std, tr} {
		stamp(ns)
	}
	return []catalog.Source{base, std, tr}
}

// stamp sets the source namespace of every definition, as the importer
// does.
func stamp(ns *model.Namespace) {
	for i := range ns.CDCs {
		ns.CDCs[i].Source = ns.ID
	}
	for i := range ns.Enumerations {
		ns.Enumerations[i].Source = ns.ID
	}
	for i := range ns.AbstractLNClasses {
		ns.AbstractLNClasses[i].Source = ns.ID
	}
	for i := range ns.LNClasses {
		ns.LNClasses[i].Source = ns.ID
	}
}

func overlapProfile(t *testing.T, ids ...model.NamespaceID) *catalog.Profile {
	t.Helper()
	c, err := catalog.New(overlapNamespaces()...)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Profile(ids...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultProfileIsUnambiguous(t *testing.T) {
	p := overlapProfile(t, idStd)
	ln, err := p.LogicalNode("DBAT")
	if err != nil || ln.Source != idStd {
		t.Fatalf("LogicalNode(DBAT) = %v, %v", ln, err)
	}
	if _, err := p.LogicalNode("XTRA"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("XTRA outside the profile: %v", err)
	}
}

func TestExtendedProfilePreservesBothDefinitions(t *testing.T) {
	p := overlapProfile(t, idStd, idTR)

	var dbat []model.NamespaceID
	for _, ln := range p.ListLogicalNodes() {
		if ln.Name == "DBAT" {
			dbat = append(dbat, ln.Source)
		}
	}
	if len(dbat) != 2 {
		t.Fatalf("DBAT definitions = %v, want both", dbat)
	}

	// Unqualified lookups of a name both define are ambiguous.
	_, err := p.LogicalNode("DBAT")
	var ae *model.AmbiguityError
	if !errors.Is(err, model.ErrAmbiguousSymbol) || !errors.As(err, &ae) || len(ae.Candidates) != 2 {
		t.Fatalf("LogicalNode(DBAT) error = %v", err)
	}
	if _, err := p.DataObjects("DBAT"); !errors.Is(err, model.ErrAmbiguousSymbol) {
		t.Errorf("DataObjects(DBAT) error = %v", err)
	}
	if _, err := p.Enumeration("ChargeKind"); !errors.Is(err, model.ErrAmbiguousSymbol) {
		t.Errorf("Enumeration(ChargeKind) error = %v", err)
	}
	// Names only one namespace defines stay unqualified.
	if ln, err := p.LogicalNode("XTRA"); err != nil || ln.Source != idTR {
		t.Errorf("LogicalNode(XTRA) = %v, %v", ln, err)
	}
}

func TestQualifiedLookups(t *testing.T) {
	p := overlapProfile(t, idStd, idTR)
	for _, tc := range []struct {
		ns   model.NamespaceID
		want []string
	}{
		{idStd, []string{"Beh", "BatSt", "Vol"}},
		{model.NamespaceID{Standard: idTR.Standard}, []string{"Beh", "BatSt", "Test"}}, // partial ID
	} {
		ln, err := p.LogicalNodeIn(tc.ns, "DBAT")
		if err != nil {
			t.Fatalf("LogicalNodeIn(%s) error = %v", tc.ns, err)
		}
		dos, err := p.DataObjectsIn(tc.ns, "DBAT")
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, d := range dos {
			names = append(names, d.Name)
		}
		if !slices.Equal(names, tc.want) {
			t.Errorf("DataObjectsIn(%s, DBAT) = %v, want %v (class from %s)", tc.ns, names, tc.want, ln.Source)
		}
	}
	chain, err := p.InheritanceIn(idTR, "DBAT")
	if err != nil || len(chain) != 2 || chain[0].Source != idTR || chain[1].Source != idBase {
		t.Errorf("InheritanceIn(TR, DBAT) = %v, %v", chain, err)
	}
	if do, err := p.DataObjectIn(idTR, "DBAT", "BatSt"); err != nil || do.Presence.Name != "O" {
		t.Errorf("DataObjectIn(TR, DBAT, BatSt) = %+v, %v", do, err)
	}
	if _, err := p.DataObjectIn(idStd, "DBAT", "Test"); !errors.Is(err, model.ErrUnknownDataObject) {
		t.Errorf("Test is not in the standard's DBAT: %v", err)
	}
	if _, err := p.LogicalNodeIn(idStd, "XTRA"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("LogicalNodeIn(std, XTRA) error = %v", err)
	}
	if e, err := p.EnumerationIn(idTR, "ChargeKind"); err != nil || e.Source != idTR {
		t.Errorf("EnumerationIn(TR, ChargeKind) = %v, %v", e, err)
	}
	if _, err := p.LogicalNodeIn(model.NamespaceID{Standard: "T 61850-7-99"}, "DBAT"); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown namespace: %v", err)
	}
	if got, err := p.ResolveNamespace(model.NamespaceID{Standard: idStd.Standard}); err != nil || got != idStd {
		t.Errorf("ResolveNamespace = %v, %v", got, err)
	}
	if scope := p.Scope(idTR); len(scope) != 2 || scope[0] != idTR || scope[1] != idBase {
		t.Errorf("Scope(TR) = %v", scope)
	}
}

func TestOverlaps(t *testing.T) {
	p := overlapProfile(t, idStd, idTR)
	got, err := p.Overlaps(idStd, idTR)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]catalog.Overlap{}
	for _, o := range got {
		byName[o.Name] = o
	}
	if len(got) != 3 {
		t.Fatalf("overlaps = %+v, want DBAT, SBAT and ChargeKind", got)
	}
	dbat := byName["DBAT"]
	if dbat.Kind != catalog.KindLogicalNode || dbat.Class != catalog.OverlapConflicting ||
		!slices.Equal(dbat.OnlyLeft, []string{"Vol"}) || !slices.Equal(dbat.OnlyRight, []string{"Test"}) ||
		len(dbat.Differences) != 1 || !strings.Contains(dbat.Differences[0], "BatSt") {
		t.Errorf("DBAT overlap = %+v", dbat)
	}
	if sbat := byName["SBAT"]; sbat.Class != catalog.OverlapCompatible || !slices.Equal(sbat.OnlyLeft, []string{"Tmp"}) {
		t.Errorf("SBAT overlap = %+v", sbat)
	}
	if e := byName["ChargeKind"]; e.Kind != catalog.KindEnumeration || e.Class != catalog.OverlapIdentical {
		t.Errorf("ChargeKind overlap = %+v", e)
	}

	all, err := p.Overlaps(model.NamespaceID{}, model.NamespaceID{})
	if err != nil || len(all) != len(got) {
		t.Errorf("all pairs = %d overlaps, %v; want %d", len(all), err, len(got))
	}
	if _, err := p.Overlaps(idStd, model.NamespaceID{}); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("one namespace only: %v", err)
	}
}
