// SPDX-License-Identifier: MIT

package reference

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
)

// IEC 61850-7-2:2010 §6.1.2.3 and §12.3.3.2.3: "(…)" indicates an
// array element, "FCDA(NumArrayElement)", e.g. phsAHar(3). MDL-02.
func TestSpec_7_2_6_1_2_3_ArrayElementSyntax(t *testing.T) {
	r, err := Parse("LD0/MHAI1.HA.phsAHar(3).cVal.mag.f[MX]")
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != "LD0/MHAI1.HA.phsAHar(3).cVal.mag.f[MX]" || r.Path[1] != "phsAHar(3)" {
		t.Errorf("round trip = %s", r)
	}
	if c, err := ParseClass("MHAI.HA.phsAHar(12).cVal"); err != nil || c.String() != "MHAI.HA.phsAHar(12).cVal" {
		t.Errorf("class ref: %v %v", c, err)
	}
	tests := []struct {
		in    string
		name  string
		index int
		ok    bool
	}{
		{"phsAHar(3)", "phsAHar", 3, true},
		{"phsAHar(0)", "phsAHar", 0, true},
		{"crvPts(007)", "crvPts", 7, true},
		{"mag", "mag", 0, false},
	}
	for _, tc := range tests {
		name, index, ok, err := SplitIndex(tc.in)
		if err != nil || name != tc.name || index != tc.index || ok != tc.ok {
			t.Errorf("SplitIndex(%q) = %q %d %v %v", tc.in, name, index, ok, err)
		}
	}
	for _, bad := range []string{
		"LD0/MHAI1.HA.phsAHar(x).cVal", "LD0/MHAI1.HA.phsAHar().cVal", "LD0/MHAI1.HA.phsAHar(1)(2).cVal",
		"LD0/MHAI1.HA.phsAHar(1", "LD0/MHAI1.HA.phsAHar1)", "LD0/MHAI1.HA.(1)", "LD0/MHAI1.HA.phs(1)x",
		"LD0/MHAI1.HA.phsAHar(99999999999999999999)", "MHAI.HA.phsAHar(-1)",
	} {
		if _, err := Normalize(bad); !errors.Is(err, model.ErrInvalidReference) {
			t.Errorf("Normalize(%q) error = %v", bad, err)
		}
	}
	ln, _ := Parse("LD0/MHAI1.HA")
	if c, err := ln.Child("phsAHar(3)"); err != nil || c.String() != "LD0/MHAI1.HA.phsAHar(3)" {
		t.Errorf("Child with index: %v %v", c, err)
	}
	if _, err := ln.Child("phsAHar(x)"); err == nil {
		t.Error("Child with a malformed index must fail")
	}
	// 8-1 addresses array elements with MMS alternate access: no item ID.
	if _, _, err := r.ToMMS(); !errors.Is(err, model.ErrInvalidReference) || !strings.Contains(err.Error(), "alternate access") {
		t.Errorf("ToMMS of an array element: %v", err)
	}
}

// IEC 61850-7-2:2010 Table 4: ObjectReference is VISIBLE STRING129, an
// LD name and a path of at most 64 characters each joined by "/"; the
// FC is not part of it (§22.2 NOTE 1). MDL-03.
func TestSpec_7_2_Table4_ObjectReferenceLength(t *testing.T) {
	ld64 := strings.Repeat("L", 64)
	// "MMXU1.Hz" plus padding: a path below the LD of exactly n characters.
	path := func(n int) string { return "MMXU1.Hz" + strings.Repeat("x", n-len("MMXU1.Hz")) }
	tests := []struct {
		in string
		ok bool
	}{
		{ld64 + "/" + path(64), true},          // 129 without FC
		{ld64 + "/" + path(64) + "[MX]", true}, // 129 + "[MX]": the FC is not counted
		{ld64 + "/" + path(65), false},         // 130, path 65
		{"LD0/" + path(64), true},              // path 64
		{"LD0/" + path(65), false},             // path 65
		{strings.Repeat("L", 65) + "/MMXU1", false},
		{ld64, true},
		{ld64 + "/LLN0[ST]", true},
	}
	for _, tc := range tests {
		_, err := ParseStrict(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("ParseStrict(len %d) error = %v, want ok %v", len(tc.in), err, tc.ok)
		}
		if err != nil && !errors.Is(err, model.ErrInvalidReference) {
			t.Errorf("error does not wrap ErrInvalidReference: %v", err)
		}
	}
	if _, err := ParseStrict(strings.Repeat("a", 200)); err == nil {
		t.Error("ParseStrict must reject long input")
	}
}

// IEC 61850-7-2:2010 §22.2: LDName ≤ 64 characters. Validate enforces
// it for every construction path, not only Parse. MDL-04.
func TestSpec_7_2_22_2_LDLengthInValidate(t *testing.T) {
	ld65 := strings.Repeat("L", 65)
	if err := (Ref{LD: ld65, LN: "LLN0"}).Validate(); !errors.Is(err, model.ErrInvalidReference) {
		t.Errorf("Validate with a 65-character LD: %v", err)
	}
	if _, err := FromMMS(ld65, "LLN0$ST$Mod$stVal"); !errors.Is(err, model.ErrInvalidReference) {
		t.Errorf("FromMMS with a 65-character domain: %v", err)
	}
	if err := (Ref{LD: strings.Repeat("L", 64), LN: "LLN0"}).Validate(); err != nil {
		t.Error(err)
	}
}

