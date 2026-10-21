// SPDX-License-Identifier: MIT

package model

// CommonDataClass is a common data class (CDC), e.g. MV, WYE or SPS.
//
// A CDC contains sub data objects (each typed by another CDC) and data
// attributes. Each slice keeps document order; the Position field of
// the elements orders the two kinds relative to each other.
type CommonDataClass struct {
	// Name is the CDC name, e.g. "MV".
	Name string `json:"name"`
	// SubDataObjects are the sub data objects, e.g. phsA of WYE.
	SubDataObjects []DataObject `json:"subDataObjects,omitempty"`
	// DataAttributes are the data attributes, e.g. mag of MV.
	DataAttributes []DataAttribute `json:"dataAttributes,omitempty"`
	// ServiceParameters are the service parameters of controllable
	// CDCs, e.g. ctlVal.
	ServiceParameters []ServiceParameter `json:"serviceParameters,omitempty"`
	// EnumParameterized reports whether attributes of kind ENUMERATED
	// without a type take the data object's underlying type.
	EnumParameterized bool `json:"enumParameterized,omitempty"`
	// TypeKindParameterized reports whether attributes without a type
	// take the data object's underlying type and type kind.
	TypeKindParameterized bool `json:"typeKindParameterized,omitempty"`
	// Statistics reports whether the CDC supports statistical
	// calculation attributes.
	Statistics bool `json:"statistics,omitempty"`
	// Deprecated marks a CDC kept for compatibility.
	Deprecated bool `json:"deprecated,omitempty"`
	// Informative marks a CDC that is informative only.
	Informative bool `json:"informative,omitempty"`
	Doc         Doc  `json:"doc,omitzero"`
	// Source is the namespace that defines the CDC.
	Source NamespaceID `json:"source"`
}

// SubDataObject returns the sub data object with the given name.
func (c *CommonDataClass) SubDataObject(name string) (*DataObject, bool) {
	d := find(c.SubDataObjects, name, func(d *DataObject) string { return d.Name })
	return d, d != nil
}

// DataAttribute returns the data attribute with the given name.
func (c *CommonDataClass) DataAttribute(name string) (*DataAttribute, bool) {
	a := find(c.DataAttributes, name, func(a *DataAttribute) string { return a.Name })
	return a, a != nil
}

// FunctionalConstraints returns the distinct functional constraints of
// the CDC's own data attributes, in first-seen order. Sub data objects
// are not included.
func (c *CommonDataClass) FunctionalConstraints() []FunctionalConstraint {
	var out []FunctionalConstraint
	seen := map[FunctionalConstraint]bool{}
	for i := range c.DataAttributes {
		fc := c.DataAttributes[i].FC
		if fc != "" && !seen[fc] {
			seen[fc] = true
			out = append(out, fc)
		}
	}
	return out
}

// ServiceParameter is a service parameter of a controllable CDC.
type ServiceParameter struct {
	Name     string            `json:"name"`
	Type     string            `json:"type,omitempty"`
	TypeKind TypeKind          `json:"typeKind,omitempty"`
	Presence PresenceCondition `json:"presence,omitzero"`
	Doc      Doc               `json:"doc,omitzero"`
	Position int               `json:"position"`
}

// ConstructedAttribute is a constructed (structured) attribute type,
// e.g. AnalogueValue or Vector.
type ConstructedAttribute struct {
	// Name is the type name, e.g. "AnalogueValue".
	Name string `json:"name"`
	// SubDataAttributes are the members, in document order. Their FC is
	// empty: members take the FC of the enclosing data attribute.
	SubDataAttributes []DataAttribute `json:"subDataAttributes,omitempty"`
	// Deprecated marks a type kept for compatibility.
	Deprecated bool `json:"deprecated,omitempty"`
	Doc        Doc  `json:"doc,omitzero"`
	// Source is the namespace that defines the type.
	Source NamespaceID `json:"source"`
}

// SubDataAttribute returns the member with the given name.
func (c *ConstructedAttribute) SubDataAttribute(name string) (*DataAttribute, bool) {
	a := find(c.SubDataAttributes, name, func(a *DataAttribute) string { return a.Name })
	return a, a != nil
}

// DataAttribute is a data attribute of a CDC or a sub data attribute of
// a constructed attribute type.
type DataAttribute struct {
	// Name is the attribute name, e.g. "mag" or "f".
	Name string `json:"name"`
	// Type is the name of the basic, constructed or enumerated type.
	// It may be empty for attributes of parameterized CDCs, which take
	// the data object's underlying type.
	Type string `json:"type,omitempty"`
	// TypeKind is the declared kind of Type.
	TypeKind TypeKind `json:"typeKind,omitempty"`
	// FC is the functional constraint. It is empty for sub data
	// attributes, which inherit the FC of their parent attribute.
	FC FunctionalConstraint `json:"fc,omitempty"`
	// Presence is the presence condition of the attribute.
	Presence PresenceCondition `json:"presence,omitzero"`
	// IsArray reports whether the attribute is an array.
	IsArray bool `json:"isArray,omitempty"`
	// SizeAttribute names the attribute that holds the array size.
	SizeAttribute string `json:"sizeAttribute,omitempty"`
	// MinIndexAttribute and MaxIndexAttribute name the attributes that
	// bound the array index.
	MinIndexAttribute string `json:"minIndexAttribute,omitempty"`
	MaxIndexAttribute string `json:"maxIndexAttribute,omitempty"`
	// Dchg, Qchg and Dupd are the trigger options of the attribute.
	Dchg bool `json:"dchg,omitempty"`
	Qchg bool `json:"qchg,omitempty"`
	Dupd bool `json:"dupd,omitempty"`
	// DefaultValue is the declared default value, verbatim.
	DefaultValue string `json:"defaultValue,omitempty"`
	// Deprecated marks an attribute kept for compatibility.
	Deprecated bool `json:"deprecated,omitempty"`
	// Informative marks an attribute that is informative only.
	Informative bool `json:"informative,omitempty"`
	Doc         Doc  `json:"doc,omitzero"`
	// Position is the zero-based position among the children of the
	// parent element, in document order.
	Position int `json:"position"`
}
