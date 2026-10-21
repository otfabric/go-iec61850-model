// SPDX-License-Identifier: MIT

package reference

import (
	"fmt"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// MaxStrictLength is the conservative maximum reference length enforced
// by [ParseStrict], inspired by the IEC 61850-7-2 VisibleString129
// type of ObjectReference attributes.
const MaxStrictLength = 129

// maxLDLength is the maximum length of a logical device name.
const maxLDLength = 64

// Ref is a parsed instance-level IEC 61850 object reference, as used on
// devices: LogicalDevice / LogicalNode . DataObject . DataAttribute,
// with an optional functional constraint in brackets.
//
// Format: LD/LN.DO.DA[FC], e.g. "LD0/MMXU1.Hz.mag.f[MX]".
//
// The format, the fields and the validation rules are those of the Ref
// type of github.com/otfabric/go-iec61850, so references convert
// between the two without loss.
type Ref struct {
	// LD is the logical device name.
	LD string
	// LN is the logical node instance name, e.g. "LLN0" or "MMXU1".
	LN string
	// Path holds the data object and data attribute names. It is empty
	// when the reference points to a logical node.
	Path []string
	// FC is the functional constraint; empty when not specified.
	FC model.FunctionalConstraint
}

// Parse parses an instance-level object reference.
//
// Accepted forms:
//
//	LD/LN.DO.DA[FC]  full reference with functional constraint
//	LD/LN.DO.DA      reference without functional constraint
//	LD/LN            logical node reference
//	LD               logical device reference
//
// It fails with a [*model.ReferenceError] wrapping
// [model.ErrInvalidReference] on malformed input. Unknown functional
// constraints are accepted when they have two characters. Parse checks
// syntax only; use the resolver package to validate a reference
// against a namespace profile.
func Parse(s string) (Ref, error) {
	if s == "" {
		return Ref{}, &model.ReferenceError{Input: s, Reason: "empty reference"}
	}
	work, fc, err := splitFC(s)
	if err != nil {
		return Ref{}, err
	}

	slash := strings.Index(work, "/")
	if slash < 0 {
		if strings.Contains(work, ".") {
			return Ref{}, &model.ReferenceError{Input: s, Reason: "missing logical device separator '/'"}
		}
		return validated(Ref{LD: work, FC: fc})
	}

	ld := work[:slash]
	if ld == "" {
		return Ref{}, &model.ReferenceError{Input: s, Reason: "empty logical device name"}
	}
	if len(ld) > maxLDLength {
		return Ref{}, &model.ReferenceError{Input: s, Reason: fmt.Sprintf("logical device name exceeds %d characters", maxLDLength)}
	}
	rest := work[slash+1:]
	if rest == "" {
		return Ref{}, &model.ReferenceError{Input: s, Reason: "empty content after '/'"}
	}
	dot := strings.Index(rest, ".")
	if dot < 0 {
		return validated(Ref{LD: ld, LN: rest, FC: fc})
	}
	ln := rest[:dot]
	if ln == "" {
		return Ref{}, &model.ReferenceError{Input: s, Reason: "empty logical node name"}
	}
	pathStr := rest[dot+1:]
	if pathStr == "" {
		return Ref{}, &model.ReferenceError{Input: s, Reason: "empty path after logical node"}
	}
	return validated(Ref{LD: ld, LN: ln, Path: strings.Split(pathStr, "."), FC: fc})
}

func validated(r Ref) (Ref, error) {
	if err := r.Validate(); err != nil {
		return Ref{}, err
	}
	return r, nil
}

// ParseStrict is like [Parse] but rejects references longer than
// [MaxStrictLength] characters.
func ParseStrict(s string) (Ref, error) {
	if len(s) > MaxStrictLength {
		return Ref{}, &model.ReferenceError{Input: s, Reason: fmt.Sprintf("exceeds maximum length (%d)", MaxStrictLength)}
	}
	return Parse(s)
}

// splitFC removes a trailing "[FC]" suffix and returns the remainder
// and the functional constraint.
func splitFC(s string) (string, model.FunctionalConstraint, error) {
	opens, closes := strings.Count(s, "["), strings.Count(s, "]")
	if opens != closes {
		return "", "", &model.ReferenceError{Input: s, Reason: "mismatched brackets"}
	}
	if opens > 1 {
		return "", "", &model.ReferenceError{Input: s, Reason: "multiple bracket pairs"}
	}
	idx := strings.Index(s, "[")
	if idx < 0 {
		return s, "", nil
	}
	end := strings.Index(s, "]")
	if end != len(s)-1 || end < idx {
		return "", "", &model.ReferenceError{Input: s, Reason: "malformed functional constraint brackets"}
	}
	fc := s[idx+1 : end]
	if len(fc) != 2 {
		return "", "", &model.ReferenceError{Input: s, Reason: fmt.Sprintf("functional constraint must be 2 characters, got %q", fc)}
	}
	return s[:idx], model.FunctionalConstraint(fc), nil
}

// Validate checks the structural invariants of the reference:
//
//   - LD is non-empty
//   - Path and FC require LN
//   - no empty path components
//   - FC, when set, has exactly two characters
//   - no component contains a separator (/ . $ [ ])
func (r Ref) Validate() error {
	if r.LD == "" {
		return &model.ReferenceError{Input: r.String(), Reason: "empty logical device name"}
	}
	if err := validateComponent(r.LD, "logical device"); err != nil {
		return &model.ReferenceError{Input: r.String(), Reason: err.Error()}
	}
	if r.LN == "" && len(r.Path) > 0 {
		return &model.ReferenceError{Input: r.String(), Reason: "path present without logical node"}
	}
	if r.FC != "" && r.LN == "" {
		return &model.ReferenceError{Input: r.String(), Reason: "functional constraint requires logical node"}
	}
	if r.LN != "" {
		if err := validateComponent(r.LN, "logical node"); err != nil {
			return &model.ReferenceError{Input: r.String(), Reason: err.Error()}
		}
	}
	if err := validatePath(r.Path); err != nil {
		return &model.ReferenceError{Input: r.String(), Reason: err.Error()}
	}
	if r.FC != "" && len(r.FC) != 2 {
		return &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("functional constraint must be 2 characters, got %q", r.FC)}
	}
	return nil
}

