// SPDX-License-Identifier: MIT

package semantic

import (
	"fmt"
	"sort"

	"github.com/otfabric/go-iec61850-model/model"
)

// AnyClass in [Rule.LNClass] makes a rule apply to every logical node
// class.
const AnyClass = "*"

// DefaultSource is the [Rule.Source] of the built-in rules.
const DefaultSource = "otfabric-default"

// Rule maps a data object of a logical node class to engineering
// semantics. Rules are explicit and auditable: a descriptor names the
// rule that produced it.
//
// A rule applies only when the data object has the CDC the rule
// expects, so a name reused with a different meaning does not match.
// For WYE and DEL data objects the phase comes from the sub data
// object (phsA, phsAB, ...), and DEL implies a line-to-line reference.
type Rule struct {
	// ID identifies the rule, e.g. "mmxu.ppv".
	ID string `json:"id"`
	// LNClass is the logical node class, or [AnyClass]. A rule for a
	// class also applies to classes that inherit from it.
	LNClass string `json:"lnClass"`
	// DataObject is the data object name, without instance number.
	DataObject string `json:"dataObject"`
	// CDC is the CDC the data object must have. Required.
	CDC string `json:"cdc"`
	// Quantity is the engineering quantity. Required.
	Quantity Quantity `json:"quantity"`
	// Phase is the phase of a single-valued data object, e.g.
	// [PhaseTotal]. It must be empty for WYE and DEL.
	Phase Phase `json:"phase,omitempty"`
	// Reference is the voltage reference, if any.
	Reference Reference `json:"reference,omitempty"`
	// Direction is the energy direction, if the model distinguishes it.
	Direction Direction `json:"direction,omitempty"`
	// Unit is the unit hint symbol; empty means the quantity's default
	// unit.
	Unit string `json:"unit,omitempty"`
	// Confidence of the mapping. Empty means [ConfidenceHigh] for a
	// class rule and [ConfidenceMedium] for an [AnyClass] rule.
	Confidence Confidence `json:"confidence,omitempty"`
	// Source names who stated the rule, e.g. "otfabric-default" or
	// "vendor:example". Required for overrides.
	Source string `json:"source,omitempty"`
	// Note is an optional remark carried into descriptors.
	Note string `json:"note,omitempty"`
}

func (r Rule) confidence() Confidence {
	if r.Confidence != "" {
		return r.Confidence
	}
	if r.LNClass == AnyClass {
		return ConfidenceMedium
	}
	return ConfidenceHigh
}

func (r Rule) unitHint() Unit {
	if r.Unit == "" {
		return r.Quantity.DefaultUnit()
	}
	if u, err := NormalizeUnit(r.Unit); err == nil {
		return u
	}
	return Unit{Symbol: r.Unit}
}

func (r Rule) validate() error {
	switch {
	case r.ID == "":
		return fmt.Errorf("%w: rule without ID", model.ErrInvalidRule)
	case r.LNClass == "" || r.DataObject == "" || r.CDC == "":
		return fmt.Errorf("%w: rule %s needs LNClass, DataObject and CDC", model.ErrInvalidRule, r.ID)
	case !r.Quantity.IsKnown():
		return fmt.Errorf("%w: rule %s has unknown quantity %q", model.ErrInvalidRule, r.ID, r.Quantity)
	case r.Phase != "" && subDataObjectPhases[r.CDC] != nil:
		return fmt.Errorf("%w: rule %s sets a phase for CDC %s, whose phases come from sub data objects", model.ErrInvalidRule, r.ID, r.CDC)
	}
	if r.Unit != "" {
		u, err := NormalizeUnit(r.Unit)
		if err != nil {
			return fmt.Errorf("%w: rule %s: %v", model.ErrInvalidRule, r.ID, err)
		}
		if !r.Quantity.Accepts(u) {
			return fmt.Errorf("%w: rule %s: unit %s does not measure %s", model.ErrInvalidRule, r.ID, r.Unit, r.Quantity)
		}
	}
	return nil
}

// DefaultRules returns the built-in OTFabric rules for the measurement
// classes MMXU (three-phase measurement), MMXN (non-phase-related
// measurement) and MMTR (metering), plus a frequency rule that applies
// to Hz of CDC MV in any class. The slice is a copy.
func DefaultRules() []Rule {
	out := make([]Rule, len(defaultRules))
	copy(out, defaultRules)
	return out
}

