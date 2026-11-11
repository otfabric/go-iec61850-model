// SPDX-License-Identifier: MIT

package resolver

import "github.com/otfabric/go-iec61850-model/presence"

// Presence evaluates the presence conditions of a logical node instance
// (see package presence): which data objects, and, for the described
// children, which attributes the instance requires, allows or forbids,
// and whether it has them. It honours [Resolver.In]. Path resolution
// never evaluates presence; this is the opt-in check.
func (r *Resolver) Presence(lnClass string, inst *presence.Instance) (*presence.Report, error) {
	return r.presence().LogicalNode(lnClass, inst)
}

// ValidatePresence evaluates a logical node instance like
// [Resolver.Presence] and returns a [*model.PresenceError] when it
// violates a presence condition. Undetermined conditions do not fail.
func (r *Resolver) ValidatePresence(lnClass string, inst *presence.Instance) error {
	return r.presence().Validate(lnClass, inst)
}

func (r *Resolver) presence() *presence.Evaluator {
	ev := presence.New(r.l)
	if r.ns != nil {
		ev = ev.In(*r.ns)
	}
	return ev
}
