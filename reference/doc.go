// SPDX-License-Identifier: MIT

// Package reference parses, validates and formats IEC 61850 object
// references.
//
// Two forms are distinguished:
//
//   - [Ref], an instance reference as used on a device:
//     "LD0/MMXU1.Hz.mag.f[MX]". Its format and validation rules are
//     those of the Ref type of github.com/otfabric/go-iec61850, so
//     values convert between the packages field by field.
//   - [ClassRef], a class reference into the normative model:
//     "MMXU.Hz.mag.f[MX]".
//
// This package checks syntax only. Three levels of validity apply to a
// reference, and each needs a different source:
//
//  1. Syntax: [Parse], [ParseClass] and the Validate methods.
//  2. Model validity: the logical node class, data objects and data
//     attributes exist in a namespace profile with the requested
//     functional constraint. The resolver package checks this.
//  3. Presence on a device: an optional data object may or may not be
//     implemented. Only SCL or device discovery can tell; the model
//     catalog cannot.
package reference
