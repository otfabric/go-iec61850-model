// SPDX-License-Identifier: MIT

package resolver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/reference"
	"github.com/otfabric/go-iec61850-model/resolver"
)

func newResolver(t *testing.T) *resolver.Resolver {
	t.Helper()
	return resolver.New(fixtures.Profile(t))
}

func stepNames(r *resolver.Resolution) string {
	var parts []string
	for _, s := range r.Steps {
		parts = append(parts, string(s.Kind)+":"+s.Name+":"+s.Type)
	}
	return strings.Join(parts, " ")
}

func TestResolveChains(t *testing.T) {
	r := newResolver(t)
	tests := []struct {
		ref, typ, kind, fc, doPath, attrPath string
	}{
		{"MMXU.Hz.mag.f", "FLOAT32", "BASIC", "MX", "Hz", "mag.f"},
		{"LD0/MMXU1.Hz.mag.f[MX]", "FLOAT32", "BASIC", "MX", "Hz", "mag.f"},
		{"MMXU.PNV.phsA.cVal.mag.f[MX]", "FLOAT32", "BASIC", "MX", "PNV.phsA", "cVal.mag.f"},
		{"MMXU.PPV.phsAB.cVal.ang.i", "INT32", "BASIC", "MX", "PPV.phsAB", "cVal.ang.i"},
		{"MMXU.Hz.units.SIUnit[CF]", "SIUnitKind", "ENUMERATED", "CF", "Hz", "units.SIUnit"},
		{"MMXU.Hz.mag", "AnalogueValue", "CONSTRUCTED", "MX", "Hz", "mag"},
		{"MMXU.Beh.stVal[ST]", "BehaviourModeKind", "ENUMERATED", "ST", "Beh", "stVal"},
		{"LD0/LLN0.Mod.stVal", "BehaviourModeKind", "ENUMERATED", "ST", "Mod", "stVal"},
		{"LD0/Q0LPHD1.PhyHealth.stVal", "HealthKind", "ENUMERATED", "ST", "PhyHealth", "stVal"},
		{"DTST.State.stVal", "TestStateKind", "ENUMERATED", "ST", "State", "stVal"},
		{"LD0/GGIO1.Ind1.stVal[ST]", "BOOLEAN", "BASIC", "ST", "Ind1", "stVal"},
		{"GGIO.IntIn12.stVal", "INT32", "BASIC", "ST", "IntIn12", "stVal"},
		{"MMXU.NamPlt.vendor[DC]", "VisibleString255", "BASIC", "DC", "NamPlt", "vendor"},
		{"MMXU.Hz.q", "Quality", "BASIC", "MX", "Hz", "q"},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			res, err := r.ResolveString(tc.ref)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if res.Type() != tc.typ || string(res.TypeKind()) != tc.kind || string(res.FC()) != tc.fc {
				t.Errorf("leaf = %s %s %s, want %s %s %s", res.Type(), res.TypeKind(), res.FC(), tc.typ, tc.kind, tc.fc)
			}
			if res.DataObjectPath() != tc.doPath || res.AttributePath() != tc.attrPath || !res.IsAttribute() {
				t.Errorf("paths = %q %q", res.DataObjectPath(), res.AttributePath())
			}
			if res.LNClass == nil || res.DataObject == nil || res.CDC == nil || res.Attribute == nil {
				t.Errorf("missing definitions in %+v", res)
			}
			for _, s := range res.Steps {
				if s.Source.IsZero() || s.DeclaredIn == "" {
					t.Errorf("step without provenance: %+v", s)
				}
			}
		})
	}
}

