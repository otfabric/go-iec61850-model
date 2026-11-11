// SPDX-License-Identifier: MIT

package catalog_test

import (
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// The fixture package applies its NSDoc (OTFabric test wording); the
// doc set adds texts for the other kinds.
func TestSearchText(t *testing.T) {
	set := nsd.NewDocSet("en")
	for k, v := range map[string]string{
		"CDC:MV": "Test title for measured value", "DA:MV.mag": "Test text for a magnitude",
		"CA:Vector": "Test title for a vector", "SDA:Vector.ang": "Test text for an angle",
		"ENUM:HealthKind": "Test title for health", "BT:FLOAT32": "Test title for a float",
	} {
		if err := set.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	p, err := nsd.LoadDir(fixtures.Dir(), nsd.WithFallbackDocs(set))
	if err != nil {
		t.Fatal(err)
	}
	c, err := catalog.New(p)
	if err != nil {
		t.Fatal(err)
	}
	prof, err := c.Profile(fixtures.IDExt, fixtures.ID74)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		text  string
		kinds []catalog.Kind
		want  []string // kind:path
	}{
		{"frequency", nil, []string{"do:MMXU.Hz"}},
		{"THREE-PHASE measurement", nil, []string{"ln:MMXU"}},
		{"total power", nil, []string{"do:MMXU.TotW"}},
		{"measured value", nil, []string{"cdc:MV"}},
		{"magnitude", []catalog.Kind{catalog.KindDataAttribute}, []string{"da:MV.mag"}},
		{"angle", nil, []string{"da:Vector.ang"}},
		{"title vector", nil, []string{"ca:Vector"}},
		{"health", []catalog.Kind{catalog.KindEnumeration}, []string{"enum:HealthKind"}},
		{"float", nil, []string{"basic:FLOAT32"}},
		{"frequency power", nil, nil},
		{"Hz", []catalog.Kind{catalog.KindDataObject}, []string{"do:MMXU.Hz"}}, // names match too
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			var got []string
			for _, r := range prof.Search(catalog.Query{Text: tc.text, Kinds: tc.kinds}) {
				got = append(got, string(r.Kind)+":"+r.Path)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
	res := prof.Search(catalog.Query{Text: "frequency"})
	if res[0].Description != "Test text for frequency." {
		t.Errorf("result texts: %+v", res[0])
	}
	if r := prof.Search(catalog.Query{Text: "frequency", Name: "Tot"}); len(r) != 0 {
		t.Errorf("Name and Text both filter: %v", r)
	}
	if r := prof.Search(catalog.Query{Text: "magnitude", FC: model.FCMX}); len(r) != 1 {
		t.Errorf("Text with FC: %v", r)
	}
}
