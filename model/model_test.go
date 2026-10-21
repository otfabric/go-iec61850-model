// SPDX-License-Identifier: MIT

package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNamespaceIDStringAndParse(t *testing.T) {
	tests := []struct {
		id   NamespaceID
		want string
	}{
		{NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 5}, "IEC 61850-7-4:2007B5"},
		{NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B"}, "IEC 61850-7-4:2007B"},
		{NamespaceID{Standard: "IEC 61850-7-420", Version: "2019", Revision: "A", Release: 4}, "IEC 61850-7-420:2019A4"},
		{NamespaceID{Standard: "IEC 61850-7-4"}, "IEC 61850-7-4"},
	}
	for _, tc := range tests {
		if got := tc.id.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
		back, err := ParseNamespaceID(tc.want)
		if err != nil {
			t.Fatalf("ParseNamespaceID(%q): %v", tc.want, err)
		}
		if back != tc.id {
			t.Errorf("ParseNamespaceID(%q) = %+v, want %+v", tc.want, back, tc.id)
		}
	}
}

func TestParseNamespaceIDErrors(t *testing.T) {
	for _, in := range []string{"", "  ", ":2007B", "IEC 61850-7-4:B5", "IEC 61850-7-4:2007B5x"} {
		if _, err := ParseNamespaceID(in); !errors.Is(err, ErrUnknownNamespace) {
			t.Errorf("ParseNamespaceID(%q) error = %v, want ErrUnknownNamespace", in, err)
		}
	}
}

func TestNamespaceIDSatisfiesAndCompare(t *testing.T) {
	r4 := NamespaceID{Standard: "S", Version: "2007", Revision: "B", Release: 4}
	r5 := NamespaceID{Standard: "S", Version: "2007", Revision: "B", Release: 5}
	anyB := NamespaceID{Standard: "S", Version: "2007", Revision: "B"}
	if !r4.Satisfies(anyB) || !r5.Satisfies(anyB) {
		t.Error("a release must satisfy a dependency without release")
	}
	if r4.Satisfies(r5) || !r5.Satisfies(r5) || !r5.Satisfies(r4) {
		t.Error("a dependency with release requires that release or a later one")
	}
	if r4.Satisfies(NamespaceID{Standard: "S", Version: "2007", Revision: "A"}) {
		t.Error("revision must match")
	}
	if r4.Compare(r5) >= 0 || r5.Compare(r4) <= 0 || r4.Compare(anyB) <= 0 {
		t.Error("Compare must order by release")
	}
	if (NamespaceID{Standard: "A"}).Compare(NamespaceID{Standard: "B"}) >= 0 {
		t.Error("Compare must order by standard first")
	}
	if !(NamespaceID{}).IsZero() || r4.IsZero() {
		t.Error("IsZero")
	}
}

func TestNamespaceLookups(t *testing.T) {
	ns := &Namespace{
		ID:                    NamespaceID{Standard: "S", Version: "1"},
		BasicTypes:            []BasicType{{Name: "FLOAT32"}},
		Enumerations:          []Enumeration{{Name: "E", Literals: []Literal{{Name: "a", Value: 1}, {Name: "b", Value: 2}}}},
		ConstructedAttributes: []ConstructedAttribute{{Name: "AV", SubDataAttributes: []DataAttribute{{Name: "f", Type: "FLOAT32"}}}},
		CDCs: []CommonDataClass{{
			Name:           "MV",
			SubDataObjects: []DataObject{{Name: "sub", Type: "MV"}},
			DataAttributes: []DataAttribute{{Name: "mag", FC: FCMX}, {Name: "q", FC: FCMX}, {Name: "d", FC: FCDC}},
		}},
		AbstractLNClasses: []LogicalNodeClass{{Name: "Base", Abstract: true}},
		LNClasses:         []LogicalNodeClass{{Name: "MMXU", Base: "Base", DataObjects: []DataObject{{Name: "Hz", Type: "MV"}}}},
	}
	if ln, ok := ns.LogicalNodeClass("MMXU"); !ok || ln.Group() != 'M' || ln.Group().Label() != "Metering and measurement" {
		t.Errorf("LogicalNodeClass(MMXU) = %v, %v", ln, ok)
	}
	if ln, ok := ns.LogicalNodeClass("Base"); !ok || ln.Group() != 0 || ln.Group().String() != "" {
		t.Errorf("abstract class must have no group: %v", ln)
	}
	if _, ok := ns.LogicalNodeClass("XXXX"); ok {
		t.Error("unknown class found")
	}
	ln, _ := ns.LogicalNodeClass("MMXU")
	if _, ok := ln.DataObject("Hz"); !ok {
		t.Error("DataObject(Hz)")
	}
	cdc, ok := ns.CDC("MV")
	if !ok {
		t.Fatal("CDC(MV)")
	}
	if _, ok := cdc.SubDataObject("sub"); !ok {
		t.Error("SubDataObject")
	}
	if _, ok := cdc.DataAttribute("mag"); !ok {
		t.Error("DataAttribute")
	}
	if got := cdc.FunctionalConstraints(); len(got) != 2 || got[0] != FCMX || got[1] != FCDC {
		t.Errorf("FunctionalConstraints = %v", got)
	}
	ca, ok := ns.ConstructedAttribute("AV")
	if !ok {
		t.Fatal("ConstructedAttribute")
	}
	if _, ok := ca.SubDataAttribute("f"); !ok {
		t.Error("SubDataAttribute")
	}
	e, ok := ns.Enumeration("E")
	if !ok {
		t.Fatal("Enumeration")
	}
	if l, ok := e.Literal("b"); !ok || l.Value != 2 {
		t.Error("Literal")
	}
	if l, ok := e.LiteralByValue(1); !ok || l.Name != "a" {
		t.Error("LiteralByValue")
	}
	if _, ok := e.LiteralByValue(9); ok {
		t.Error("LiteralByValue(9) found")
	}
	if _, ok := ns.BasicType("FLOAT32"); !ok {
		t.Error("BasicType")
	}
	if got := ns.Namespaces(); len(got) != 1 || got[0] != ns {
		t.Error("Namespaces")
	}
}

