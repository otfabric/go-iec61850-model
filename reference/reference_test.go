// SPDX-License-Identifier: MIT

package reference

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
)

// The valid and invalid inputs below are those of the ParseRef tests of
// github.com/otfabric/go-iec61850; both parsers must agree on them.

func TestParseCompatibleWithGoIEC61850(t *testing.T) {
	tests := []struct {
		input string
		want  Ref
	}{
		{"simpleIOGenericIO/GGIO1.Ind1.stVal[ST]", Ref{LD: "simpleIOGenericIO", LN: "GGIO1", Path: []string{"Ind1", "stVal"}, FC: model.FCST}},
		{"LD1/LLN0.NamPlt.vendor[DC]", Ref{LD: "LD1", LN: "LLN0", Path: []string{"NamPlt", "vendor"}, FC: model.FCDC}},
		{"LD1/MMXU1.TotW.mag.f[MX]", Ref{LD: "LD1", LN: "MMXU1", Path: []string{"TotW", "mag", "f"}, FC: model.FCMX}},
		{"LD1/LLN0", Ref{LD: "LD1", LN: "LLN0"}},
		{"LD1/GGIO1.Ind1.stVal", Ref{LD: "LD1", LN: "GGIO1", Path: []string{"Ind1", "stVal"}}},
		{"LD1", Ref{LD: "LD1"}},
		{"LD1/GGIO1[XX]", Ref{LD: "LD1", LN: "GGIO1", FC: "XX"}},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse = %+v, want %+v", got, tc.want)
			}
			if got.String() != tc.input {
				t.Errorf("String() = %q, want %q", got.String(), tc.input)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	inputs := []string{
		"", "LD1/", "LD1/LN.", "LD1/LN..DO", "LD1/.DO", "ref[S]", "ref[ST", "ref[ST]extra",
		"LN.DO.DA", "LD1/LN]", "LD1/LN]ST[", "LD1/LN[ST][MX]", "LD1[ST]", "/LN", "LD1/LN.a$b",
		strings.Repeat("x", 65) + "/LN", "LD1/LN.DO[]",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			_, err := Parse(in)
			var re *model.ReferenceError
			if !errors.As(err, &re) || !errors.Is(err, model.ErrInvalidReference) {
				t.Fatalf("Parse(%q) error = %v, want *ReferenceError", in, err)
			}
		})
	}
	if _, err := ParseStrict(strings.Repeat("a", 130)); !errors.Is(err, model.ErrInvalidReference) {
		t.Error("ParseStrict must reject long references")
	}
	if _, err := ParseStrict("LD1/LLN0"); err != nil {
		t.Error(err)
	}
}

func TestRefMethods(t *testing.T) {
	r, _ := Parse("LD0/Q0MMXU1.PNV.phsA.cVal.mag.f[MX]")
	if r.ObjectReference() != "LD0/Q0MMXU1.PNV.phsA.cVal.mag.f" || !r.IsObject() || r.IsLN() || r.IsLD() {
		t.Errorf("ObjectReference/Is* for %v", r)
	}
	if w := r.WithFC(model.FCST); w.FC != model.FCST || r.FC != model.FCMX {
		t.Error("WithFC must return a copy")
	}
	p, ok := r.Parent()
	if !ok || p.String() != "LD0/Q0MMXU1.PNV.phsA.cVal.mag[MX]" {
		t.Errorf("Parent = %v", p)
	}
	ln, _ := Parse("LD0/MMXU1")
	if p, ok := ln.Parent(); !ok || !p.IsLD() || p.String() != "LD0" {
		t.Errorf("LN parent = %v", p)
	}
	if _, ok := (Ref{LD: "LD0"}).Parent(); ok {
		t.Error("LD has no parent")
	}
	c, err := ln.Child("Hz")
	if err != nil || c.String() != "LD0/MMXU1.Hz" || !ln.IsLN() {
		t.Errorf("Child = %v, %v", c, err)
	}
	for _, bad := range []string{"", "a.b", "a$b"} {
		if _, err := ln.Child(bad); err == nil {
			t.Errorf("Child(%q) must fail", bad)
		}
	}
	if _, err := (Ref{LD: "LD0"}).Child("x"); err == nil {
		t.Error("Child of LD must fail")
	}
	n, err := r.LNName()
	if err != nil || n != (LNName{Prefix: "Q0", Class: "MMXU", Inst: "1"}) || n.String() != "Q0MMXU1" {
		t.Errorf("LNName = %+v, %v", n, err)
	}
	cls, err := r.Class()
	if err != nil || cls.String() != "MMXU.PNV.phsA.cVal.mag.f[MX]" || cls.DataObject() != "PNV" {
		t.Errorf("Class = %v, %v", cls, err)
	}
	if _, err := (Ref{LD: "LD0"}).Class(); err == nil {
		t.Error("Class of LD must fail")
	}
	for _, bad := range []Ref{
		{LD: "LD1", Path: []string{"x"}}, {LD: "LD1", LN: "LN", Path: []string{"x", ""}}, {LD: "L.D"},
		{LD: "LD1", LN: "L$N"}, {LD: "LD1", LN: "LN", FC: "MXX"}, {},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("Validate(%+v) must fail", bad)
		}
	}
}

