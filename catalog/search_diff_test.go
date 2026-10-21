// SPDX-License-Identifier: MIT

package catalog_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
)

func paths(rs []catalog.Result) string {
	var out []string
	for _, r := range rs {
		out = append(out, string(r.Kind)+":"+r.Path)
	}
	return strings.Join(out, " ")
}

func TestSearch(t *testing.T) {
	p := fixtures.Profile(t)
	tests := []struct {
		name string
		q    catalog.Query
		want string
	}{
		{"exact class", catalog.Query{Name: "MMXU", Exact: true}, "ln:MMXU"},
		{"by CDC", catalog.Query{CDC: "WYE"}, "ln:MMXU do:MMXU.A do:MMXU.PNV do:MMXU.PhV do:MMXU.W cdc:WYE da:WYE.d"},
		{"DOs of a CDC", catalog.Query{Kinds: []catalog.Kind{catalog.KindDataObject}, CDC: "DEL"}, "do:MMXU.PPV"},
		{"substring", catalog.Query{Name: "tot", Kinds: []catalog.Kind{catalog.KindDataObject}}, "do:MMTR.TotVArh do:MMTR.TotWh do:MMXU.TotVA do:MMXU.TotVAr do:MMXU.TotW"},
		{"group", catalog.Query{Group: 'G', Kinds: []catalog.Kind{catalog.KindLogicalNode}}, "ln:GGIO"},
		{"group D", catalog.Query{Group: 'D'}, "ln:DTST do:DTST.OutW do:DTST.State"},
		{"standard", catalog.Query{Standard: "OTF TEST-EXT"}, "ln:DTST do:DTST.OutW do:DTST.State enum:TestStateKind"},
		{"fc", catalog.Query{FC: model.FCCF}, "da:BCR.pulsQty da:BCR.units da:CMV.units da:MV.units"},
		{"type", catalog.Query{Type: "AnalogueValue"}, "da:MV.instMag da:MV.mag da:Vector.ang da:Vector.mag"},
		{"abstract", catalog.Query{Name: "Beh", IncludeAbstract: true}, "do:DomainLN.Beh enum:BehaviourModeKind"},
		{"abstract excluded", catalog.Query{Name: "Beh", Exact: true}, ""},
		{"enum and basic", catalog.Query{Name: "Kind", Kinds: []catalog.Kind{catalog.KindEnumeration, catalog.KindBasicType}}, "enum:BehaviourModeKind enum:HealthKind enum:MultiplierKind enum:SIUnitKind enum:TestStateKind"},
		{"constructed", catalog.Query{Kinds: []catalog.Kind{catalog.KindConstructedAttribute}}, "ca:AnalogueValue ca:Unit ca:Vector"},
		{"basic", catalog.Query{Name: "float", Kinds: []catalog.Kind{catalog.KindBasicType}}, "basic:FLOAT32"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := paths(p.Search(tc.q)); got != tc.want {
				t.Errorf("Search(%+v)\n got: %s\nwant: %s", tc.q, got, tc.want)
			}
		})
	}
	all := fixtures.Catalog(t).Search(catalog.Query{Name: "Hz", Exact: true})
	if len(all) != 2 || all[0].Namespace != fixtures.ID74 || all[1].Namespace != fixtures.ID74R2 {
		t.Errorf("catalog search keeps both releases: %v", all)
	}
	if k, ok := catalog.ParseKind(" DO "); !ok || k != catalog.KindDataObject {
		t.Error("ParseKind")
	}
	if _, ok := catalog.ParseKind("x"); ok {
		t.Error("ParseKind(x)")
	}
	if len(catalog.Kinds()) != 7 {
		t.Error("Kinds")
	}
}

func TestDiffReleases(t *testing.T) {
	c := fixtures.Catalog(t)
	r1, _ := c.Namespace(fixtures.ID74)
	r2, _ := c.Namespace(fixtures.ID74R2)
	changes := catalog.Diff(r1, r2)
	var got []string
	for _, ch := range changes {
		got = append(got, fmt.Sprintf("%s %s %s %s", ch.Kind, ch.Element, ch.Path, ch.Field))
		if ch.Description == "" {
			t.Errorf("change without description: %+v", ch)
		}
	}
	want := []string{
		"changed DataObject GGIO.IntIn type",
		"added DataObject MMXU.TotPF ",
		"changed DataObject MMXU.TotVA presence",
		"removed DataObject MMXU.W ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Diff\n got: %q\nwant: %q", got, want)
	}
	if len(catalog.Diff(r1, r1)) != 0 {
		t.Error("diff of a namespace with itself must be empty")
	}
}

