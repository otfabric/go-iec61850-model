// SPDX-License-Identifier: MIT

package model

// LogicalNodeClass is a logical node class (e.g. MMXU) or an abstract
// logical node class (e.g. DomainLN) as defined by a namespace.
//
// DataObjects holds only the data objects the class declares itself;
// inherited data objects are found by following Base. The resolver
// package provides the flattened view.
type LogicalNodeClass struct {
	// Name is the class name, e.g. "MMXU".
	Name string `json:"name"`
	// Base is the name of the class this class inherits from, if any.
	Base string `json:"base,omitempty"`
	// Abstract reports whether this is an abstract class.
	Abstract bool `json:"abstract,omitempty"`
	// CanHaveLOG reports whether instances may contain a log.
	CanHaveLOG bool `json:"canHaveLOG,omitempty"`
	// IsExtension marks a class that extends a class of another
	// namespace.
	IsExtension bool `json:"isExtension,omitempty"`
	// Deprecated marks a class the namespace keeps for compatibility.
	Deprecated bool `json:"deprecated,omitempty"`
	// Informative marks a class that is informative only.
	Informative bool `json:"informative,omitempty"`
	// DataObjects are the data objects declared by this class, in
	// document order.
	DataObjects []DataObject `json:"dataObjects,omitempty"`
	Doc         Doc          `json:"doc,omitzero"`
	// Source is the namespace that defines the class.
	Source NamespaceID `json:"source"`
}

// Group returns the logical node group of a concrete class, derived
// from the first letter of its name (e.g. 'M' for MMXU). Abstract
// classes and names that do not start with an upper-case letter return
// the zero group.
func (ln *LogicalNodeClass) Group() LNGroup {
	if ln.Abstract || ln.Name == "" {
		return 0
	}
	g := LNGroup(ln.Name[0])
	if g < 'A' || g > 'Z' {
		return 0
	}
	return g
}

// DataObject returns the data object declared by this class with the
// given name. Inherited data objects are not searched.
func (ln *LogicalNodeClass) DataObject(name string) (*DataObject, bool) {
	d := find(ln.DataObjects, name, func(d *DataObject) string { return d.Name })
	return d, d != nil
}

// DataObject is a data object of a logical node class, or a sub data
// object of a CDC.
type DataObject struct {
	// Name is the data object name, e.g. "Hz" or "phsA".
	Name string `json:"name"`
	// Type is the name of the CDC of the data object, e.g. "MV".
	Type string `json:"type"`
	// UnderlyingType is the type that parameterizes the CDC, e.g. the
	// enumeration of an ENS data object.
	UnderlyingType string `json:"underlyingType,omitempty"`
	// UnderlyingTypeKind is the type kind of UnderlyingType.
	UnderlyingTypeKind TypeKind `json:"underlyingTypeKind,omitempty"`
	// Presence is the presence condition of the data object.
	Presence PresenceCondition `json:"presence,omitzero"`
	// DSPresence is the presence condition in data sets, when declared.
	DSPresence PresenceCondition `json:"dsPresence,omitzero"`
	// IsArray reports whether the data object is an array.
	IsArray bool `json:"isArray,omitempty"`
	// SizeAttribute names the attribute that holds the array size.
	SizeAttribute string `json:"sizeAttribute,omitempty"`
	// MinIndexAttribute and MaxIndexAttribute name the attributes that
	// bound the array index.
	MinIndexAttribute string `json:"minIndexAttribute,omitempty"`
	MaxIndexAttribute string `json:"maxIndexAttribute,omitempty"`
	// Transient marks a data object whose value is transient.
	Transient bool `json:"transient,omitempty"`
	// Deprecated marks a data object kept for compatibility.
	Deprecated bool `json:"deprecated,omitempty"`
	// Informative marks a data object that is informative only.
	Informative bool `json:"informative,omitempty"`
	Doc         Doc  `json:"doc,omitzero"`
	// Position is the zero-based position of the element among the
	// children of its parent, in document order. In a CDC, it orders
	// sub data objects and data attributes relative to each other.
	Position int `json:"position"`
}