func TestResolveDetails(t *testing.T) {
	r := newResolver(t)
	res, err := r.ResolveString("MMXU.PNV.phsA.cVal.mag.f")
	if err != nil {
		t.Fatal(err)
	}
	want := "LogicalNode:MMXU:MMXU DataObject:PNV:WYE SubDataObject:phsA:CMV DataAttribute:cVal:Vector SubDataAttribute:mag:AnalogueValue SubDataAttribute:f:FLOAT32"
	if got := stepNames(res); got != want {
		t.Errorf("steps\n got: %s\nwant: %s", got, want)
	}
	if res.Path() != "MMXU.PNV.phsA.cVal.mag.f" || res.ClassRef().String() != "MMXU.PNV.phsA.cVal.mag.f[MX]" {
		t.Errorf("Path %s ClassRef %s", res.Path(), res.ClassRef())
	}
	if res.Basic == nil || res.Basic.Name != "FLOAT32" || res.CDC.Name != "CMV" {
		t.Errorf("leaf definitions %+v", res)
	}
	if s := res.Steps[2]; s.DeclaredIn != "WYE" || s.Source != fixtures.ID73 {
		t.Errorf("sub data object provenance %+v", s)
	}
	if s := res.Steps[1]; s.DeclaredIn != "MMXU" || s.Source != fixtures.ID74 {
		t.Errorf("data object provenance %+v", s)
	}

	beh, err := r.ResolveAttribute("MMXU", "Beh", "stVal")
	if err != nil {
		t.Fatal(err)
	}
	if !beh.Leaf().Parameterized || beh.Enumeration == nil || beh.Enumeration.Name != "BehaviourModeKind" || beh.Steps[1].DeclaredIn != "DomainLN" {
		t.Errorf("parameterized stVal: %+v", beh.Leaf())
	}
	mag, err := r.ResolveAttribute("MMXU", "Hz", "mag")
	if err != nil || mag.Constructed == nil || mag.Constructed.Name != "AnalogueValue" {
		t.Errorf("constructed leaf: %v", err)
	}
	ind, err := r.ResolveString("GGIO.Ind3")
	if err != nil || ind.Leaf().Instance != "3" || ind.DataObject.Name != "Ind" || ind.IsAttribute() || ind.Type() != "SPS" {
		t.Errorf("multi-instance DO: %+v, %v", ind, err)
	}

	cls, err := r.ResolveString("MMXU")
	if err != nil || len(cls.Steps) != 1 || cls.DataObject != nil || cls.Type() != "MMXU" {
		t.Errorf("class only: %+v %v", cls, err)
	}
	do, err := r.ResolveString("MMXU.Hz[MX]")
	if err != nil || do.IsAttribute() || do.Type() != "MV" || do.FC() != "" || do.ClassRef().String() != "MMXU.Hz" {
		t.Errorf("data object with FC: %+v %v", do, err)
	}
	if _, err := r.ResolveString("MMXU.PNV[MX]"); err != nil {
		t.Errorf("FC found in sub data objects: %v", err)
	}
	cdc, err := r.ResolveCDC("WYE")
	if err != nil || cdc.Name != "WYE" {
		t.Error(err)
	}
	chain, err := r.ResolveInheritance("MMXU")
	if err != nil || len(chain) != 3 {
		t.Errorf("inheritance %v %v", chain, err)
	}
}