func TestPresenceAndFC(t *testing.T) {
	p := PresenceCondition{Name: "AtLeastOne", Args: "1"}
	if p.String() != "AtLeastOne(1)" || p.IsMandatory() || p.IsOptional() || p.IsZero() {
		t.Errorf("presence %v", p)
	}
	if !(PresenceCondition{Name: "M"}).IsMandatory() || !(PresenceCondition{Name: "O"}).IsOptional() || (PresenceCondition{Name: "O"}).String() != "O" {
		t.Error("M/O")
	}
	if !FCMX.IsStandard() || FunctionalConstraint("XX").IsStandard() {
		t.Error("IsStandard")
	}
	if LNGroup('Q').Label() == "" || LNGroup('B').Label() != "" || LNGroup('M').String() != "M" {
		t.Error("LNGroup")
	}
	if Path("MMXU", "Hz", "mag") != "MMXU.Hz.mag" {
		t.Error("Path")
	}
	if (&LogicalNodeClass{Name: "1abc"}).Group() != 0 || (&LogicalNodeClass{}).Group() != 0 {
		t.Error("Group of invalid names must be zero")
	}
}

func TestErrors(t *testing.T) {
	ns := NamespaceID{Standard: "S", Version: "1"}
	tests := []struct {
		err      error
		sentinel error
		contains string
	}{
		{&ReferenceError{Input: "x", Reason: "bad"}, ErrInvalidReference, `invalid reference "x": bad`},
		{&ReferenceError{Input: "x", Reason: "bad", Wrapped: ErrUnknownLogicalNode}, ErrUnknownLogicalNode, "unknown logical node"},
		{&ResolutionError{Namespace: ns, Path: "MMXU.Hz.mag", Element: "mag", Reason: "nope", Expected: "MX", Actual: "ST", Err: ErrUnknownAttribute}, ErrUnknownAttribute, `"MMXU.Hz.mag": nope (expected MX, got ST) [S:1]`},
		{&ResolutionError{Element: "X", Err: ErrUnknownCDC, Actual: "y"}, ErrUnknownCDC, `"X" (expected -, got y)`},
		{&AmbiguityError{Kind: "CDC", Name: "MV", Candidates: []NamespaceID{ns, {Standard: "T"}}}, ErrAmbiguousReference, `ambiguous CDC "MV": defined in S:1, T`},
		{&DependencyError{Namespace: ns, Required: NamespaceID{Standard: "D", Version: "2"}, Err: ErrMissingDependency}, ErrMissingDependency, "requires D:2, which is not loaded"},
		{&DependencyError{Namespace: ns, Required: NamespaceID{Standard: "D"}, Candidates: []NamespaceID{{Standard: "D", Version: "1"}}, Err: ErrAmbiguousReference}, ErrAmbiguousReference, "candidates D:1"},
		{&DependencyError{Namespace: ns, Required: NamespaceID{Standard: "D"}, Reason: "conflicts", Err: ErrNamespaceConflict}, ErrNamespaceConflict, "dependency D of S:1: conflicts"},
		{&SourceError{Source: "f.nsd", Reason: "boom", Err: ErrInvalidNSD}, ErrInvalidNSD, "f.nsd: boom: "},
		{&SourceError{Source: "f.nsd", Err: ErrLimitExceeded}, ErrLimitExceeded, "f.nsd: iec61850-model: input limit exceeded"},
		{&SourceError{Source: "f.nsd", Reason: "only"}, nil, "f.nsd: only"},
		{&ValidationError{}, ErrInvalidNSD, "validation failed"},
		{&ValidationError{Diagnostics: Diagnostics{{Severity: SeverityError, Message: "a"}}}, ErrInvalidNSD, "validation failed: error: a"},
		{&ValidationError{Diagnostics: Diagnostics{{Severity: SeverityError, Message: "a"}, {Severity: SeverityError, Message: "b"}}, Err: ErrUnsupportedSchema}, ErrUnsupportedSchema, "and 1 more errors"},
	}
	for _, tc := range tests {
		if tc.sentinel != nil && !errors.Is(tc.err, tc.sentinel) {
			t.Errorf("%T does not wrap %v", tc.err, tc.sentinel)
		}
		if !strings.Contains(tc.err.Error(), tc.contains) {
			t.Errorf("%T.Error() = %q, want it to contain %q", tc.err, tc.err.Error(), tc.contains)
		}
	}
}