// IEC 61850-7-2:2010 §22.2 naming conventions, checked by ValidateNames
// (Parse stays lenient for device interoperability). MDL-04.
func TestSpec_7_2_22_2_NamingConventions(t *testing.T) {
	tests := []struct {
		in   string
		rule string // "" for valid
	}{
		{"LD0/MMXU1.PhV.phsA.cVal.mag.f[MX]", ""},
		{"LD0/LLN0.Mod.stVal[ST]", ""},
		{"LD0/Q0XCBR1.Pos", ""},
		{"LD0/ABCDEFGHIJKLMMXU", ""},         // m+n = 12
		{"LD0/MHAI1.HA.phsAHar(3).cVal", ""}, // the index is not part of the name
		{"LD_0", ""},
		{"1LD/MMXU1.Hz", "start with an alpha"},   // LDName
		{"LD0/0MMXU1.Hz", "start with an alpha"},  // LN-Prefix
		{"LD0/ABCDEFGHIJKLMMXU1.Hz", "exceed 12"}, // m+n = 13
		{"LD0/MMXU1.Hä", "allows A-Z"},            // character set
		{"LD0/MMXU1.Hz-x", "allows A-Z"},
		{"@DataSet5", "allows A-Z"},               // TPAA scope of §22.3, not an LD name
		{"LD0/GGIO1.ABCDEFGHIJKLM", "exceeds 12"}, // DataObjectName ≤ 12
		{"LD0/GGIO1.ind1", "upper-case letter"},
		{"LD0/mmxu1.Hz", "upper-case"},                    // LN class: 4 upper-case letters
		{"LD0/MMXU1.Hz." + strings.Repeat("a", 53), "61"}, // FCDA ≤ 61: 62 characters
	}
	for _, tc := range tests {
		r, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.in, err)
		}
		err = r.ValidateNames()
		switch {
		case tc.rule == "" && err != nil:
			t.Errorf("ValidateNames(%q) = %v", tc.in, err)
		case tc.rule != "" && (err == nil || !strings.Contains(err.Error(), tc.rule) || !errors.Is(err, model.ErrInvalidReference)):
			t.Errorf("ValidateNames(%q) = %v, want rule %q", tc.in, err, tc.rule)
		}
	}
	if err := (Ref{}).ValidateNames(); err == nil {
		t.Error("ValidateNames runs Validate first")
	}
}

// IEC 61850-8-1:2011 §8.1.3.2.2: MMS object and domain names are
// limited by the 64-character MMS identifier; §7.3.1 bounds the
// flattened "$" names the same way. MDL-05.
func TestSpec_8_1_8_1_3_2_2_MMSIdentifierLength(t *testing.T) {
	item := func(n int) string { return "MMXU1$MX$Hz" + strings.Repeat("x", n-len("MMXU1$MX$Hz")) }
	for _, tc := range []struct {
		domain, item string
		ok           bool
	}{
		{"LD0", item(64), true},
		{"LD0", item(65), false},
		{strings.Repeat("D", 64), "LLN0", true},
		{strings.Repeat("D", 65), "LLN0", false},
	} {
		r, err := FromMMS(tc.domain, tc.item)
		if (err == nil) != tc.ok {
			t.Errorf("FromMMS(%d, %d) = %v, want ok %v", len(tc.domain), len(tc.item), err, tc.ok)
			continue
		}
		if !tc.ok {
			if !errors.Is(err, model.ErrInvalidReference) || (len(tc.domain) <= 64 && !errors.Is(err, model.ErrLimitExceeded)) {
				t.Errorf("FromMMS error = %v", err)
			}
			continue
		}
		if _, _, err := r.ToMMS(); err != nil {
			t.Errorf("ToMMS: %v", err)
		}
	}
	long, err := Parse("LD0/MMXU1.Hz" + strings.Repeat("x", 60) + "[MX]")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := long.ToMMS(); !errors.Is(err, model.ErrLimitExceeded) || !errors.Is(err, model.ErrInvalidReference) {
		t.Errorf("ToMMS of a 79-character item: %v", err)
	}
}

// IEC 61850-7-2:2010 Table 20 and IEC 61850-8-1:2011 Figure 4: FCs are
// two upper-case letters; "[mx]" is not one. MDL-06.
func TestSpec_7_2_Table20_FunctionalConstraintSyntax(t *testing.T) {
	for _, bad := range []string{"LD0/MMXU1.Hz[mx]", "LD0/MMXU1.Hz[Mx]", "LD0/MMXU1.Hz[M1]", "MMXU.Hz[mx]"} {
		if _, err := Normalize(bad); !errors.Is(err, model.ErrInvalidReference) {
			t.Errorf("Normalize(%q) error = %v", bad, err)
		}
	}
	if err := (Ref{LD: "LD0", LN: "MMXU1", FC: "mx"}).Validate(); err == nil {
		t.Error("Validate must reject a lower-case FC")
	}
	if err := (ClassRef{LNClass: "MMXU", FC: "mx"}).Validate(); err == nil {
		t.Error("ClassRef.Validate must reject a lower-case FC")
	}
	if _, err := Parse("LD0/MMXU1.Hz[XX]"); err != nil {
		t.Errorf("unknown upper-case FCs are accepted: %v", err)
	}
}
