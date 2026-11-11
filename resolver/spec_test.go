// SPDX-License-Identifier: MIT

package resolver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// Spec-verification tests against the embedded official namespaces
// (IEC 61850-7-2/7-3/7-4 2007B6).

func officialResolver(t *testing.T) *resolver.Resolver {
	t.Helper()
	p, err := official.Default()
	if err != nil {
		t.Fatal(err)
	}
	return resolver.New(p)
}

// IEC 61850-8-1:2011 §20.3 and Tables 80, 83, 85, 88 and 90: the
// control service parameters are mapped within the controllable data
// object with FC CO as SBO, SBOw, Oper and Cancel; ctlVal is typed by
// the CDC (7-3:2010 Table 40 onwards, "parameters for control
// services"). MDL-01.
func TestSpec_8_1_20_3_ControlServiceParameters(t *testing.T) {
	r := officialResolver(t)
	tests := []struct {
		ref, typ string
		kind     model.TypeKind
		param    bool
	}{
		{"LD0/XCBR1.Pos.Oper.ctlVal[CO]", "BOOLEAN", model.TypeKindBasic, false},              // DPC
		{"LD0/CSWI1.Pos.SBOw.ctlVal[CO]", "BOOLEAN", model.TypeKindBasic, false},              // DPC, SBO with enhanced security
		{"LD0/CSWI1.Pos.Cancel.ctlVal[CO]", "BOOLEAN", model.TypeKindBasic, false},            // DPC
		{"LD0/LLN0.Mod.Oper.ctlVal[CO]", "BehaviourModeKind", model.TypeKindEnumerated, true}, // ENC, parameterized
		{"XCBR.BlkOpn.Oper.ctlVal[CO]", "BOOLEAN", model.TypeKindBasic, false},                // SPC
		{"ATCC.TapChg.Oper.ctlVal[CO]", "Tcmd", model.TypeKindBasic, false},                   // BSC
		{"YLTC.TapPos.Oper.ctlVal[CO]", "INT8", model.TypeKindBasic, false},                   // ISC
		{"GGIO.AnOut1.Oper.ctlVal.f[CO]", "FLOAT32", model.TypeKindBasic, false},              // APC: AnalogueValueCtl.f
		{"XCBR.Pos.Oper.origin.orCat[CO]", "OriginatorCategoryKind", model.TypeKindEnumerated, false},
		{"XCBR.Pos.Oper.origin.orIdent", "Octet64", model.TypeKindBasic, false},
		{"XCBR.Pos.Oper.operTm[CO]", "Timestamp", model.TypeKindBasic, false},
		{"XCBR.Pos.Oper.ctlNum[CO]", "INT8U", model.TypeKindBasic, false},
		{"XCBR.Pos.Oper.T[CO]", "Timestamp", model.TypeKindBasic, false},
		{"XCBR.Pos.Oper.Test[CO]", "BOOLEAN", model.TypeKindBasic, false},
		{"XCBR.Pos.Oper.Check[CO]", "Check", model.TypeKindBasic, false},
		{"XCBR.Pos.SBO[CO]", "VisString129", model.TypeKindBasic, false}, // Table 83
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			res, err := r.ResolveString(tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			if res.Type() != tc.typ || res.TypeKind() != tc.kind || res.FC() != model.FCCO || !res.IsAttribute() {
				t.Errorf("leaf = %s %s %s, want %s %s CO", res.Type(), res.TypeKind(), res.FC(), tc.typ, tc.kind)
			}
			if tc.param != res.Leaf().Parameterized && res.Leaf().Kind == resolver.StepServiceParameter {
				t.Errorf("parameterized = %v", res.Leaf().Parameterized)
			}
			if !strings.HasPrefix(res.AttributePath(), res.Steps[2].Name) || res.Steps[2].Kind != resolver.StepControlService {
				t.Errorf("steps: %+v", res.Steps)
			}
			if got := res.ClassRef().FC; got != model.FCCO {
				t.Errorf("ClassRef FC = %s", got)
			}
		})
	}
}

