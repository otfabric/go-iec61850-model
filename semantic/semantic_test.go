// SPDX-License-Identifier: MIT

package semantic_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/resolver"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func TestDescribe(t *testing.T) {
	p := fixtures.Profile(t)
	tests := []struct {
		ref       string
		quantity  semantic.Quantity
		phase     semantic.Phase
		reference semantic.Reference
		component semantic.Component
		unit      string
		rule      string
	}{
		{"MMXU.Hz", semantic.QuantityFrequency, "", "", "", "Hz", "mmxu.hz"},
		{"MMXU.Hz.mag.f", semantic.QuantityFrequency, "", "", semantic.ComponentMagnitude, "Hz", "mmxu.hz"},
		{"LD0/MMXU1.Hz.mag.f[MX]", semantic.QuantityFrequency, "", "", semantic.ComponentMagnitude, "Hz", "mmxu.hz"},
		{"MMXU.Hz.instMag.f", semantic.QuantityFrequency, "", "", semantic.ComponentInstantaneous, "Hz", "mmxu.hz"},
		{"MMXU.PNV.phsA", semantic.QuantityVoltage, semantic.PhaseA, semantic.ReferenceLineNeutral, "", "V", "mmxu.pnv"},
		{"MMXU.PNV.phsA.cVal.mag.f", semantic.QuantityVoltage, semantic.PhaseA, semantic.ReferenceLineNeutral, semantic.ComponentMagnitude, "V", "mmxu.pnv"},
		{"MMXU.PNV.phsA.cVal.ang.f", semantic.QuantityVoltage, semantic.PhaseA, semantic.ReferenceLineNeutral, semantic.ComponentAngle, "deg", "mmxu.pnv"},
		{"MMXU.PNV.phsA.cVal", semantic.QuantityVoltage, semantic.PhaseA, semantic.ReferenceLineNeutral, semantic.ComponentComplex, "V", "mmxu.pnv"},
		{"MMXU.PhV.phsB", semantic.QuantityVoltage, semantic.PhaseB, semantic.ReferenceLineGround, "", "V", "mmxu.phv"},
		{"MMXU.PPV.phsAB", semantic.QuantityVoltage, semantic.PhaseAB, semantic.ReferenceLineLine, "", "V", "mmxu.ppv"},
		{"MMXU.PPV", semantic.QuantityVoltage, "", semantic.ReferenceLineLine, "", "V", "mmxu.ppv"},
		{"MMXU.A.phsA", semantic.QuantityCurrent, semantic.PhaseA, "", "", "A", "mmxu.a"},
		{"MMXU.A.neut", semantic.QuantityCurrent, semantic.PhaseNeutral, "", "", "A", "mmxu.a"},
		{"MMXU.W.net", semantic.QuantityActivePower, semantic.PhaseNet, "", "", "W", "mmxu.w"},
		{"MMXU.TotW", semantic.QuantityActivePower, semantic.PhaseTotal, "", "", "W", "mmxu.totw"},
		{"MMXU.TotVAr.mag.f", semantic.QuantityReactivePower, semantic.PhaseTotal, "", semantic.ComponentMagnitude, "VAr", "mmxu.totvar"},
		{"MMXU.TotVA", semantic.QuantityApparentPower, semantic.PhaseTotal, "", "", "VA", "mmxu.totva"},
		{"MMXU.Hz.q", semantic.QuantityFrequency, "", "", semantic.ComponentQuality, "", "mmxu.hz"},
		{"MMXU.Hz.t", semantic.QuantityFrequency, "", "", semantic.ComponentTimestamp, "", "mmxu.hz"},
		{"MMXU.Hz.units", semantic.QuantityFrequency, "", "", semantic.ComponentUnitConfig, "", "mmxu.hz"},
		{"MMXU.Hz.d", semantic.QuantityFrequency, "", "", semantic.ComponentOther, "", "mmxu.hz"},
		{"MMTR.TotWh.actVal", semantic.QuantityActiveEnergy, semantic.PhaseTotal, "", semantic.ComponentCounter, "Wh", "mmtr.totwh"},
		{"DTST.OutW", "", "", "", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			d, err := semantic.Describe(p, tc.ref)
			if tc.rule == "" {
				if !errors.Is(err, model.ErrNoMapping) {
					t.Fatalf("error = %v, want ErrNoMapping", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if d.Quantity != tc.quantity || d.Phase != tc.phase || d.Reference != tc.reference || d.Component != tc.component || d.UnitHint.String() != tc.unit || d.Rule != tc.rule {
				t.Errorf("got %s/%s/%s/%s/%s/%s", d.Quantity, d.Phase, d.Reference, d.Component, d.UnitHint, d.Rule)
			}
			if d.Basis != semantic.BasisRule || d.Source != semantic.DefaultSource || d.Confidence != semantic.ConfidenceHigh || d.LNClass == "" || d.CDC == "" || d.Namespace.IsZero() {
				t.Errorf("audit fields %+v", d)
			}
		})
	}
}

func TestDescribeDetails(t *testing.T) {
	p := fixtures.Profile(t)
	d, err := semantic.Describe(p, "MMXU.PNV.phsC.cVal.mag.i")
	if err != nil {
		t.Fatal(err)
	}
	if d.SubDataObject != "phsC" || d.Path != "MMXU.PNV.phsC.cVal.mag.i" || !d.Presence.IsOptional() || d.Namespace != fixtures.ID74 || len(d.Notes) != 1 || !strings.Contains(d.Notes[0], "sVC") {
		t.Errorf("descriptor %+v", d)
	}
	d, err = semantic.Describe(p, "MMTR.SupWh")
	if err != nil || d.Direction != semantic.DirectionSupply {
		t.Errorf("SupWh: %+v %v", d, err)
	}
	d, err = semantic.Describe(p, "MMTR.TotWh.actVal")
	if err != nil || len(d.Notes) == 0 || !strings.Contains(d.Notes[0], "pulsQty") {
		t.Errorf("counter notes: %+v %v", d, err)
	}
	for _, ref := range []string{"MMXU", "MMXU.Beh", "XXXX.Hz", "MMXU.Nope"} {
		if _, err := semantic.Describe(p, ref); err == nil {
			t.Errorf("Describe(%s) must fail", ref)
		}
	}
	if _, err := semantic.Describe(p, "MMXU"); !errors.Is(err, model.ErrNoMapping) {
		t.Errorf("class: %v", err)
	}
	if _, err := semantic.Describe(p, "XXXX.Hz"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown class keeps the resolver error: %v", err)
	}
}

func TestFindMeasurements(t *testing.T) {
	p := fixtures.Profile(t)
	ds, err := semantic.FindMeasurements(p, "MMXU")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range ds {
		got = append(got, d.DataObject+"."+d.SubDataObject)
	}
	// TotW, TotVAr, TotVA, Hz, PPV (3), PNV (5), PhV (5), A (5), W (5).
	if len(ds) != 4+3+5*4 || got[0] != "TotW." || got[4] != "PPV.phsAB" || got[7] != "PNV.phsA" {
		t.Errorf("measurements (%d): %v", len(ds), got)
	}
	if ds, err := semantic.FindMeasurements(p, "GGIO"); err != nil || len(ds) != 0 {
		t.Errorf("GGIO has no measurements: %v %v", ds, err)
	}
	if _, err := semantic.FindMeasurements(p, "XXXX"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown class: %v", err)
	}
}

func TestMatch(t *testing.T) {
	p := fixtures.Profile(t)
	ds, err := semantic.Match(p, semantic.Criteria{Quantity: semantic.QuantityVoltage, Phase: semantic.PhaseA, Reference: semantic.ReferenceLineNeutral})
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Path != "MMXU.PNV.phsA" {
		t.Errorf("Match = %+v", ds)
	}
	ds, _ = semantic.Match(p, semantic.Criteria{Quantity: semantic.QuantityActiveEnergy})
	if len(ds) != 3 {
		t.Errorf("active energy measurements = %d", len(ds))
	}
	ds, _ = semantic.Match(p, semantic.Criteria{Direction: semantic.DirectionDemand, LNClass: "MMTR", MinConfidence: semantic.ConfidenceHigh})
	if len(ds) != 1 || ds[0].DataObject != "DmdWh" {
		t.Errorf("demand = %+v", ds)
	}
	if ds, _ := semantic.Match(p, semantic.Criteria{LNClass: "MMXU", Quantity: semantic.QuantityFrequency}); len(ds) != 1 {
		t.Errorf("frequency in MMXU = %d", len(ds))
	}
	if _, err := semantic.Match(fixtures.Catalog(t), semantic.Criteria{}); !errors.Is(err, model.ErrAmbiguousReference) {
		t.Errorf("Match on an ambiguous catalog: %v", err)
	}
}

func TestOverridesAndRules(t *testing.T) {
	p := fixtures.Profile(t)
	m, err := semantic.Default.WithOverrides(semantic.Rule{
		ID: "vendor.dtst.outw", LNClass: "DTST", DataObject: "OutW", CDC: "MV",
		Quantity: semantic.QuantityActivePower, Phase: semantic.PhaseTotal, Unit: "kW",
		Confidence: semantic.ConfidenceLow, Source: "vendor:example", Note: "vendor documentation",
	}, semantic.Rule{
		ID: "vendor.hz", LNClass: "MMXU", DataObject: "Hz", CDC: "MV", Quantity: semantic.QuantityFrequency, Source: "vendor:example",
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := m.Describe(p, "DTST.OutW.mag.f")
	if err != nil {
		t.Fatal(err)
	}
	if d.Basis != semantic.BasisOverride || d.Source != "vendor:example" || d.UnitHint.String() != "kW" || d.Confidence != semantic.ConfidenceLow || d.Notes[0] != "vendor documentation" {
		t.Errorf("override descriptor %+v", d)
	}
	if d, _ := m.Describe(p, "MMXU.Hz"); d.Rule != "vendor.hz" || d.Basis != semantic.BasisOverride {
		t.Errorf("override must win over the default rule: %+v", d)
	}
	if len(m.Overrides()) != 2 || len(m.Rules()) != len(semantic.DefaultRules()) {
		t.Error("rule listings")
	}

	// The wildcard rule applies to Hz of any class, with medium confidence.
	any, err := semantic.NewMapper([]semantic.Rule{{ID: "x", LNClass: semantic.AnyClass, DataObject: "OutW", CDC: "MV", Quantity: semantic.QuantityActivePower}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d, err := any.Describe(p, "DTST.OutW"); err != nil || d.Confidence != semantic.ConfidenceMedium {
		t.Errorf("wildcard: %+v %v", d, err)
	}
	// A rule whose CDC does not match the model is not applied.
	wrong, _ := semantic.NewMapper([]semantic.Rule{{ID: "w", LNClass: "MMXU", DataObject: "Hz", CDC: "CMV", Quantity: semantic.QuantityFrequency}}, nil)
	if _, err := wrong.Describe(p, "MMXU.Hz"); !errors.Is(err, model.ErrNoMapping) {
		t.Errorf("CDC mismatch: %v", err)
	}
	// Rules for base classes apply to subclasses.
	base, _ := semantic.NewMapper([]semantic.Rule{{ID: "b", LNClass: "StatisticsLN", DataObject: "ClcExp", CDC: "SPS", Quantity: semantic.QuantityFrequency}}, nil)
	if d, err := base.Describe(p, "MMXU.ClcExp"); err != nil || d.Rule != "b" {
		t.Errorf("inherited rule: %v", err)
	}

	bad := []struct {
		rules, overrides []semantic.Rule
	}{
		{rules: []semantic.Rule{{}}},
		{rules: []semantic.Rule{{ID: "a", LNClass: "X"}}},
		{rules: []semantic.Rule{{ID: "a", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: "speed"}}},
		{rules: []semantic.Rule{{ID: "a", LNClass: "X", DataObject: "D", CDC: "WYE", Quantity: semantic.QuantityCurrent, Phase: semantic.PhaseA}}},
		{rules: []semantic.Rule{{ID: "a", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityCurrent, Unit: "furlong"}}},
		{rules: []semantic.Rule{{ID: "a", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityCurrent, Unit: "V"}}},
		{rules: []semantic.Rule{
			{ID: "a", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityCurrent},
			{ID: "b", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityCurrent},
		}},
		{rules: []semantic.Rule{
			{ID: "a", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityCurrent},
			{ID: "a", LNClass: "Y", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityCurrent},
		}},
		{overrides: []semantic.Rule{{ID: "o", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityCurrent}}},
	}
	for i, tc := range bad {
		if _, err := semantic.NewMapper(tc.rules, tc.overrides); !errors.Is(err, model.ErrInvalidRule) {
			t.Errorf("case %d: error = %v, want ErrInvalidRule", i, err)
		}
	}
}

func TestDescribeResolutionAndWYEEdgeCases(t *testing.T) {
	p := fixtures.Profile(t)
	res, err := resolver.New(p).ResolveString("MMXU.A.phsB.cVal.mag.f")
	if err != nil {
		t.Fatal(err)
	}
	if d, err := semantic.Default.DescribeResolution(p, res); err != nil || d.Phase != semantic.PhaseB {
		t.Errorf("DescribeResolution: %+v %v", d, err)
	}
	// A WYE data object whose sub data object has no phase mapping.
	id := model.NamespaceID{Standard: "W", Version: "1"}
	ns := &model.Namespace{
		ID: id,
		CDCs: []model.CommonDataClass{
			{Name: "WYE", Source: id, SubDataObjects: []model.DataObject{{Name: "odd", Type: "X"}}},
			{Name: "X", Source: id},
		},
		LNClasses: []model.LogicalNodeClass{{Name: "MMXU", Source: id, DataObjects: []model.DataObject{{Name: "A", Type: "WYE"}, {Name: "Ind", Type: "X", Presence: model.PresenceCondition{Name: "Omulti"}}}}},
	}
	c, err := catalog.New(ns)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := semantic.Describe(c, "MMXU.A.odd"); !errors.Is(err, model.ErrNoMapping) {
		t.Errorf("unmapped sub data object: %v", err)
	}
	ds, err := semantic.FindMeasurements(c, "MMXU")
	if err != nil || len(ds) != 0 {
		t.Errorf("unmapped phases are skipped: %v %v", ds, err)
	}
	m, _ := semantic.NewMapper([]semantic.Rule{{ID: "i", LNClass: "MMXU", DataObject: "Ind", CDC: "X", Quantity: semantic.QuantityCurrent}}, nil)
	d, err := m.Describe(c, "MMXU.Ind2")
	if err != nil || d.DataObject != "Ind" || len(d.Notes) != 1 || !strings.Contains(d.Notes[0], "instance 2") {
		t.Errorf("multi-instance: %+v %v", d, err)
	}
}

func TestConcurrentDescribe(t *testing.T) {
	p := fixtures.Profile(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = semantic.Describe(p, "MMXU.PNV.phsA.cVal.mag.f")
				_, _ = semantic.Match(p, semantic.Criteria{Quantity: semantic.QuantityVoltage})
			}
		}()
	}
	wg.Wait()
}
