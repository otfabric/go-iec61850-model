// SPDX-License-Identifier: MIT

package model

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for the failure categories of this module. Every
// package of the module returns errors that wrap one of these values,
// either directly or through one of the typed errors below.
//
// Use [errors.Is] to test against these values.
var (
	// ErrInvalidNSD indicates that an NSD document is malformed or
	// structurally invalid.
	ErrInvalidNSD = errors.New("iec61850-model: invalid NSD")

	// ErrUnsupportedSchema indicates an NSD document whose root element
	// or XML namespace is not supported by the importer.
	ErrUnsupportedSchema = errors.New("iec61850-model: unsupported NSD schema")

	// ErrLimitExceeded indicates that an input exceeded a configured
	// safety bound (size, depth, entry count or compression ratio).
	ErrLimitExceeded = errors.New("iec61850-model: input limit exceeded")

	// ErrUnsafeArchive indicates a ZIP archive with unsafe entry names
	// (absolute paths, path traversal) or duplicate entries.
	ErrUnsafeArchive = errors.New("iec61850-model: unsafe archive")

	// ErrMissingDependency indicates that a namespace depends on a
	// namespace that is not available.
	ErrMissingDependency = errors.New("iec61850-model: missing namespace dependency")

	// ErrNamespaceConflict indicates two namespaces, or two definitions
	// within a profile, that cannot coexist.
	ErrNamespaceConflict = errors.New("iec61850-model: namespace conflict")

	// ErrUnknownNamespace indicates that a requested namespace is not
	// loaded in the catalog.
	ErrUnknownNamespace = errors.New("iec61850-model: unknown namespace")

	// ErrUnknownLogicalNode indicates that a logical node class is not
	// defined.
	ErrUnknownLogicalNode = errors.New("iec61850-model: unknown logical node")

	// ErrUnknownDataObject indicates that a data object is not defined
	// in a logical node class or CDC.
	ErrUnknownDataObject = errors.New("iec61850-model: unknown data object")

	// ErrUnknownCDC indicates that a common data class is not defined.
	ErrUnknownCDC = errors.New("iec61850-model: unknown CDC")

	// ErrUnknownAttribute indicates that a data attribute is not
	// defined, or not defined with the requested functional constraint.
	ErrUnknownAttribute = errors.New("iec61850-model: unknown data attribute")

	// ErrUnknownType indicates that an attribute type (basic,
	// constructed or enumerated) cannot be found or determined.
	ErrUnknownType = errors.New("iec61850-model: unknown type")

	// ErrInvalidReference indicates a syntactically invalid object
	// reference.
	ErrInvalidReference = errors.New("iec61850-model: invalid object reference")

	// ErrAmbiguousReference indicates a lookup that matches more than
	// one definition: a name that two namespaces of the profile define
	// (DBAT in IEC 61850-7-420 and IEC TR 61850-90-9). Look it up with
	// its namespace (LogicalNodeIn and the other …In lookups).
	ErrAmbiguousReference = errors.New("iec61850-model: ambiguous reference")

	// ErrAmbiguousSymbol is ErrAmbiguousReference, under the name the
	// namespace policy uses (REQUIREMENTS.md, section 5.2.1).
	ErrAmbiguousSymbol = ErrAmbiguousReference

	// ErrCycle indicates a cyclic inheritance or type definition.
	ErrCycle = errors.New("iec61850-model: cyclic definition")

	// ErrSourceUnavailable indicates that an NSD source could not be
	// opened or read.
	ErrSourceUnavailable = errors.New("iec61850-model: source unavailable")

	// ErrNoMapping indicates that no semantic mapping rule applies to a
	// model element, or that the rule does not fit the model.
	ErrNoMapping = errors.New("iec61850-model: no semantic mapping")

	// ErrUnknownUnit indicates a unit that cannot be interpreted.
	ErrUnknownUnit = errors.New("iec61850-model: unknown unit")

	// ErrUnknownConcept indicates a concept that is not part of an
	// ontology graph.
	ErrUnknownConcept = errors.New("iec61850-model: unknown concept")

	// ErrInvalidRule indicates a semantic mapping rule or override that
	// is incomplete or conflicts with another rule.
	ErrInvalidRule = errors.New("iec61850-model: invalid mapping rule")
)

// ReferenceError reports a malformed object reference.
//
// It unwraps to Wrapped when set, otherwise to [ErrInvalidReference].
type ReferenceError struct {
	// Input is the reference string that failed to parse.
	Input string
	// Reason explains what is wrong with the input.
	Reason string
	// Wrapped is an optional underlying error.
	Wrapped error
}

func (e *ReferenceError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("iec61850-model: invalid reference %q: %s: %v", e.Input, e.Reason, e.Wrapped)
	}
	return fmt.Sprintf("iec61850-model: invalid reference %q: %s", e.Input, e.Reason)
}

func (e *ReferenceError) Unwrap() error {
	if e.Wrapped != nil {
		return e.Wrapped
	}
	return ErrInvalidReference
}

