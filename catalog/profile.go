// SPDX-License-Identifier: MIT

package catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Profile is a consistent selection of namespace releases: at most one
// release per standard, with every declared dependency satisfied by a
// release in the profile.
//
// A Profile offers the same lookups as a [Catalog], restricted to its
// namespaces, plus inheritance-aware data object lookups. It is safe
// for concurrent use.
type Profile struct {
	view
	diags model.Diagnostics
}

// Profile builds a profile from the selected namespaces and their
// dependencies.
//
// Each ID may be partial: "IEC 61850-7-4" with no version selects the
// only loaded release of that standard, and fails with a
// [*model.AmbiguityError] when several are loaded. Dependencies that
// are not selected are added when exactly one loaded release satisfies
// them; otherwise the call fails with a [*model.DependencyError]
// (missing, ambiguous, or conflicting with a selected release).
//
// Problems that do not prevent lookups, such as a name defined by two
// namespaces of the profile or a type reference that resolves to no
// definition, are reported by [Profile.Diagnostics].
func (c *Catalog) Profile(ids ...model.NamespaceID) (*Profile, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no namespace selected", model.ErrUnknownNamespace)
	}
	selected := map[string]*model.Namespace{} // by standard
	var queue []*model.Namespace
	for _, id := range ids {
		ns, err := c.match(id)
		if err != nil {
			return nil, err
		}
		if prev, ok := selected[ns.ID.Standard]; ok {
			if prev == ns {
				continue
			}
			return nil, fmt.Errorf("%w: %s and %s selected together", model.ErrNamespaceConflict, prev.ID, ns.ID)
		}
		selected[ns.ID.Standard] = ns
		queue = append(queue, ns)
	}
	for len(queue) > 0 {
		ns := queue[0]
		queue = queue[1:]
		for _, dep := range ns.Dependencies {
			if have, ok := selected[dep.Standard]; ok {
				if !have.ID.Satisfies(dep) {
					return nil, &model.DependencyError{
						Namespace: ns.ID, Required: dep, Candidates: []model.NamespaceID{have.ID},
						Reason: fmt.Sprintf("the profile selects %s, which does not satisfy it", have.ID),
						Err:    model.ErrNamespaceConflict,
					}
				}
				continue
			}
			var cands []*model.Namespace
			for _, cand := range c.namespaces {
				if cand.ID.Satisfies(dep) {
					cands = append(cands, cand)
				}
			}
			switch len(cands) {
			case 0:
				return nil, &model.DependencyError{Namespace: ns.ID, Required: dep, Err: model.ErrMissingDependency}
			case 1:
				selected[dep.Standard] = cands[0]
				queue = append(queue, cands[0])
			default:
				cids := make([]model.NamespaceID, len(cands))
				for i, cand := range cands {
					cids[i] = cand.ID
				}
				return nil, &model.DependencyError{Namespace: ns.ID, Required: dep, Candidates: cids, Err: model.ErrAmbiguousReference}
			}
		}
	}
	nss := make([]*model.Namespace, 0, len(selected))
	for _, ns := range selected {
		nss = append(nss, ns)
	}
	sort.Slice(nss, func(i, j int) bool { return nss[i].ID.Compare(nss[j].ID) < 0 })
	p := &Profile{view: newView(nss)}
	p.diags = p.check()
	return p, nil
}

// Diagnostics returns the findings of the profile check: names defined
// by more than one namespace (warnings; lookups of those names are
// ambiguous) and references to types, classes, functional constraints
// or presence conditions that the profile does not define (errors).
func (p *Profile) Diagnostics() model.Diagnostics { return append(model.Diagnostics(nil), p.diags...) }

// String returns the IDs of the profile, separated by ", ".
func (p *Profile) String() string {
	ids := make([]string, len(p.namespaces))
	for i, ns := range p.namespaces {
		ids[i] = ns.ID.String()
	}
	return strings.Join(ids, ", ")
}