var defaultRules = []Rule{
	{ID: "mmxu.hz", LNClass: "MMXU", DataObject: "Hz", CDC: "MV", Quantity: QuantityFrequency},
	{ID: "mmxu.totw", LNClass: "MMXU", DataObject: "TotW", CDC: "MV", Quantity: QuantityActivePower, Phase: PhaseTotal},
	{ID: "mmxu.totvar", LNClass: "MMXU", DataObject: "TotVAr", CDC: "MV", Quantity: QuantityReactivePower, Phase: PhaseTotal},
	{ID: "mmxu.totva", LNClass: "MMXU", DataObject: "TotVA", CDC: "MV", Quantity: QuantityApparentPower, Phase: PhaseTotal},
	{ID: "mmxu.totpf", LNClass: "MMXU", DataObject: "TotPF", CDC: "MV", Quantity: QuantityPowerFactor, Phase: PhaseTotal},
	{ID: "mmxu.ppv", LNClass: "MMXU", DataObject: "PPV", CDC: "DEL", Quantity: QuantityVoltage, Reference: ReferenceLineLine},
	{ID: "mmxu.pnv", LNClass: "MMXU", DataObject: "PNV", CDC: "WYE", Quantity: QuantityVoltage, Reference: ReferenceLineNeutral},
	{ID: "mmxu.phv", LNClass: "MMXU", DataObject: "PhV", CDC: "WYE", Quantity: QuantityVoltage, Reference: ReferenceLineGround},
	{ID: "mmxu.a", LNClass: "MMXU", DataObject: "A", CDC: "WYE", Quantity: QuantityCurrent},
	{ID: "mmxu.w", LNClass: "MMXU", DataObject: "W", CDC: "WYE", Quantity: QuantityActivePower},
	{ID: "mmxu.var", LNClass: "MMXU", DataObject: "VAr", CDC: "WYE", Quantity: QuantityReactivePower},
	{ID: "mmxu.va", LNClass: "MMXU", DataObject: "VA", CDC: "WYE", Quantity: QuantityApparentPower},
	{ID: "mmxu.pf", LNClass: "MMXU", DataObject: "PF", CDC: "WYE", Quantity: QuantityPowerFactor},
	{ID: "mmxu.z", LNClass: "MMXU", DataObject: "Z", CDC: "WYE", Quantity: QuantityImpedance},

	{ID: "mmxn.amp", LNClass: "MMXN", DataObject: "Amp", CDC: "MV", Quantity: QuantityCurrent},
	{ID: "mmxn.vol", LNClass: "MMXN", DataObject: "Vol", CDC: "MV", Quantity: QuantityVoltage},
	{ID: "mmxn.watt", LNClass: "MMXN", DataObject: "Watt", CDC: "MV", Quantity: QuantityActivePower},
	{ID: "mmxn.volampr", LNClass: "MMXN", DataObject: "VolAmpr", CDC: "MV", Quantity: QuantityReactivePower},
	{ID: "mmxn.volamp", LNClass: "MMXN", DataObject: "VolAmp", CDC: "MV", Quantity: QuantityApparentPower},
	{ID: "mmxn.pwrfact", LNClass: "MMXN", DataObject: "PwrFact", CDC: "MV", Quantity: QuantityPowerFactor},
	{ID: "mmxn.hz", LNClass: "MMXN", DataObject: "Hz", CDC: "MV", Quantity: QuantityFrequency},

	{ID: "mmtr.totwh", LNClass: "MMTR", DataObject: "TotWh", CDC: "BCR", Quantity: QuantityActiveEnergy, Phase: PhaseTotal},
	{ID: "mmtr.totvarh", LNClass: "MMTR", DataObject: "TotVArh", CDC: "BCR", Quantity: QuantityReactiveEnergy, Phase: PhaseTotal},
	{ID: "mmtr.totvah", LNClass: "MMTR", DataObject: "TotVAh", CDC: "BCR", Quantity: QuantityApparentEnergy, Phase: PhaseTotal},
	{ID: "mmtr.supwh", LNClass: "MMTR", DataObject: "SupWh", CDC: "BCR", Quantity: QuantityActiveEnergy, Phase: PhaseTotal, Direction: DirectionSupply},
	{ID: "mmtr.supvarh", LNClass: "MMTR", DataObject: "SupVArh", CDC: "BCR", Quantity: QuantityReactiveEnergy, Phase: PhaseTotal, Direction: DirectionSupply},
	{ID: "mmtr.dmdwh", LNClass: "MMTR", DataObject: "DmdWh", CDC: "BCR", Quantity: QuantityActiveEnergy, Phase: PhaseTotal, Direction: DirectionDemand},
	{ID: "mmtr.dmdvarh", LNClass: "MMTR", DataObject: "DmdVArh", CDC: "BCR", Quantity: QuantityReactiveEnergy, Phase: PhaseTotal, Direction: DirectionDemand},

	{ID: "any.hz", LNClass: AnyClass, DataObject: "Hz", CDC: "MV", Quantity: QuantityFrequency},
}