func TestSpec_8_1_20_3_ControlServiceErrors(t *testing.T) {
	r := officialResolver(t)
	tests := []struct {
		ref    string
		reason string
	}{
		{"XCBR.Pos.Oper.ctlVal[ST]", "functional constraint CO"},
		{"XCBR.Pos.ctlVal[CO]", "Oper.ctlVal[CO]"},      // the 7-2 name is mapped below Oper
		{"XCBR.Pos.Cancel.Check", "no parameter Check"}, // Table 88 has no Check
		{"XCBR.Pos.Oper.stVal", "no parameter stVal"},   // not a Table 80 parameter
		{"MMXU.Hz.Oper", "no member Oper"},              // MV is not controllable
		{"XCBR.Pos.SBO.x", "has no members"},            // SBO is a VISIBLE STRING129
		{"XCBR.Pos.Oper.ctlNum.x", "has no members"},    // basic type
		{"XCBR.Pos.Oper(1)", "not an array"},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			_, err := r.ResolveString(tc.ref)
			if !errors.Is(err, model.ErrUnknownAttribute) || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("error = %v, want ErrUnknownAttribute with %q", err, tc.reason)
			}
		})
	}
	if res, err := r.ResolveString("LD0/XCBR1.Pos[CO]"); err != nil || res.IsAttribute() {
		t.Errorf("a controllable data object has FC CO members: %v", err)
	}
	if _, err := r.ResolveString("LD0/MMXU1.Hz[CO]"); !errors.Is(err, model.ErrUnknownAttribute) {
		t.Errorf("MV has no FC CO members: %v", err)
	}
}