// Inheritance returns the class chain of a logical node class, starting
// with the class itself and ending with its root class. It fails with
// [model.ErrCycle] on cyclic inheritance and with
// [model.ErrUnknownLogicalNode] when a base class is not defined.
func (v *view) Inheritance(name string) ([]*model.LogicalNodeClass, error) {
	var chain []*model.LogicalNodeClass
	seen := map[string]bool{}
	for cur := name; cur != ""; {
		if seen[cur] {
			return nil, &model.ResolutionError{Element: name, Reason: "cyclic inheritance through " + cur, Err: model.ErrCycle}
		}
		seen[cur] = true
		ln, err := v.LogicalNode(cur)
		if err != nil {
			return nil, err
		}
		chain = append(chain, ln)
		cur = ln.Base
	}
	return chain, nil
}

// InheritedDataObject is a data object of a logical node class together
// with the class that declares it.
type InheritedDataObject struct {
	*model.DataObject
	// DeclaredIn is the class that declares the data object: the class
	// itself or one of its base classes.
	DeclaredIn *model.LogicalNodeClass
}

// DataObjects returns every data object of a logical node class,
// including inherited ones: those of the root class first, then each
// subclass in turn, each in document order. A data object redeclared
// by a subclass replaces the inherited one at its original position.
func (v *view) DataObjects(lnClass string) ([]InheritedDataObject, error) {
	chain, err := v.Inheritance(lnClass)
	if err != nil {
		return nil, err
	}
	var out []InheritedDataObject
	pos := map[string]int{}
	for i := len(chain) - 1; i >= 0; i-- {
		ln := chain[i]
		for j := range ln.DataObjects {
			do := InheritedDataObject{DataObject: &ln.DataObjects[j], DeclaredIn: ln}
			if k, ok := pos[do.Name]; ok {
				out[k] = do
				continue
			}
			pos[do.Name] = len(out)
			out = append(out, do)
		}
	}
	return out, nil
}

// DataObject returns the data object of a logical node class with the
// given name, searching base classes too. It fails with
// [model.ErrUnknownDataObject] when the class has no such data object.
func (v *view) DataObject(lnClass, name string) (InheritedDataObject, error) {
	chain, err := v.Inheritance(lnClass)
	if err != nil {
		return InheritedDataObject{}, err
	}
	for _, ln := range chain {
		if do, ok := ln.DataObject(name); ok {
			return InheritedDataObject{DataObject: do, DeclaredIn: ln}, nil
		}
	}
	return InheritedDataObject{}, &model.ResolutionError{
		Namespace: chain[0].Source, Path: model.Path(lnClass, name), Element: name,
		Reason: fmt.Sprintf("class %s has no data object %s", lnClass, name), Err: model.ErrUnknownDataObject,
	}
}

