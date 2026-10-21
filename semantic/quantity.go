// SPDX-License-Identifier: MIT

package semantic

// Quantity is an engineering quantity.
type Quantity string

// Engineering quantities. The identifiers and labels are OTFabric
// vocabulary, not IEC terms, so they can be shared with other protocol
// mappings.
const (
	QuantityFrequency      Quantity = "frequency"
	QuantityVoltage        Quantity = "voltage"
	QuantityCurrent        Quantity = "current"
	QuantityActivePower    Quantity = "active-power"
	QuantityReactivePower  Quantity = "reactive-power"
	QuantityApparentPower  Quantity = "apparent-power"
	QuantityPowerFactor    Quantity = "power-factor"
	QuantityActiveEnergy   Quantity = "active-energy"
	QuantityReactiveEnergy Quantity = "reactive-energy"
	QuantityApparentEnergy Quantity = "apparent-energy"
	QuantityImpedance      Quantity = "impedance"
)

type quantityInfo struct {
	label string
	unit  string   // default unit symbol
	units []string // compatible unit symbols
}

var quantities = map[Quantity]quantityInfo{
	QuantityFrequency:      {"Frequency", "Hz", []string{"Hz"}},
	QuantityVoltage:        {"Voltage", "V", []string{"V"}},
	QuantityCurrent:        {"Current", "A", []string{"A"}},
	QuantityActivePower:    {"Active power", "W", []string{"W"}},
	QuantityReactivePower:  {"Reactive power", "VAr", []string{"VAr"}},
	QuantityApparentPower:  {"Apparent power", "VA", []string{"VA"}},
	QuantityPowerFactor:    {"Power factor", "1", []string{"1", "%"}},
	QuantityActiveEnergy:   {"Active energy", "Wh", []string{"Wh", "J"}},
	QuantityReactiveEnergy: {"Reactive energy", "VArh", []string{"VArh"}},
	QuantityApparentEnergy: {"Apparent energy", "VAh", []string{"VAh"}},
	QuantityImpedance:      {"Impedance", "ohm", []string{"ohm"}},
}

// Quantities returns every known quantity, in a stable order.
func Quantities() []Quantity {
	return []Quantity{
		QuantityFrequency, QuantityVoltage, QuantityCurrent,
		QuantityActivePower, QuantityReactivePower, QuantityApparentPower, QuantityPowerFactor,
		QuantityActiveEnergy, QuantityReactiveEnergy, QuantityApparentEnergy, QuantityImpedance,
	}
}

// IsKnown reports whether q is one of the quantities of this package.
func (q Quantity) IsKnown() bool { _, ok := quantities[q]; return ok }

// Label returns a short English label, e.g. "Active power".
func (q Quantity) Label() string { return quantities[q].label }

// DefaultUnit returns the base unit of the quantity, e.g. W for active
// power, or the zero Unit for an unknown quantity.
func (q Quantity) DefaultUnit() Unit {
	if info, ok := quantities[q]; ok {
		return Unit{Symbol: info.unit}
	}
	return Unit{}
}

// Accepts reports whether u measures q: its symbol is one of the
// quantity's units, with any multiplier (kV for voltage, MWh for active
// energy).
func (q Quantity) Accepts(u Unit) bool {
	for _, s := range quantities[q].units {
		if u.Symbol == s {
			return true
		}
	}
	return false
}

// Phase identifies the phase or phase pair a value belongs to.
type Phase string

// Phases. PhaseTotal is a value for all phases together (e.g. total
// active power); PhaseNet and PhaseResidual are the net sum and the
// residual of the phase values as WYE defines them.
const (
	PhaseNone     Phase = ""
	PhaseA        Phase = "A"
	PhaseB        Phase = "B"
	PhaseC        Phase = "C"
	PhaseNeutral  Phase = "N"
	PhaseAB       Phase = "AB"
	PhaseBC       Phase = "BC"
	PhaseCA       Phase = "CA"
	PhaseNet      Phase = "net"
	PhaseResidual Phase = "residual"
	PhaseTotal    Phase = "total"
)

