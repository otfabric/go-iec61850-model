// SPDX-License-Identifier: MIT

package reference

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
)

// The fuzz targets of the class and MMS forms of a reference. A parsed
// reference prints to a string that parses to the same reference, and
// every error is a *model.ReferenceError that matches
// model.ErrInvalidReference.

func checkReferenceError(t *testing.T, input string, err error) {
	t.Helper()
	var re *model.ReferenceError
	if !errors.As(err, &re) || !errors.Is(err, model.ErrInvalidReference) {
		t.Fatalf("%q: error %T does not match ErrInvalidReference: %v", input, err, err)
	}
}

func FuzzParseClass(f *testing.F) {
	for _, s := range []string{"MMXU", "MMXU.Hz", "MMXU.Hz.mag.f[MX]", "MHAI.HA.phsAHar(3).cVal[MX]", "LLN0.Mod.stVal[ST]", "",
		"LD0/MMXU1.Hz", "MMXU..Hz", "MMXU.Hz[mx]", "MMXU.Hz[MX", "MMXU.Hz(", "MMXU.Hz(1)(2)", ".Hz", "MMXU.", "MMXU.Hz[]", "MM XU"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := ParseClass(s)
		if err != nil {
			checkReferenceError(t, s, err)
			if !reflect.DeepEqual(r, ClassRef{}) {
				t.Fatalf("%q: a reference with an error: %+v", s, r)
			}
			return
		}
		if r.LNClass == "" || strings.ContainsAny(r.LNClass, "./[]") || r.Validate() != nil {
			t.Fatalf("%q: accepted %+v", s, r)
		}
		for _, p := range r.Path {
			if p == "" || strings.ContainsAny(p, "./[]") {
				t.Fatalf("%q: path element %q", s, p)
			}
			if _, _, _, err := SplitIndex(p); err != nil {
				t.Fatalf("%q: accepted path element %q with %v", s, p, err)
			}
		}
		// The canonical form is the input itself, and parses to the same reference.
		if r.String() != s {
			t.Fatalf("%q prints as %q", s, r.String())
		}
		again, err := ParseClass(r.String())
		if err != nil || !reflect.DeepEqual(r, again) {
			t.Fatalf("round trip of %q: %+v, %v", s, again, err)
		}
		if n, err := Normalize(s); err != nil || n != s {
			t.Fatalf("Normalize(%q) = %q, %v", s, n, err)
		}
		if got := r.DataObject(); len(r.Path) > 0 && got != r.Path[0] || len(r.Path) == 0 && got != "" {
			t.Fatalf("%q: data object %q", s, got)
		}
	})
}

func FuzzNormalize(f *testing.F) {
	for _, s := range []string{"LD0/MMXU1.Hz.mag.f[MX]", "LD0/MMXU1$MX$Hz$mag$f", "MMXU.Hz.mag.f[MX]", "MMXU1$MX$Hz", "LD0/MMXU1$MX", "LD0/", "/", "$",
		"LD0/LLN0", "LD0", strings.Repeat("L", 65) + "/MMXU1$MX$Hz", "LD0/" + strings.Repeat("M", 65) + "$MX$Hz"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, err := Normalize(s)
		if err != nil {
			checkReferenceError(t, s, err)
			if n != "" {
				t.Fatalf("%q: a result with an error: %q", s, n)
			}
			return
		}
		// A canonical string is its own canonical form, and holds no
		// MMS separator.
		again, err := Normalize(n)
		if err != nil || again != n || strings.Contains(n, "$") {
			t.Fatalf("Normalize(%q) = %q, then %q, %v", s, n, again, err)
		}
		// Only the notation changes: without an MMS separator the input
		// is canonical already.
		if !strings.Contains(s, "$") && n != s {
			t.Fatalf("Normalize(%q) = %q", s, n)
		}
	})
}

func FuzzFromMMS(f *testing.F) {
	for _, s := range [][2]string{{"LD0", "MMXU1$MX$Hz$mag$f"}, {"LD0", "LLN0"}, {"LD0", ""}, {"", "MMXU1"}, {"LD0", "$MX"}, {"LD0", "MMXU1$"},
		{"LD0", "MMXU1$MX$"}, {"LD0", "MMXU1$mx$Hz"}, {strings.Repeat("L", 64), strings.Repeat("M", 64)}, {strings.Repeat("L", 65), "MMXU1"}} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, domain, item string) {
		r, err := FromMMS(domain, item)
		if err != nil {
			checkReferenceError(t, domain+"/"+item, err)
			if !reflect.DeepEqual(r, Ref{}) {
				t.Fatalf("%q/%q: a reference with an error: %+v", domain, item, r)
			}
			return
		}
		if r.LD != domain || r.Validate() != nil || len(domain) > 64 || len(item) > 64 {
			t.Fatalf("%q/%q: accepted %+v", domain, item, r)
		}
		// The reference prints to a string that parses to itself.
		again, err := Parse(r.String())
		if err != nil || !reflect.DeepEqual(r, again) {
			t.Fatalf("%q/%q: %q parses to %+v, %v", domain, item, r.String(), again, err)
		}
		// Back to MMS: the same domain and item, when the reference has
		// what an item ID needs.
		if d, i, err := r.ToMMS(); err == nil && (d != domain || i != item) {
			t.Fatalf("%q/%q: back to MMS as %q/%q", domain, item, d, i)
		}
	})
}

func FuzzParseLNName(f *testing.F) {
	for _, s := range []string{"LLN0", "MMXU1", "Q0MMXU1", "MMXU", "Q0_MMXU12", "mmxu1", "MXU1", "", "1", "Q0MMXU", "Q-MMXU1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, err := ParseLNName(s)
		if err != nil {
			checkReferenceError(t, s, err)
			if n != (LNName{}) {
				t.Fatalf("%q: a name with an error: %+v", s, n)
			}
			return
		}
		if n.String() != s || len(n.Class) != 4 || strings.Trim(n.Inst, "0123456789") != "" {
			t.Fatalf("%q: split into %+v", s, n)
		}
		again, err := ParseLNName(n.String())
		if err != nil || again != n {
			t.Fatalf("round trip of %q: %+v, %v", s, again, err)
		}
	})
}