func TestResolveErrors(t *testing.T) {
	r := newResolver(t)
	tests := []struct {
		ref      string
		sentinel error
	}{
		{"XXXX.Hz", model.ErrUnknownLogicalNode},
		{"LD0/XXXX1.Hz", model.ErrUnknownLogicalNode},
		{"MMXU.Nope", model.ErrUnknownDataObject},
		{"MMXU.Hz1", model.ErrUnknownDataObject},
		{"MMXU.Hz.nope", model.ErrUnknownAttribute},
		{"MMXU.Hz.mag.f.x", model.ErrUnknownAttribute},
		{"MMXU.Hz.mag.x", model.ErrUnknownAttribute},
		{"MMXU.Hz.mag.f[ST]", model.ErrUnknownAttribute},
		{"MMXU.Hz.units[MX]", model.ErrUnknownAttribute},
		{"MMXU.Hz[ST]", model.ErrUnknownAttribute},
		{"MMXU.PNV.phsX", model.ErrUnknownAttribute},
		{"LD0", model.ErrUnknownLogicalNode},
		{"LD0/", model.ErrInvalidReference},
		{"LD0/mmxu1.Hz", model.ErrInvalidReference},
		{"MMXU..Hz", model.ErrInvalidReference},
		{"LD0/MMXU1..Hz", model.ErrInvalidReference},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			_, err := r.ResolveString(tc.ref)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("error = %v, want %v", err, tc.sentinel)
			}
		})
	}
	_, err := r.ResolveString("MMXU.Hz.mag.f[ST]")
	var re *model.ResolutionError
	if !errors.As(err, &re) || re.Expected != "ST" || re.Actual != "MX" || re.Path != "MMXU.Hz.mag" || re.Namespace != fixtures.ID73 {
		t.Errorf("FC mismatch detail: %+v", re)
	}
	_, err = r.ResolveString("MMXU.Nope")
	if !errors.As(err, &re) || re.Path != "MMXU.Nope" {
		t.Errorf("unknown DO detail: %+v", re)
	}
	if _, err := r.ResolveClass(reference.ClassRef{}); !errors.Is(err, model.ErrInvalidReference) {
		t.Errorf("invalid class ref: %v", err)
	}
}

func TestResolveAmbiguousWithoutProfile(t *testing.T) {
	r := resolver.New(fixtures.Catalog(t))
	_, err := r.ResolveString("MMXU.Hz.mag.f")
	var ae *model.AmbiguityError
	if !errors.As(err, &ae) || len(ae.Candidates) != 2 {
		t.Fatalf("error = %v, want ambiguity", err)
	}
	p, err := fixtures.Catalog(t).Profile(fixtures.ID74R2)
	if err != nil {
		t.Fatal(err)
	}
	r2 := resolver.New(p)
	if res, err := r2.ResolveString("MMXU.TotPF.mag.f"); err != nil || res.Steps[1].Source != fixtures.ID74R2 {
		t.Errorf("release 2 resolution: %v", err)
	}
	if _, err := r2.ResolveString("MMXU.W"); !errors.Is(err, model.ErrUnknownDataObject) {
		t.Errorf("MMXU.W was removed in release 2: %v", err)
	}
}

