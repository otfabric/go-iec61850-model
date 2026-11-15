// SPDX-License-Identifier: MIT

package nsd

import (
	"fmt"

	"github.com/otfabric/go-iec61850-model/model"
)

// Validate checks the structure of one namespace and returns its
// findings: duplicate definitions, duplicate members and literals,
// self-inheritance and dependencies on itself.
//
// References to types of other namespaces cannot be checked here; the
// catalog package checks them when it builds a profile. Parse runs
// Validate on every NS document it reads.
func Validate(ns *model.Namespace) model.Diagnostics {
	ds, _ := validate(ns, -1)
	return ds
}

// validate is Validate with a bound on the findings it keeps: limit,
// or none when limit is negative. It also returns the number of
// findings beyond the limit, which are not kept.
func validate(ns *model.Namespace, limit int) (model.Diagnostics, int) {
	if ns == nil {
		return model.Diagnostics{{Severity: model.SeverityError, Code: "nil-namespace", Message: "namespace is nil"}}, 0
	}
	v := &validator{ns: ns, limit: limit}

	for _, dep := range ns.Dependencies {
		if dep.Standard == ns.ID.Standard {
			v.add("self-dependency", elemPath("/NS", "DependsOn", dep.Standard), "namespace depends on itself")
		}
	}

	v.unique("/NS/BasicTypes", "BasicType", len(ns.BasicTypes), func(i int) string { return ns.BasicTypes[i].Name })
	v.unique("/NS/FunctionalConstraints", "FunctionalConstraint", len(ns.FunctionalConstraints), func(i int) string {
		return string(ns.FunctionalConstraints[i].Abbreviation)
	})
	v.unique("/NS/TriggerOptions", "TriggerOption", len(ns.TriggerOptions), func(i int) string { return ns.TriggerOptions[i].Abbreviation })
	v.unique("/NS/PresenceConditions", "PresenceCondition", len(ns.PresenceConditions), func(i int) string { return ns.PresenceConditions[i].Name })
	v.unique("/NS/Abbreviations", "Abbreviation", len(ns.Abbreviations), func(i int) string { return ns.Abbreviations[i].Name })
	v.unique("/NS/Enumerations", "Enumeration", len(ns.Enumerations), func(i int) string { return ns.Enumerations[i].Name })
	v.unique("/NS/ConstructedAttributes", "ConstructedAttribute", len(ns.ConstructedAttributes), func(i int) string {
		return ns.ConstructedAttributes[i].Name
	})
	v.unique("/NS/CDCs", "CDC", len(ns.CDCs), func(i int) string { return ns.CDCs[i].Key() })
	// The definitions per CDC name, counted in one pass.
	variants := make(map[string]int, len(ns.CDCs))
	for i := range ns.CDCs {
		variants[ns.CDCs[i].Name]++
	}
	for i := range ns.CDCs {
		c := &ns.CDCs[i]
		if variants[c.Name] > 1 && c.Variant == "" {
			v.add("missing-variant", elemPath("/NS/CDCs", "CDC", c.Name), "CDC is defined more than once and this definition has no variant")
		}
	}

	// Abstract and concrete classes share one name space.
	lns := make([]*model.LogicalNodeClass, 0, len(ns.AbstractLNClasses)+len(ns.LNClasses))
	for i := range ns.AbstractLNClasses {
		lns = append(lns, &ns.AbstractLNClasses[i])
	}
	for i := range ns.LNClasses {
		lns = append(lns, &ns.LNClasses[i])
	}
	v.unique("/NS/LNClasses", "LNClass", len(lns), func(i int) string { return lns[i].Name })

	for i := range ns.Enumerations {
		e := &ns.Enumerations[i]
		p := elemPath("/NS/Enumerations", "Enumeration", e.Name)
		v.unique(p, "Literal", len(e.Literals), func(i int) string { return e.Literals[i].Name })
		seen := map[int]string{}
		for _, l := range e.Literals {
			if prev, ok := seen[l.Value]; ok {
				v.add("duplicate-literal-value", elemPath(p, "Literal", l.Name), "value %d is also used by literal %q", l.Value, clip(prev))
				continue
			}
			seen[l.Value] = l.Name
		}
		if e.InheritedFrom == e.Name && e.Name != "" {
			v.add("self-inheritance", p, "enumeration inherits from itself")
		}
	}
	for i := range ns.ConstructedAttributes {
		c := &ns.ConstructedAttributes[i]
		p := elemPath("/NS/ConstructedAttributes", "ConstructedAttribute", c.Name)
		v.unique(p, "SubDataAttribute", len(c.SubDataAttributes), func(i int) string { return c.SubDataAttributes[i].Name })
	}
	for i := range ns.CDCs {
		c := &ns.CDCs[i]
		p := elemPath("/NS/CDCs", "CDC", c.Key())
		names := make([]string, 0, len(c.SubDataObjects)+len(c.DataAttributes))
		for _, d := range c.SubDataObjects {
			names = append(names, d.Name)
		}
		for _, a := range c.DataAttributes {
			names = append(names, a.Name)
		}
		v.unique(p, "member", len(names), func(i int) string { return names[i] })
	}
	for _, ln := range lns {
		elem := "LNClass"
		parent := "/NS/LNClasses"
		if ln.Abstract {
			elem, parent = "AbstractLNClass", "/NS/AbstractLNClasses"
		}
		p := elemPath(parent, elem, ln.Name)
		v.unique(p, "DataObject", len(ln.DataObjects), func(i int) string { return ln.DataObjects[i].Name })
		if ln.Base == ln.Name && ln.Name != "" {
			v.add("self-inheritance", p, "class inherits from itself")
		}
	}
	return v.diags, v.dropped
}

type validator struct {
	ns    *model.Namespace
	diags model.Diagnostics
	// limit bounds len(diags) when it is not negative; dropped counts
	// the findings beyond it.
	limit   int
	dropped int
}

// full reports whether the validator keeps no further finding.
func (v *validator) full() bool { return v.limit >= 0 && len(v.diags) >= v.limit }

// add records a finding. The message is formatted only when the
// finding is kept.
func (v *validator) add(code, path, format string, args ...any) {
	if v.full() {
		v.dropped++
		return
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	v.diags = append(v.diags, model.Diagnostic{Severity: model.SeverityError, Code: code, Namespace: v.ns.ID, Path: path, Message: msg})
}

func (v *validator) unique(parent, elem string, n int, name func(int) string) {
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		nm := name(i)
		if nm == "" {
			continue
		}
		if seen[nm] {
			if v.full() {
				v.dropped++
				continue
			}
			v.add("duplicate-definition", elemPath(parent, elem, nm), "%s %q is defined more than once", elem, clip(nm))
			continue
		}
		seen[nm] = true
	}
}