func TestMMS(t *testing.T) {
	r, _ := Parse("LD1/MMXU1.TotW.mag.f[MX]")
	d, item, err := r.ToMMS()
	if err != nil || d != "LD1" || item != "MMXU1$MX$TotW$mag$f" {
		t.Fatalf("ToMMS = %q %q %v", d, item, err)
	}
	back, err := FromMMS(d, item)
	if err != nil || !reflect.DeepEqual(back, r) {
		t.Errorf("FromMMS = %+v, %v", back, err)
	}
	for _, tc := range []struct{ in, domain, item string }{
		{"LD1", "LD1", ""}, {"LD1/LLN0", "LD1", "LLN0"}, {"LD1/LLN0[ST]", "LD1", "LLN0$ST"},
	} {
		r, _ := Parse(tc.in)
		d, i, err := r.ToMMS()
		if err != nil || d != tc.domain || i != tc.item {
			t.Errorf("ToMMS(%s) = %q %q %v", tc.in, d, i, err)
		}
		back, err := FromMMS(d, i)
		if err != nil || back.String() != tc.in {
			t.Errorf("FromMMS(%q, %q) = %v, %v", d, i, back, err)
		}
	}
	noFC, _ := Parse("LD1/MMXU1.TotW")
	if _, _, err := noFC.ToMMS(); err == nil {
		t.Error("ToMMS without FC must fail")
	}
	if _, _, err := (Ref{}).ToMMS(); err == nil {
		t.Error("ToMMS of invalid ref must fail")
	}
	for _, tc := range [][2]string{{"", "LLN0$ST$x"}, {"LD1", "$ST$x"}, {"LD1", "LLN0$$foo"}, {"LD1", "LLN0$ST$$stVal"}, {"LD1", "LLN0$ST$"}} {
		if _, err := FromMMS(tc[0], tc[1]); !errors.Is(err, model.ErrInvalidReference) {
			t.Errorf("FromMMS(%q, %q) error = %v", tc[0], tc[1], err)
		}
	}
}

func TestParseClass(t *testing.T) {
	r, err := ParseClass("MMXU.Hz.mag.f[MX]")
	if err != nil || r.LNClass != "MMXU" || strings.Join(r.Path, ".") != "Hz.mag.f" || r.FC != model.FCMX || r.String() != "MMXU.Hz.mag.f[MX]" {
		t.Fatalf("ParseClass = %+v, %v", r, err)
	}
	if r, err := ParseClass("MMXU"); err != nil || len(r.Path) != 0 || r.DataObject() != "" {
		t.Errorf("ParseClass(MMXU) = %+v, %v", r, err)
	}
	for _, bad := range []string{"", "LD0/MMXU1.Hz", ".Hz", "MMXU..Hz", "MMXU.Hz[M]", "MMXU.Hz[MX", "MMXU.a$b"} {
		if _, err := ParseClass(bad); !errors.Is(err, model.ErrInvalidReference) {
			t.Errorf("ParseClass(%q) error = %v", bad, err)
		}
	}
	if err := (ClassRef{LNClass: "MMXU", FC: "X"}).Validate(); err == nil {
		t.Error("short FC must fail")
	}
}

func TestParseLNName(t *testing.T) {
	tests := []struct {
		in   string
		want LNName
	}{
		{"LLN0", LNName{Class: "LLN0"}},
		{"MMXU1", LNName{Class: "MMXU", Inst: "1"}},
		{"MMXU", LNName{Class: "MMXU"}},
		{"Q0MMXU12", LNName{Prefix: "Q0", Class: "MMXU", Inst: "12"}},
		{"Feeder_1XCBR3", LNName{Prefix: "Feeder_1", Class: "XCBR", Inst: "3"}},
		{"ABMMXU1", LNName{Prefix: "AB", Class: "MMXU", Inst: "1"}},
	}
	for _, tc := range tests {
		got, err := ParseLNName(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseLNName(%q) = %+v, %v; want %+v", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "MMX1", "mmxu1", "1234", "a-bMMXU1", "MMxU1"} {
		if _, err := ParseLNName(bad); !errors.Is(err, model.ErrInvalidReference) {
			t.Errorf("ParseLNName(%q) error = %v", bad, err)
		}
	}
}

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"LD0/MMXU1$MX$Hz$mag$f":  "LD0/MMXU1.Hz.mag.f[MX]",
		"LD0/MMXU1.Hz.mag.f[MX]": "LD0/MMXU1.Hz.mag.f[MX]",
		"MMXU.Hz.mag.f[MX]":      "MMXU.Hz.mag.f[MX]",
		"LD0/LLN0":               "LD0/LLN0",
	}
	for in, want := range tests {
		if got, err := Normalize(in); err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"MMXU1$MX$Hz", "LD0/$MX", "LD0/MMXU1..Hz", "MMXU..Hz"} {
		if _, err := Normalize(bad); !errors.Is(err, model.ErrInvalidReference) {
			t.Errorf("Normalize(%q) error = %v", bad, err)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"LD1/MMXU1.TotW.mag.f[MX]", "LD1", "LD1/LLN0", "a[ST]", "LD/LN..x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := Parse(s)
		if err != nil {
			return
		}
		again, err := Parse(r.String())
		if err != nil {
			t.Fatalf("String() of a parsed ref does not parse: %q -> %q: %v", s, r.String(), err)
		}
		if !reflect.DeepEqual(r, again) {
			t.Fatalf("round trip changed %q: %+v vs %+v", s, r, again)
		}
	})
}