// IsPair reports whether the phase is a phase pair (AB, BC, CA).
func (p Phase) IsPair() bool { return p == PhaseAB || p == PhaseBC || p == PhaseCA }

// subDataObjectPhases maps the sub data object names of WYE and DEL to
// phases.
var subDataObjectPhases = map[string]map[string]Phase{
	"WYE": {"phsA": PhaseA, "phsB": PhaseB, "phsC": PhaseC, "neut": PhaseNeutral, "net": PhaseNet, "res": PhaseResidual},
	"DEL": {"phsAB": PhaseAB, "phsBC": PhaseBC, "phsCA": PhaseCA},
}

// Reference is the reference a voltage is measured against.
type Reference string

// Voltage references.
const (
	ReferenceNone        Reference = ""
	ReferenceLineNeutral Reference = "line-neutral"
	ReferenceLineGround  Reference = "line-ground"
	ReferenceLineLine    Reference = "line-line"
)

// Direction is the energy flow direction a value counts, when the model
// distinguishes it.
type Direction string

// Directions. Which physical direction "supply" and "demand" denote
// depends on the device's reference convention; the library only
// reports that the model distinguishes them.
const (
	DirectionNone   Direction = ""
	DirectionSupply Direction = "supply"
	DirectionDemand Direction = "demand"
)

// Component is the part of a measured value an attribute holds.
type Component string

// Components, from the attribute path below a data object.
const (
	// ComponentNone: the descriptor refers to the data object as a whole.
	ComponentNone Component = ""
	// ComponentMagnitude: the (deadbanded) magnitude, e.g. MV.mag or
	// CMV.cVal.mag.
	ComponentMagnitude Component = "magnitude"
	// ComponentInstantaneous: the instantaneous magnitude, e.g. MV.instMag.
	ComponentInstantaneous Component = "instantaneous-magnitude"
	// ComponentAngle: the phase angle of a complex value, e.g. CMV.cVal.ang.
	ComponentAngle Component = "angle"
	// ComponentComplex: a complex value as a whole, e.g. CMV.cVal.
	ComponentComplex Component = "complex"
	// ComponentCounter: an accumulated counter value, e.g. BCR.actVal.
	ComponentCounter Component = "counter"
	// ComponentQuality: the quality of the value.
	ComponentQuality Component = "quality"
	// ComponentTimestamp: the time stamp of the value.
	ComponentTimestamp Component = "timestamp"
	// ComponentUnitConfig: the configured units of the value.
	ComponentUnitConfig Component = "unit-config"
	// ComponentOther: any other attribute (ranges, dead bands, scaling,
	// descriptions); not a measured value.
	ComponentOther Component = "other"
)

// IsValue reports whether the component carries the measured value
// itself (magnitude, instantaneous magnitude, angle, complex value or
// counter).
func (c Component) IsValue() bool {
	switch c {
	case ComponentMagnitude, ComponentInstantaneous, ComponentAngle, ComponentComplex, ComponentCounter:
		return true
	default:
		return false
	}
}

// Confidence is how well a mapping is supported.
type Confidence string

// Confidence levels.
const (
	// ConfidenceHigh: a class-specific rule, with the CDC verified
	// against the model.
	ConfidenceHigh Confidence = "high"
	// ConfidenceMedium: a rule that applies by data object name across
	// classes, with the CDC verified.
	ConfidenceMedium Confidence = "medium"
	// ConfidenceLow: a weak or unverified mapping, typically from an
	// override.
	ConfidenceLow Confidence = "low"
)

// Basis says where a mapping comes from.
type Basis string

// Bases.
const (
	// BasisRule: an OTFabric default rule.
	BasisRule Basis = "rule"
	// BasisOverride: a user or vendor override.
	BasisOverride Basis = "override"
)
