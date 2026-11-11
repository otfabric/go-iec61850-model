// SPDX-License-Identifier: MIT

package presence_test

import (
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/presence"
)

// The tests in this file use synthetic element lists: each one names a
// presence condition of IEC 61850-7-3 Table 1 (Edition 2) or its NSD
// name (7-2 PresenceConditions, as docs/model.md maps them) and checks
// the requirement and verdict for a set of present elements.

// cond parses "Name(args)".
func cond(s string) model.PresenceCondition {
	name, args, ok := strings.Cut(s, "(")
	if !ok {
		return model.PresenceCondition{Name: s}
	}
	return model.PresenceCondition{Name: name, Args: strings.TrimSuffix(args, ")")}
}

// els builds elements from "name=condition" pairs.
func els(pairs ...string) []presence.Element {
	out := make([]presence.Element, len(pairs))
	for i, p := range pairs {
		n, c, _ := strings.Cut(p, "=")
		out[i] = presence.Element{Name: n, Kind: presence.KindDataAttribute, Presence: cond(c)}
	}
	return out
}

type want struct {
	req presence.Requirement
	v   presence.Verdict
}

var (
	reqSat  = want{presence.Required, presence.Satisfied}
	reqVio  = want{presence.Required, presence.Violated}
	optSat  = want{presence.Optional, presence.Satisfied}
	forSat  = want{presence.Forbidden, presence.Satisfied}
	forVio  = want{presence.Forbidden, presence.Violated}
	undUnd  = want{presence.Undetermined, presence.Unresolved}
	undSat  = want{presence.Undetermined, presence.Satisfied}
	nothing = []string{}
)

// check evaluates elems against inst and compares each named result.
func check(t *testing.T, elems []presence.Element, inst *presence.Instance, wants map[string]want) *presence.Report {
	t.Helper()
	rep := presence.Evaluate("T", elems, inst)
	got := map[string]presence.Result{}
	for _, r := range rep.Results {
		got[r.Path] = r
	}
	for name, w := range wants {
		r, ok := got[name]
		if !ok {
			t.Errorf("%s: no result", name)
			continue
		}
		if r.Requirement != w.req || r.Verdict != w.v {
			t.Errorf("%s: got %s/%s, want %s/%s (%s)", name, r.Requirement, r.Verdict, w.req, w.v, r.Reason)
		}
		if r.Reason == "" {
			t.Errorf("%s: empty reason", name)
		}
		if r.Requirement == presence.Undetermined && r.Missing == "" {
			t.Errorf("%s: undetermined without the missing fact", name)
		}
	}
	return rep
}

func present(names ...string) *presence.Instance { return &presence.Instance{Present: names} }

// 7-3 Table 1: M, O; NSD F and na.
func TestSpec_7_3_Table1_Unconditional(t *testing.T) {
	e := els("a=M", "b=M", "c=O", "d=O", "e=F", "f=F", "g=na")
	check(t, e, present("a", "c", "e"), map[string]want{
		"a": reqSat, "b": reqVio, "c": optSat, "d": optSat, "e": forVio, "f": forSat, "g": optSat,
	})
}

// Multi-instance data objects (7-2 §22.2 Data-Instance-ID; NSD Mmulti,
// Omulti, MmultiRange, OmultiRange).
func TestSpec_7_2_22_2_MultiInstance(t *testing.T) {
	e := els("Ind=Omulti", "Alm=Mmulti", "Chr=OmultiRange(33, 48)", "Set=MmultiRange(1, 4)", "Plain=O")
	rep := check(t, e, present("Ind1", "Ind2", "Chr33", "Set1", "Plain2"), map[string]want{
		"Ind": optSat, "Alm": reqVio, "Chr": optSat, "Set": reqSat,
	})
	if got := rep.Results[0].Instances; len(got) != 2 || got[0] != "Ind1" {
		t.Errorf("instances = %v", got)
	}
	if len(rep.Unknown) != 1 || rep.Unknown[0] != "Plain2" {
		t.Errorf("unknown = %v, want [Plain2]: only multi-instance elements take a number", rep.Unknown)
	}
	// Out of range and mixed widths.
	check(t, e, present("Chr32", "Set1", "Set02", "Alm"), map[string]want{
		"Chr": {presence.Optional, presence.Violated}, "Set": {presence.Required, presence.Violated}, "Alm": reqSat,
	})
	// A malformed range is not enforced.
	check(t, els("X=OmultiRange(9)"), present("X1"), map[string]want{"X": optSat})
}

