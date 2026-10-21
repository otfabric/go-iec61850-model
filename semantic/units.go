// SPDX-License-Identifier: MIT

package semantic

import (
	"fmt"
	"math"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Enumeration names IEC 61850-7-3 uses for the units attribute of
// measured values.
const (
	SIUnitEnumeration     = "SIUnitKind"
	MultiplierEnumeration = "MultiplierKind"
)

// Unit is a unit symbol with a decimal multiplier, e.g. {V, 3} for kV.
//
// A Unit returned by [Quantity.DefaultUnit] or found in
// [Descriptor.UnitHint] is a hint derived from the model and OTFabric
// rules. The unit a device actually reports comes from its configured
// units attribute; read it with [DeviceUnit] and compare with
// [Quantity.Accepts].
type Unit struct {
	// Symbol is the canonical base symbol, e.g. "V", "VAr", "Wh",
	// "ohm", "deg" or "1" for dimensionless values.
	Symbol string `json:"symbol"`
	// Multiplier is the decimal exponent, e.g. 3 for kilo.
	Multiplier int `json:"multiplier,omitempty"`
}

// IsZero reports whether no unit is set.
func (u Unit) IsZero() bool { return u.Symbol == "" }

// Factor returns 10^Multiplier, the factor that converts a value in u
// to the base unit.
func (u Unit) Factor() float64 { return math.Pow10(u.Multiplier) }

// String returns the unit with an SI prefix, e.g. "kV", or with an
// explicit exponent when no prefix exists, e.g. "V·10^4".
func (u Unit) String() string {
	if u.Symbol == "" {
		return ""
	}
	if u.Multiplier == 0 {
		return u.Symbol
	}
	if p, ok := prefixSymbols[u.Multiplier]; ok {
		return p + u.Symbol
	}
	return fmt.Sprintf("%s·10^%d", u.Symbol, u.Multiplier)
}

// baseUnits maps accepted spellings to canonical symbols.
var baseUnits = map[string]string{
	"V": "V", "A": "A", "Hz": "Hz", "W": "W", "VA": "VA",
	"VAr": "VAr", "var": "VAr", "VAR": "VAr", "Var": "VAr",
	"Wh": "Wh", "VAh": "VAh", "VArh": "VArh", "varh": "VArh", "VARh": "VArh", "Varh": "VArh",
	"J": "J", "s": "s", "m": "m", "K": "K", "degC": "degC", "°C": "degC",
	"deg": "deg", "°": "deg", "rad": "rad",
	"ohm": "ohm", "Ohm": "ohm", "Ω": "ohm",
	"%": "%", "1": "1", "Hz/s": "Hz/s", "V/Hz": "V/Hz",
}

// unprefixable symbols are never combined with an SI prefix.
var unprefixable = map[string]bool{"%": true, "1": true, "deg": true, "degC": true}

var prefixes = []struct {
	symbol string
	exp    int
}{
	{"da", 1}, {"p", -12}, {"n", -9}, {"µ", -6}, {"μ", -6}, {"u", -6}, {"m", -3}, {"c", -2}, {"d", -1},
	{"h", 2}, {"k", 3}, {"M", 6}, {"G", 9}, {"T", 12},
}

var prefixSymbols = map[int]string{-12: "p", -9: "n", -6: "µ", -3: "m", -2: "c", -1: "d", 1: "da", 2: "h", 3: "k", 6: "M", 9: "G", 12: "T"}

// NormalizeUnit parses a unit string such as "kV", "MVAr", "kvar",
// "kWh", "Hz" or "%" into a canonical [Unit]. Prefixes are
// case-sensitive ("mA" is milliampere, "MA" megaampere). It fails with
// [model.ErrUnknownUnit] for units it does not know rather than
// guessing.
func NormalizeUnit(s string) (Unit, error) {
	s = strings.TrimSpace(s)
	if sym, ok := baseUnits[s]; ok {
		return Unit{Symbol: sym}, nil
	}
	for _, p := range prefixes {
		rest, ok := strings.CutPrefix(s, p.symbol)
		if !ok {
			continue
		}
		if sym, ok := baseUnits[rest]; ok && !unprefixable[sym] {
			return Unit{Symbol: sym, Multiplier: p.exp}, nil
		}
	}
	return Unit{}, fmt.Errorf("%w: %q", model.ErrUnknownUnit, s)
}

// EnumerationLookup finds enumerations by name. *catalog.Profile and
// *catalog.Catalog implement it.
type EnumerationLookup interface {
	Enumeration(name string) (*model.Enumeration, error)
}

// DeviceUnit interprets the values of a device's configured units
// attribute (units.SIUnit and units.multiplier) through the SIUnitKind
// and MultiplierKind enumerations of the loaded model, so no unit
// codes are hard-coded. A multiplier of 0 is accepted even when the
// enumeration lacks it.
//
// Symbols the package does not know are returned verbatim as the
// literal name.
func DeviceUnit(l EnumerationLookup, siUnit, multiplier int) (Unit, error) {
	si, err := l.Enumeration(SIUnitEnumeration)
	if err != nil {
		return Unit{}, err
	}
	lit, ok := si.LiteralByValue(siUnit)
	if !ok {
		return Unit{}, fmt.Errorf("%w: %s has no literal %d", model.ErrUnknownUnit, SIUnitEnumeration, siUnit)
	}
	u := Unit{Symbol: lit.Name}
	if n, err := NormalizeUnit(lit.Name); err == nil && n.Multiplier == 0 {
		u.Symbol = n.Symbol
	}
	if multiplier != 0 {
		mk, err := l.Enumeration(MultiplierEnumeration)
		if err != nil {
			return Unit{}, err
		}
		if _, ok := mk.LiteralByValue(multiplier); !ok {
			return Unit{}, fmt.Errorf("%w: %s has no literal %d", model.ErrUnknownUnit, MultiplierEnumeration, multiplier)
		}
		// MultiplierKind literal values are the decimal exponents.
		u.Multiplier = multiplier
	}
	return u, nil
}
