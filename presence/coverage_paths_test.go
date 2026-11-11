// SPDX-License-Identifier: MIT

package presence_test

import (
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/presence"
)

// A sibling that is no element of the class counts when the instance
// describes it or gives it a value.
func TestSibling_OutsideTheClass(t *testing.T) {
	e := els("a=MF(other)")
	check(t, e, &presence.Instance{Values: map[string]string{"other": "1"}}, map[string]want{"a": reqVio})
	check(t, e, &presence.Instance{Children: map[string]*presence.Instance{"other": {}}}, map[string]want{"a": reqVio})
	check(t, e, &presence.Instance{Present: []string{"other.sub"}}, map[string]want{"a": reqVio})
	check(t, e, present("unrelated"), map[string]want{"a": forSat})
}

// An instance number too long for an int is not range-checked.
func TestMultiInstance_HugeNumber(t *testing.T) {
	check(t, els("Chr=OmultiRange(1, 9)"), present("Chr"+strings.Repeat("9", 30)), map[string]want{"Chr": optSat})
}

// Facts of the logical node and of a child merge; the child's win.
func TestEvaluator_FactsMerge(t *testing.T) {
	rep, err := presence.New(defaultProfile(t)).LogicalNode("XCBR", &presence.Instance{
		Present: []string{"Pos"},
		Context: presence.Context{Facts: map[string]bool{"MFsubst": true, "MOcond(1)": true}},
		Children: map[string]*presence.Instance{
			"Pos": {Present: []string{"stVal"}, Context: presence.Context{Facts: map[string]bool{"MFsubst": false}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := byPath(rep)
	if r := got["Pos.subEna"]; r.Requirement != presence.Forbidden {
		t.Errorf("Pos.subEna: %+v", r)
	}
	if r := got["Mir"]; r.Requirement != presence.Required {
		t.Errorf("Mir: %+v", r)
	}
}

// Without an instance, a CDC with variants takes the first; a
// constructed type evaluates; a child of a basic member is not
// followed.
func TestEvaluator_Defaults(t *testing.T) {
	ev := presence.New(defaultProfile(t))
	rep, err := ev.CDC("SPG", "", nil)
	if err != nil || !strings.HasPrefix(rep.Class, "SPG") {
		t.Errorf("SPG: %v %v", rep, err)
	}
	if rep, err := ev.ConstructedAttribute("AnalogueValue", nil); err != nil || len(rep.Results) == 0 {
		t.Errorf("AnalogueValue: %v %v", rep, err)
	}
	rep, err = ev.ConstructedAttribute("AnalogueValue", &presence.Instance{Children: map[string]*presence.Instance{"f": {}}})
	if err != nil || len(rep.Children) != 0 || byPath(rep)["f"].Present != true {
		t.Errorf("AnalogueValue.f child: %+v %v", rep, err)
	}
}