// NSD MF, MO, OF, OM, FM and MmultiF take a sibling name (7-3 Table 1
// GC_CON_attr is the OF form: a configuration attribute only with its
// attribute).
func TestSpec_7_3_Table1_Sibling(t *testing.T) {
	e := els("range=O", "rangeC=MO(range)", "db=O", "dbRef=MF(db)", "frVal=O", "strTm=OF(frVal)",
		"x=O", "y=OM(x)", "z=FM(x)", "Spt=Omulti", "Ref=MmultiF(Spt)", "bad=MO()")
	check(t, e, present("range", "strTm", "Spt2"), map[string]want{
		"rangeC": reqVio, "dbRef": forSat, "strTm": forVio, "y": reqVio, "z": reqVio, "Ref": reqVio,
		"bad": undUnd,
	})
	check(t, e, present("rangeC", "db", "dbRef", "frVal", "strTm", "x", "y", "z"), map[string]want{
		"rangeC": optSat, "dbRef": reqSat, "strTm": optSat, "y": optSat, "z": forVio, "Ref": forSat,
	})
	// A sibling outside the element list counts by its presence.
	check(t, els("a=MF(other)"), present("a", "other"), map[string]want{"a": reqSat})
}

// 7-3 Table 1 GC_1 is NSD AtLeastOne(n).
func TestSpec_7_3_Table1_AtLeastOne(t *testing.T) {
	e := els("phsA=AtLeastOne(1)", "phsB=AtLeastOne(1)", "neut=AtLeastOne(2)")
	check(t, e, present("phsB"), map[string]want{"phsA": optSat, "phsB": reqSat, "neut": reqVio})
	rep := check(t, e, present(), map[string]want{"phsA": reqVio, "phsB": reqVio})
	if !strings.Contains(rep.Results[0].Reason, "phsA, phsB") {
		t.Errorf("reason does not name the group: %s", rep.Results[0].Reason)
	}
}

// 7-3 Table 1 GC_1_EXCL is NSD AtMostOne.
func TestSpec_7_3_Table1_AtMostOne(t *testing.T) {
	e := els("a=AtMostOne", "b=AtMostOne", "c=AtMostOne")
	check(t, e, present("a"), map[string]want{"a": optSat, "b": forSat, "c": forSat})
	check(t, e, present("a", "b"), map[string]want{"a": forVio, "b": forVio, "c": forSat})
	check(t, e, present(nothing...), map[string]want{"a": optSat})
}

// 7-3 Table 1 GC_2_n is NSD AllOrNonePerGroup(n).
func TestSpec_7_3_Table1_AllOrNonePerGroup(t *testing.T) {
	e := els("phsA=AllOrNonePerGroup(1)", "dirPhsA=AllOrNonePerGroup(1)", "phsB=AllOrNonePerGroup(2)", "dirPhsB=AllOrNonePerGroup(2)")
	check(t, e, present("phsA"), map[string]want{"phsA": reqSat, "dirPhsA": reqVio, "phsB": optSat, "dirPhsB": optSat})
	check(t, e, present("phsA", "dirPhsA"), map[string]want{"phsA": reqSat, "dirPhsA": reqSat})
}

// 7-3 Table 1 GC_2_XOR_n is NSD AllOnlyOneGroup(n): one group, complete.
func TestSpec_7_3_Table1_AllOnlyOneGroup(t *testing.T) {
	e := els("i=AllOnlyOneGroup(1)", "f=AllOnlyOneGroup(2)", "x=AllOnlyOneGroup(2)")
	check(t, e, present("i"), map[string]want{"i": reqSat, "f": forSat, "x": forSat})
	check(t, e, present("f"), map[string]want{"i": forSat, "f": reqSat, "x": reqVio})
	check(t, e, present("i", "f", "x"), map[string]want{"i": forVio, "f": forVio, "x": forVio})
	check(t, e, present(), map[string]want{"i": reqVio, "f": reqVio})
}

