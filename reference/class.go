// SPDX-License-Identifier: MIT

package reference

import (
	"fmt"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// ClassRef is a class-level reference into the normative model: a
// logical node class followed by data object and data attribute names,
// with an optional functional constraint, e.g. "MMXU.Hz.mag.f[MX]".
//
// A class reference names what the standard defines; it says nothing
// about which logical device or instance a device exposes.
type ClassRef struct {
	// LNClass is the logical node class name, e.g. "MMXU".
	LNClass string
	// Path holds the data object and data attribute names.
	Path []string
	// FC is the functional constraint; empty when not specified.
	FC model.FunctionalConstraint
}

// ParseClass parses a class-level reference of the form
// LNClass.DO.DA[FC]. It fails with a [*model.ReferenceError] when the
// input contains a logical device separator: use [Parse] for instance
// references.
func ParseClass(s string) (ClassRef, error) {
	if s == "" {
		return ClassRef{}, &model.ReferenceError{Input: s, Reason: "empty reference"}
	}
	if strings.Contains(s, "/") {
		return ClassRef{}, &model.ReferenceError{Input: s, Reason: "class reference must not contain a logical device ('/')"}
	}
	work, fc, err := splitFC(s)
	if err != nil {
		return ClassRef{}, err
	}
	parts := strings.Split(work, ".")
	r := ClassRef{LNClass: parts[0], FC: fc}
	if len(parts) > 1 {
		r.Path = parts[1:]
	}
	if err := r.Validate(); err != nil {
		return ClassRef{}, err
	}
	return r, nil
}

// Validate checks the structural invariants of the reference.
func (r ClassRef) Validate() error {
	if r.LNClass == "" {
		return &model.ReferenceError{Input: r.String(), Reason: "empty logical node class"}
	}
	if err := validateComponent(r.LNClass, "logical node class"); err != nil {
		return &model.ReferenceError{Input: r.String(), Reason: err.Error()}
	}
	if err := validatePath(r.Path); err != nil {
		return &model.ReferenceError{Input: r.String(), Reason: err.Error()}
	}
	if r.FC != "" && len(r.FC) != 2 {
		return &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf("functional constraint must be 2 characters, got %q", r.FC)}
	}
	return nil
}

// String returns the canonical form, e.g. "MMXU.Hz.mag.f[MX]".
func (r ClassRef) String() string {
	var b strings.Builder
	b.WriteString(r.LNClass)
	for _, p := range r.Path {
		b.WriteByte('.')
		b.WriteString(p)
	}
	writeFC(&b, r.FC)
	return b.String()
}

// DataObject returns the first path element, the data object name, or
// "" when the reference points to the class.
func (r ClassRef) DataObject() string {
	if len(r.Path) == 0 {
		return ""
	}
	return r.Path[0]
}

// LNName is a logical node instance name split into its parts, e.g.
// "Q0MMXU1" into prefix "Q0", class "MMXU" and instance "1".
type LNName struct {
	Prefix string
	Class  string
	Inst   string
}

// String joins the parts back into the instance name.
func (n LNName) String() string { return n.Prefix + n.Class + n.Inst }

// ParseLNName splits a logical node instance name into prefix, class
// and instance number. The instance is the trailing run of digits and
// the class the four characters before it, which must be upper-case
// letters; anything before the class is the prefix. "LLN0" is the
// class LLN0 without prefix or instance.
//
// The split is syntactic: whether the class exists is checked by
// resolving the reference against a profile.
func ParseLNName(s string) (LNName, error) {
	if s == "LLN0" {
		return LNName{Class: "LLN0"}, nil
	}
	if s == "" {
		return LNName{}, &model.ReferenceError{Input: s, Reason: "empty logical node name"}
	}
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	head, inst := s[:i], s[i:]
	if len(head) < 4 {
		return LNName{}, &model.ReferenceError{Input: s, Reason: "logical node name has no four-letter class"}
	}
	class, prefix := head[len(head)-4:], head[:len(head)-4]
	for _, c := range class {
		if c < 'A' || c > 'Z' {
			return LNName{}, &model.ReferenceError{Input: s, Reason: fmt.Sprintf("logical node class %q must be four upper-case letters", class)}
		}
	}
	for _, c := range prefix {
		if !isNameChar(c) {
			return LNName{}, &model.ReferenceError{Input: s, Reason: fmt.Sprintf("logical node prefix %q contains invalid character %q", prefix, c)}
		}
	}
	return LNName{Prefix: prefix, Class: class, Inst: inst}, nil
}

// Normalize parses a reference in any supported form and returns its
// canonical string:
//
//   - instance references "LD0/MMXU1.Hz.mag.f[MX]"
//   - MMS-style references "LD0/MMXU1$MX$Hz$mag$f"
//   - class references "MMXU.Hz.mag.f[MX]"
//
// Input without "/" is read as a class reference. Normalization changes
// only the notation, never the content: an MMS reference becomes the
// bracketed FC form, and nothing is trimmed, case-folded or inferred.
func Normalize(s string) (string, error) {
	switch {
	case strings.Contains(s, "$"):
		slash := strings.Index(s, "/")
		if slash < 0 {
			return "", &model.ReferenceError{Input: s, Reason: "MMS-style reference needs a domain ('LD/LN$FC$...')"}
		}
		r, err := FromMMS(s[:slash], s[slash+1:])
		if err != nil {
			return "", err
		}
		return r.String(), nil
	case strings.Contains(s, "/"):
		r, err := Parse(s)
		if err != nil {
			return "", err
		}
		return r.String(), nil
	default:
		r, err := ParseClass(s)
		if err != nil {
			return "", err
		}
		return r.String(), nil
	}
}

func isNameChar(c rune) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}
