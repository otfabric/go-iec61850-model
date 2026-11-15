// SPDX-License-Identifier: MIT

package model

import (
	"errors"
	"strings"
	"testing"
)

// A parsed namespace ID prints to a string that parses, to the same ID
// when it has a revision or no release, and every error matches
// ErrUnknownNamespace.
func FuzzParseNamespaceID(f *testing.F) {
	for _, s := range []string{"IEC 61850-7-4:2007B5", "IEC 61850-7-4:2007B", "IEC 61850-7-4", "(Tr)IEC 61850-90-9:2018A3", "x:2003", "",
		":2007", "a:b", "a:1b2c", "a:1B-1", "a:1B+5", "a:1B99999999999999999999", " a : 1 ", "a:b:2007A1", "a:1B0", "a:01B007", "x:2+01"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseNamespaceID(s)
		if err != nil {
			if !errors.Is(err, ErrUnknownNamespace) || !id.IsZero() {
				t.Fatalf("%q: ID %+v, error %v", s, id, err)
			}
			return
		}
		if id.Standard == "" || id.Standard != strings.TrimSpace(id.Standard) || id.Release < 0 {
			t.Fatalf("%q: accepted %+v", s, id)
		}
		if strings.Trim(id.Version, "0123456789") != "" || strings.TrimFunc(id.Revision, func(r rune) bool {
			return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
		}) != "" {
			t.Fatalf("%q: version %q, revision %q", s, id.Version, id.Revision)
		}
		again, err := ParseNamespaceID(id.String())
		if err != nil {
			t.Fatalf("%q: %q does not parse: %v", s, id.String(), err)
		}
		// The canonical form reads back as the same ID. The exception is
		// a release without a revision ("x:2+01"): its release prints
		// next to the version, and the string reads as a longer version.
		if (id.Revision != "" || id.Release == 0) && (again != id || id.Compare(again) != 0 || !id.SameVersion(again)) {
			t.Fatalf("%q: %q parses to %+v", s, id.String(), again)
		}
		if again.Standard != id.Standard {
			t.Fatalf("%q: %+v read back as %+v", s, id, again)
		}
	})
}
