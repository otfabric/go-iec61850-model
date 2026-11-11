// SPDX-License-Identifier: MIT

package presence_test

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/presence"
)

func defaultProfile(t *testing.T) *catalog.Profile {
	t.Helper()
	p, err := official.Default()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func byPath(rep *presence.Report) map[string]presence.Result {
	out := map[string]presence.Result{}
	for _, r := range rep.All() {
		out[r.Path] = r
	}
	return out
}

// conditionNames collects every condition name the embedded catalog
// uses or defines.
func conditionNames(t *testing.T) map[string][]string {
	t.Helper()
	c, err := official.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string][]string{}
	add := func(p model.PresenceCondition, where string) {
		if !p.IsZero() && len(names[p.Name]) < 3 {
			names[p.Name] = append(names[p.Name], where)
		} else if !p.IsZero() {
			names[p.Name] = names[p.Name][:3]
		}
	}
	for _, ns := range c.Namespaces() {
		for _, d := range ns.PresenceConditions {
			add(model.PresenceCondition{Name: d.Name}, ns.ID.String()+" definition")
		}
		for _, list := range [][]model.LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
			for _, ln := range list {
				for _, d := range ln.DataObjects {
					add(d.Presence, ln.Name+"."+d.Name)
					add(d.DSPresence, ln.Name+"."+d.Name+" (data set)")
				}
			}
		}
		for _, cdc := range ns.CDCs {
			for _, s := range cdc.SubDataObjects {
				add(s.Presence, cdc.Key()+"."+s.Name)
			}
			for _, a := range cdc.DataAttributes {
				add(a.Presence, cdc.Key()+"."+a.Name)
			}
			for _, sp := range cdc.ServiceParameters {
				add(sp.Presence, cdc.Key()+"."+sp.Name)
			}
		}
		for _, ca := range ns.ConstructedAttributes {
			for _, a := range ca.SubDataAttributes {
				add(a.Presence, ca.Name+"."+a.Name)
			}
		}
	}
	return names
}

// Every condition name in the embedded catalog has a rule: an evaluator,
// or a fact rule that is explicitly undetermined without the fact.
func TestCoverage_EveryCatalogConditionHasARule(t *testing.T) {
	names := conditionNames(t)
	if len(names) < 40 {
		t.Fatalf("only %d condition names in the catalog", len(names))
	}
	for name, where := range names {
		c, ok := presence.Describe(name)
		if !ok {
			t.Errorf("condition %s (%s) has no rule", name, strings.Join(where, ", "))
			continue
		}
		if c.Family == presence.FamilyFact && c.Needs != "fact" {
			t.Errorf("fact condition %s does not say it needs a fact", name)
		}
	}
	// And every rule is a condition some namespace defines.
	for _, c := range presence.Conditions() {
		if _, ok := names[c.Name]; !ok {
			t.Errorf("rule %s matches no condition of the catalog", c.Name)
		}
	}
}

// Every rule without a fact is undetermined and names it.
func TestCoverage_FactRulesAreUndeterminedWithoutTheFact(t *testing.T) {
	for _, c := range presence.Conditions() {
		if c.Family != presence.FamilyFact {
			continue
		}
		rep := presence.Evaluate("T", []presence.Element{{Name: "x", Presence: model.PresenceCondition{Name: c.Name, Args: "1"}}}, nil)
		r := rep.Results[0]
		if r.Requirement != presence.Undetermined || !strings.HasPrefix(r.Missing, "fact ") {
			t.Errorf("%s: %+v", c.Name, r)
		}
	}
}

