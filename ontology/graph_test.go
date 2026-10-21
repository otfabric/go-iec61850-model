// SPDX-License-Identifier: MIT

package ontology_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/ontology"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func build(t *testing.T, sem bool) *ontology.Graph {
	t.Helper()
	opts := ontology.Options{}
	if sem {
		opts.Semantics = semantic.Default
	}
	g, err := ontology.BuildGraph(fixtures.Profile(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func rels(rs []ontology.Relation) string {
	var out []string
	for _, r := range rs {
		out = append(out, string(r.Predicate)+">"+r.To)
	}
	return strings.Join(out, " ")
}

func TestModelRelations(t *testing.T) {
	g := build(t, false)
	mmxu, ok := g.Concept(ontology.LogicalNodeID("MMXU"))
	if !ok || mmxu.Kind != ontology.KindLogicalNode || mmxu.Namespace != fixtures.ID74 || mmxu.Label != "Test measurement class" {
		t.Fatalf("MMXU concept %+v", mmxu)
	}
	if v, ok := mmxu.Property("group"); !ok || v != "M Metering and measurement" {
		t.Errorf("group property %q", v)
	}
	if _, ok := mmxu.Property("nope"); ok {
		t.Error("unknown property found")
	}
	if got := rels(g.Relations("ln:MMXU", ontology.InheritsFrom, ontology.DefinedIn)); got != "definedIn>ns:IEC 61850-7-4:2007B1 inheritsFrom>ln:StatisticsLN" {
		t.Errorf("MMXU relations: %s", got)
	}
	if got := len(g.Relations("ln:MMXU", ontology.Contains)); got != 9 {
		t.Errorf("MMXU contains %d data objects", got)
	}
	for from, want := range map[string]string{
		"do:MMXU.Hz":         "uses>cdc:MV",
		"do:DomainLN.Beh":    "hasType>enum:BehaviourModeKind uses>cdc:ENS",
		"sdo:WYE.phsA":       "uses>cdc:CMV",
		"da:MV.mag":          "hasType>ca:AnalogueValue",
		"da:AnalogueValue.f": "hasType>basic:FLOAT32",
		"da:Unit.SIUnit":     "hasType>enum:SIUnitKind",
		"da:ENS.q":           "hasType>basic:Quality",
		"da:ENS.stVal":       "",
	} {
		if got := rels(g.Relations(from)); got != want {
			t.Errorf("%s: %q, want %q", from, got, want)
		}
	}
	if got := rels(g.Relations("ns:IEC 61850-7-4:2007B1", ontology.DependsOn)); got != "dependsOn>ns:IEC 61850-7-3:2007B1" {
		t.Errorf("dependencies: %s", got)
	}
	in := g.Incoming("cdc:WYE", ontology.Uses)
	if len(in) != 4 || in[0].From != "do:MMXU.A" {
		t.Errorf("incoming uses of WYE: %+v", in)
	}
	if len(g.Incoming("cdc:WYE")) != 4 {
		t.Error("Incoming without predicates")
	}
	if _, ok := g.Concept("measurement:MMXU.Hz"); ok {
		t.Error("no measurements without semantics")
	}
	da, _ := g.Concept("da:MV.mag")
	if fc, _ := da.Property("fc"); fc != "MX" {
		t.Errorf("fc property %q", fc)
	}
}

func TestMeasurements(t *testing.T) {
	g := build(t, true)
	m, ok := g.Concept(ontology.MeasurementID("MMXU.PNV.phsA"))
	if !ok || m.Label != "Voltage, phase A, line-neutral" {
		t.Fatalf("measurement %+v", m)
	}
	if v, _ := m.Property("rule"); v != "mmxu.pnv" {
		t.Errorf("rule property %q", v)
	}
	if v, _ := m.Property("unitHint"); v != "V" {
		t.Errorf("unit hint %q", v)
	}
	want := "hasPhase>phase:A hasReference>reference:line-neutral measuredBy>sdo:WYE.phsA represents>quantity:voltage"
	if got := rels(g.Relations(m.ID)); got != want {
		t.Errorf("measurement relations\n got: %s\nwant: %s", got, want)
	}
	if got := rels(g.Relations("measurement:MMXU.Hz")); got != "measuredBy>do:MMXU.Hz represents>quantity:frequency" {
		t.Errorf("Hz relations: %s", got)
	}
	if got := rels(g.Relations("measurement:MMTR.SupWh")); got != "hasPhase>phase:total measuredBy>do:MMTR.SupWh represents>quantity:active-energy" {
		t.Errorf("SupWh relations: %s", got)
	}
	sup, _ := g.Concept("measurement:MMTR.SupWh")
	if !strings.HasSuffix(sup.Label, "supply") {
		t.Errorf("SupWh label %q", sup.Label)
	}
	volts := g.Incoming(ontology.QuantityID(semantic.QuantityVoltage), ontology.Represents)
	if len(volts) != 3+5+5 {
		t.Errorf("voltage measurements = %d", len(volts))
	}
	if q, ok := g.Concept(ontology.QuantityID(semantic.QuantityVoltage)); !ok || q.Label != "Voltage" {
		t.Errorf("quantity concept %+v", q)
	}
	if _, ok := g.Concept(ontology.PhaseID(semantic.PhaseAB)); !ok {
		t.Error("phase concept")
	}
}

func TestInheritedMeasurementCarrier(t *testing.T) {
	id := model.NamespaceID{Standard: "I", Version: "1"}
	ns := &model.Namespace{
		ID:                id,
		CDCs:              []model.CommonDataClass{{Name: "MV", Source: id}},
		AbstractLNClasses: []model.LogicalNodeClass{{Name: "Base", Abstract: true, Source: id, DataObjects: []model.DataObject{{Name: "Hz", Type: "MV"}}}},
		LNClasses:         []model.LogicalNodeClass{{Name: "ZZZZ", Base: "Base", Source: id}},
	}
	c, _ := catalog.New(ns)
	p, err := c.Profile(id)
	if err != nil {
		t.Fatal(err)
	}
	g, err := ontology.BuildGraph(p, ontology.Options{Semantics: semantic.Default})
	if err != nil {
		t.Fatal(err)
	}
	if got := rels(g.Relations("measurement:ZZZZ.Hz", ontology.MeasuredBy)); got != "measuredBy>do:Base.Hz" {
		t.Errorf("inherited carrier: %s", got)
	}
}

func TestFindConceptsAndWalk(t *testing.T) {
	g := build(t, true)
	cs := g.FindConcepts(ontology.Query{Kinds: []ontology.Kind{ontology.KindCDC}, Name: "w"})
	if len(cs) != 1 || cs[0].Name != "WYE" {
		t.Errorf("FindConcepts = %+v", cs)
	}
	if cs := g.FindConcepts(ontology.Query{Name: "Hz", Exact: true, Kinds: []ontology.Kind{ontology.KindDataObject}}); len(cs) != 1 {
		t.Errorf("exact = %+v", cs)
	}
	if cs := g.FindConcepts(ontology.Query{Standard: "OTF TEST-EXT"}); len(cs) != 5 {
		t.Errorf("by standard = %d %+v", len(cs), cs)
	}
	if cs := g.FindConcepts(ontology.Query{Kinds: []ontology.Kind{ontology.KindQuantity}}); len(cs) != 8 {
		t.Errorf("quantities = %d", len(cs))
	}
	var visited []string
	err := g.Walk("do:MMXU.Hz", func(c ontology.Concept, depth int) bool {
		visited = append(visited, c.ID)
		return depth < 2
	}, ontology.Uses, ontology.Contains)
	if err != nil {
		t.Fatal(err)
	}
	want := "do:MMXU.Hz cdc:MV da:MV.d da:MV.instMag da:MV.mag da:MV.q da:MV.t da:MV.units"
	if strings.Join(visited, " ") != want {
		t.Errorf("walk\n got: %s\nwant: %s", strings.Join(visited, " "), want)
	}
	if err := g.Walk("nope", func(ontology.Concept, int) bool { return true }); !errors.Is(err, model.ErrUnknownConcept) {
		t.Errorf("unknown start: %v", err)
	}
	n := 0
	_ = g.Walk("ln:MMXU", func(ontology.Concept, int) bool { n++; return true })
	if n < 50 {
		t.Errorf("full walk visited %d concepts", n)
	}
}

func TestExportDeterministic(t *testing.T) {
	var a, b bytes.Buffer
	if err := build(t, true).ExportJSON(&a); err != nil {
		t.Fatal(err)
	}
	if err := build(t, true).ExportJSON(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("export differs between builds")
	}
	var doc struct {
		Concepts  []ontology.Concept  `json:"concepts"`
		Relations []ontology.Relation `json:"relations"`
	}
	if err := json.Unmarshal(a.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	g := build(t, true)
	if len(doc.Concepts) != len(g.Concepts()) || len(doc.Relations) != len(g.RelationsAll()) {
		t.Errorf("export sizes %d/%d", len(doc.Concepts), len(doc.Relations))
	}
	for i := 1; i < len(doc.Concepts); i++ {
		if doc.Concepts[i-1].ID >= doc.Concepts[i].ID {
			t.Fatalf("concepts not sorted at %d", i)
		}
	}
	compact, err := json.Marshal(g)
	if err != nil || !bytes.Contains(compact, []byte(`"id":"ln:MMXU"`)) {
		t.Errorf("MarshalJSON: %v", err)
	}
}

func TestBuildGraphConflicts(t *testing.T) {
	a := &model.Namespace{ID: model.NamespaceID{Standard: "A", Version: "1"}, CDCs: []model.CommonDataClass{{Name: "X"}}}
	b := &model.Namespace{ID: model.NamespaceID{Standard: "B", Version: "1"}, CDCs: []model.CommonDataClass{{Name: "X"}}}
	c, _ := catalog.New(a, b)
	p, err := c.Profile(a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	g, err := ontology.BuildGraph(p, ontology.Options{})
	if err != nil || len(g.Conflicts()) != 1 || !strings.Contains(g.Conflicts()[0], "cdc:X is defined by A:1 and B:1") {
		t.Errorf("duplicate concept: %v %v", g, err)
	}
	if c, _ := g.Concept("cdc:X"); c.Namespace != a.ID {
		t.Error("first definition must win")
	}
	dupLN := &model.Namespace{ID: model.NamespaceID{Standard: "D", Version: "1"}, CDCs: []model.CommonDataClass{{Name: "MV"}},
		LNClasses: []model.LogicalNodeClass{{Name: "MMXU", DataObjects: []model.DataObject{{Name: "Hz", Type: "MV"}}}}}
	dupLN2 := &model.Namespace{ID: model.NamespaceID{Standard: "E", Version: "1"},
		LNClasses: []model.LogicalNodeClass{{Name: "MMXU", DataObjects: []model.DataObject{{Name: "Hz", Type: "MV"}}}}}
	c3, _ := catalog.New(dupLN, dupLN2)
	p3, _ := c3.Profile(dupLN.ID, dupLN2.ID)
	g3, err := ontology.BuildGraph(p3, ontology.Options{Semantics: semantic.Default})
	if err != nil || len(g3.Conflicts()) != 3 {
		t.Errorf("ambiguous class measurements: %v %v", g3.Conflicts(), err)
	}
	amb := &model.Namespace{
		ID:        model.NamespaceID{Standard: "C", Version: "1"},
		CDCs:      []model.CommonDataClass{{Name: "MV"}},
		LNClasses: []model.LogicalNodeClass{{Name: "MMXU", Base: "Missing", DataObjects: []model.DataObject{{Name: "Hz", Type: "MV"}}}},
	}
	c2, _ := catalog.New(amb)
	p2, _ := c2.Profile(amb.ID)
	if _, err := ontology.BuildGraph(p2, ontology.Options{Semantics: semantic.Default}); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("semantic error is passed on: %v", err)
	}
}