// ResolutionError reports a failure to resolve a model path, such as an
// unknown data object, an attribute with a different functional
// constraint or an unresolvable type.
//
// It unwraps to Err, which is one of the package sentinels.
type ResolutionError struct {
	// Namespace is the namespace of the element where resolution
	// stopped, when known.
	Namespace NamespaceID
	// Path is the model path resolved so far, e.g. "MMXU.Hz.mag".
	Path string
	// Element is the name that could not be resolved.
	Element string
	// Expected describes what was expected (a type, an FC), if relevant.
	Expected string
	// Actual describes what was found instead, if relevant.
	Actual string
	// Reason is a human-readable explanation.
	Reason string
	// Err is the sentinel this error wraps.
	Err error
}

func (e *ResolutionError) Error() string {
	var b strings.Builder
	b.WriteString("iec61850-model: cannot resolve ")
	if e.Path != "" {
		fmt.Fprintf(&b, "%q", e.Path)
	} else {
		fmt.Fprintf(&b, "%q", e.Element)
	}
	if e.Reason != "" {
		b.WriteString(": ")
		b.WriteString(e.Reason)
	}
	if e.Expected != "" || e.Actual != "" {
		fmt.Fprintf(&b, " (expected %s, got %s)", orDash(e.Expected), orDash(e.Actual))
	}
	if !e.Namespace.IsZero() {
		fmt.Fprintf(&b, " [%s]", e.Namespace)
	}
	return b.String()
}

func (e *ResolutionError) Unwrap() error { return e.Err }

// AmbiguityError reports a lookup that matched definitions in more
// than one namespace. Select a namespace profile to disambiguate.
//
// It unwraps to [ErrAmbiguousReference].
type AmbiguityError struct {
	// Kind is the kind of element looked up, e.g. "logical node class".
	Kind string
	// Name is the name that was looked up.
	Name string
	// Candidates are the namespaces that define Name, in catalog order.
	Candidates []NamespaceID
}

func (e *AmbiguityError) Error() string {
	ids := make([]string, len(e.Candidates))
	for i, c := range e.Candidates {
		ids[i] = c.String()
	}
	return fmt.Sprintf("iec61850-model: ambiguous %s %q: defined in %s", e.Kind, e.Name, strings.Join(ids, ", "))
}

func (e *AmbiguityError) Unwrap() error { return ErrAmbiguousReference }

// DependencyError reports a namespace dependency that cannot be
// satisfied.
//
// It unwraps to Err, which is [ErrMissingDependency],
// [ErrAmbiguousReference] (more than one candidate release) or
// [ErrNamespaceConflict] (the profile selects an incompatible release).
type DependencyError struct {
	// Namespace is the namespace that declares the dependency.
	Namespace NamespaceID
	// Required is the dependency as declared.
	Required NamespaceID
	// Candidates are loaded namespaces that could satisfy the
	// dependency when the error is an ambiguity, or the selected
	// namespace that conflicts with it.
	Candidates []NamespaceID
	// Reason overrides the default explanation when set.
	Reason string
	// Err is the sentinel this error wraps.
	Err error
}

func (e *DependencyError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("iec61850-model: dependency %s of %s: %s", e.Required, e.Namespace, e.Reason)
	}
	if len(e.Candidates) > 0 {
		ids := make([]string, len(e.Candidates))
		for i, c := range e.Candidates {
			ids[i] = c.String()
		}
		return fmt.Sprintf("iec61850-model: dependency %s of %s is ambiguous: candidates %s; select one explicitly",
			e.Required, e.Namespace, strings.Join(ids, ", "))
	}
	return fmt.Sprintf("iec61850-model: %s requires %s, which is not loaded", e.Namespace, e.Required)
}

func (e *DependencyError) Unwrap() error { return e.Err }

// SourceError reports a failure tied to an input source: a file, an
// archive entry or a reader.
//
// It unwraps to Err.
type SourceError struct {
	// Source names the input, e.g. a file path or "archive.zip!entry.nsd".
	Source string
	// Reason is a human-readable explanation.
	Reason string
	// Err is the underlying error or sentinel.
	Err error
}

func (e *SourceError) Error() string {
	if e.Err != nil && e.Reason != "" {
		return fmt.Sprintf("iec61850-model: %s: %s: %v", e.Source, e.Reason, e.Err)
	}
	if e.Err != nil {
		return fmt.Sprintf("iec61850-model: %s: %v", e.Source, e.Err)
	}
	return fmt.Sprintf("iec61850-model: %s: %s", e.Source, e.Reason)
}

func (e *SourceError) Unwrap() error { return e.Err }

// ValidationError carries the diagnostics of a failed validation. Only
// diagnostics of severity [SeverityError] cause a ValidationError; the
// slice may also contain warnings for context.
//
// It unwraps to Err, which defaults to [ErrInvalidNSD].
type ValidationError struct {
	// Diagnostics lists every finding, in document order.
	Diagnostics []Diagnostic
	// Err is the sentinel this error wraps; nil means [ErrInvalidNSD].
	Err error
}

func (e *ValidationError) Error() string {
	errs := Diagnostics(e.Diagnostics).Errors()
	if len(errs) == 0 {
		return "iec61850-model: validation failed"
	}
	if len(errs) == 1 {
		return "iec61850-model: validation failed: " + errs[0].String()
	}
	return fmt.Sprintf("iec61850-model: validation failed: %s (and %d more errors)", errs[0].String(), len(errs)-1)
}

func (e *ValidationError) Unwrap() error {
	if e.Err != nil {
		return e.Err
	}
	return ErrInvalidNSD
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
