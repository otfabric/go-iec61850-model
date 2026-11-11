// SPDX-License-Identifier: MIT

package semantic

// Quantity is an engineering quantity.
type Quantity string

// Engineering quantities. The identifiers and labels are OTFabric
// vocabulary, not IEC terms, so they can be shared with other protocol
// mappings.
const (
	// Electrical quantities.
	QuantityFrequency      Quantity = "frequency"
	QuantityFrequencyRate  Quantity = "frequency-rate"
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
	QuantityResistance     Quantity = "resistance"
	QuantityPhaseAngle     Quantity = "phase-angle"
	QuantityCharge         Quantity = "charge"
	QuantityStateOfCharge  Quantity = "state-of-charge"

	// Power quality: ratios of a current or voltage, see [Descriptor.Of].
	QuantityHarmonicDistortion Quantity = "total-harmonic-distortion"
	QuantityDemandDistortion   Quantity = "total-demand-distortion"
	QuantityKFactor            Quantity = "k-factor"
	QuantityCrestFactor        Quantity = "crest-factor"
	QuantityImbalance          Quantity = "imbalance"

	// Process and environment.
	QuantityTemperature     Quantity = "temperature"
	QuantityTemperatureRate Quantity = "temperature-rate"
	QuantityPressure        Quantity = "pressure"
	QuantityPressureRate    Quantity = "pressure-rate"
	QuantityFlowRate        Quantity = "flow-rate"
	QuantityVolume          Quantity = "volume"
	QuantityLevel           Quantity = "level"
	QuantityDisplacement    Quantity = "displacement"
	QuantitySpeed           Quantity = "speed"
	QuantityRotationalSpeed Quantity = "rotational-speed"
	QuantityDirection       Quantity = "direction"
	QuantityHumidity        Quantity = "humidity"
	QuantityMoisture        Quantity = "moisture"
	QuantityIrradiance      Quantity = "irradiance"
	QuantityDensity         Quantity = "density"
	QuantitySalinity        Quantity = "salinity"
	QuantityConductivity    Quantity = "conductivity"
	QuantityPH              Quantity = "ph"
	QuantityConcentration   Quantity = "concentration"
	QuantitySoundLevel      Quantity = "sound-level"
	QuantityVibration       Quantity = "vibration"
	QuantityValvePosition   Quantity = "valve-position"
	QuantityDuration        Quantity = "duration"

	// States and counts: no unit hint.
	QuantitySwitchPosition Quantity = "switch-position"
	QuantityTapPosition    Quantity = "tap-position"
	QuantityOperatingState Quantity = "operating-state"
	QuantityOperationCount Quantity = "operation-count"
	QuantityCount          Quantity = "count"
)

type quantityInfo struct {
	label string
	unit  string   // default unit symbol; empty when the standard fixes none
	units []string // compatible unit symbols
}