// check reports conflicting definitions and unresolved references.
func (p *Profile) check() model.Diagnostics {
	var ds model.Diagnostics
	conflicts := func(kind string, names []string, ids func(string) []model.NamespaceID) {
		sort.Strings(names)
		for _, n := range names {
			srcs := ids(n)
			if len(srcs) < 2 {
				continue
			}
			s := make([]string, len(srcs))
			for i, id := range srcs {
				s[i] = id.String()
			}
			ds = append(ds, model.Diagnostic{
				Severity: model.SeverityWarning, Code: "conflicting-definition", Namespace: srcs[0], Path: n,
				Message: fmt.Sprintf("%s %q is defined by %s; lookups of it are ambiguous", kind, n, strings.Join(s, ", ")),
			})
		}
	}
	conflicts("logical node class", keys(p.lns), func(n string) []model.NamespaceID {
		return sources(p.lns[n], func(l *model.LogicalNodeClass) model.NamespaceID { return l.Source })
	})
	conflicts("CDC", keys(p.cdcs), func(n string) []model.NamespaceID {
		return sources(p.cdcs[n], func(c *model.CommonDataClass) model.NamespaceID { return c.Source })
	})
	conflicts("constructed attribute", keys(p.cas), func(n string) []model.NamespaceID {
		return sources(p.cas[n], func(c *model.ConstructedAttribute) model.NamespaceID { return c.Source })
	})
	conflicts("enumeration", keys(p.enums), func(n string) []model.NamespaceID {
		return sources(p.enums[n], func(e *model.Enumeration) model.NamespaceID { return e.Source })
	})
	conflicts("basic type", keys(p.basics), func(n string) []model.NamespaceID {
		return sources(p.basics[n], func(b *model.BasicType) model.NamespaceID { return b.Source })
	})

	unresolved := func(ns model.NamespaceID, path, msg string) {
		ds = append(ds, model.Diagnostic{Severity: model.SeverityError, Code: "unresolved-reference", Namespace: ns, Path: path, Message: msg})
	}
	hasFCs, hasPresence := len(p.fcs) > 0, len(p.presence) > 0
	checkPresence := func(ns model.NamespaceID, path string, pc model.PresenceCondition) {
		if hasPresence && pc.Name != "" && len(p.presence[pc.Name]) == 0 {
			unresolved(ns, path, fmt.Sprintf("presence condition %q is not defined", pc.Name))
		}
	}
	checkDO := func(ns model.NamespaceID, path string, do *model.DataObject) {
		if len(p.cdcs[do.Type]) == 0 {
			unresolved(ns, path, fmt.Sprintf("CDC %q is not defined", do.Type))
		}
		if do.UnderlyingType != "" && !p.typeDefined(do.UnderlyingType) {
			unresolved(ns, path, fmt.Sprintf("underlying type %q is not defined", do.UnderlyingType))
		}
		checkPresence(ns, path, do.Presence)
	}
	checkDA := func(ns model.NamespaceID, path string, da *model.DataAttribute) {
		if da.Type != "" && !p.typeDefined(da.Type) {
			unresolved(ns, path, fmt.Sprintf("type %q is not defined", da.Type))
		}
		if hasFCs && da.FC != "" && len(p.fcs[da.FC]) == 0 {
			unresolved(ns, path, fmt.Sprintf("functional constraint %q is not defined", da.FC))
		}
		checkPresence(ns, path, da.Presence)
	}
	for _, ns := range p.namespaces {
		for _, lns := range [][]model.LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
			for i := range lns {
				ln := &lns[i]
				if ln.Base != "" && len(p.lns[ln.Base]) == 0 {
					unresolved(ns.ID, ln.Name, fmt.Sprintf("base class %q is not defined", ln.Base))
				}
				for j := range ln.DataObjects {
					checkDO(ns.ID, model.Path(ln.Name, ln.DataObjects[j].Name), &ln.DataObjects[j])
				}
			}
		}
		for i := range ns.CDCs {
			c := &ns.CDCs[i]
			for j := range c.SubDataObjects {
				checkDO(ns.ID, model.Path(c.Name, c.SubDataObjects[j].Name), &c.SubDataObjects[j])
			}
			for j := range c.DataAttributes {
				checkDA(ns.ID, model.Path(c.Name, c.DataAttributes[j].Name), &c.DataAttributes[j])
			}
		}
		for i := range ns.ConstructedAttributes {
			c := &ns.ConstructedAttributes[i]
			for j := range c.SubDataAttributes {
				checkDA(ns.ID, model.Path(c.Name, c.SubDataAttributes[j].Name), &c.SubDataAttributes[j])
			}
		}
		for i := range ns.Enumerations {
			e := &ns.Enumerations[i]
			if e.InheritedFrom != "" && len(p.enums[e.InheritedFrom]) == 0 {
				unresolved(ns.ID, e.Name, fmt.Sprintf("parent enumeration %q is not defined", e.InheritedFrom))
			}
		}
	}
	return ds
}

func (p *Profile) typeDefined(name string) bool {
	return len(p.basics[name]) > 0 || len(p.cas[name]) > 0 || len(p.enums[name]) > 0 || len(p.cdcs[name]) > 0
}

func keys[T any](m map[string][]*T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sources[T any](defs []*T, src func(*T) model.NamespaceID) []model.NamespaceID {
	out := make([]model.NamespaceID, len(defs))
	for i, d := range defs {
		out[i] = src(d)
	}
	return out
}