func TestSpec_8_1_Table81_LeavesIncludeControlServices(t *testing.T) {
	r := officialResolver(t)
	co, err := r.Leaves("XCBR", []string{"Pos"}, model.FCCO)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range co {
		if l.FC() != model.FCCO {
			t.Errorf("%s has FC %s", l.Path(), l.FC())
		}
		got = append(got, l.AttributePath())
	}
	want := "SBO SBOw.ctlVal SBOw.operTm SBOw.origin.orCat SBOw.origin.orIdent SBOw.ctlNum SBOw.T SBOw.Test SBOw.Check " +
		"Oper.ctlVal Oper.operTm Oper.origin.orCat Oper.origin.orIdent Oper.ctlNum Oper.T Oper.Test Oper.Check " +
		"Cancel.ctlVal Cancel.operTm Cancel.origin.orCat Cancel.origin.orIdent Cancel.ctlNum Cancel.T Cancel.Test"
	if strings.Join(got, " ") != want {
		t.Errorf("CO leaves\n got: %s\nwant: %s", strings.Join(got, " "), want)
	}
	all, err := r.Leaves("XCBR", []string{"Pos"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) <= len(co) || all[len(all)-1].AttributePath() != "Cancel.Test" {
		t.Errorf("leaves without FC end with the control services: %d", len(all))
	}
	st, err := r.Leaves("XCBR", []string{"Pos"}, model.FCST)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range st {
		if l.FC() != model.FCST {
			t.Errorf("%s has FC %s", l.Path(), l.FC())
		}
	}
	if _, err := r.Leaves("XCBR", []string{"Pos", "Oper"}, ""); err != nil {
		t.Errorf("leaves of Oper: %v", err)
	}
}

// IEC 61850-7-2:2010 §6.1.2.3 ("The "(…)" shall indicate an array
// element") and §12.3.3.2.3 ("[MX] FCDA phsAHar(3).cVal"). MDL-02.
func TestSpec_7_2_6_1_2_3_ArrayElements(t *testing.T) {
	r := officialResolver(t)
	res, err := r.ResolveString("LD0/MHAI1.HA.phsAHar(3).cVal.mag.f[MX]")
	if err != nil {
		t.Fatal(err)
	}
	sdo := res.Steps[2]
	if sdo.Name != "phsAHar(3)" || sdo.Index != "3" || !sdo.IsArray || res.Type() != "FLOAT32" {
		t.Errorf("array element step = %+v", sdo)
	}
	if res.Path() != "MHAI.HA.phsAHar(3).cVal.mag.f" || res.ClassRef().String() != "MHAI.HA.phsAHar(3).cVal.mag.f[MX]" {
		t.Errorf("path = %s", res.Path())
	}
	if _, err := r.ResolveString("MHAI.HA.phsAHar.cVal.mag.f"); err != nil {
		t.Errorf("the whole array resolves without an index: %v", err)
	}
	if _, err := r.ResolveString("FCSD.Crv.crvPts(0).xVal"); err != nil {
		t.Errorf("array data attribute element: %v", err)
	}
	for ref, sentinel := range map[string]error{
		"MMXU.Hz(0)":                   model.ErrUnknownDataObject, // Hz is not an array
		"MHAI.HA.phsAHar(3).cVal(1)":   model.ErrUnknownAttribute,
		"MHAI.HA.phsAHar(3).cVal.f(1)": model.ErrUnknownAttribute,
		"MHAI.HA.phsAHar(x)":           model.ErrInvalidReference,
		"MHAI.HA.phsAHar()":            model.ErrInvalidReference,
		"MHAI.HA.phsAHar(1)(2)":        model.ErrInvalidReference,
	} {
		if _, err := r.ResolveString(ref); !errors.Is(err, sentinel) {
			t.Errorf("%s: error = %v, want %v", ref, err, sentinel)
		}
	}
}

// IEC 61850-7-2:2010 §22.2: Data-Instance-ID; the NSD bounds the
// instances of MmultiRange/OmultiRange (TmAChr OmultiRange(33, 48) of
// 7-4 CurrentProtectionLN). MDL-07.
func TestSpec_7_2_22_2_MultiInstanceRange(t *testing.T) {
	r := officialResolver(t)
	for _, ok := range []string{"LD0/PTOC1.TmAChr33[SP]", "LD0/PTOC1.TmAChr48[SP]", "PTOC.TmAChr", "GGIO.Ind1", "GGIO.Ind"} {
		if _, err := r.ResolveString(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"LD0/PTOC1.TmAChr32[SP]", "LD0/PTOC1.TmAChr49[SP]", "PTOC.TmAChr99"} {
		_, err := r.ResolveString(bad)
		if !errors.Is(err, model.ErrUnknownDataObject) || !strings.Contains(err.Error(), "33..48") {
			t.Errorf("%s: error = %v", bad, err)
		}
	}
}

func TestSpec_7_7_MultiInstancePresenceConditions(t *testing.T) {
	for name, want := range map[string]bool{
		"Mmulti": true, "Omulti": true, "MmultiRange": true, "OmultiRange": true, "MmultiF": true,
		"multiplier": false, "Multiple": false, "O": false, "AtLeastOne": false,
	} {
		if got := resolver.IsMultiInstance(model.PresenceCondition{Name: name}); got != want {
			t.Errorf("IsMultiInstance(%s) = %v, want %v", name, got, want)
		}
	}
	// A range argument that is not "min, max" does not bound instances.
	id := model.NamespaceID{Standard: "R", Version: "1"}
	ns := &model.Namespace{ID: id,
		BasicTypes: []model.BasicType{{Name: "B", Source: id}},
		CDCs:       []model.CommonDataClass{{Name: "C", Source: id, DataAttributes: []model.DataAttribute{{Name: "v", FC: "ST", Type: "B"}}}},
		LNClasses: []model.LogicalNodeClass{{Name: "TEST", Source: id, DataObjects: []model.DataObject{
			{Name: "Odd", Type: "C", Presence: model.PresenceCondition{Name: "OmultiRange", Args: "x"}},
			{Name: "Rev", Type: "C", Presence: model.PresenceCondition{Name: "OmultiRange", Args: "9, 1"}},
		}}},
	}
	c, err := catalog.New(ns)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"TEST.Odd5.v", "TEST.Rev5.v"} {
		if _, err := resolver.New(c).ResolveString(ref); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
}

// IEC 61850-7-2:2010 §6.1.2.3: an instance reference is
// "LDName/LNName[.Name[. ...]]"; input without "/" is a class
// reference, and an instance name there gets a hint. MDL-10.
func TestSpec_7_2_6_1_2_3_InstanceNameNeedsLD(t *testing.T) {
	r := officialResolver(t)
	_, err := r.ResolveString("MMXU1.PhV.phsA.cVal.mag.f[MX]")
	if !errors.Is(err, model.ErrUnknownLogicalNode) || !strings.Contains(err.Error(), "needs its logical device") {
		t.Errorf("error = %v", err)
	}
	if _, err := r.ResolveString("ZZZZ1.Hz"); err == nil || strings.Contains(err.Error(), "needs its logical device") {
		t.Errorf("no hint for an unknown class: %v", err)
	}
	if _, err := r.ResolveString("LD0/MMXU1.Hz"); err != nil {
		t.Error(err)
	}
}

// A controllable CDC without a ctlVal service parameter (the deprecated
// IEC 61850-7-410 TAG declares only operTm) has control structures
// without ctlVal: its leaves resolve, and ctlVal itself is refused.
func TestSpec_8_1_20_3_ControlWithoutCtlVal(t *testing.T) {
	p, err := official.Extended()
	if err != nil {
		t.Fatal(err)
	}
	var ns model.NamespaceID
	for _, n := range p.Namespaces() {
		if n.ID.Standard == "IEC 61850-7-410" {
			ns = n.ID
		}
	}
	if ns.Standard == "" {
		t.Skip("no IEC 61850-7-410 namespace in the extended profile")
	}
	r := resolver.New(p).In(ns)
	leaves, err := r.Leaves("HGTE", []string{"Tag"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var oper int
	for _, l := range leaves {
		path := l.AttributePath()
		if strings.Contains(path, "ctlVal") {
			t.Errorf("leaf %s: TAG declares no ctlVal", path)
		}
		if strings.HasPrefix(path, "Oper.") {
			oper++
		}
	}
	if oper == 0 {
		t.Error("no Oper leaves")
	}
	if _, err := r.Leaves("HGTE", []string{"Tag", "Oper", "ctlVal"}, ""); err == nil || !strings.Contains(err.Error(), "declares no service parameter ctlVal") {
		t.Errorf("Tag.Oper.ctlVal: %v", err)
	}
}