var quantities = map[Quantity]quantityInfo{
	QuantityFrequency:      {"Frequency", "Hz", []string{"Hz"}},
	QuantityFrequencyRate:  {"Rate of change of frequency", "Hz/s", []string{"Hz/s"}},
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
	QuantityResistance:     {"Resistance", "ohm", []string{"ohm"}},
	QuantityPhaseAngle:     {"Phase angle", "deg", []string{"deg", "rad"}},
	QuantityCharge:         {"Electric charge", "Ah", []string{"Ah", "As", "C"}},
	QuantityStateOfCharge:  {"State of charge", "%", []string{"%", "1"}},

	QuantityHarmonicDistortion: {"Total harmonic distortion", "%", []string{"%", "1"}},
	QuantityDemandDistortion:   {"Total demand distortion", "%", []string{"%", "1"}},
	QuantityKFactor:            {"K-factor", "1", []string{"1"}},
	QuantityCrestFactor:        {"Crest factor", "1", []string{"1"}},
	QuantityImbalance:          {"Imbalance", "", []string{"%", "1", "A", "V"}},

	QuantityTemperature:     {"Temperature", "degC", []string{"degC", "K"}},
	QuantityTemperatureRate: {"Rate of change of temperature", "K/s", []string{"K/s"}},
	QuantityPressure:        {"Pressure", "Pa", []string{"Pa"}},
	QuantityPressureRate:    {"Rate of change of pressure", "Pa/s", []string{"Pa/s"}},
	QuantityFlowRate:        {"Volume flow rate", "m³/s", []string{"m³/s", "l/s"}},
	QuantityVolume:          {"Volume", "m³", []string{"m³"}},
	QuantityLevel:           {"Level", "m", []string{"m", "%"}},
	QuantityDisplacement:    {"Displacement", "m", []string{"m"}},
	QuantitySpeed:           {"Speed", "m/s", []string{"m/s"}},
	QuantityRotationalSpeed: {"Rotational speed", "", []string{"1/s", "rad/s", "%"}},
	QuantityDirection:       {"Direction", "deg", []string{"deg", "rad"}},
	QuantityHumidity:        {"Relative humidity", "%", []string{"%"}},
	QuantityMoisture:        {"Relative moisture saturation", "%", []string{"%"}},
	QuantityIrradiance:      {"Irradiance", "W/m²", []string{"W/m²"}},
	QuantityDensity:         {"Density", "kg/m³", []string{"kg/m³"}},
	QuantitySalinity:        {"Salinity", "kg/m³", []string{"kg/m³"}},
	QuantityConductivity:    {"Electrical conductivity", "S/m", []string{"S/m"}},
	QuantityPH:              {"pH", "1", []string{"1"}},
	QuantityConcentration:   {"Concentration", "ppm", []string{"ppm"}},
	QuantitySoundLevel:      {"Sound pressure level", "dB", []string{"dB"}},
	QuantityVibration:       {"Vibration", "", []string{"m", "m/s", "m/s²"}},
	QuantityValvePosition:   {"Valve position", "%", []string{"%"}},
	QuantityDuration:        {"Duration", "s", []string{"s", "min", "h"}},

	QuantitySwitchPosition: {"Switch position", "", nil},
	QuantityTapPosition:    {"Tap position", "", nil},
	QuantityOperatingState: {"Operating state", "", nil},
	QuantityOperationCount: {"Operation count", "", nil},
	QuantityCount:          {"Count", "", nil},
}

// quantityOrder is the stable order of [Quantities].
var quantityOrder = []Quantity{
	QuantityFrequency, QuantityFrequencyRate, QuantityVoltage, QuantityCurrent,
	QuantityActivePower, QuantityReactivePower, QuantityApparentPower, QuantityPowerFactor,
	QuantityActiveEnergy, QuantityReactiveEnergy, QuantityApparentEnergy, QuantityImpedance,
	QuantityResistance, QuantityPhaseAngle, QuantityCharge, QuantityStateOfCharge,
	QuantityHarmonicDistortion, QuantityDemandDistortion, QuantityKFactor, QuantityCrestFactor, QuantityImbalance,
	QuantityTemperature, QuantityTemperatureRate, QuantityPressure, QuantityPressureRate,
	QuantityFlowRate, QuantityVolume, QuantityLevel, QuantityDisplacement, QuantitySpeed,
	QuantityRotationalSpeed, QuantityDirection, QuantityHumidity, QuantityMoisture, QuantityIrradiance,
	QuantityDensity, QuantitySalinity, QuantityConductivity, QuantityPH, QuantityConcentration,
	QuantitySoundLevel, QuantityVibration, QuantityValvePosition, QuantityDuration,
	QuantitySwitchPosition, QuantityTapPosition, QuantityOperatingState, QuantityOperationCount, QuantityCount,
}

// Quantities returns every known quantity, in a stable order.
func Quantities() []Quantity { return append([]Quantity(nil), quantityOrder...) }

// IsState reports whether the quantity is a state or a count (switch
// position, tap position, operating state, counts), which has no unit.
func (q Quantity) IsState() bool {
	switch q {
	case QuantitySwitchPosition, QuantityTapPosition, QuantityOperatingState, QuantityOperationCount, QuantityCount:
		return true
	default:
		return false
	}
}

