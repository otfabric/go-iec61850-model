// SPDX-License-Identifier: MIT

package catalog

import (
	"fmt"

	"github.com/otfabric/go-iec61850-model/model"
)

// Namespace identity is part of a definition's identity: two classes
// named DBAT in IEC TR 61850-90-9 and IEC 61850-7-420 are different
// definitions. Three kinds of lookup follow from that:
//
//   - Unqualified lookups (LogicalNode, CDC, DataObjects, ...) return a
//     definition only when exactly one namespace of the set defines the
//     name, and fail with a *model.AmbiguityError otherwise. There is no
//     implicit precedence and definitions are never merged.
//   - Qualified lookups (LogicalNodeIn, DataObjectsIn, CDCIn, ...) return
//     the definition of the given namespace.
//   - References inside a definition (a base class, the CDC of a data
//     object, the type of an attribute) are resolved in the scope of the
//     namespace that declares them: that namespace first, then its
//     dependencies, breadth first. A name defined twice elsewhere does
//     not make such a reference ambiguous; a name the scope does not
//     define is looked up in the whole set.

// computeScopes returns, per namespace, its scope by level: the
// namespace itself, then the namespaces satisfying its dependencies,
// then theirs, and so on, each namespace at its first level.
func computeScopes(nss []*model.Namespace) map[model.NamespaceID][][]model.NamespaceID {
	out := make(map[model.NamespaceID][][]model.NamespaceID, len(nss))
	for _, ns := range nss {
		levels := [][]model.NamespaceID{{ns.ID}}
		seen := map[model.NamespaceID]bool{ns.ID: true}
		for {
			var next []model.NamespaceID
			for _, id := range levels[len(levels)-1] {
				cur := findNamespace(nss, id)
				for _, dep := range cur.Dependencies {
					for _, cand := range nss {
						if cand.ID.Satisfies(dep) && !seen[cand.ID] {
							seen[cand.ID] = true
							next = append(next, cand.ID)
						}
					}
				}
			}
			if len(next) == 0 {
				break
			}
			levels = append(levels, next)
		}
		out[ns.ID] = levels
	}
	return out
}

func findNamespace(nss []*model.Namespace, id model.NamespaceID) *model.Namespace {
	for _, ns := range nss {
		if ns.ID == id {
			return ns
		}
	}
	return nil
}

// Scope returns the namespaces a definition of namespace id sees: id
// itself, then its dependencies in the set, breadth first.
func (v *view) Scope(id model.NamespaceID) []model.NamespaceID {
	var out []model.NamespaceID
	for _, level := range v.scopes[id] {
		out = append(out, level...)
	}
	return out
}

// ResolveNamespace returns the namespace of the set that a possibly
// partial ID designates: the exact ID, or the only namespace matching
// the fields that are set ("IEC 61850-7-420" without a version). It
// returns [model.ErrUnknownNamespace] when none matches and an
// [model.AmbiguityError] when several releases do.
func (v *view) ResolveNamespace(id model.NamespaceID) (model.NamespaceID, error) {
	var found []model.NamespaceID
	for _, ns := range v.namespaces {
		switch {
		case ns.ID == id:
			return ns.ID, nil
		case ns.ID.Standard != id.Standard,
			id.Version != "" && ns.ID.Version != id.Version,
			id.Revision != "" && ns.ID.Revision != id.Revision,
			id.Release != 0 && ns.ID.Release != id.Release:
			continue
		}
		found = append(found, ns.ID)
	}
	switch len(found) {
	case 0:
		return model.NamespaceID{}, fmt.Errorf("%w: %s", model.ErrUnknownNamespace, id)
	case 1:
		return found[0], nil
	default:
		return model.NamespaceID{}, &model.AmbiguityError{Kind: "namespace", Name: id.String(), Candidates: found}
	}
}

// in returns the definitions of namespace id.
func in[T any](defs []*T, src func(*T) model.NamespaceID, id model.NamespaceID) []*T {
	var out []*T
	for _, d := range defs {
		if src(d) == id {
			out = append(out, d)
		}
	}
	return out
}