func TestLeaves(t *testing.T) {
	r := newResolver(t)
	leaves, err := r.Leaves("MMXU", []string{"Hz"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range leaves {
		got = append(got, l.AttributePath()+":"+l.Type())
	}
	want := "instMag.i:INT32 instMag.f:FLOAT32 mag.i:INT32 mag.f:FLOAT32 q:Quality t:Timestamp units.SIUnit:SIUnitKind units.multiplier:MultiplierKind d:VisibleString255"
	if strings.Join(got, " ") != want {
		t.Errorf("leaves\n got: %s\nwant: %s", strings.Join(got, " "), want)
	}
	mx, err := r.Leaves("MMXU", []string{"PNV"}, model.FCMX)
	if err != nil {
		t.Fatal(err)
	}
	if len(mx) != 5*6 {
		t.Errorf("PNV MX leaves = %d, want 30", len(mx))
	}
	for _, l := range mx {
		if l.FC() != model.FCMX {
			t.Errorf("leaf %s has FC %s", l.Path(), l.FC())
		}
	}
	all, err := r.Leaves("MMXU", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 100 || all[0].Path() != "MMXU.Beh.stVal" {
		t.Errorf("MMXU leaves: %d, first %s", len(all), all[0].Path())
	}
	if leaf, err := r.Leaves("MMXU", []string{"Hz", "mag", "f"}, ""); err != nil || len(leaf) != 1 {
		t.Errorf("leaf of a leaf: %v %v", leaf, err)
	}
	if _, err := r.Leaves("XXXX", nil, ""); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown class: %v", err)
	}
}

// cyclicNamespace builds a namespace whose types contain themselves.
func cyclicNamespace() *model.Namespace {
	id := model.NamespaceID{Standard: "CYC", Version: "1"}
	return &model.Namespace{
		ID:         id,
		BasicTypes: []model.BasicType{{Name: "B", Source: id}, {Name: "Dual", Source: id}},
		Enumerations: []model.Enumeration{
			{Name: "Dual", Source: id},
		},
		ConstructedAttributes: []model.ConstructedAttribute{
			{Name: "Loop", Source: id, SubDataAttributes: []model.DataAttribute{{Name: "self", Type: "Loop", TypeKind: model.TypeKindConstructed}}},
		},
		CDCs: []model.CommonDataClass{
			{Name: "SELF", Source: id, SubDataObjects: []model.DataObject{{Name: "again", Type: "SELF"}}, DataAttributes: []model.DataAttribute{{Name: "a", FC: "ST", Type: "B", Position: 1}}},
			{Name: "LOOPY", Source: id, DataAttributes: []model.DataAttribute{{Name: "l", FC: "ST", Type: "Loop", TypeKind: model.TypeKindConstructed}}},
			{Name: "PARAM", Source: id, EnumParameterized: true, DataAttributes: []model.DataAttribute{{Name: "stVal", FC: "ST", TypeKind: model.TypeKindEnumerated}}},
			{Name: "INFER", Source: id, DataAttributes: []model.DataAttribute{
				{Name: "b", FC: "ST", Type: "B"},
				{Name: "u", FC: "ST", Type: "B", TypeKind: model.TypeKindUndefined},
				{Name: "missing", FC: "ST", Type: "Nope"},
				{Name: "dual", FC: "ST", Type: "Dual"},
				{Name: "notype", FC: "ST"},
				{Name: "packed", FC: "ST", Type: "Packed", TypeKind: model.TypeKindPackedList},
				{Name: "badenum", FC: "ST", Type: "Nope", TypeKind: model.TypeKindEnumerated},
			}},
			{Name: "SUBS", Source: id, SubDataObjects: []model.DataObject{{Name: "x", Type: "NOCDC"}}},
		},
		LNClasses: []model.LogicalNodeClass{{Name: "TEST", Source: id, DataObjects: []model.DataObject{
			{Name: "S", Type: "SELF"}, {Name: "L", Type: "LOOPY"}, {Name: "P", Type: "PARAM"},
			{Name: "Q", Type: "PARAM", UnderlyingType: "B"}, {Name: "I", Type: "INFER"}, {Name: "M", Type: "MISSING"},
			{Name: "X", Type: "SUBS"},
		}}},
	}
}

func TestResolveSyntheticEdgeCases(t *testing.T) {
	c, err := catalog.New(cyclicNamespace())
	if err != nil {
		t.Fatal(err)
	}
	r := resolver.New(c)
	tests := []struct {
		path     string
		sentinel error
	}{
		{"TEST.P.stVal", model.ErrUnknownType},
		{"TEST.I.missing", model.ErrUnknownType},
		{"TEST.I.dual", model.ErrAmbiguousReference},
		{"TEST.I.notype", model.ErrUnknownType},
		{"TEST.I.badenum", model.ErrUnknownType},
		{"TEST.M", model.ErrUnknownCDC},
		{"TEST.X[ST]", model.ErrUnknownCDC},
		{"TEST.S.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again.again", model.ErrLimitExceeded},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if _, err := r.ResolveString(tc.path); !errors.Is(err, tc.sentinel) {
				t.Fatalf("error = %v, want %v", err, tc.sentinel)
			}
		})
	}
	if res, err := r.ResolveString("TEST.I.u"); err != nil || res.TypeKind() != model.TypeKindBasic {
		t.Errorf("inferred kind: %v", err)
	}
	if res, err := r.ResolveString("TEST.I.packed"); err != nil || res.TypeKind() != model.TypeKindPackedList || res.Basic != nil {
		t.Errorf("packed list leaf: %v", err)
	}
	if _, err := r.ResolveString("TEST.Q.stVal"); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("ENUMERATED attribute with a basic underlying type: %v", err)
	}
	if _, err := r.ResolveString("TEST.L.l.self.self.self"); err != nil {
		t.Errorf("an explicit finite path through a recursive type resolves: %v", err)
	}
	if _, err := r.Leaves("TEST", []string{"S"}, ""); !errors.Is(err, model.ErrCycle) {
		t.Errorf("CDC cycle error = %v", err)
	}
	if _, err := r.Leaves("TEST", []string{"L"}, ""); !errors.Is(err, model.ErrCycle) {
		t.Errorf("constructed cycle error = %v", err)
	}
	if _, err := r.ResolveString("TEST.S[MX]"); !errors.Is(err, model.ErrCycle) {
		t.Errorf("FC search through cyclic CDC error = %v", err)
	}
	if _, err := r.Leaves("TEST", nil, ""); err == nil {
		t.Error("expanding a class with cyclic types must fail")
	}
}

