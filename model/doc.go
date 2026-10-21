// SPDX-License-Identifier: MIT

// Package model defines the typed, normalized representation of IEC
// 61850 namespace definitions (NSD): namespaces, logical node classes,
// data objects, common data classes (CDCs), data attributes,
// constructed attributes, enumerations, functional constraints and
// presence conditions, together with provenance, diagnostics and the
// module's error taxonomy.
//
// The types describe the normative class and type model of the
// standard, not the configuration of a device. A [LogicalNodeClass]
// lists every data object the namespace defines for the class,
// including optional ones; whether a device implements them is a
// question for SCL or discovery.
//
// Values of these types are produced by the nsd package and indexed by
// the catalog package. Values held by a catalog are shared and must be
// treated as read-only.
//
// # Errors
//
// All packages of the module return errors that wrap the sentinels
// declared here ([ErrInvalidNSD], [ErrUnknownLogicalNode], ...), so a
// single [errors.Is] check works across packages. Typed errors
// ([*ResolutionError], [*AmbiguityError], [*DependencyError],
// [*SourceError], [*ValidationError], [*ReferenceError]) carry detail
// for [errors.As].
package model
