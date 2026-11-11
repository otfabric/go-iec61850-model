// SPDX-License-Identifier: MIT

package model

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors of the presence evaluation (package presence).
var (
	// ErrInvalidInstance indicates an instance description that cannot
	// be read: malformed JSON, unknown fields, or input beyond the
	// reader's limits.
	ErrInvalidInstance = errors.New("iec61850-model: invalid instance")

	// ErrPresenceViolation indicates an instance whose present elements
	// violate a presence condition of the model.
	ErrPresenceViolation = errors.New("iec61850-model: presence condition violated")
)

// PresenceError reports the presence conditions an instance violates.
//
// It unwraps to [ErrPresenceViolation].
type PresenceError struct {
	// Class is the logical node class, CDC or constructed type the
	// instance was checked against.
	Class string
	// Violations describe each violated condition, e.g.
	// "Pos.sboTimeout: MOsbo: required for ctlModel
	// sbo-with-enhanced-security, but absent".
	Violations []string
}

func (e *PresenceError) Error() string {
	n := len(e.Violations)
	noun := "violations"
	if n == 1 {
		noun = "violation"
	}
	return fmt.Sprintf("iec61850-model: %s: %d presence %s: %s", orDash(e.Class), n, noun, strings.Join(e.Violations, "; "))
}

func (e *PresenceError) Unwrap() error { return ErrPresenceViolation }
