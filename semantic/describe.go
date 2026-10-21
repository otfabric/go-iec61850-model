// SPDX-License-Identifier: MIT

package semantic

import (
	"errors"
	"fmt"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// Lookup is what the semantic functions need from a model: the
// resolver's lookups plus the list of logical node classes.
// *catalog.Profile implements it, and so does *catalog.Catalog when
// names are unambiguous.
type Lookup interface {
	resolver.Lookup
	ListLogicalNodes() []*model.LogicalNodeClass
}

// Descriptor describes the engineering meaning of a data object, sub
// data object or attribute.
//
// A descriptor states what the standard model and an OTFabric rule say
// an element represents. It never claims that a device implements the
// element, and its UnitHint is not the unit a device reports: read the
// configured units with [DeviceUnit].
type Descriptor struct {
	// Path is the resolved class path, e.g. "MMXU.PNV.phsA.cVal.mag.f".
	Path string `json:"path"`
	// LNClass, DataObject and SubDataObject locate the element. The
	// data object name is the one of the model, without instance
	// number.
	LNClass       string `json:"lnClass"`
	DataObject    string `json:"dataObject"`
	SubDataObject string `json:"subDataObject,omitempty"`
	// CDC is the CDC of the data object.
	CDC string `json:"cdc"`
	// Presence is the presence condition of the data object in the
	// model, e.g. O for optional.
	Presence model.PresenceCondition `json:"presence,omitzero"`
	// Namespace is the namespace that declares the data object.
	Namespace model.NamespaceID `json:"namespace"`

	Quantity  Quantity  `json:"quantity"`
	Phase     Phase     `json:"phase,omitempty"`
	Reference Reference `json:"reference,omitempty"`
	Direction Direction `json:"direction,omitempty"`
	// Component is the part of the value the path designates; empty for
	// a data object or sub data object as a whole.
	Component Component `json:"component,omitempty"`
	// UnitHint is the unit the value is expected in: the quantity's
	// unit, degrees for angles, none for quality, time stamps and
	// configuration. It is a hint, not the device's configured unit.
	UnitHint Unit `json:"unitHint,omitzero"`

	// Confidence, Basis, Rule and Source make the mapping auditable.
	Confidence Confidence `json:"confidence"`
	Basis      Basis      `json:"basis"`
	Rule       string     `json:"rule"`
	Source     string     `json:"source"`
	// Notes are remarks, e.g. that an integer value needs scaling.
	Notes []string `json:"notes,omitempty"`
}

// Describe describes the element a reference designates, using the
// default rules. The reference may be a class reference ("MMXU.Hz",
// "MMXU.PNV.phsA.cVal.mag.f") or an instance reference
// ("LD0/MMXU1.Hz.mag.f[MX]").
//
// It fails with the resolver's errors when the path is not valid in
// the model, and with [model.ErrNoMapping] when no rule applies or the
// rule does not fit the model (wrong CDC, unknown sub data object).
func Describe(l Lookup, ref string) (*Descriptor, error) { return Default.Describe(l, ref) }

// Describe is like the package-level [Describe], with the rules and
// overrides of m.
func (m *Mapper) Describe(l Lookup, ref string) (*Descriptor, error) {
	res, err := resolver.New(l).ResolveString(ref)
	if err != nil {
		return nil, err
	}
	return m.describe(l, res)
}

// DescribeResolution describes an already resolved path.
func (m *Mapper) DescribeResolution(l Lookup, res *resolver.Resolution) (*Descriptor, error) {
	return m.describe(l, res)
}

func noMapping(path, reason string) error {
	return &model.ResolutionError{Path: path, Reason: reason, Err: model.ErrNoMapping}
}

func (m *Mapper) describe(l Lookup, res *resolver.Resolution) (*Descriptor, error) {
	path := res.Path()
	if res.DataObject == nil {
		return nil, noMapping(path, "a logical node class is not a measured value")
	}
	chain, err := l.Inheritance(res.LNClass.Name)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(chain))
	for i, c := range chain {
		names[i] = c.Name
	}
	rule, basis, ok := m.find(names, res.DataObject.Name)
	if !ok {
		return nil, noMapping(path, fmt.Sprintf("no rule for %s.%s", res.LNClass.Name, res.DataObject.Name))
	}
	if rule.CDC != res.DataObject.Type {
		return nil, noMapping(path, fmt.Sprintf("rule %s expects CDC %s, the model has %s", rule.ID, rule.CDC, res.DataObject.Type))
	}
	doStep := res.Steps[1]
	d := &Descriptor{
		Path: path, LNClass: res.LNClass.Name, DataObject: res.DataObject.Name, CDC: res.DataObject.Type,
		Presence: res.DataObject.Presence, Namespace: doStep.Source,
		Quantity: rule.Quantity, Phase: rule.Phase, Reference: rule.Reference, Direction: rule.Direction,
		UnitHint: rule.unitHint(), Confidence: rule.confidence(), Basis: basis, Rule: rule.ID, Source: rule.Source,
	}
	if rule.Note != "" {
		d.Notes = append(d.Notes, rule.Note)
	}
	if doStep.Instance != "" {
		d.Notes = append(d.Notes, "instance "+doStep.Instance+" of multi-instance data object "+res.DataObject.Name)
	}

	rest := res.Steps[2:]
	if phases := subDataObjectPhases[d.CDC]; phases != nil {
		if len(rest) == 0 || rest[0].Kind != resolver.StepSubDataObject {
			// The data object as a whole: a set of phase values.
			if d.CDC == "DEL" {
				d.Reference = ReferenceLineLine
			}
			return d, nil
		}
		ph, ok := phases[rest[0].Name]
		if !ok {
			return nil, noMapping(path, fmt.Sprintf("sub data object %s of %s has no phase mapping", rest[0].Name, d.CDC))
		}
		d.SubDataObject, d.Phase = rest[0].Name, ph
		if d.CDC == "DEL" {
			d.Reference = ReferenceLineLine
		}
		rest = rest[1:]
	}
	component(d, rest)
	return d, nil
}