// IsKnown reports whether q is one of the quantities of this package.
func (q Quantity) IsKnown() bool { _, ok := quantities[q]; return ok }

// Label returns a short English label, e.g. "Active power".
func (q Quantity) Label() string { return quantities[q].label }

// DefaultUnit returns the base unit of the quantity, e.g. W for active
// power. It is the zero Unit for an unknown quantity, for states and
// counts, and where the standard leaves the unit to the device
// (rotational speed, vibration, imbalance).
func (q Quantity) DefaultUnit() Unit {
	if info, ok := quantities[q]; ok && info.unit != "" {
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

// subDataObject is what a sub data object of a multi-valued CDC says.
type subDataObject struct {
	phase    Phase
	harmonic bool   // an array indexed by harmonic order
	note     string // a remark for descriptors
}

const seqNote = "sequence component; whether it is positive, negative or zero (or direct, quadrature, zero) is the seqT attribute of the device"

// subDataObjects maps the sub data objects of the multi-valued CDCs
// (WYE, DEL, their harmonic forms HWYE and HDEL, HMV and SEQ) to what
// they say. A rule for one of these CDCs sets no phase.
var subDataObjects = map[string]map[string]subDataObject{
	"WYE":  {"phsA": {phase: PhaseA}, "phsB": {phase: PhaseB}, "phsC": {phase: PhaseC}, "neut": {phase: PhaseNeutral}, "net": {phase: PhaseNet}, "res": {phase: PhaseResidual}},
	"DEL":  {"phsAB": {phase: PhaseAB}, "phsBC": {phase: PhaseBC}, "phsCA": {phase: PhaseCA}},
	"HWYE": {"phsAHar": {phase: PhaseA, harmonic: true}, "phsBHar": {phase: PhaseB, harmonic: true}, "phsCHar": {phase: PhaseC, harmonic: true}, "neutHar": {phase: PhaseNeutral, harmonic: true}, "netHar": {phase: PhaseNet, harmonic: true}, "resHar": {phase: PhaseResidual, harmonic: true}},
	"HDEL": {"phsABHar": {phase: PhaseAB, harmonic: true}, "phsBCHar": {phase: PhaseBC, harmonic: true}, "phsCAHar": {phase: PhaseCA, harmonic: true}},
	"HMV":  {"har": {harmonic: true}},
	"SEQ":  {"c1": {note: seqNote}, "c2": {note: seqNote}, "c3": {note: seqNote}},
}

// lineLine are the CDCs whose values are phase-to-phase.
var lineLine = map[string]bool{"DEL": true, "HDEL": true}

// Reference is the reference a voltage (or a DC resistance to earth)
// is measured against.
type Reference string

// References. The pole references are those of DC circuits (MMDC).
const (
	ReferenceNone           Reference = ""
	ReferenceLineNeutral    Reference = "line-neutral"
	ReferenceLineGround     Reference = "line-ground"
	ReferenceLineLine       Reference = "line-line"
	ReferencePolePole       Reference = "pole-pole"
	ReferencePositiveGround Reference = "positive-ground"
	ReferenceNegativeGround Reference = "negative-ground"
)

// Sequence is a symmetrical component.
type Sequence string

// Sequences, for values that name the component they hold (e.g. the
// negative-sequence imbalance of MSQI). The sub data objects c1, c2
// and c3 of SEQ carry no sequence: which component they hold is the
// device's seqT attribute.
const (
	SequenceNone     Sequence = ""
	SequencePositive Sequence = "positive"
	SequenceNegative Sequence = "negative"
	SequenceZero     Sequence = "zero"
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
	// ComponentState: a status value, e.g. DPC.stVal, INS.stVal or
	// ISC.valWTr.posVal.
	ComponentState Component = "state"
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
// itself (magnitude, instantaneous magnitude, angle, complex value,
// counter or status value).
func (c Component) IsValue() bool {
	switch c {
	case ComponentMagnitude, ComponentInstantaneous, ComponentAngle, ComponentComplex, ComponentCounter, ComponentState:
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