func validatePath(path []string) error {
	for i, p := range path {
		if p == "" {
			return fmt.Errorf("empty path component at position %d", i)
		}
		if err := validateComponent(p, fmt.Sprintf("path component [%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

func validateComponent(name, label string) error {
	if strings.ContainsAny(name, "/.$[]") {
		return fmt.Errorf("%s %q contains illegal separator character", label, name)
	}
	return nil
}

// String returns the canonical form, e.g. "LD0/MMXU1.Hz.mag.f[MX]".
func (r Ref) String() string {
	var b strings.Builder
	b.WriteString(r.LD)
	if r.LN != "" {
		b.WriteByte('/')
		b.WriteString(r.LN)
		if len(r.Path) > 0 {
			b.WriteByte('.')
			b.WriteString(strings.Join(r.Path, "."))
		}
	}
	writeFC(&b, r.FC)
	return b.String()
}

func writeFC(b *strings.Builder, fc model.FunctionalConstraint) {
	if fc != "" {
		b.WriteByte('[')
		b.WriteString(string(fc))
		b.WriteByte(']')
	}
}

// ObjectReference returns the reference without the FC suffix.
func (r Ref) ObjectReference() string {
	r.FC = ""
	return r.String()
}

// WithFC returns a copy of r with the functional constraint set to fc.
func (r Ref) WithFC(fc model.FunctionalConstraint) Ref {
	r.FC = fc
	r.Path = append([]string(nil), r.Path...)
	return r
}

// Parent returns the parent reference, or false when r is LD-only. The
// FC is kept unless the parent is LD-only.
func (r Ref) Parent() (Ref, bool) {
	if len(r.Path) > 0 {
		return Ref{LD: r.LD, LN: r.LN, Path: append([]string(nil), r.Path[:len(r.Path)-1]...), FC: r.FC}, true
	}
	if r.LN != "" {
		return Ref{LD: r.LD}, true
	}
	return Ref{}, false
}

// Child returns a copy of r with name appended to the path.
func (r Ref) Child(name string) (Ref, error) {
	if r.LN == "" {
		return Ref{}, &model.ReferenceError{Input: r.String(), Reason: "cannot add child path without logical node"}
	}
	if name == "" {
		return Ref{}, &model.ReferenceError{Input: r.String(), Reason: "empty child name"}
	}
	if strings.ContainsAny(name, "/.$[]") {
		return Ref{}, &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("child name %q contains illegal separator character", name)}
	}
	path := make([]string, len(r.Path), len(r.Path)+1)
	copy(path, r.Path)
	return Ref{LD: r.LD, LN: r.LN, Path: append(path, name), FC: r.FC}, nil
}

// IsLD reports whether the reference points to a logical device only.
func (r Ref) IsLD() bool { return r.LD != "" && r.LN == "" && len(r.Path) == 0 }

// IsLN reports whether the reference points to a logical node.
func (r Ref) IsLN() bool { return r.LN != "" && len(r.Path) == 0 }

// IsObject reports whether the reference points to a data object or
// data attribute.
func (r Ref) IsObject() bool { return r.LN != "" && len(r.Path) > 0 }

// LNName splits the logical node instance name into prefix, class and
// instance; see [ParseLNName].
func (r Ref) LNName() (LNName, error) {
	if r.LN == "" {
		return LNName{}, &model.ReferenceError{Input: r.String(), Reason: "reference has no logical node"}
	}
	return ParseLNName(r.LN)
}

// Class returns the class-level reference of r: the logical node class
// derived from the instance name, with the same path and FC.
func (r Ref) Class() (ClassRef, error) {
	n, err := r.LNName()
	if err != nil {
		return ClassRef{}, err
	}
	return ClassRef{LNClass: n.Class, Path: append([]string(nil), r.Path...), FC: r.FC}, nil
}

// ToMMS returns the MMS domain and item ID of the reference, following
// the IEC 61850-8-1 mapping LN$FC$DO$DA. A reference with a path needs
// a functional constraint.
func (r Ref) ToMMS() (domain, item string, err error) {
	if err := r.Validate(); err != nil {
		return "", "", err
	}
	if r.LN == "" {
		return r.LD, "", nil
	}
	if len(r.Path) == 0 && r.FC == "" {
		return r.LD, r.LN, nil
	}
	if r.FC == "" {
		return "", "", &model.ReferenceError{Input: r.String(), Reason: "cannot convert to MMS item ID: functional constraint required when path is present"}
	}
	parts := append([]string{r.LN, string(r.FC)}, r.Path...)
	return r.LD, strings.Join(parts, "$"), nil
}

// FromMMS builds a reference from an MMS domain and item ID of the form
// LN$FC$DO$DA.
func FromMMS(domain, item string) (Ref, error) {
	if domain == "" {
		return Ref{}, &model.ReferenceError{Input: item, Reason: "empty MMS domain"}
	}
	if item == "" {
		return validated(Ref{LD: domain})
	}
	parts := strings.Split(item, "$")
	if parts[0] == "" {
		return Ref{}, &model.ReferenceError{Input: item, Reason: "empty logical node in MMS item ID"}
	}
	r := Ref{LD: domain, LN: parts[0]}
	if len(parts) > 1 {
		if parts[1] == "" {
			return Ref{}, &model.ReferenceError{Input: item, Reason: "empty functional constraint in MMS item ID"}
		}
		r.FC = model.FunctionalConstraint(parts[1])
	}
	if len(parts) > 2 {
		r.Path = parts[2:]
	}
	return validated(r)
}