func TestDiagnostics(t *testing.T) {
	ds := Diagnostics{
		{Severity: SeverityInfo, Message: "i"},
		{Severity: SeverityWarning, Code: "w", Source: "s", Path: "/NS", Message: "warn"},
	}
	if ds.HasErrors() || ds.Err() != nil || len(ds.Errors()) != 0 || len(ds.AtLeast(SeverityWarning)) != 1 {
		t.Error("no errors expected")
	}
	if got := ds[1].String(); got != "warning: s: /NS: warn [w]" {
		t.Errorf("String() = %q", got)
	}
	ds = append(ds, Diagnostic{Severity: SeverityError, Message: "e"})
	var ve *ValidationError
	if !errors.As(ds.Err(), &ve) || len(ve.Diagnostics) != 3 {
		t.Errorf("Err() = %v", ds.Err())
	}
	if Severity(7).String() != "severity(7)" {
		t.Error("unknown severity string")
	}
	b, err := json.Marshal(Diagnostic{Severity: SeverityError, Message: "m"})
	if err != nil || !strings.Contains(string(b), `"severity":"error"`) {
		t.Errorf("json = %s, %v", b, err)
	}
}

func TestNamespaceDocs(t *testing.T) {
	ns := &Namespace{
		BasicTypes:            []BasicType{{}},
		FunctionalConstraints: []FunctionalConstraintDef{{}},
		TriggerOptions:        []TriggerOption{{}},
		PresenceConditions:    []PresenceConditionDef{{}},
		Abbreviations:         []Abbreviation{{}},
		Enumerations:          []Enumeration{{Literals: []Literal{{}}}},
		ConstructedAttributes: []ConstructedAttribute{{SubDataAttributes: []DataAttribute{{}}}},
		CDCs:                  []CommonDataClass{{SubDataObjects: []DataObject{{}}, DataAttributes: []DataAttribute{{}}, ServiceParameters: []ServiceParameter{{}}}},
		AbstractLNClasses:     []LogicalNodeClass{{DataObjects: []DataObject{{}}}},
		LNClasses:             []LogicalNodeClass{{DataObjects: []DataObject{{}}}},
	}
	docs := ns.Docs()
	if len(docs) != 18 || docs[0] != &ns.Doc {
		t.Fatalf("Docs() returned %d entries", len(docs))
	}
	for _, d := range docs {
		d.Title = "x"
	}
	if ns.LNClasses[0].DataObjects[0].Doc.Title != "x" || ns.CDCs[0].ServiceParameters[0].Doc.Title != "x" {
		t.Error("Docs() must return pointers into the namespace")
	}
}

func TestCDCVariants(t *testing.T) {
	ns := &Namespace{CDCs: []CommonDataClass{
		{Name: "SPG", Variant: "SP"}, {Name: "SPG", Variant: "SG"}, {Name: "MV"},
	}}
	vs := ns.CDCVariants("SPG")
	if len(vs) != 2 || vs[0].Key() != "SPG/SP" || vs[1].Key() != "SPG/SG" {
		t.Errorf("CDCVariants(SPG) = %v", vs)
	}
	if c, ok := ns.CDC("MV"); !ok || c.Key() != "MV" {
		t.Errorf("CDC(MV) = %v", c)
	}
	if vs := ns.CDCVariants("WYE"); len(vs) != 0 {
		t.Errorf("CDCVariants(WYE) = %v", vs)
	}
	for name, want := range map[string]string{"setMag": "setMag", "_setMag": "setMag", "__setMag": "setMag"} {
		if got := (&DataAttribute{Name: name}).BaseName(); got != want {
			t.Errorf("BaseName(%s) = %s", name, got)
		}
	}
}
