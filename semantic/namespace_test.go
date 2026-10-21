// SPDX-License-Identifier: MIT

package semantic_test

import (
	"errors"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func TestFindMeasurementsIn(t *testing.T) {
	c := fixtures.Catalog(t) // two releases of IEC 61850-7-4 define MMXU
	if _, err := semantic.FindMeasurements(c, "MMXU"); !errors.Is(err, model.ErrAmbiguousSymbol) {
		t.Fatalf("unqualified: %v", err)
	}
	ds, err := semantic.FindMeasurementsIn(c, fixtures.ID74R2, "MMXU")
	if err != nil || len(ds) == 0 {
		t.Fatalf("FindMeasurementsIn(R2): %d, %v", len(ds), err)
	}
	for _, d := range ds {
		if d.LNClass != "MMXU" {
			t.Errorf("descriptor of %s", d.LNClass)
		}
	}
	if _, err := semantic.FindMeasurementsIn(c, fixtures.ID73, "MMXU"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("class not in namespace: %v", err)
	}
}
