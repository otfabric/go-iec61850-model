// SPDX-License-Identifier: MIT

package presence_test

import (
	"fmt"

	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/presence"
)

// Check a circuit switch controller whose position object uses
// select-before-operate with enhanced security but lacks the attributes
// that control model requires.
func ExampleEvaluator_LogicalNode() {
	p, err := official.Default()
	if err != nil {
		panic(err)
	}
	rep, err := presence.New(p).LogicalNode("CSWI", &presence.Instance{
		Present: []string{"Beh", "Pos"},
		Children: map[string]*presence.Instance{
			"Pos": {
				Present: []string{"stVal", "q", "t", "ctlModel"},
				Values:  map[string]string{"ctlModel": "sbo-with-enhanced-security"},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	for _, v := range rep.Violations() {
		fmt.Printf("%s %s: %s\n", v.Path, v.Condition, v.Requirement)
	}
	// Output:
	// Pos.stSeld MOsbo: required
	// Pos.sboTimeout MOsbo: required
	// Pos.operTimeout MOenhanced: required
}

// An undetermined condition names the fact it needs.
func ExampleEvaluate() {
	rep := presence.Evaluate("XSWI", []presence.Element{
		{Name: "Mir", Presence: cond("MOcond(1)")},
	}, nil)
	r := rep.Results[0]
	fmt.Println(r.Requirement, r.Verdict, r.Missing)
	// Output: undetermined undetermined fact MOcond(1)
}
