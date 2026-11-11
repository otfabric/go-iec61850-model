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
// For WYE, DEL, HWYE and HDEL data objects the phase comes from the
// sub data object (phsA, phsAB, phsAHar, ...), and DEL and HDEL imply
// a line-to-line reference.
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
	// Of is the quantity a ratio or distortion refers to, e.g. current
	// for the total harmonic distortion of a current.
	Of Quantity `json:"of,omitempty"`
	// Sequence is the symmetrical component the value holds, if the
	// data object names one.
	Sequence Sequence `json:"sequence,omitempty"`
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
	case r.Phase != "" && subDataObjects[r.CDC] != nil:
		return fmt.Errorf("%w: rule %s sets a phase for CDC %s, whose values are sub data objects", model.ErrInvalidRule, r.ID, r.CDC)
	case r.Of != "" && !r.Of.IsKnown():
		return fmt.Errorf("%w: rule %s refers to unknown quantity %q", model.ErrInvalidRule, r.ID, r.Of)
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

// DefaultRules returns the built-in OTFabric rules: the measurement
// classes of IEC 61850-7-4 (MMXU, MMXN, MMTR and MMTN, MSQI, MHAI and
// MHAN, MMDC, MMET, MFLW, MENV, MHYD), the supervision classes (SIML,
// SARC, SPRS, STMP, SVBR, ZBAT, ZAXN), switching and tap-changer
// positions and counters (XCBR, XSWI, YLTC, ATCC), pumps and fans
// (KPMP, KFAN), the DER and storage classes of IEC 61850-7-420 (DGEN,
// DSTO, DBAT, DINV, SBAT, SINV) and the battery charger of IEC TR
// 61850-90-3 (ZBTC), plus a frequency rule that applies to Hz of CDC
// MV in any class. Most rules name the abstract class that declares
// the data object, so they apply to every class that inherits it. The
// slice is a copy; [DefaultExclusions] lists what is deliberately left
// unmapped.
func DefaultRules() []Rule {
	out := make([]Rule, len(defaultRules))
	copy(out, defaultRules)
	return out
}

// Exclusion records that a data object is deliberately not mapped, and
// why: its unit or meaning is not stated, or it is a statistic or a
// derived value the vocabulary does not model. An excluded data object
// fails [Mapper.Describe] with [model.ErrNoMapping] and the reason, and
// [Mapper.Coverage] reports it as excluded rather than as a gap. An
// override for the same class and data object takes precedence.
type Exclusion struct {
	// LNClass is the logical node class; the exclusion also applies to
	// classes that inherit from it.
	LNClass string `json:"lnClass"`
	// DataObject is the data object name, without instance number.
	DataObject string `json:"dataObject"`
	// Reason says why the data object is not mapped. Required.
	Reason string `json:"reason"`
}

// DefaultExclusions returns the data objects of the classes the
// default rules cover that are deliberately left unmapped, with the
// reason. The slice is a copy.
func DefaultExclusions() []Exclusion {
	out := make([]Exclusion, len(defaultExclusions))
	copy(out, defaultExclusions)
	return out
}

// Mapper applies mapping rules and overrides. It is immutable and safe
// for concurrent use.
type Mapper struct {
	rules      []Rule // defaults, by key
	overrides  []Rule
	exclusions []Exclusion
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

// Default is the mapper with [DefaultRules], [DefaultExclusions] and no
// overrides.
var Default = mustDefault()

func mustDefault() *Mapper {
	m, err := NewMapper(defaultRules, nil)
	if err == nil {
		m, err = m.WithExclusions(defaultExclusions...)
	}
	if err != nil {
		panic(err)
	}
	return m
}

// WithOverrides returns a mapper with the rules and exclusions of m and
// the given overrides added to those of m.
func (m *Mapper) WithOverrides(overrides ...Rule) (*Mapper, error) {
	n, err := NewMapper(m.rules, append(append([]Rule(nil), m.overrides...), overrides...))
	if err != nil {
		return nil, err
	}
	n.exclusions = m.exclusions
	return n, nil
}

// WithExclusions returns a mapper with the rules and overrides of m and
// the given exclusions added to those of m. An exclusion needs a class,
// a data object and a reason, and may not exclude what a rule of m
// maps or what another exclusion already excludes; otherwise it fails
// with [model.ErrInvalidRule].
func (m *Mapper) WithExclusions(exclusions ...Exclusion) (*Mapper, error) {
	keys := map[string]bool{}
	for _, r := range m.rules {
		keys[r.LNClass+"."+r.DataObject] = true
	}
	all := append(append([]Exclusion(nil), m.exclusions...), exclusions...)
	seen := map[string]bool{}
	for _, e := range all {
		key := e.LNClass + "." + e.DataObject
		switch {
		case e.LNClass == "" || e.DataObject == "" || e.Reason == "":
			return nil, fmt.Errorf("%w: exclusion %s needs LNClass, DataObject and Reason", model.ErrInvalidRule, key)
		case keys[key]:
			return nil, fmt.Errorf("%w: %s is both mapped and excluded", model.ErrInvalidRule, key)
		case seen[key]:
			return nil, fmt.Errorf("%w: %s is excluded twice", model.ErrInvalidRule, key)
		}
		seen[key] = true
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].LNClass != all[j].LNClass {
			return all[i].LNClass < all[j].LNClass
		}
		return all[i].DataObject < all[j].DataObject
	})
	return &Mapper{rules: m.rules, overrides: m.overrides, exclusions: all}, nil
}

// Exclusions returns the exclusions of the mapper, ordered by class and
// data object.
func (m *Mapper) Exclusions() []Exclusion { return append([]Exclusion(nil), m.exclusions...) }

// excluded returns the exclusion for a data object of one of the
// classes of an inheritance chain, the most specific class first.
func (m *Mapper) excluded(chain []string, do string) (Exclusion, bool) {
	for _, cls := range chain {
		for _, e := range m.exclusions {
			if e.LNClass == cls && e.DataObject == do {
				return e, true
			}
		}
	}
	return Exclusion{}, false
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
