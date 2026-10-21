// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"strings"
)

// ContentKind is the kind of an NSD package.
type ContentKind string

// NSD package kinds.
const (
	ContentUnknown ContentKind = ""
	ContentLight   ContentKind = "light"
	ContentFull    ContentKind = "full"
)

// Provenance records where a namespace was loaded from. It contains
// metadata only, never source content.
type Provenance struct {
	// Source names the input, e.g. a file path or
	// "package.zip!IEC_61850-7-4.nsd".
	Source string `json:"source,omitempty"`
	// SHA256 is the hex-encoded SHA-256 digest of the NSD document.
	SHA256 string `json:"sha256,omitempty"`
	// PackageSHA256 is the hex-encoded SHA-256 digest of the enclosing
	// archive, when loaded from one.
	PackageSHA256 string `json:"packageSHA256,omitempty"`
	// Content is the package kind, when known.
	Content ContentKind `json:"content,omitempty"`
	// SchemaNamespace is the XML namespace URI of the NSD document.
	SchemaNamespace string `json:"schemaNamespace,omitempty"`
}

// Severity is the severity of a diagnostic.
type Severity int

// Diagnostic severities.
const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// String returns "info", "warning" or "error".
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// MarshalText implements [encoding.TextMarshaler].
func (s Severity) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Diagnostic is one finding of an import, validation or profile
// construction, located by source and element path.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	// Code is a stable machine-readable code, e.g. "unknown-attribute".
	Code string `json:"code"`
	// Namespace is the namespace concerned, when known.
	Namespace NamespaceID `json:"namespace,omitzero"`
	// Source names the input, e.g. a file path or archive entry.
	Source string `json:"source,omitempty"`
	// Path locates the element, e.g.
	// "/NS/LNClasses/LNClass[MMXU]/DataObject[Hz]".
	Path string `json:"path,omitempty"`
	// Message is a human-readable explanation.
	Message string `json:"message"`
}

// String formats the diagnostic on one line.
func (d Diagnostic) String() string {
	var b strings.Builder
	b.WriteString(d.Severity.String())
	b.WriteString(": ")
	if d.Source != "" {
		b.WriteString(d.Source)
		b.WriteString(": ")
	}
	if d.Path != "" {
		b.WriteString(d.Path)
		b.WriteString(": ")
	}
	b.WriteString(d.Message)
	if d.Code != "" {
		b.WriteString(" [")
		b.WriteString(d.Code)
		b.WriteString("]")
	}
	return b.String()
}

// Diagnostics is a list of diagnostics with filtering helpers.
type Diagnostics []Diagnostic

// Errors returns the diagnostics of severity [SeverityError].
func (ds Diagnostics) Errors() Diagnostics { return ds.AtLeast(SeverityError) }

// AtLeast returns the diagnostics with severity s or higher.
func (ds Diagnostics) AtLeast(s Severity) Diagnostics {
	var out Diagnostics
	for _, d := range ds {
		if d.Severity >= s {
			out = append(out, d)
		}
	}
	return out
}

// HasErrors reports whether any diagnostic has severity
// [SeverityError].
func (ds Diagnostics) HasErrors() bool {
	for _, d := range ds {
		if d.Severity >= SeverityError {
			return true
		}
	}
	return false
}

// Err returns a [*ValidationError] when the list contains errors, and
// nil otherwise.
func (ds Diagnostics) Err() error {
	if !ds.HasErrors() {
		return nil
	}
	return &ValidationError{Diagnostics: ds}
}