// NSD AllAtLeastOneGroup(n): every group all or none, at least one
// group complete (BCR: actual value with its time stamp, frozen value
// with its settings).
func TestSpec_7_3_Table1_AllAtLeastOneGroup(t *testing.T) {
	e := els("actVal=AllAtLeastOneGroup(2)", "t=AllAtLeastOneGroup(2)", "frVal=AllAtLeastOneGroup(1)", "frTm=AllAtLeastOneGroup(1)")
	check(t, e, present("actVal", "t"), map[string]want{"actVal": reqSat, "t": reqSat, "frVal": optSat, "frTm": optSat})
	check(t, e, present("actVal", "t", "frVal"), map[string]want{"frTm": reqVio})
	check(t, e, present("actVal"), map[string]want{"t": reqVio, "frVal": optSat})
	check(t, e, present(), map[string]want{"actVal": reqVio, "frVal": reqVio})
}

// NSD MAllOrNonePerGroup(n) (7-3 Table 1 AC_ST): the status value,
// quality and time stamp of a controllable data object are present
// together, and required when it has status information.
func TestSpec_7_3_Table1_MAllOrNonePerGroup(t *testing.T) {
	e := els("stVal=MAllOrNonePerGroup(1)", "q=MAllOrNonePerGroup(1)", "t=MAllOrNonePerGroup(1)")
	check(t, e, present("stVal"), map[string]want{"stVal": reqSat, "q": reqVio, "t": reqVio})
	rep := check(t, e, present(), map[string]want{"stVal": undUnd})
	if rep.Results[0].Missing != "fact MAllOrNonePerGroup(1)" {
		t.Errorf("missing = %q", rep.Results[0].Missing)
	}
	status := &presence.Instance{Values: map[string]string{"ctlModel": "status-only"}}
	check(t, e, status, map[string]want{"stVal": reqVio})
	fact := &presence.Instance{Context: presence.Context{Facts: map[string]bool{"MAllOrNonePerGroup(1)": true}}}
	check(t, e, fact, map[string]want{"stVal": reqVio})
	fact.Context.Facts["MAllOrNonePerGroup(1)"] = false
	check(t, e, fact, map[string]want{"stVal": optSat})
}

// 7-3 Table 1 AC_LN0_M / AC_LN0_EX are NSD MOln0 / MFln0; NSD MOrootLD.
func TestSpec_7_3_Table1_LN0AndRootLD(t *testing.T) {
	e := els("configRev=MOln0", "ldNs=MFln0", "Health=MOrootLD")
	check(t, e, present("ldNs"), map[string]want{"configRev": undUnd, "ldNs": undUnd, "Health": undUnd})
	yes, no := true, false
	ln0 := &presence.Instance{Present: []string{"ldNs"}, Context: presence.Context{LNClass: "LLN0", RootLD: &yes}}
	check(t, e, ln0, map[string]want{"configRev": reqVio, "ldNs": reqSat, "Health": reqVio})
	other := &presence.Instance{Present: []string{"ldNs", "Health"}, Context: presence.Context{LNClass: "MMXU", RootLD: &no}}
	check(t, e, other, map[string]want{"configRev": optSat, "ldNs": forVio, "Health": optSat})
}

