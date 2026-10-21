// SPDX-License-Identifier: MIT

package resolver_test

import (
	"errors"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// The fixture catalog holds two releases of IEC 61850-7-4, which both
// define MMXU; release 2 adds MMXU.TotPF.
func TestResolverIn(t *testing.T) {
	c := fixtures.Catalog(t)
	r := resolver.New(c)
	if _, err := r.ResolveString("MMXU.Hz.mag.f"); !errors.Is(err, model.ErrAmbiguousSymbol) {
		t.Fatalf("unqualified MMXU: %v", err)
	}

	r2 := r.In(fixtures.ID74R2)
	res, err := r2.ResolveString("LD0/MMXU1.TotPF.mag.f[MX]")
	if err != nil || res.Type() != "FLOAT32" || res.LNClass.Source != fixtures.ID74R2 {
		t.Fatalf("In(R2) TotPF: %v, %v", res, err)
	}
	if _, err := r.In(fixtures.ID74).ResolveString("MMXU.TotPF"); !errors.Is(err, model.ErrUnknownDataObject) {
		t.Errorf("In(R1) TotPF: %v", err)
	}
	if res, err := r2.ResolveAttribute("MMXU", "Hz", "mag", "f"); err != nil || res.FC() != model.FCMX {
		t.Errorf("In(R2) ResolveAttribute: %v, %v", res, err)
	}
	if chain, err := r2.ResolveInheritance("MMXU"); err != nil || chain[0].Source != fixtures.ID74R2 {
		t.Errorf("In(R2) ResolveInheritance: %v, %v", chain, err)
	}
	if _, err := r.ResolveInheritance("MMXU"); !errors.Is(err, model.ErrAmbiguousSymbol) {
		t.Errorf("unqualified ResolveInheritance: %v", err)
	}
	leaves, err := r2.Leaves("MMXU", []string{"TotPF"}, model.FCMX)
	if err != nil || len(leaves) == 0 {
		t.Errorf("In(R2) Leaves: %d, %v", len(leaves), err)
	}
	// A partial ID that matches both releases is ambiguous; one outside
	// the catalog is unknown.
	if _, err := r.In(model.NamespaceID{Standard: "IEC 61850-7-4"}).ResolveString("MMXU.Hz"); !errors.Is(err, model.ErrAmbiguousSymbol) {
		t.Errorf("partial ID over two releases: %v", err)
	}
	if _, err := r.In(model.NamespaceID{Standard: "IEC 61850-7-99"}).ResolveString("MMXU.Hz"); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown namespace: %v", err)
	}
}
