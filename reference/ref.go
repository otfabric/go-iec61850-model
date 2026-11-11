// SPDX-License-Identifier: MIT

package reference

import (
	"errors"
	"fmt"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// MaxStrictLength is the maximum length of an ObjectReference enforced
// by [ParseStrict]: IEC 61850-7-2:2010 Table 4 types ObjectReference as
// VISIBLE STRING129. The functional constraint suffix "[FC]" is not
// part of the ObjectReference (7-2 §22.2 NOTE 1) and is not counted.
const MaxStrictLength = 129

// MaxLDLength is the maximum length of a logical device name, and
// MaxPathLength the maximum length of the path below it (LN.DO.DA),
// IEC 61850-7-2:2010 Table 4: at most 64 characters each, joined by a
// single "/".
const (
	MaxLDLength   = 64
	MaxPathLength = 64
)

// MaxMMSIdentifier is the MMS maxIdentifier size that bounds MMS domain
// and object names, IEC 61850-8-1:2011 §8.1.3.2.2 (see F.3.2).
const MaxMMSIdentifier = 64

// maxLDLength is the maximum length of a logical device name.
const maxLDLength = MaxLDLength

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

// ParseStrict is like [Parse] and also enforces the length limits of
// IEC 61850-7-2:2010 Table 4 on the ObjectReference, the reference
// without its "[FC]" suffix: at most [MaxStrictLength] characters, a
// logical device name of at most [MaxLDLength] and a path below it of
// at most [MaxPathLength] characters. The naming rules of 7-2 §22.2
// (character set, start characters, name lengths) are checked
// separately by [Ref.ValidateNames].
func ParseStrict(s string) (Ref, error) {
	if len(s) > MaxStrictLength+4 { // cheap bound before parsing: "[FC]" is 4 characters
		return Ref{}, &model.ReferenceError{Input: s, Reason: fmt.Sprintf("exceeds maximum length (%d)", MaxStrictLength)}
	}
	r, err := Parse(s)
	if err != nil {
		return Ref{}, err
	}
	if err := r.checkLengths(); err != nil {
		return Ref{}, err
	}
	return r, nil
}

// checkLengths enforces the limits of 7-2:2010 Table 4.
func (r Ref) checkLengths() error {
	obj := r.ObjectReference()
	if len(obj) > MaxStrictLength {
		return &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("exceeds maximum length (%d)", MaxStrictLength)}
	}
	if below := len(obj) - len(r.LD) - 1; r.LN != "" && below > MaxPathLength {
		return &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("path below the logical device exceeds %d characters", MaxPathLength)}
	}
	return nil
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
	if !validFC(fc) {
		return "", "", &model.ReferenceError{Input: s, Reason: fmt.Sprintf("functional constraint must be 2 upper-case letters, got %q", fc)}
	}
	return s[:idx], model.FunctionalConstraint(fc), nil
}

// validFC reports whether fc is two upper-case letters. The FC values
// of IEC 61850-7-2:2010 Table 20 and IEC 61850-8-1:2011 Figure 4 are
// all of that form; "[mx]" is not a functional constraint.
func validFC(fc string) bool {
	return len(fc) == 2 && fc[0] >= 'A' && fc[0] <= 'Z' && fc[1] >= 'A' && fc[1] <= 'Z'
}

// Validate checks the structural invariants of the reference:
//
//   - LD is non-empty and at most [MaxLDLength] characters
//     (IEC 61850-7-2:2010 §22.2 and Table 4)
//   - Path and FC require LN
//   - no empty path components
//   - FC, when set, is two upper-case letters
//   - no component contains a separator (/ . $ [ ])
//   - an array index "(n)" is a decimal number closing a path
//     component (7-2:2010 §6.1.2.3)
func (r Ref) Validate() error {
	if r.LD == "" {
		return &model.ReferenceError{Input: r.String(), Reason: "empty logical device name"}
	}
	if len(r.LD) > maxLDLength {
		return &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("logical device name exceeds %d characters", maxLDLength)}
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
	if r.FC != "" && !validFC(string(r.FC)) {
		return &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("functional constraint must be 2 upper-case letters, got %q", r.FC)}
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
		if _, _, _, err := SplitIndex(p); err != nil {
			return fmt.Errorf("path component [%d]: %w", i, err)
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
	if _, _, _, err := SplitIndex(name); err != nil {
		return Ref{}, &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("child name %q: %v", name, err)}
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
//
// The domain and the item ID are MMS identifiers: each is at most
// [MaxMMSIdentifier] characters (IEC 61850-8-1:2011 §8.1.3.2.2 and
// §7.3.1); longer names fail with a [*model.ReferenceError] that wraps
// both [model.ErrInvalidReference] and [model.ErrLimitExceeded]. An
// array element "(n)" has no item ID form, because 8-1 addresses array
// elements with MMS alternate access, so a reference with an index
// fails too.
func (r Ref) ToMMS() (domain, item string, err error) {
	if err := r.Validate(); err != nil {
		return "", "", err
	}
	for _, p := range r.Path {
		if _, _, ok, _ := SplitIndex(p); ok {
			return "", "", &model.ReferenceError{Input: r.String(), Reason: "an array element has no MMS item ID; IEC 61850-8-1 addresses it with alternate access"}
		}
	}
	domain, item, err = r.toMMS()
	if err != nil {
		return "", "", err
	}
	if err := checkMMSIdentifiers(r.String(), domain, item); err != nil {
		return "", "", err
	}
	return domain, item, nil
}

// checkMMSIdentifiers enforces the MMS maxIdentifier size of 64 on the
// domain and the item ID (8-1:2011 §8.1.3.2.2).
func checkMMSIdentifiers(input, domain, item string) error {
	for _, n := range []struct{ what, v string }{{"MMS domain name", domain}, {"MMS item ID", item}} {
		if len(n.v) > MaxMMSIdentifier {
			return &model.ReferenceError{Input: input, Reason: fmt.Sprintf("%s exceeds the MMS maxIdentifier size %d (%d characters)", n.what, MaxMMSIdentifier, len(n.v)),
				Wrapped: errors.Join(model.ErrInvalidReference, model.ErrLimitExceeded)}
		}
	}
	return nil
}

func (r Ref) toMMS() (domain, item string, err error) {
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
// LN$FC$DO$DA. The domain and item ID are bounded by the MMS
// maxIdentifier size, as in [Ref.ToMMS].
func FromMMS(domain, item string) (Ref, error) {
	if domain == "" {
		return Ref{}, &model.ReferenceError{Input: item, Reason: "empty MMS domain"}
	}
	if err := checkMMSIdentifiers(domain+"/"+item, domain, item); err != nil {
		return Ref{}, err
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