// 7-3 Table 1 AC_DLD_M, AC_DLN_M and AC_DLNDA_M are NSD MOlnNs,
// MOdataNs and MOcdcNs: a namespace attribute is required when the
// namespace differs from the one above it. Releases do not count.
func TestSpec_7_3_Table1_Namespaces(t *testing.T) {
	cdcNs := model.NamespaceID{Standard: "IEC 61850-7-3", Version: "2007", Revision: "B", Release: 6}
	e := els("lnNs=MOlnNs", "dataNs=MOdataNs", "cdcNs=MOcdcNs")
	e[2].Namespace = cdcNs
	check(t, e, present("lnNs"), map[string]want{"lnNs": undSat, "dataNs": undUnd, "cdcNs": undUnd})
	same := &presence.Instance{Context: presence.Context{
		LDNamespace: "IEC 61850-7-4:2007B", LNNamespace: "IEC 61850-7-4:2007B6",
		DataNamespace: "IEC 61850-7-4:2007B5"}}
	check(t, e, same, map[string]want{"lnNs": optSat, "dataNs": optSat, "cdcNs": reqVio})
	ext := &presence.Instance{Present: []string{"lnNs", "dataNs", "cdcNs"}, Context: presence.Context{
		LDNamespace: "IEC 61850-7-4:2007B", LNNamespace: "IEC 61850-7-420:2019A",
		DataNamespace: "IEC 61850-7-3:2007B"}}
	check(t, e, ext, map[string]want{"lnNs": reqSat, "dataNs": reqSat, "cdcNs": optSat})
	// dataNs falls back to the logical device; partial context.
	check(t, e, &presence.Instance{Context: presence.Context{LDNamespace: "x"}}, map[string]want{"lnNs": undUnd, "dataNs": undUnd})
	check(t, e, &presence.Instance{Context: presence.Context{LNNamespace: "x"}}, map[string]want{"lnNs": undUnd})
	check(t, e, &presence.Instance{Context: presence.Context{DataNamespace: "x"}}, map[string]want{"dataNs": undUnd})
	check(t, els("cdcNs=MOcdcNs"), &presence.Instance{Context: presence.Context{DataNamespace: "x"}}, map[string]want{"cdcNs": undUnd})
}

// 7-3 Table 1 AC_CO_SBO is NSD MOsbo; NSD MOenhanced. ctlModel by name
// or by CtlModelKind number, from the values or the context.
func TestSpec_7_3_Table1_ControlModel(t *testing.T) {
	e := els("sboTimeout=MOsbo", "operTimeout=MOenhanced")
	for _, tc := range []struct {
		model   string
		sbo, en want
	}{
		{"status-only", optSat, optSat},
		{"direct-with-normal-security", optSat, optSat},
		{"sbo-with-normal-security", reqVio, optSat},
		{"direct-with-enhanced-security", optSat, reqVio},
		{"sbo-with-enhanced-security", reqVio, reqVio},
		{"4", reqVio, reqVio},
		{"9", undUnd, undUnd},
	} {
		t.Run(tc.model, func(t *testing.T) {
			check(t, e, &presence.Instance{Values: map[string]string{"ctlModel": tc.model}},
				map[string]want{"sboTimeout": tc.sbo, "operTimeout": tc.en})
		})
	}
	check(t, e, &presence.Instance{Context: presence.Context{CtlModel: "sbo-with-normal-security"}},
		map[string]want{"sboTimeout": reqVio})
	rep := check(t, e, present(), map[string]want{"sboTimeout": undUnd})
	if rep.Results[0].Missing != "ctlModel" {
		t.Errorf("missing = %q", rep.Results[0].Missing)
	}
}

// 7-3 Table 1 AC_RMS_M is NSD MOrms.
func TestSpec_7_3_Table1_RMS(t *testing.T) {
	e := els("rmsCyc=MOrms")
	for v, w := range map[string]want{"rms": reqVio, "2": reqVio, "fundamental": optSat, "3": optSat, "peak": undUnd} {
		check(t, e, &presence.Instance{Values: map[string]string{"hvRef": v}}, map[string]want{"rmsCyc": w})
	}
	check(t, e, present("rmsCyc"), map[string]want{"rmsCyc": undSat})
}

// 7-3 Table 1 PICS_SUBST is NSD MFsubst: one substitution attribute
// shows that the data object supports substitution.
func TestSpec_7_3_Table1_Substitution(t *testing.T) {
	e := els("subEna=MFsubst", "subVal=MFsubst")
	check(t, e, present("subEna"), map[string]want{"subEna": reqSat, "subVal": reqVio})
	check(t, e, present(), map[string]want{"subEna": undUnd, "subVal": undUnd})
	check(t, e, &presence.Instance{Context: presence.Context{Facts: map[string]bool{"MFsubst": true}}},
		map[string]want{"subEna": reqVio})
	check(t, e, &presence.Instance{Context: presence.Context{Facts: map[string]bool{"MFsubst": false}}},
		map[string]want{"subEna": forSat})
}