// Mapper applies mapping rules and overrides. It is immutable and safe
// for concurrent use.
type Mapper struct {
	rules     []Rule // defaults, by key
	overrides []Rule
}

// NewMapper builds a mapper from rules and overrides. Overrides take
// precedence over rules for the same class and data object; they must
// name their Source. Two rules (or two overrides) for the same class
// and data object are rejected, as are incomplete rules, all with
// [model.ErrInvalidRule].
func NewMapper(rules []Rule, overrides []Rule) (*Mapper, error) {
	m := &Mapper{}
	ids := map[string]bool{}
	add := func(list []Rule, override bool) ([]Rule, error) {
		keys := map[string]string{}
		out := make([]Rule, 0, len(list))
		for _, r := range list {
			if err := r.validate(); err != nil {
				return nil, err
			}
			if override && r.Source == "" {
				return nil, fmt.Errorf("%w: override %s must name its source", model.ErrInvalidRule, r.ID)
			}
			if ids[r.ID] {
				return nil, fmt.Errorf("%w: duplicate rule ID %s", model.ErrInvalidRule, r.ID)
			}
			ids[r.ID] = true
			key := r.LNClass + "." + r.DataObject
			if prev, dup := keys[key]; dup {
				return nil, fmt.Errorf("%w: rules %s and %s both map %s", model.ErrInvalidRule, prev, r.ID, key)
			}
			keys[key] = r.ID
			if r.Source == "" {
				r.Source = DefaultSource
			}
			out = append(out, r)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	}
	var err error
	if m.rules, err = add(rules, false); err != nil {
		return nil, err
	}
	if m.overrides, err = add(overrides, true); err != nil {
		return nil, err
	}
	return m, nil
}

// Default is the mapper with [DefaultRules] and no overrides.
var Default = mustDefault()

func mustDefault() *Mapper {
	m, err := NewMapper(defaultRules, nil)
	if err != nil {
		panic(err)
	}
	return m
}

// WithOverrides returns a mapper with the rules of m and the given
// overrides added to those of m.
func (m *Mapper) WithOverrides(overrides ...Rule) (*Mapper, error) {
	return NewMapper(m.rules, append(append([]Rule(nil), m.overrides...), overrides...))
}

// Rules returns the rules of the mapper, ordered by ID.
func (m *Mapper) Rules() []Rule { return append([]Rule(nil), m.rules...) }

// Overrides returns the overrides of the mapper, ordered by ID.
func (m *Mapper) Overrides() []Rule { return append([]Rule(nil), m.overrides...) }

// find returns the rule for a data object of one of the classes of an
// inheritance chain: overrides before rules, the most specific class
// before its base classes, a class rule before an AnyClass rule.
func (m *Mapper) find(chain []string, do string) (Rule, Basis, bool) {
	for _, list := range []struct {
		rules []Rule
		basis Basis
	}{{m.overrides, BasisOverride}, {m.rules, BasisRule}} {
		for _, cls := range append(append([]string(nil), chain...), AnyClass) {
			for _, r := range list.rules {
				if r.LNClass == cls && r.DataObject == do {
					return r, list.basis, true
				}
			}
		}
	}
	return Rule{}, "", false
}