func TestDiffAllElements(t *testing.T) {
	id := model.NamespaceID{Standard: "S", Version: "1"}
	oldNS := &model.Namespace{
		ID:                    id,
		Dependencies:          []model.NamespaceID{{Standard: "D", Version: "1"}, {Standard: "R", Version: "1"}},
		BasicTypes:            []model.BasicType{{Name: "B1"}},
		FunctionalConstraints: []model.FunctionalConstraintDef{{Abbreviation: "ST"}},
		PresenceConditions:    []model.PresenceConditionDef{{Name: "M"}},
		Enumerations:          []model.Enumeration{{Name: "E", Literals: []model.Literal{{Name: "a", Value: 1}, {Name: "b", Value: 2}}}},
		ConstructedAttributes: []model.ConstructedAttribute{{Name: "C", SubDataAttributes: []model.DataAttribute{{Name: "x", Type: "B1"}}}},
		CDCs: []model.CommonDataClass{{Name: "X", DataAttributes: []model.DataAttribute{{Name: "a", Type: "B1", FC: "ST"}},
			SubDataObjects: []model.DataObject{{Name: "s", Type: "X"}}}},
		LNClasses: []model.LogicalNodeClass{{Name: "ABCD", Base: "R1"}, {Name: "GONE"}},
	}
	newNS := &model.Namespace{
		ID:                    id,
		Dependencies:          []model.NamespaceID{{Standard: "D", Version: "2"}, {Standard: "N", Version: "1"}},
		BasicTypes:            []model.BasicType{{Name: "B2"}},
		FunctionalConstraints: []model.FunctionalConstraintDef{{Abbreviation: "MX"}},
		PresenceConditions:    []model.PresenceConditionDef{{Name: "O"}},
		Enumerations:          []model.Enumeration{{Name: "E", InheritedFrom: "P", Literals: []model.Literal{{Name: "a", Value: 3}, {Name: "c", Value: 2}}}},
		ConstructedAttributes: []model.ConstructedAttribute{{Name: "C", Deprecated: true, SubDataAttributes: []model.DataAttribute{{Name: "x", Type: "B2", Dchg: true}}}},
		CDCs: []model.CommonDataClass{{Name: "X", EnumParameterized: true, DataAttributes: []model.DataAttribute{{Name: "a", Type: "B1", FC: "MX", IsArray: true}},
			SubDataObjects: []model.DataObject{{Name: "s", Type: "Y", Transient: true}}}},
		LNClasses: []model.LogicalNodeClass{{Name: "ABCD", Base: "R2", CanHaveLOG: true}, {Name: "NEWW"}},
	}
	changes := catalog.Diff(oldNS, newNS)
	text := ""
	for _, c := range changes {
		text += fmt.Sprintf("%s|%s|%s|%s\n", c.Kind, c.Element, c.Path, c.Field)
	}
	for _, want := range []string{
		"changed|Dependency|S -> D|version", "added|Dependency|S -> N|", "removed|Dependency|S -> R|",
		"added|BasicType|B2|", "removed|BasicType|B1|", "added|FunctionalConstraint|MX|", "removed|PresenceCondition|M|",
		"changed|Enumeration|E|inheritedFrom", "changed|Literal|E.a|value", "removed|Literal|E.b|", "added|Literal|E.c|",
		"changed|ConstructedAttribute|C|deprecated", "changed|SubDataAttribute|C.x|type", "changed|SubDataAttribute|C.x|dchg",
		"changed|CDC|X|enumParameterized", "changed|DataAttribute|X.a|fc", "changed|DataAttribute|X.a|isArray",
		"changed|SubDataObject|X.s|type", "changed|SubDataObject|X.s|transient",
		"changed|LNClass|ABCD|base", "changed|LNClass|ABCD|canHaveLOG", "removed|LNClass|GONE|", "added|LNClass|NEWW|",
	} {
		if !strings.Contains(text, want+"\n") {
			t.Errorf("missing change %q in\n%s", want, text)
		}
	}
	if got := catalog.Diff(nil, oldNS); len(got) == 0 || got[0].Kind != catalog.Added {
		t.Error("diff from nil must report additions")
	}
}
