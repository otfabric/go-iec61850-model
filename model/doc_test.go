// SPDX-License-Identifier: MIT

package model

import "testing"

func TestDocText(t *testing.T) {
	tests := []struct {
		d          Doc
		text, long string
	}{
		{Doc{}, "", ""},
		{Doc{Title: "T"}, "T", "T"},
		{Doc{Description: "D"}, "D", "D"},
		{Doc{Title: "T", Description: "D", TitleID: "id"}, "T", "D"},
	}
	for _, tc := range tests {
		if tc.d.Text() != tc.text || tc.d.Long() != tc.long || tc.d.HasText() != (tc.text != "") {
			t.Errorf("%+v: Text %q Long %q", tc.d, tc.d.Text(), tc.d.Long())
		}
		d := tc.d
		d.Clear()
		if d.HasText() || d.TitleID != tc.d.TitleID {
			t.Errorf("Clear: %+v", d)
		}
	}
}

func TestPresences(t *testing.T) {
	ns := &Namespace{
		ConstructedAttributes: []ConstructedAttribute{{SubDataAttributes: []DataAttribute{{Name: "x"}}}},
		CDCs: []CommonDataClass{{
			SubDataObjects:    []DataObject{{Name: "s"}},
			DataAttributes:    []DataAttribute{{Name: "a"}},
			ServiceParameters: []ServiceParameter{{Name: "ctlVal"}},
		}},
		AbstractLNClasses: []LogicalNodeClass{{DataObjects: []DataObject{{Name: "A"}}}},
		LNClasses:         []LogicalNodeClass{{DataObjects: []DataObject{{Name: "D"}}}},
	}
	ps := ns.Presences()
	if len(ps) != 9 {
		t.Fatalf("Presences = %d, want 9", len(ps))
	}
	ps[len(ps)-1].ArgsText = "set"
	if ns.LNClasses[0].DataObjects[0].DSPresence.ArgsText != "set" {
		t.Error("Presences must return pointers into the namespace")
	}
}