// 7-3 Table 1 AC_SCAV is NSD MFscaledAV (MFscaledMagV, MFscaledAngV
// for Vector magnitude and angle): the scaled value configuration is
// required when the value has its integer member i.
func TestSpec_7_3_Table1_ScaledValues(t *testing.T) {
	av := []presence.Element{
		{Name: "mag", Type: "AnalogueValue", Presence: cond("M")},
		{Name: "sVC", Type: "ScaledValueConfig", Presence: cond("MFscaledAV")},
	}
	check(t, av, present("mag", "mag.i", "mag.f"), map[string]want{"sVC": reqVio})
	check(t, av, present("mag", "mag.f"), map[string]want{"sVC": forSat})
	check(t, av, present("mag"), map[string]want{"sVC": undUnd})
	check(t, av, present(), map[string]want{"sVC": forSat})
	children := &presence.Instance{Children: map[string]*presence.Instance{"mag": {Present: []string{"i"}}}}
	check(t, av, children, map[string]want{"sVC": reqVio})

	vec := []presence.Element{
		{Name: "cVal", Type: "Vector", Presence: cond("M")},
		{Name: "magSVC", Type: "ScaledValueConfig", Presence: cond("MFscaledMagV")},
		{Name: "angSVC", Type: "ScaledValueConfig", Presence: cond("MFscaledAngV")},
	}
	check(t, vec, present("cVal", "cVal.mag.i", "cVal.ang.f", "magSVC"), map[string]want{"magSVC": reqSat, "angSVC": forSat})
	nested := &presence.Instance{Children: map[string]*presence.Instance{
		"cVal": {Present: []string{"mag", "ang"}, Children: map[string]*presence.Instance{"ang": {Present: []string{"i"}}}},
	}}
	check(t, vec, nested, map[string]want{"magSVC": undUnd, "angSVC": reqVio})
}

// NSD MOcond, MFcond and OFcond refer to a condition the class text
// states; only a fact decides them. A fact keyed by element wins.
func TestSpec_7_4_ConditionIDs(t *testing.T) {
	e := els("Mir=MOcond(1)", "KP=MFcond(1)", "PAct=OFcond(2)")
	e[0].Presence.ArgsText = "the mirror text"
	rep := check(t, e, present("PAct"), map[string]want{"Mir": undUnd, "KP": undUnd, "PAct": undUnd})
	if r := rep.Results[0]; r.Missing != "fact MOcond(1)" || !strings.Contains(r.Reason, "the mirror text") {
		t.Errorf("Mir: missing %q, reason %q", r.Missing, r.Reason)
	}
	facts := map[string]bool{"MOcond(1)": true, "MFcond(1)": true, "KP:MFcond(1)": false, "OFcond(2)": false}
	check(t, e, &presence.Instance{Present: []string{"PAct"}, Context: presence.Context{Facts: facts}},
		map[string]want{"Mir": reqVio, "KP": forSat, "PAct": forVio})
}

// NSD MOoperTm, MONamPlt, MORange, OMSynPh, MFsg and MFnsg depend on
// facts the model does not show.
func TestSpec_7_3_FactConditions(t *testing.T) {
	e := els("operTm=MOoperTm", "NamPlt=MONamPlt", "hhLim=MORange", "c2=OMSynPh", "_setMag=MFsg", "setMag=MFnsg")
	check(t, e, present("c2"), map[string]want{
		"operTm": undUnd, "NamPlt": undUnd, "hhLim": undUnd, "c2": undSat, "setMag": undUnd,
	})
	facts := map[string]bool{"MOoperTm": true, "OMSynPh": true, "MFsg": false, "MFnsg": true}
	check(t, e, &presence.Instance{Context: presence.Context{Facts: facts}},
		map[string]want{"operTm": reqVio, "c2": reqVio, "setMag": reqVio})
}

// Conditions the evaluator has no rule for stay undetermined.
func TestEvaluate_UnknownCondition(t *testing.T) {
	e := append(els("a=Mystery(3)"), presence.Element{Name: "b"})
	rep := presence.Evaluate("T", e, nil)
	if r := rep.Results[0]; r.Requirement != presence.Undetermined || r.Missing != "rule for Mystery" {
		t.Errorf("unknown condition: %+v", r)
	}
	if r := rep.Results[1]; r.Requirement != presence.Undetermined || r.Missing != "presence condition" {
		t.Errorf("no condition: %+v", r)
	}
}
