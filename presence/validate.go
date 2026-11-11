// SPDX-License-Identifier: MIT

package presence

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Diagnostic codes of [ValidateNamespace].
const (
	CodeUnknownCondition   = "presence-unknown-condition"
	CodeMissingArgument    = "presence-missing-argument"
	CodeInvalidArgument    = "presence-invalid-argument"
	CodeUnexpectedArgument = "presence-unexpected-argument"
	CodeUnknownSibling     = "presence-unknown-sibling"
)

// ValidateNamespace checks the presence conditions a namespace uses on
// its data objects, attributes and members and returns warnings: a
// condition without an evaluator rule, a missing or malformed argument
// (group number, "min, max"), an argument on a condition that takes
// none, and a sibling argument that names no element of the class, CDC
// or type. Siblings inherited from a class of another namespace cannot
// be checked here and are not reported.
func ValidateNamespace(ns *model.Namespace) model.Diagnostics {
	if ns == nil {
		return nil
	}
	v := &nsValidator{ns: ns}
	lns := map[string]*model.LogicalNodeClass{}
	for _, list := range [][]model.LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
		for i := range list {
			lns[list[i].Name] = &list[i]
		}
	}
	for _, list := range []struct {
		path string
		lns  []model.LogicalNodeClass
	}{{"/NS/AbstractLNClasses", ns.AbstractLNClasses}, {"/NS/LNClasses", ns.LNClasses}} {
		for i := range list.lns {
			ln := &list.lns[i]
			names, complete := lnNames(ln, lns)
			p := fmt.Sprintf("%s/LNClass[%s]", list.path, ln.Name)
			for _, d := range ln.DataObjects {
				v.check(fmt.Sprintf("%s/DataObject[%s]", p, d.Name), d.Presence, names, complete)
			}
		}
	}
	for i := range ns.CDCs {
		c := &ns.CDCs[i]
		names := map[string]bool{}
		for _, s := range c.SubDataObjects {
			names[s.Name] = true
		}
		for _, a := range c.DataAttributes {
			names[a.BaseName()] = true
		}
		p := fmt.Sprintf("/NS/CDCs/CDC[%s]", c.Key())
		for _, s := range c.SubDataObjects {
			v.check(fmt.Sprintf("%s/SubDataObject[%s]", p, s.Name), s.Presence, names, true)
		}
		for _, a := range c.DataAttributes {
			v.check(fmt.Sprintf("%s/DataAttribute[%s]", p, a.Name), a.Presence, names, true)
		}
	}
	for i := range ns.ConstructedAttributes {
		ca := &ns.ConstructedAttributes[i]
		names := map[string]bool{}
		for _, a := range ca.SubDataAttributes {
			names[a.Name] = true
		}
		p := fmt.Sprintf("/NS/ConstructedAttributes/ConstructedAttribute[%s]", ca.Name)
		for _, a := range ca.SubDataAttributes {
			v.check(fmt.Sprintf("%s/SubDataAttribute[%s]", p, a.Name), a.Presence, names, true)
		}
	}
	return v.ds
}

// lnNames returns the data object names of a class and its bases in
// the namespace; complete is false when a base is defined elsewhere.
func lnNames(ln *model.LogicalNodeClass, lns map[string]*model.LogicalNodeClass) (map[string]bool, bool) {
	names := map[string]bool{}
	seen := map[string]bool{}
	for cur := ln; cur != nil && !seen[cur.Name]; {
		seen[cur.Name] = true
		for _, d := range cur.DataObjects {
			names[d.Name] = true
		}
		if cur.Base == "" {
			return names, !ln.IsExtension
		}
		cur = lns[cur.Base]
	}
	return names, false
}

type nsValidator struct {
	ns *model.Namespace
	ds model.Diagnostics
}

func (v *nsValidator) add(code, path, msg string) {
	v.ds = append(v.ds, model.Diagnostic{Severity: model.SeverityWarning, Code: code, Namespace: v.ns.ID, Path: path, Message: msg})
}

func (v *nsValidator) check(path string, p model.PresenceCondition, siblings map[string]bool, complete bool) {
	if p.IsZero() {
		return
	}
	r, ok := rules[p.Name]
	if !ok {
		v.add(CodeUnknownCondition, path, fmt.Sprintf("presence condition %s has no evaluator rule", p.Name))
		return
	}
	args := strings.TrimSpace(p.Args)
	switch r.Argument {
	case argNone:
		if args != "" {
			v.add(CodeUnexpectedArgument, path, fmt.Sprintf("%s takes no argument; %q is ignored", p.Name, args))
		}
	case argGroup:
		if n, err := strconv.Atoi(args); err != nil || n < 1 {
			v.add(CodeInvalidArgument, path, fmt.Sprintf("%s needs a group number, got %q", p.Name, args))
		}
	case argRange:
		if _, _, ok := ParseRange(args); !ok {
			v.add(CodeInvalidArgument, path, fmt.Sprintf("%s needs \"min, max\", got %q", p.Name, args))
		}
	case argSibling, argCondID:
		if args == "" {
			v.add(CodeMissingArgument, path, fmt.Sprintf("%s needs a %s argument", p.Name, r.Argument))
			return
		}
		if r.Argument == argSibling && complete && !siblings[args] {
			v.add(CodeUnknownSibling, path, fmt.Sprintf("%s names %s, which the class, CDC or type does not declare", p, args))
		}
	}
}
