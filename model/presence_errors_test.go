// SPDX-License-Identifier: MIT

package model_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
)

func TestPresenceError(t *testing.T) {
	for _, tc := range []struct {
		err  *model.PresenceError
		want string
	}{
		{&model.PresenceError{Class: "XCBR", Violations: []string{"Pos.stSeld: MOsbo: absent"}},
			"iec61850-model: XCBR: 1 presence violation: Pos.stSeld: MOsbo: absent"},
		{&model.PresenceError{Violations: []string{"a", "b"}}, "iec61850-model: -: 2 presence violations: a; b"},
	} {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
		if !errors.Is(tc.err, model.ErrPresenceViolation) || errors.Is(tc.err, model.ErrInvalidInstance) {
			t.Errorf("%v: wrong sentinel", tc.err)
		}
	}
	if !strings.HasPrefix(model.ErrInvalidInstance.Error(), "iec61850-model:") {
		t.Error("ErrInvalidInstance prefix")
	}
}