// fromScope returns the definitions of the nearest scope level that
// defines the name. Two namespaces at that level defining it (e.g. two
// releases satisfying one dependency) make the reference ambiguous.
// It returns nil, nil when the scope does not define the name.
func fromScope[T any](defs []*T, src func(*T) model.NamespaceID, levels [][]model.NamespaceID, kind, name string) ([]*T, error) {
	for _, level := range levels {
		var found []*T
		var ids []model.NamespaceID
		for _, id := range level {
			if d := in(defs, src, id); len(d) > 0 {
				found = append(found, d...)
				ids = append(ids, id)
			}
		}
		switch {
		case len(ids) > 1:
			return nil, &model.AmbiguityError{Kind: kind, Name: name, Candidates: ids}
		case len(ids) == 1:
			return found, nil
		}
	}
	return nil, nil
}

func notIn(kind, name string, id model.NamespaceID, sentinel error) error {
	return &model.ResolutionError{Namespace: id, Element: name, Reason: fmt.Sprintf("%s is not defined in %s", kind, id), Err: sentinel}
}

func lnSource(l *model.LogicalNodeClass) model.NamespaceID     { return l.Source }
func cdcSource(c *model.CommonDataClass) model.NamespaceID     { return c.Source }
func caSource(c *model.ConstructedAttribute) model.NamespaceID { return c.Source }
func enumSource(e *model.Enumeration) model.NamespaceID        { return e.Source }
func basicSource(b *model.BasicType) model.NamespaceID         { return b.Source }
func preferBase(defs []*model.LogicalNodeClass) []*model.LogicalNodeClass {
	var base []*model.LogicalNodeClass
	for _, ln := range defs {
		if !ln.IsExtension {
			base = append(base, ln)
		}
	}
	if len(base) == 0 {
		return defs
	}
	return base
}

// LogicalNodeIn returns the logical node class with the given name as
// namespace ns defines it. The ID may be partial ("IEC 61850-7-420").
func (v *view) LogicalNodeIn(ns model.NamespaceID, name string) (*model.LogicalNodeClass, error) {
	id, err := v.ResolveNamespace(ns)
	if err != nil {
		return nil, err
	}
	defs := preferBase(in(v.lns[name], lnSource, id))
	if len(defs) == 0 {
		return nil, notIn("logical node class", name, id, model.ErrUnknownLogicalNode)
	}
	return defs[0], nil
}

// logicalNodeFrom resolves a class referenced from namespace from (a
// base class) in its scope, then in the whole set.
func (v *view) logicalNodeFrom(from model.NamespaceID, name string) (*model.LogicalNodeClass, error) {
	var base []*model.LogicalNodeClass
	for _, ln := range v.lns[name] {
		if !ln.IsExtension {
			base = append(base, ln)
		}
	}
	defs, err := fromScope(base, lnSource, v.scopes[from], "logical node class", name)
	if err != nil {
		return nil, err
	}
	if len(defs) > 0 {
		return defs[0], nil
	}
	return v.LogicalNode(name)
}

// CDCIn returns the CDC with the given name as namespace ns defines it
// (its first variant for a CDC defined in variants).
func (v *view) CDCIn(ns model.NamespaceID, name string) (*model.CommonDataClass, error) {
	defs, err := v.CDCVariantsIn(ns, name)
	if err != nil {
		return nil, err
	}
	return defs[0], nil
}

// CDCVariantsIn returns every variant of the CDC with the given name as
// namespace ns defines it.
func (v *view) CDCVariantsIn(ns model.NamespaceID, name string) ([]*model.CommonDataClass, error) {
	id, err := v.ResolveNamespace(ns)
	if err != nil {
		return nil, err
	}
	defs := in(v.cdcs[name], cdcSource, id)
	if len(defs) == 0 {
		return nil, notIn("CDC", name, id, model.ErrUnknownCDC)
	}
	return defs, nil
}

// CDCVariantsFrom resolves a CDC referenced from namespace from (the CDC
// of a data object) in its scope, then in the whole set.
func (v *view) CDCVariantsFrom(from model.NamespaceID, name string) ([]*model.CommonDataClass, error) {
	defs, err := fromScope(v.cdcs[name], cdcSource, v.scopes[from], "CDC", name)
	if err != nil || len(defs) > 0 {
		return defs, err
	}
	return v.CDCVariants(name)
}