// Only the known upstream anomaly is reported for the embedded
// namespaces: AEMC.VolFunRef is Mmulti with a sibling argument.
func TestValidateNamespace_EmbeddedCatalog(t *testing.T) {
	c, err := official.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ns := range c.Namespaces() {
		for _, d := range presence.ValidateNamespace(ns) {
			got = append(got, d.Code+" "+d.Path)
		}
	}
	want := []string{"presence-unexpected-argument /NS/LNClasses/LNClass[AEMC]/DataObject[VolFunRef]"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Every concrete class of the default and extended profiles evaluates,
// empty and with every data object present.
func TestEvaluator_EveryClass(t *testing.T) {
	for _, open := range []func() (*catalog.Profile, error){official.Default, official.Extended} {
		p, err := open()
		if err != nil {
			t.Fatal(err)
		}
		ev := presence.New(p)
		for _, ln := range p.ListLogicalNodes() {
			dos, err := p.DataObjectsIn(ln.Source, ln.Name)
			if err != nil {
				continue // ambiguous names are covered by Evaluator.In
			}
			all := &presence.Instance{}
			for _, d := range dos {
				all.Present = append(all.Present, d.Name)
			}
			for _, inst := range []*presence.Instance{nil, all} {
				rep, err := ev.In(ln.Source).LogicalNode(ln.Name, inst)
				if err != nil {
					t.Fatalf("%s: %v", ln.Name, err)
				}
				if len(rep.Results) != len(dos) {
					t.Errorf("%s: %d results for %d data objects", ln.Name, len(rep.Results), len(dos))
				}
			}
		}
	}
}

func TestEvaluator_LogicalNodeWithChildren(t *testing.T) {
	ev := presence.New(defaultProfile(t))
	inst := &presence.Instance{
		Present: []string{"Beh", "Pos", "NamPlt"},
		Context: presence.Context{LDNamespace: "IEC 61850-7-4:2007B"},
		Children: map[string]*presence.Instance{
			"Pos": {Present: []string{"stVal", "q", "t", "ctlModel", "subEna", "dataNs"},
				Values: map[string]string{"ctlModel": "sbo-with-enhanced-security"}},
			"NamPlt": {Present: []string{"vendor", "swRev"}},
		},
	}
	rep, err := ev.LogicalNode("CSWI", inst)
	if err != nil {
		t.Fatal(err)
	}
	got := byPath(rep)
	for path, w := range map[string]want{
		"Beh": reqSat, "Pos": reqSat,
		"Pos.stSeld": reqVio, "Pos.sboTimeout": reqVio, "Pos.operTimeout": reqVio,
		"Pos.subEna": reqSat, "Pos.subVal": reqVio,
		"Pos.dataNs":  optSat, // CSWI is a 7-4 class in a 7-4 logical device
		"NamPlt.lnNs": optSat, "NamPlt.ldNs": forSat, "NamPlt.configRev": optSat,
	} {
		r := got[path]
		if r.Requirement != w.req || r.Verdict != w.v {
			t.Errorf("%s: got %s/%s, want %s/%s (%s)", path, r.Requirement, r.Verdict, w.req, w.v, r.Reason)
		}
	}
	if len(rep.Children) != 2 || rep.Children[0].Class == "" {
		t.Errorf("children: %+v", rep.Children)
	}
	err = rep.Err()
	var pe *model.PresenceError
	if !errors.As(err, &pe) || !errors.Is(err, model.ErrPresenceViolation) || pe.Class != "CSWI" {
		t.Fatalf("Err() = %v", err)
	}
	if !strings.Contains(err.Error(), "Pos.sboTimeout: MOsbo") {
		t.Errorf("error lacks the violation: %v", err)
	}
	if rep.OK() || len(rep.Undetermined()) == 0 {
		t.Errorf("OK=%v undetermined=%d", rep.OK(), len(rep.Undetermined()))
	}
	if err := ev.Validate("CSWI", inst); !errors.Is(err, model.ErrPresenceViolation) {
		t.Errorf("Validate = %v", err)
	}
}

func TestEvaluator_LLN0AndRootLD(t *testing.T) {
	ev := presence.New(defaultProfile(t))
	root := true
	rep, err := ev.LogicalNode("LLN0", &presence.Instance{
		Present: []string{"Beh", "NamPlt"},
		Context: presence.Context{RootLD: &root, LDNamespace: "IEC 61850-7-4:2007B"},
		Children: map[string]*presence.Instance{
			"NamPlt": {Present: []string{"vendor", "swRev", "ldNs"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := byPath(rep)
	for path, w := range map[string]want{
		"NamPlt": reqSat, "Health": reqVio, "Mod": reqVio,
		"NamPlt.ldNs": reqSat, "NamPlt.configRev": reqVio,
	} {
		if r := got[path]; r.Requirement != w.req || r.Verdict != w.v {
			t.Errorf("%s: got %s/%s, want %s/%s (%s)", path, r.Requirement, r.Verdict, w.req, w.v, r.Reason)
		}
	}
}

func TestEvaluator_MultiInstanceAndUnknown(t *testing.T) {
	ev := presence.New(defaultProfile(t))
	rep, err := ev.LogicalNode("GGIO", &presence.Instance{Present: []string{"Beh", "Ind1", "Ind2", "Ind03", "XyzVendor"}})
	if err != nil {
		t.Fatal(err)
	}
	r := byPath(rep)["Ind"]
	if r.Verdict != presence.Violated || len(r.Instances) != 3 || !strings.Contains(r.Reason, "digits") {
		t.Errorf("Ind: %+v", r)
	}
	if len(rep.Unknown) != 1 || rep.Unknown[0] != "XyzVendor" {
		t.Errorf("unknown = %v", rep.Unknown)
	}
}

func TestEvaluator_CDCAndConstructed(t *testing.T) {
	ev := presence.New(defaultProfile(t))
	rep, err := ev.CDC("MV", "", &presence.Instance{Present: []string{"mag", "q", "t", "range"},
		Children: map[string]*presence.Instance{"mag": {Present: []string{"i"}}}})
	if err != nil {
		t.Fatal(err)
	}
	got := byPath(rep)
	if got["sVC"].Verdict != presence.Violated || got["rangeC"].Verdict != presence.Violated {
		t.Errorf("MV: sVC %+v, rangeC %+v", got["sVC"], got["rangeC"])
	}
	if len(rep.Children) != 1 || rep.Children[0].Class != "AnalogueValue" || got["mag.i"].Verdict != presence.Satisfied {
		t.Errorf("MV children: %+v", rep.Children)
	}

	// A CDC with variants: by key, by FC, by the instance class.
	for _, tc := range []struct {
		name string
		fc   model.FunctionalConstraint
		inst *presence.Instance
		want string
	}{
		{"SPG/SG", "", nil, "SPG/SG"},
		{"SPG", model.FCSE, nil, "SPG/SE"},
		{"SPG", "", &presence.Instance{Class: "SPG/SG"}, "SPG/SG"},
	} {
		rep, err := ev.CDC(tc.name, tc.fc, tc.inst)
		if err != nil || rep.Class != tc.want {
			t.Errorf("CDC(%s, %s): %v, %v", tc.name, tc.fc, rep, err)
		}
	}

	rep, err = ev.ConstructedAttribute("AnalogueValue", &presence.Instance{Present: []string{"f"}})
	if err != nil || byPath(rep)["i"].Requirement != presence.Optional {
		t.Errorf("AnalogueValue: %v %v", rep, err)
	}
	// Nested constructed members: Vector.mag → AnalogueValue.
	rep, err = ev.ConstructedAttribute("Vector", &presence.Instance{Present: []string{"mag"},
		Children: map[string]*presence.Instance{"mag": {Present: []string{"f"}}}})
	if err != nil || len(rep.Children) != 1 || rep.Children[0].Path != "mag" {
		t.Errorf("Vector: %+v %v", rep, err)
	}
	// Sub data objects: WYE.phsA → CMV.
	rep, err = ev.CDC("WYE", "", &presence.Instance{Present: []string{"phsA"},
		Children: map[string]*presence.Instance{"phsA": {Present: []string{"cVal", "q", "t"}}}})
	if err != nil || len(rep.Children) != 1 || rep.Children[0].Class != "CMV" {
		t.Errorf("WYE: %+v %v", rep, err)
	}
}

func TestEvaluator_Check(t *testing.T) {
	ev := presence.New(defaultProfile(t))
	for class, want := range map[string]string{"MMXU": "MMXU", "MV": "MV", "Vector": "Vector"} {
		rep, err := ev.Check(&presence.Instance{Class: class})
		if err != nil || rep.Class != want {
			t.Errorf("Check(%s) = %v, %v", class, rep, err)
		}
	}
	if _, err := ev.Check(&presence.Instance{Class: "NOPE"}); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("unknown class: %v", err)
	}
	if _, err := ev.Check(&presence.Instance{}); !errors.Is(err, model.ErrInvalidInstance) {
		t.Errorf("no class: %v", err)
	}
	if _, err := ev.Check(nil); !errors.Is(err, model.ErrInvalidInstance) {
		t.Errorf("nil: %v", err)
	}
}

func TestEvaluator_Errors(t *testing.T) {
	ev := presence.New(defaultProfile(t))
	if _, err := ev.LogicalNode("NOPE", nil); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown LN: %v", err)
	}
	if err := ev.Validate("NOPE", nil); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("Validate unknown LN: %v", err)
	}
	if _, err := ev.CDC("NOPE", "", nil); !errors.Is(err, model.ErrUnknownCDC) {
		t.Errorf("unknown CDC: %v", err)
	}
	if _, err := ev.ConstructedAttribute("NOPE", nil); err == nil {
		t.Error("unknown constructed type: no error")
	}
	ns, _ := model.ParseNamespaceID("IEC 99999")
	if _, err := ev.In(ns).LogicalNode("MMXU", nil); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("MMXU in an unloaded namespace: %v", err)
	}
}

// loopLookup defines a constructed type that contains itself and a CDC
// whose constructed member names a missing type.
type loopLookup struct{ *catalog.Profile }

var loopNS = model.NamespaceID{Standard: "X", Version: "2024", Revision: "A"}

func (loopLookup) ConstructedAttribute(name string) (*model.ConstructedAttribute, error) {
	return loopLookup{}.ConstructedAttributeFrom(loopNS, name)
}

func (loopLookup) ConstructedAttributeFrom(_ model.NamespaceID, name string) (*model.ConstructedAttribute, error) {
	if name == "Loop" {
		return &model.ConstructedAttribute{Name: "Loop", Source: loopNS, SubDataAttributes: []model.DataAttribute{
			{Name: "next", Type: "Loop", TypeKind: model.TypeKindConstructed, Presence: model.PresenceCondition{Name: "O"}},
			{Name: "gone", Type: "Gone", TypeKind: model.TypeKindConstructed, Presence: model.PresenceCondition{Name: "O"}},
		}}, nil
	}
	return nil, &model.ResolutionError{Element: name, Err: model.ErrUnknownType}
}

func (l loopLookup) CDCVariants(name string) ([]*model.CommonDataClass, error) {
	return l.CDCVariantsFrom(loopNS, name)
}

func (loopLookup) CDCVariantsFrom(_ model.NamespaceID, name string) ([]*model.CommonDataClass, error) {
	if name != "BAD" {
		return nil, &model.ResolutionError{Element: name, Err: model.ErrUnknownCDC}
	}
	return []*model.CommonDataClass{{Name: "BAD", Source: loopNS,
		SubDataObjects: []model.DataObject{{Name: "sub", Type: "NOCDC", Presence: model.PresenceCondition{Name: "O"}}},
		DataAttributes: []model.DataAttribute{{Name: "ca", Type: "Gone", TypeKind: model.TypeKindConstructed, Presence: model.PresenceCondition{Name: "O"}}},
	}}, nil
}

func TestEvaluator_Limits(t *testing.T) {
	ev := presence.New(loopLookup{defaultProfile(t)})
	deep := &presence.Instance{}
	cur := deep
	for range presence.MaxDepth + 2 {
		next := &presence.Instance{}
		cur.Children = map[string]*presence.Instance{"next": next}
		cur = next
	}
	if _, err := ev.ConstructedAttribute("Loop", deep); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("deep nesting: %v", err)
	}
	if _, err := ev.ConstructedAttribute("Loop", &presence.Instance{Children: map[string]*presence.Instance{"gone": {}}}); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("missing member type: %v", err)
	}
	if _, err := ev.CDC("BAD", "", &presence.Instance{Children: map[string]*presence.Instance{"ca": {}}}); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("missing attribute type: %v", err)
	}
	if _, err := ev.CDC("BAD", "", &presence.Instance{Children: map[string]*presence.Instance{"sub": {}}}); !errors.Is(err, model.ErrUnknownCDC) {
		t.Errorf("missing sub data object CDC: %v", err)
	}
	// A child of a basic member is not followed; a child for an
	// unknown name is reported as unknown.
	rep, err := presence.New(defaultProfile(t)).CDC("MV", "", &presence.Instance{
		Children: map[string]*presence.Instance{"q": {Present: []string{"x"}}, "zzz": {}}})
	if err != nil || len(rep.Children) != 0 || len(rep.Unknown) != 1 {
		t.Errorf("MV: %+v %v", rep, err)
	}
	// A data object whose CDC is unknown.
	if _, err := presence.New(loopLookup{defaultProfile(t)}).LogicalNode("MMXU",
		&presence.Instance{Children: map[string]*presence.Instance{"Hz": {}}}); !errors.Is(err, model.ErrUnknownCDC) {
		t.Errorf("unknown CDC of a data object: %v", err)
	}
}

