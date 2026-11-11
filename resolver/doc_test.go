// SPDX-License-Identifier: MIT

package resolver_test

import "testing"

// The fixture package applies its NSDoc (OTFabric test wording).
func TestResolutionDocs(t *testing.T) {
	r := newResolver(t)
	res, err := r.ResolveString("MMXU.Hz.mag.f")
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps[0].Doc.Title != "Test measurement class" || res.Steps[1].Doc.Description != "Test text for frequency." {
		t.Errorf("step docs: %+v %+v", res.Steps[0].Doc, res.Steps[1].Doc)
	}
	if res.Description() != "" || res.Doc().HasText() {
		t.Errorf("leaf f has no text: %q", res.Description())
	}
	hz, err := r.ResolveString("MMXU.Hz")
	if err != nil {
		t.Fatal(err)
	}
	if hz.Description() != "Test text for frequency." {
		t.Errorf("Description = %q", hz.Description())
	}
	if mmxu, _ := r.ResolveString("MMXU"); mmxu.Description() == "" {
		t.Error("class description")
	}
}