// EnumerationIn returns the enumeration with the given name as namespace
// ns defines it.
func (v *view) EnumerationIn(ns model.NamespaceID, name string) (*model.Enumeration, error) {
	return qualified(v, ns, v.enums[name], enumSource, "enumeration", name, model.ErrUnknownType)
}

// ConstructedAttributeIn returns the constructed attribute type with the
// given name as namespace ns defines it.
func (v *view) ConstructedAttributeIn(ns model.NamespaceID, name string) (*model.ConstructedAttribute, error) {
	return qualified(v, ns, v.cas[name], caSource, "constructed attribute", name, model.ErrUnknownType)
}

func qualified[T any](v *view, ns model.NamespaceID, defs []*T, src func(*T) model.NamespaceID, kind, name string, sentinel error) (*T, error) {
	id, err := v.ResolveNamespace(ns)
	if err != nil {
		return nil, err
	}
	found := in(defs, src, id)
	if len(found) == 0 {
		return nil, notIn(kind, name, id, sentinel)
	}
	return found[0], nil
}

// EnumerationFrom resolves an enumeration referenced from namespace from
// in its scope, then in the whole set.
func (v *view) EnumerationFrom(from model.NamespaceID, name string) (*model.Enumeration, error) {
	defs, err := fromScope(v.enums[name], enumSource, v.scopes[from], "enumeration", name)
	if err != nil {
		return nil, err
	}
	if len(defs) > 0 {
		return defs[0], nil
	}
	return v.Enumeration(name)
}

// ConstructedAttributeFrom resolves a constructed attribute type
// referenced from namespace from in its scope, then in the whole set.
func (v *view) ConstructedAttributeFrom(from model.NamespaceID, name string) (*model.ConstructedAttribute, error) {
	defs, err := fromScope(v.cas[name], caSource, v.scopes[from], "constructed attribute", name)
	if err != nil {
		return nil, err
	}
	if len(defs) > 0 {
		return defs[0], nil
	}
	return v.ConstructedAttribute(name)
}

// BasicTypeFrom resolves a basic type referenced from namespace from in
// its scope, then in the whole set.
func (v *view) BasicTypeFrom(from model.NamespaceID, name string) (*model.BasicType, error) {
	defs, err := fromScope(v.basics[name], basicSource, v.scopes[from], "basic type", name)
	if err != nil {
		return nil, err
	}
	if len(defs) > 0 {
		return defs[0], nil
	}
	return v.BasicType(name)
}

// InheritanceIn returns the class chain of the class as namespace ns
// defines it, base classes resolved in the scope of each class.
func (v *view) InheritanceIn(ns model.NamespaceID, name string) ([]*model.LogicalNodeClass, error) {
	ln, err := v.LogicalNodeIn(ns, name)
	if err != nil {
		return nil, err
	}
	return v.chainFrom(ln)
}

// DataObjectsIn returns every data object of the class as namespace ns
// defines it, including inherited ones and those of extension classes.
func (v *view) DataObjectsIn(ns model.NamespaceID, lnClass string) ([]InheritedDataObject, error) {
	chain, err := v.InheritanceIn(ns, lnClass)
	if err != nil {
		return nil, err
	}
	return v.dataObjectsOf(chain), nil
}

// DataObjectIn returns a data object of the class as namespace ns
// defines it, searching base classes and extension classes too.
func (v *view) DataObjectIn(ns model.NamespaceID, lnClass, name string) (InheritedDataObject, error) {
	chain, err := v.InheritanceIn(ns, lnClass)
	if err != nil {
		return InheritedDataObject{}, err
	}
	return v.dataObjectOf(chain, lnClass, name)
}

// chainFrom follows the base classes of ln, each resolved in the scope of
// the class that names it.
func (v *view) chainFrom(ln *model.LogicalNodeClass) ([]*model.LogicalNodeClass, error) {
	chain := []*model.LogicalNodeClass{ln}
	seen := map[*model.LogicalNodeClass]bool{ln: true}
	for cur := ln; cur.Base != ""; {
		base, err := v.logicalNodeFrom(cur.Source, cur.Base)
		if err != nil {
			return nil, err
		}
		if seen[base] {
			return nil, &model.ResolutionError{Element: ln.Name, Reason: "cyclic inheritance through " + base.Name, Err: model.ErrCycle}
		}
		seen[base] = true
		chain = append(chain, base)
		cur = base
	}
	return chain, nil
}