func TestIsMultiInstance(t *testing.T) {
	for name, want := range map[string]bool{"Omulti": true, "Mmulti": true, "OmultiRange": true, "O": false, "M": false} {
		if got := resolver.IsMultiInstance(model.PresenceCondition{Name: name}); got != want {
			t.Errorf("IsMultiInstance(%s) = %v", name, got)
		}
	}
}

func TestCDCVariants(t *testing.T) {
	id := model.NamespaceID{Standard: "V", Version: "1"}
	setting := func(variant string, fc model.FunctionalConstraint) model.CommonDataClass {
		return model.CommonDataClass{Name: "SPG", Variant: variant, Source: id, DataAttributes: []model.DataAttribute{
			{Name: "setVal", Type: "BOOLEAN", TypeKind: model.TypeKindBasic, FC: fc, Position: 0},
			{Name: "d", Type: "BOOLEAN", TypeKind: model.TypeKindBasic, FC: model.FCDC, Position: 1},
		}}
	}
	ns := &model.Namespace{
		ID:         id,
		BasicTypes: []model.BasicType{{Name: "BOOLEAN", Source: id}},
		CDCs: []model.CommonDataClass{setting("SP", model.FCSP), setting("SG", model.FCSG), setting("SE", model.FCSE),
			{Name: "WRAP", Source: id, SubDataObjects: []model.DataObject{{Name: "s", Type: "SPG"}, {Name: "o", Type: "ONLYDC", Position: 1}}},
			{Name: "ONLYDC", Source: id, DataAttributes: []model.DataAttribute{{Name: "d", Type: "BOOLEAN", TypeKind: model.TypeKindBasic, FC: model.FCDC}}}},
		LNClasses: []model.LogicalNodeClass{{Name: "PTST", Source: id, DataObjects: []model.DataObject{{Name: "StrVal", Type: "SPG"}, {Name: "W", Type: "WRAP"}}}},
	}
	c, err := catalog.New(ns)
	if err != nil {
		t.Fatal(err)
	}
	r := resolver.New(c)
	res, err := r.ResolveString("PTST.StrVal.setVal[SG]")
	if err != nil || res.FC() != model.FCSG || res.Leaf().Variant != "SG" || res.CDC.Key() != "SPG/SG" || res.Leaf().DeclaredIn != "SPG/SG" {
		t.Fatalf("variant by FC: %+v %v", res, err)
	}
	open, err := r.ResolveString("PTST.StrVal.setVal")
	if err != nil || open.FC() != "" || strings.Join(open.Leaf().Variants, ",") != "SPG/SP.setVal,SPG/SG.setVal,SPG/SE.setVal" {
		t.Fatalf("FC open without request: %+v %v", open.Leaf(), err)
	}
	if d, err := r.ResolveString("PTST.StrVal.d"); err != nil || d.FC() != model.FCDC || len(d.Leaf().Variants) != 0 {
		t.Errorf("same FC in every variant: %+v %v", d, err)
	}
	if _, err := r.ResolveString("PTST.StrVal.setVal[MX]"); !errors.Is(err, model.ErrUnknownAttribute) || !strings.Contains(err.Error(), "SP/SG/SE") {
		t.Errorf("no variant with FC: %v", err)
	}
	if _, err := r.ResolveString("PTST.StrVal[SE]"); err != nil {
		t.Errorf("FC on a data object with variants: %v", err)
	}
	leaves, err := r.Leaves("PTST", []string{"StrVal"}, model.FCSE)
	if err != nil || len(leaves) != 1 || leaves[0].FC() != model.FCSE {
		t.Errorf("leaves by FC: %v %v", leaves, err)
	}
	if leaves, err := r.Leaves("PTST", []string{"W"}, model.FCSG); err != nil || len(leaves) != 1 || leaves[0].Path() != "PTST.W.s.setVal" {
		t.Errorf("sub data object without the FC contributes nothing: %v %v", leaves, err)
	}
	if vs, err := c.CDCVariants("SPG"); err != nil || len(vs) != 3 {
		t.Errorf("CDCVariants: %v %v", vs, err)
	}
	all, err := r.Leaves("PTST", []string{"StrVal"}, "")
	var got []string
	for _, l := range all {
		got = append(got, l.Path()+"["+string(l.FC())+"]")
	}
	if err != nil || strings.Join(got, " ") != "PTST.StrVal.setVal[SP] PTST.StrVal.setVal[SG] PTST.StrVal.setVal[SE] PTST.StrVal.d[DC]" {
		t.Errorf("leaves over all variants: %v %v", got, err)
	}
}