// component derives the component and unit hint from the attribute
// steps below the data object.
func component(d *Descriptor, attrs []resolver.Step) {
	if len(attrs) == 0 {
		return
	}
	switch attrs[0].Name {
	case "mag":
		d.Component = ComponentMagnitude
	case "instMag":
		d.Component = ComponentInstantaneous
	case "cVal", "instCVal":
		d.Component = ComponentComplex
		if len(attrs) > 1 {
			switch attrs[1].Name {
			case "mag":
				d.Component = ComponentMagnitude
				if attrs[0].Name == "instCVal" {
					d.Component = ComponentInstantaneous
				}
			case "ang":
				d.Component = ComponentAngle
				d.UnitHint = Unit{Symbol: "deg"}
			}
		}
	case "actVal":
		d.Component = ComponentCounter
		d.Notes = append(d.Notes, "counter value; multiply by pulsQty for the quantity")
	case "q":
		d.Component = ComponentQuality
	case "t":
		d.Component = ComponentTimestamp
	case "units":
		d.Component = ComponentUnitConfig
	default:
		d.Component = ComponentOther
	}
	if !d.Component.IsValue() {
		d.UnitHint = Unit{}
		return
	}
	if last := attrs[len(attrs)-1]; last.Name == "i" && last.TypeKind == model.TypeKindBasic {
		d.Notes = append(d.Notes, "integer value; scale with the sVC configuration")
	}
}

// FindMeasurements returns the descriptors of every mapped data object
// of a logical node class, including inherited ones and those of
// extension classes, in model order. Data objects of WYE and DEL are
// expanded into one descriptor per phase. Data objects without a rule
// are skipped, also when the profile cannot resolve them; a mapped data
// object that cannot be resolved is an error.
func FindMeasurements(l Lookup, lnClass string) ([]*Descriptor, error) {
	return Default.FindMeasurements(l, lnClass)
}

// FindMeasurements is like the package-level [FindMeasurements], with
// the rules and overrides of m.
func (m *Mapper) FindMeasurements(l Lookup, lnClass string) ([]*Descriptor, error) {
	dos, err := l.DataObjects(lnClass)
	if err != nil {
		return nil, err
	}
	chain, err := l.Inheritance(lnClass)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(chain))
	for i, c := range chain {
		names[i] = c.Name
	}
	r := resolver.New(l)
	var out []*Descriptor
	for _, ido := range dos {
		res, err := r.ResolvePath(lnClass, []string{ido.Name}, "")
		if err != nil {
			// A data object the profile cannot resolve (e.g. its CDC is
			// defined by a namespace that is not loaded) is skipped
			// unless a rule maps it.
			if _, _, mapped := m.find(names, ido.Name); !mapped {
				continue
			}
			return nil, err
		}
		d, err := m.describe(l, res)
		if errors.Is(err, model.ErrNoMapping) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if subDataObjectPhases[d.CDC] == nil {
			out = append(out, d)
			continue
		}
		for _, sdo := range res.CDC.SubDataObjects {
			sres, err := r.ResolvePath(lnClass, []string{ido.Name, sdo.Name}, "")
			if err != nil {
				return nil, err
			}
			sd, err := m.describe(l, sres)
			if errors.Is(err, model.ErrNoMapping) {
				continue
			}
			if err != nil {
				return nil, err
			}
			out = append(out, sd)
		}
	}
	return out, nil
}

// Criteria select descriptors in [Match]. Empty fields do not filter.
type Criteria struct {
	Quantity  Quantity
	Phase     Phase
	Reference Reference
	Direction Direction
	// LNClass restricts the search to one logical node class.
	LNClass string
	// MinConfidence excludes weaker mappings: with ConfidenceHigh only
	// high-confidence descriptors are returned.
	MinConfidence Confidence
}

var confidenceRank = map[Confidence]int{ConfidenceLow: 1, ConfidenceMedium: 2, ConfidenceHigh: 3}

// Matches reports whether d satisfies c.
func (c Criteria) Matches(d *Descriptor) bool {
	switch {
	case c.Quantity != "" && d.Quantity != c.Quantity,
		c.Phase != "" && d.Phase != c.Phase,
		c.Reference != "" && d.Reference != c.Reference,
		c.Direction != "" && d.Direction != c.Direction,
		c.LNClass != "" && d.LNClass != c.LNClass,
		c.MinConfidence != "" && confidenceRank[d.Confidence] < confidenceRank[c.MinConfidence]:
		return false
	}
	return true
}

// Match returns the descriptors of every logical node class of the
// model that satisfy c, ordered by class name and then model order.
// Use it to answer "which objects carry phase A line-to-neutral
// voltage?".
func Match(l Lookup, c Criteria) ([]*Descriptor, error) { return Default.Match(l, c) }

// Match is like the package-level [Match], with the rules and
// overrides of m.
func (m *Mapper) Match(l Lookup, c Criteria) ([]*Descriptor, error) {
	var out []*Descriptor
	for _, ln := range l.ListLogicalNodes() {
		if c.LNClass != "" && ln.Name != c.LNClass {
			continue
		}
		ds, err := m.FindMeasurements(l, ln.Name)
		if err != nil {
			return nil, err
		}
		for _, d := range ds {
			if c.Matches(d) {
				out = append(out, d)
			}
		}
	}
	return out, nil
}
