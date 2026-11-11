// SPDX-License-Identifier: MIT

package semantic_test

import (
	"errors"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func TestNormalizeUnit(t *testing.T) {
	tests := []struct {
		in     string
		symbol string
		mult   int
		str    string
	}{
		{"V", "V", 0, "V"}, {"kV", "V", 3, "kV"}, {" kV ", "V", 3, "kV"}, {"mA", "A", -3, "mA"}, {"MA", "A", 6, "MA"},
		{"Hz", "Hz", 0, "Hz"}, {"MW", "W", 6, "MW"}, {"kvar", "VAr", 3, "kVAr"}, {"MVAr", "VAr", 6, "MVAr"},
		{"kWh", "Wh", 3, "kWh"}, {"kvarh", "VArh", 3, "kVArh"}, {"GWh", "Wh", 9, "GWh"}, {"%", "%", 0, "%"},
		{"Ω", "ohm", 0, "ohm"}, {"kOhm", "ohm", 3, "kohm"}, {"°", "deg", 0, "deg"}, {"°C", "degC", 0, "degC"},
		{"m", "m", 0, "m"}, {"µA", "A", -6, "µA"}, {"uA", "A", -6, "µA"}, {"daV", "V", 1, "daV"}, {"1", "1", 0, "1"},
		{"Watts", "W", 0, "W"}, {"kPa", "Pa", 3, "kPa"}, {"hPa", "Pa", 2, "hPa"}, {"mm/s", "m/s", -3, "mm/s"},
		{"m3/s", "m³/s", 0, "m³/s"}, {"W/m2", "W/m²", 0, "W/m²"}, {"kg/m3", "kg/m³", 0, "kg/m³"}, {"ppm", "ppm", 0, "ppm"},
		{"kAh", "Ah", 3, "kAh"}, {"mS/m", "S/m", -3, "mS/m"}, {"dB", "dB", 0, "dB"}, {"min", "min", 0, "min"}, {"h", "h", 0, "h"},
		{"K/s", "K/s", 0, "K/s"}, {"rad/s", "rad/s", 0, "rad/s"}, {"l/s", "l/s", 0, "l/s"},
	}
	for _, tc := range tests {
		u, err := semantic.NormalizeUnit(tc.in)
		if err != nil || u.Symbol != tc.symbol || u.Multiplier != tc.mult || u.String() != tc.str {
			t.Errorf("NormalizeUnit(%q) = %+v (%s), %v", tc.in, u, u.String(), err)
		}
	}
	for _, bad := range []string{"", "furlong", "fortnight", "k%", "kdeg", "Kv", "kppm", "kmin", "Mh", "kdB"} {
		if _, err := semantic.NormalizeUnit(bad); !errors.Is(err, model.ErrUnknownUnit) {
			t.Errorf("NormalizeUnit(%q) error = %v", bad, err)
		}
	}
	u := semantic.Unit{Symbol: "V", Multiplier: 4}
	if u.String() != "V·10^4" || u.Factor() != 1e4 || u.IsZero() || (semantic.Unit{}).String() != "" {
		t.Errorf("Unit helpers: %s %v", u, u.Factor())
	}
}

func TestQuantities(t *testing.T) {
	// Quantities without a default unit: states and counts, and those
	// whose unit the standard leaves to the device.
	noUnit := map[semantic.Quantity]bool{semantic.QuantityImbalance: true, semantic.QuantityRotationalSpeed: true, semantic.QuantityVibration: true}
	for _, q := range semantic.Quantities() {
		if !q.IsKnown() || q.Label() == "" {
			t.Errorf("quantity %s incomplete", q)
		}
		switch u := q.DefaultUnit(); {
		case q.IsState() || noUnit[q]:
			if !u.IsZero() {
				t.Errorf("quantity %s has default unit %s", q, u)
			}
		case u.IsZero() || !q.Accepts(u):
			t.Errorf("quantity %s: default unit %q", q, u)
		}
	}
	if semantic.QuantityVoltage.IsState() || !semantic.QuantityTapPosition.IsState() {
		t.Error("IsState")
	}
	kv, _ := semantic.NormalizeUnit("kV")
	if !semantic.QuantityVoltage.Accepts(kv) || semantic.QuantityCurrent.Accepts(kv) {
		t.Error("Accepts")
	}
	if semantic.Quantity("furlongs-per-fortnight").IsKnown() || !semantic.Quantity("furlongs-per-fortnight").DefaultUnit().IsZero() {
		t.Error("unknown quantity")
	}
	if !semantic.PhaseAB.IsPair() || semantic.PhaseA.IsPair() {
		t.Error("IsPair")
	}
	if !semantic.ComponentAngle.IsValue() || semantic.ComponentQuality.IsValue() {
		t.Error("IsValue")
	}
}

func TestDeviceUnit(t *testing.T) {
	p := fixtures.Profile(t)
	tests := []struct {
		si, mult int
		want     string
	}{
		{33, 0, "Hz"}, {29, 3, "kV"}, {38, 6, "MW"}, {63, 3, "kVAr"}, {5, -3, "mA"},
	}
	for _, tc := range tests {
		u, err := semantic.DeviceUnit(p, tc.si, tc.mult)
		if err != nil || u.String() != tc.want {
			t.Errorf("DeviceUnit(%d, %d) = %s, %v; want %s", tc.si, tc.mult, u, err, tc.want)
		}
	}
	if _, err := semantic.DeviceUnit(p, 999, 0); !errors.Is(err, model.ErrUnknownUnit) {
		t.Errorf("unknown SI unit: %v", err)
	}
	if _, err := semantic.DeviceUnit(p, 29, 4); !errors.Is(err, model.ErrUnknownUnit) {
		t.Errorf("unknown multiplier: %v", err)
	}
	empty, _ := catalog.New(&model.Namespace{ID: model.NamespaceID{Standard: "E", Version: "1"}})
	if _, err := semantic.DeviceUnit(empty, 29, 0); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("no SIUnitKind: %v", err)
	}
	onlySI, _ := catalog.New(&model.Namespace{
		ID:           model.NamespaceID{Standard: "E", Version: "1"},
		Enumerations: []model.Enumeration{{Name: semantic.SIUnitEnumeration, Literals: []model.Literal{{Name: "m²", Value: 41}}}},
	})
	if u, err := semantic.DeviceUnit(onlySI, 41, 0); err != nil || u.Symbol != "m²" {
		t.Errorf("verbatim literal: %+v %v", u, err)
	}
	if _, err := semantic.DeviceUnit(onlySI, 41, 3); !errors.Is(err, model.ErrUnknownType) {
		t.Errorf("no MultiplierKind: %v", err)
	}
}