func TestEvaluator_In(t *testing.T) {
	p, err := official.Extended()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, id := range p.IDs() {
		ids = append(ids, id.String())
	}
	sort.Strings(ids)
	ns, _ := model.ParseNamespaceID("IEC 61850-7-4")
	rep, err := presence.New(p).In(ns).LogicalNode("MMXU", nil)
	if err != nil || rep.Class != "MMXU" {
		t.Errorf("In(7-4) MMXU: %v %v (%v)", rep, err, ids)
	}
}

func TestReport_Accessors(t *testing.T) {
	rep := presence.Evaluate("T", els("a=M", "b=MOsbo"), present("b"))
	if rep.Err() == nil || rep.OK() || len(rep.Violations()) != 1 || len(rep.Undetermined()) != 0 {
		t.Errorf("report: %+v", rep)
	}
	if s := rep.Results[0].String(); !strings.HasPrefix(s, "a: M: required, violated") {
		t.Errorf("String() = %q", s)
	}
	ok := presence.Evaluate("T", els("a=O"), nil)
	if ok.Err() != nil || !ok.OK() {
		t.Errorf("empty report: %v", ok.Err())
	}
	single := &model.PresenceError{Class: "X", Violations: []string{"a"}}
	if !strings.Contains(single.Error(), "1 presence violation:") {
		t.Errorf("singular: %s", single.Error())
	}
}

func TestConditions(t *testing.T) {
	cs := presence.Conditions()
	if !sort.SliceIsSorted(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name }) {
		t.Error("Conditions not sorted")
	}
	if !presence.Known("MOsbo") || presence.Known("nope") {
		t.Error("Known")
	}
	if _, ok := presence.Describe("nope"); ok {
		t.Error("Describe(nope)")
	}
	if !presence.IsMultiInstance("MmultiF") || presence.IsMultiInstance("M") || presence.IsMultiInstance("nope") {
		t.Error("IsMultiInstance")
	}
	for _, tc := range []struct {
		in     string
		lo, hi int
		ok     bool
	}{{"33, 48", 33, 48, true}, {"1,1", 1, 1, true}, {"5, 4", 0, 0, false}, {"x, 4", 0, 0, false}, {"-1, 4", 0, 0, false}, {"3", 0, 0, false}} {
		lo, hi, ok := presence.ParseRange(tc.in)
		if lo != tc.lo || hi != tc.hi || ok != tc.ok {
			t.Errorf("ParseRange(%q) = %d, %d, %v", tc.in, lo, hi, ok)
		}
	}
}
