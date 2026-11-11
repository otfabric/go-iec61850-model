// SPDX-License-Identifier: MIT

package semantic_test

import (
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/semantic"
)

// The fixture package applies its NSDoc (OTFabric test wording).
func TestDescribeDocTexts(t *testing.T) {
	p := fixtures.Profile(t)
	d, err := semantic.Describe(p, "MMXU.Hz.mag.f")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Test text for frequency." || d.Description != "Test text for frequency." {
		t.Errorf("title %q description %q", d.Title, d.Description)
	}
	d, err = semantic.Describe(p, "MMXU.TotVAr")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "" || d.Description != "" {
		t.Errorf("undocumented element: %q %q", d.Title, d.Description)
	}
}
