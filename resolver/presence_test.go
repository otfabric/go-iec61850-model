// SPDX-License-Identifier: MIT

package resolver_test

import (
	"errors"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/presence"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// Presence evaluation is opt-in: Resolve never checks it, Presence and
// ValidatePresence do.
func TestResolverPresence(t *testing.T) {
	p, err := official.Default()
	if err != nil {
		t.Fatal(err)
	}
	r := resolver.New(p)
	inst := &presence.Instance{Present: []string{"Beh", "Pos"}, Children: map[string]*presence.Instance{
		"Pos": {Present: []string{"stVal", "q", "t", "ctlModel"}, Values: map[string]string{"ctlModel": "sbo-with-normal-security"}},
	}}
	rep, err := r.Presence("XCBR", inst)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Class != "XCBR" || rep.OK() {
		t.Errorf("report: %+v", rep)
	}
	if err := r.ValidatePresence("XCBR", inst); !errors.Is(err, model.ErrPresenceViolation) {
		t.Errorf("ValidatePresence = %v", err)
	}
	inst.Children["Pos"].Values["ctlModel"] = "direct-with-normal-security"
	inst.Present = append(inst.Present, "Loc", "EEHealth", "OpCnt", "CBOpCap", "BlkOpn", "BlkCls")
	if err := r.ValidatePresence("XCBR", inst); err != nil {
		t.Errorf("valid XCBR: %v", err)
	}
	ns, _ := model.ParseNamespaceID("IEC 99999")
	if _, err := r.In(ns).Presence("XCBR", nil); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("In(unloaded): %v", err)
	}
}
