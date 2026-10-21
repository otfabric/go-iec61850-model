// SPDX-License-Identifier: MIT

// Package semantic describes IEC 61850 model elements in engineering
// terms: quantity (frequency, voltage, active power, ...), phase (A,
// AB, total, ...), voltage reference (line-neutral, line-ground,
// line-line), energy direction, the value component an attribute holds
// (magnitude, angle, counter, quality, ...) and a unit hint.
//
//	d, err := semantic.Describe(profile, "MMXU.PNV.phsA.cVal.mag.f")
//	// d.Quantity == semantic.QuantityVoltage, d.Phase == semantic.PhaseA,
//	// d.Reference == semantic.ReferenceLineNeutral, d.UnitHint.String() == "V"
//
// # Interpretation, not normative content
//
// The mapping is OTFabric interpretation layered on top of the
// normative model; it never changes what the model says. Every
// descriptor names the [Rule] that produced it, its source and a
// confidence. Rules are explicit: a data object without a rule is not
// described, and a rule applies only when the data object has the CDC
// the rule expects. Nothing is inferred from name fragments.
//
// Users and vendors can add overrides ([Mapper.WithOverrides]), which
// take precedence over the default rules and are marked as such.
//
// # Hints and device values
//
// [Descriptor.UnitHint] is the unit a value is expected in according
// to the rule. A device's actual unit and scaling come from its
// configured units (SIUnit, multiplier) and scaling (sVC) attributes;
// [DeviceUnit] interprets the former through the SIUnitKind and
// MultiplierKind enumerations of the loaded model.
//
// A descriptor never implies that a device implements the element.
//
// The quantities and phases are protocol-neutral and are meant to move
// to a shared ontology module later; this package will then adapt to
// it.
package semantic
