// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"strings"
)

// Enumeration is an enumerated type, e.g. BehaviourModeKind.
type Enumeration struct {
	// Name is the enumeration name.
	Name string `json:"name"`
	// InheritedFrom names the enumeration this one extends, if any.
	// Literals of the parent are not copied into Literals.
	InheritedFrom string `json:"inheritedFrom,omitempty"`
	// Literals are the literals declared by this enumeration, in
	// document order.
	Literals []Literal `json:"literals,omitempty"`
	// Deprecated marks an enumeration kept for compatibility.
	Deprecated bool `json:"deprecated,omitempty"`
	Doc        Doc  `json:"doc,omitzero"`
	// Source is the namespace that defines the enumeration.
	Source NamespaceID `json:"source"`
}

// Literal returns the literal with the given name.
func (e *Enumeration) Literal(name string) (*Literal, bool) {
	l := find(e.Literals, name, func(l *Literal) string { return l.Name })
	return l, l != nil
}

// LiteralByValue returns the literal with the given value.
func (e *Enumeration) LiteralByValue(v int) (*Literal, bool) {
	for i := range e.Literals {
		if e.Literals[i].Value == v {
			return &e.Literals[i], true
		}
	}
	return nil, false
}

// Literal is one literal of an enumeration.
type Literal struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
	// Deprecated marks a literal kept for compatibility.
	Deprecated bool `json:"deprecated,omitempty"`
	Doc        Doc  `json:"doc,omitzero"`
}

// TypeKind is the kind of an attribute type as declared in NSD.
type TypeKind string

// Type kinds defined by IEC 61850-7-7.
const (
	TypeKindBasic       TypeKind = "BASIC"
	TypeKindEnumerated  TypeKind = "ENUMERATED"
	TypeKindConstructed TypeKind = "CONSTRUCTED"
	TypeKindPackedList  TypeKind = "PACKED_LIST"
	TypeKindUndefined   TypeKind = "undefined"
)

// FunctionalConstraint is a two-letter functional constraint, e.g. ST
// or MX. The string form matches the FunctionalConstraint type of
// github.com/otfabric/go-iec61850.
type FunctionalConstraint string

// Functional constraints of IEC 61850-7-2. The set a profile actually
// defines comes from its NSD files; these constants are conveniences.
const (
	FCST FunctionalConstraint = "ST" // Status information
	FCMX FunctionalConstraint = "MX" // Measurands (analogue values)
	FCSP FunctionalConstraint = "SP" // Setpoint
	FCSV FunctionalConstraint = "SV" // Substitution
	FCCF FunctionalConstraint = "CF" // Configuration
	FCDC FunctionalConstraint = "DC" // Description
	FCSG FunctionalConstraint = "SG" // Setting group
	FCSE FunctionalConstraint = "SE" // Setting group editable
	FCSR FunctionalConstraint = "SR" // Service response
	FCOR FunctionalConstraint = "OR" // Operate received
	FCBL FunctionalConstraint = "BL" // Blocking
	FCEX FunctionalConstraint = "EX" // Extended definition
	FCCO FunctionalConstraint = "CO" // Control
	FCUS FunctionalConstraint = "US" // Unicast sampled values
	FCMS FunctionalConstraint = "MS" // Multicast sampled values
	FCRP FunctionalConstraint = "RP" // Unbuffered report
	FCBR FunctionalConstraint = "BR" // Buffered report
	FCLG FunctionalConstraint = "LG" // Log control
	FCGO FunctionalConstraint = "GO" // GOOSE control
)

var standardFCs = map[FunctionalConstraint]bool{
	FCST: true, FCMX: true, FCSP: true, FCSV: true, FCCF: true, FCDC: true,
	FCSG: true, FCSE: true, FCSR: true, FCOR: true, FCBL: true, FCEX: true,
	FCCO: true, FCUS: true, FCMS: true, FCRP: true, FCBR: true, FCLG: true,
	FCGO: true,
}

// IsStandard reports whether fc is one of the IEC 61850-7-2 functional
// constraints listed as constants in this package.
func (fc FunctionalConstraint) IsStandard() bool { return standardFCs[fc] }

// PresenceCondition is the presence condition of a data object or
// attribute, with its optional argument, e.g. {Name: "M"} or
// {Name: "AtLeastOne", Args: "1"}.
type PresenceCondition struct {
	// Name is the condition name, e.g. "M", "O" or "MOcond".
	Name string `json:"name"`
	// Args is the condition argument, verbatim, if any.
	Args string `json:"args,omitempty"`
	// ArgsID references a documentation text for the argument.
	ArgsID string `json:"argsID,omitempty"`
}

// IsZero reports whether no presence condition is set.
func (p PresenceCondition) IsZero() bool { return p == PresenceCondition{} }

// IsMandatory reports whether the condition is unconditionally
// mandatory ("M").
func (p PresenceCondition) IsMandatory() bool { return p.Name == "M" }

// IsOptional reports whether the condition is unconditionally optional
// ("O").
func (p PresenceCondition) IsOptional() bool { return p.Name == "O" }

// String returns the condition with its argument, e.g. "AtLeastOne(1)".
func (p PresenceCondition) String() string {
	if p.Args == "" {
		return p.Name
	}
	return fmt.Sprintf("%s(%s)", p.Name, p.Args)
}

// Basic type names defined by IEC 61850-7-2 that this module relies on.
// The set a profile actually defines comes from its NSD files.
const (
	BasicBOOLEAN = "BOOLEAN"
	BasicINT8    = "INT8"
	BasicINT16   = "INT16"
	BasicINT32   = "INT32"
	BasicINT64   = "INT64"
	BasicINT8U   = "INT8U"
	BasicINT16U  = "INT16U"
	BasicINT32U  = "INT32U"
	BasicFLOAT32 = "FLOAT32"
	BasicFLOAT64 = "FLOAT64"
)

// LNGroup is the logical node group, the first letter of a concrete
// logical node class name (e.g. 'M' for MMXU).
type LNGroup byte

// lnGroupNames are OTFabric-authored short labels for the logical node
// groups used by IEC 61850-7-4 and its domain extensions.
var lnGroupNames = map[LNGroup]string{
	'A': "Automatic control",
	'C': "Supervisory control",
	'D': "Distributed energy resources",
	'F': "Functional blocks",
	'G': "Generic function references",
	'H': "Hydropower",
	'I': "Interfacing and archiving",
	'K': "Mechanical and non-electrical primary equipment",
	'L': "System logical nodes",
	'M': "Metering and measurement",
	'P': "Protection functions",
	'Q': "Power quality events",
	'R': "Protection related functions",
	'S': "Supervision and monitoring",
	'T': "Instrument transformers and sensors",
	'W': "Wind power",
	'X': "Switchgear",
	'Y': "Power transformers",
	'Z': "Further power system equipment",
}

// String returns the group letter, or "" for the zero group.
func (g LNGroup) String() string {
	if g == 0 {
		return ""
	}
	return string(rune(g))
}

// Label returns a short English label for the group, e.g. "Metering
// and measurement", or "" when the group is not known. Labels are
// informative and authored by OTFabric, not quoted from the standard.
func (g LNGroup) Label() string { return lnGroupNames[g] }

// Path joins model path elements with ".", e.g. "MMXU.Hz.mag.f".
func Path(elems ...string) string { return strings.Join(elems, ".") }
