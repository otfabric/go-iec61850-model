// SPDX-License-Identifier: MIT

// Package resolver resolves references and model paths against a
// namespace profile, from a logical node class through data objects,
// sub data objects and CDCs down to data attributes, constructed types
// and basic or enumerated leaf types:
//
//	MMXU → Hz (MV) → mag (AnalogueValue) → f (FLOAT32)
//	MMXU → PNV (WYE) → phsA (CMV) → cVal (Vector) → mag → f
//
// Inherited data objects are found through the base classes of the
// logical node class. Attributes of parameterized CDCs, such as stVal
// of ENS, take the underlying type of their data object. When a
// functional constraint is given, every data attribute on the path
// must have it.
//
// Every [Step] of a [Resolution] records the class, CDC or type that
// declares it and the namespace that defines that, so results keep
// their provenance. Unknown elements fail with a
// [*model.ResolutionError]; nothing degrades to an anonymous generic
// type. Names that the lookup cannot resolve unambiguously fail with a
// [*model.AmbiguityError].
//
// A successful resolution means the path is valid in the normative
// model. It does not mean that a device implements it: optional data
// objects and attributes may be absent, which only SCL or discovery can
// tell.
package resolver