func TestPerFCAttributeCopies(t *testing.T) {
	id := model.NamespaceID{Standard: "U", Version: "1"}
	ns := &model.Namespace{
		ID:         id,
		BasicTypes: []model.BasicType{{Name: "FLOAT32", Source: id}},
		CDCs: []model.CommonDataClass{{Name: "RST", Source: id, DataAttributes: []model.DataAttribute{
			{Name: "setMag", Type: "FLOAT32", TypeKind: model.TypeKindBasic, FC: model.FCSP, Position: 0},
			{Name: "_setMag", Type: "FLOAT32", TypeKind: model.TypeKindBasic, FC: model.FCSG, Position: 1},
			{Name: "__setMag", Type: "FLOAT32", TypeKind: model.TypeKindBasic, FC: model.FCSE, Position: 2},
		}}},
		LNClasses: []model.LogicalNodeClass{{Name: "XTST", Source: id, DataObjects: []model.DataObject{{Name: "Rs", Type: "RST"}}}},
	}
	c, _ := catalog.New(ns)
	r := resolver.New(c)
	res, err := r.ResolveString("XTST.Rs.setMag[SE]")
	if err != nil || res.Attribute.Name != "__setMag" || res.FC() != model.FCSE || res.Leaf().Name != "setMag" {
		t.Fatalf("per-FC copy by base name: %+v %v", res, err)
	}
	if res, err := r.ResolveString("XTST.Rs._setMag"); err != nil || res.FC() != model.FCSG {
		t.Errorf("exact model name: %v", err)
	}
	if res, _ := r.ResolveString("XTST.Rs.setMag"); len(res.Leaf().Variants) != 3 || res.FC() != "" {
		t.Errorf("alternatives: %+v", res.Leaf())
	}
	leaves, err := r.Leaves("XTST", nil, "")
	if err != nil || len(leaves) != 3 || leaves[2].Path() != "XTST.Rs.setMag" || leaves[2].FC() != model.FCSE {
		t.Errorf("leaves use device names: %v %v", leaves, err)
	}
	if (&model.DataAttribute{Name: "__x"}).BaseName() != "x" {
		t.Error("BaseName")
	}
}
