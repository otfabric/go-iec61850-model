// SPDX-License-Identifier: MIT

package reference

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Name limits of IEC 61850-7-2:2010 §22.2, checked by [Ref.ValidateNames].
const (
	// MaxLNPrefixInstance bounds the LN-Prefix plus LN-Instance-ID
	// (m+n ≤ 12).
	MaxLNPrefixInstance = 12
	// MaxDataObjectName bounds a DataObjectName (≤ 12 characters).
	MaxDataObjectName = 12
	// MaxFCDALength bounds an FCD or FCDA without FC and LD name,
	// including the "." separators (≤ 61 characters).
	MaxFCDALength = 61
)

// SplitIndex splits a path component into its name and an optional
// array element index, IEC 61850-7-2:2010 §6.1.2.3 ("The "(…)" shall
// indicate an array element") and §12.3.3.2.3 ("FCDA(NumArrayElement)",
// e.g. "phsAHar(3)"). It returns ok false for a component without an
// index. A malformed index — empty "()", a non-decimal "(x)", more
// than one "(1)(2)", text after ")" or a "(" without ")" — is an error.
func SplitIndex(component string) (name string, index int, ok bool, err error) {
	open := strings.IndexByte(component, '(')
	if open < 0 {
		if strings.IndexByte(component, ')') >= 0 {
			return "", 0, false, errors.New("array index has ')' without '('")
		}
		return component, 0, false, nil
	}
	if open == 0 {
		return "", 0, false, errors.New("array index without a name")
	}
	if !strings.HasSuffix(component, ")") || strings.Count(component, "(") != 1 || strings.Count(component, ")") != 1 {
		return "", 0, false, fmt.Errorf("malformed array index in %q", component)
	}
	digits := component[open+1 : len(component)-1]
	if digits == "" {
		return "", 0, false, fmt.Errorf("empty array index in %q", component)
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return "", 0, false, fmt.Errorf("array index %q is not a decimal number", digits)
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return "", 0, false, fmt.Errorf("array index %q out of range", digits)
	}
	return component[:open], n, true, nil
}

// ValidateNames checks the naming conventions of IEC 61850-7-2:2010
// §22.2 that [Parse] leaves to the device, for references that must
// conform strictly (engineering tools, tests):
//
//   - names use the characters A-Z, a-z, 0-9 and "_" only
//   - the LD name starts with an alpha character
//   - the LN-Prefix starts with an alpha character, and prefix plus
//     instance number are at most [MaxLNPrefixInstance] characters
//   - the LN class is four upper-case letters
//   - the DataObjectName is at most [MaxDataObjectName] characters and
//     starts with an upper-case letter
//   - LN.DO.DA, without LD name and FC, is at most [MaxFCDALength]
//     characters
//
// The structural rules of [Ref.Validate] are checked first. Errors are
// [*model.ReferenceError] values.
func (r Ref) ValidateNames() error {
	if err := r.Validate(); err != nil {
		return err
	}
	fail := func(format string, args ...any) error {
		return &model.ReferenceError{Input: r.String(), Reason: fmt.Sprintf(format, args...)}
	}
	if err := checkCharset(r.LD); err != nil {
		return fail("logical device name: %v", err)
	}
	if !isAlpha(r.LD[0]) {
		return fail("logical device name %q must start with an alpha character", r.LD)
	}
	if r.LN == "" {
		return nil
	}
	n, err := ParseLNName(r.LN)
	if err != nil {
		return err
	}
	if n.Class != "LLN0" {
		if n.Prefix != "" && !isAlpha(n.Prefix[0]) {
			return fail("logical node prefix %q must start with an alpha character", n.Prefix)
		}
		if len(n.Prefix)+len(n.Inst) > MaxLNPrefixInstance {
			return fail("logical node prefix and instance %q+%q exceed %d characters", n.Prefix, n.Inst, MaxLNPrefixInstance)
		}
	}
	for i, p := range r.Path {
		name, _, _, _ := SplitIndex(p) // checked by Validate
		if err := checkCharset(name); err != nil {
			return fail("path component [%d]: %v", i, err)
		}
		if i == 0 {
			if len(name) > MaxDataObjectName {
				return fail("data object name %q exceeds %d characters", name, MaxDataObjectName)
			}
			if name[0] < 'A' || name[0] > 'Z' {
				return fail("data object name %q must start with an upper-case letter", name)
			}
		}
	}
	if len(r.Path) > 0 {
		if l := len(r.LN) + 1 + len(strings.Join(r.Path, ".")); l > MaxFCDALength {
			return fail("FCD/FCDA without LD name and FC exceeds %d characters (%d)", MaxFCDALength, l)
		}
	}
	return nil
}

func checkCharset(s string) error {
	for i := 0; i < len(s); i++ {
		if !isNameChar(rune(s[i])) {
			return fmt.Errorf("%q contains %q; 7-2 §22.2 allows A-Z, a-z, 0-9 and '_' only", s, s[i])
		}
	}
	return nil
}

func isAlpha(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
