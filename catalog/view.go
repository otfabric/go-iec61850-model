// SPDX-License-Identifier: MIT

package catalog

import (
	"sort"

	"github.com/otfabric/go-iec61850-model/model"
)

// view indexes the definitions of a set of namespaces by name. Catalog
// and Profile embed it, so both offer the same lookups: a name defined
// by exactly one namespace of the set is returned, a name defined by
// several yields a [*model.AmbiguityError].
type view struct {
	namespaces []*model.Namespace
	scopes     map[model.NamespaceID][][]model.NamespaceID
	lns        map[string][]*model.LogicalNodeClass
	cdcs       map[string][]*model.CommonDataClass
	cas        map[string][]*model.ConstructedAttribute
	enums      map[string][]*model.Enumeration
	basics     map[string][]*model.BasicType
	fcs        map[model.FunctionalConstraint][]*model.FunctionalConstraintDef
	presence   map[string][]*model.PresenceConditionDef
}

func newView(nss []*model.Namespace) view {
	v := view{
		namespaces: nss,
		lns:        map[string][]*model.LogicalNodeClass{},
		cdcs:       map[string][]*model.CommonDataClass{},
		cas:        map[string][]*model.ConstructedAttribute{},
		enums:      map[string][]*model.Enumeration{},
		basics:     map[string][]*model.BasicType{},
		fcs:        map[model.FunctionalConstraint][]*model.FunctionalConstraintDef{},
		presence:   map[string][]*model.PresenceConditionDef{},
		scopes:     computeScopes(nss),
	}
	for _, ns := range nss {
		for i := range ns.AbstractLNClasses {
			ln := &ns.AbstractLNClasses[i]
			v.lns[ln.Name] = append(v.lns[ln.Name], ln)
		}
		for i := range ns.LNClasses {
			ln := &ns.LNClasses[i]
			v.lns[ln.Name] = append(v.lns[ln.Name], ln)
		}
		for i := range ns.CDCs {
			c := &ns.CDCs[i]
			v.cdcs[c.Name] = append(v.cdcs[c.Name], c)
		}
		for i := range ns.ConstructedAttributes {
			c := &ns.ConstructedAttributes[i]
			v.cas[c.Name] = append(v.cas[c.Name], c)
		}
		for i := range ns.Enumerations {
			e := &ns.Enumerations[i]
			v.enums[e.Name] = append(v.enums[e.Name], e)
		}
		for i := range ns.BasicTypes {
			b := &ns.BasicTypes[i]
			v.basics[b.Name] = append(v.basics[b.Name], b)
		}
		for i := range ns.FunctionalConstraints {
			f := &ns.FunctionalConstraints[i]
			v.fcs[f.Abbreviation] = append(v.fcs[f.Abbreviation], f)
		}
		for i := range ns.PresenceConditions {
			p := &ns.PresenceConditions[i]
			v.presence[p.Name] = append(v.presence[p.Name], p)
		}
	}
	return v
}

func lookup[T any](defs []*T, kind, name string, src func(*T) model.NamespaceID, notFound error) (*T, error) {
	switch len(defs) {
	case 0:
		return nil, &model.ResolutionError{Element: name, Reason: kind + " is not defined", Err: notFound}
	case 1:
		return defs[0], nil
	default:
		ids := make([]model.NamespaceID, len(defs))
		for i, d := range defs {
			ids[i] = src(d)
		}
		return nil, &model.AmbiguityError{Kind: kind, Name: name, Candidates: ids}
	}
}

// Namespaces returns the namespaces of the set, ordered by ID.
func (v *view) Namespaces() []*model.Namespace {
	return append([]*model.Namespace(nil), v.namespaces...)
}

// IDs returns the IDs of the namespaces of the set, in order.
func (v *view) IDs() []model.NamespaceID {
	ids := make([]model.NamespaceID, len(v.namespaces))
	for i, ns := range v.namespaces {
		ids[i] = ns.ID
	}
	return ids
}

// LogicalNode returns the concrete or abstract logical node class with
// the given name. It fails with [model.ErrUnknownLogicalNode] when no
// namespace defines it, and with a [*model.AmbiguityError] when more
// than one does.
//
// Extension classes (isExtension, e.g. MMXU in IEC 61850-7-420) add data
// objects to the class of the same name and are not candidates: the
// base definition is returned, and [view.Extensions] lists the
// extensions. An extension whose base class is not loaded is returned
// itself.
func (v *view) LogicalNode(name string) (*model.LogicalNodeClass, error) {
	defs := v.lns[name]
	var base []*model.LogicalNodeClass
	for _, ln := range defs {
		if !ln.IsExtension {
			base = append(base, ln)
		}
	}
	if len(base) == 0 {
		base = defs
	}
	return lookup(base, "logical node class", name, func(l *model.LogicalNodeClass) model.NamespaceID { return l.Source }, model.ErrUnknownLogicalNode)
}

// Extensions returns the extension classes of the class with the given
// name (isExtension), in namespace order. Their data objects belong to
// the class: [view.DataObjects] includes them.
func (v *view) Extensions(name string) []*model.LogicalNodeClass {
	var out []*model.LogicalNodeClass
	for _, ln := range v.lns[name] {
		if ln.IsExtension {
			out = append(out, ln)
		}
	}
	return out
}

// hasBase reports whether a non-extension definition of the class is
// loaded.
func (v *view) hasBase(name string) bool {
	for _, ln := range v.lns[name] {
		if !ln.IsExtension {
			return true
		}
	}
	return false
}

// CDC returns the common data class with the given name. A CDC defined
// in variants by one namespace (e.g. SPG for SP, SG and SE) is one CDC:
// CDC returns its first variant and [view.CDCVariants] all of them. A
// name defined by several namespaces is ambiguous.
func (v *view) CDC(name string) (*model.CommonDataClass, error) {
	defs, err := v.CDCVariants(name)
	if err != nil {
		return nil, err
	}
	return defs[0], nil
}

// CDCVariants returns every definition of the CDC with the given name:
// one, or one per variant, in document order.
func (v *view) CDCVariants(name string) ([]*model.CommonDataClass, error) {
	defs := v.cdcs[name]
	if len(defs) == 0 {
		return nil, &model.ResolutionError{Element: name, Reason: "CDC is not defined", Err: model.ErrUnknownCDC}
	}
	if ids := distinctSources(defs, func(c *model.CommonDataClass) model.NamespaceID { return c.Source }); len(ids) > 1 {
		return nil, &model.AmbiguityError{Kind: "CDC", Name: name, Candidates: ids}
	}
	return append([]*model.CommonDataClass(nil), defs...), nil
}

// distinctSources returns the distinct namespaces of definitions, in
// order of first appearance.
func distinctSources[T any](defs []*T, src func(*T) model.NamespaceID) []model.NamespaceID {
	var out []model.NamespaceID
	for _, d := range defs {
		id := src(d)
		if len(out) == 0 || !containsID(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func containsID(ids []model.NamespaceID, id model.NamespaceID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// ConstructedAttribute returns the constructed attribute type with the
// given name.
func (v *view) ConstructedAttribute(name string) (*model.ConstructedAttribute, error) {
	return lookup(v.cas[name], "constructed attribute", name, func(c *model.ConstructedAttribute) model.NamespaceID { return c.Source }, model.ErrUnknownType)
}

// Enumeration returns the enumeration with the given name.
func (v *view) Enumeration(name string) (*model.Enumeration, error) {
	return lookup(v.enums[name], "enumeration", name, func(e *model.Enumeration) model.NamespaceID { return e.Source }, model.ErrUnknownType)
}

// BasicType returns the basic type with the given name.
func (v *view) BasicType(name string) (*model.BasicType, error) {
	return lookup(v.basics[name], "basic type", name, func(b *model.BasicType) model.NamespaceID { return b.Source }, model.ErrUnknownType)
}

// FunctionalConstraint returns the definition of a functional
// constraint.
func (v *view) FunctionalConstraint(fc model.FunctionalConstraint) (*model.FunctionalConstraintDef, error) {
	return lookup(v.fcs[fc], "functional constraint", string(fc), func(f *model.FunctionalConstraintDef) model.NamespaceID { return f.Source }, model.ErrUnknownType)
}

// PresenceCondition returns the definition of a presence condition.
func (v *view) PresenceCondition(name string) (*model.PresenceConditionDef, error) {
	return lookup(v.presence[name], "presence condition", name, func(p *model.PresenceConditionDef) model.NamespaceID { return p.Source }, model.ErrUnknownType)
}

// ListLogicalNodes returns the concrete logical node classes, ordered
// by name and namespace. Extension classes are listed only when their
// base class is not loaded; see [view.Extensions].
func (v *view) ListLogicalNodes() []*model.LogicalNodeClass {
	return v.listLNs(false)
}

// ListAbstractLogicalNodes returns the abstract logical node classes,
// ordered by name and namespace.
func (v *view) ListAbstractLogicalNodes() []*model.LogicalNodeClass {
	return v.listLNs(true)
}

func (v *view) listLNs(abstract bool) []*model.LogicalNodeClass {
	var out []*model.LogicalNodeClass
	for _, defs := range v.lns {
		for _, ln := range defs {
			if ln.Abstract == abstract && (!ln.IsExtension || !v.hasBase(ln.Name)) {
				out = append(out, ln)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i].Name, out[i].Source, out[j].Name, out[j].Source) })
	return out
}

// ListCDCs returns the common data classes, every variant separately,
// ordered by name, namespace and variant.
func (v *view) ListCDCs() []*model.CommonDataClass {
	return list(v.cdcs, func(c *model.CommonDataClass) (string, model.NamespaceID) { return c.Key(), c.Source })
}

// ListConstructedAttributes returns the constructed attribute types,
// ordered by name and namespace.
func (v *view) ListConstructedAttributes() []*model.ConstructedAttribute {
	return list(v.cas, func(c *model.ConstructedAttribute) (string, model.NamespaceID) { return c.Name, c.Source })
}

// ListEnumerations returns the enumerations, ordered by name and
// namespace.
func (v *view) ListEnumerations() []*model.Enumeration {
	return list(v.enums, func(e *model.Enumeration) (string, model.NamespaceID) { return e.Name, e.Source })
}

// ListBasicTypes returns the basic types, ordered by name and
// namespace.
func (v *view) ListBasicTypes() []*model.BasicType {
	return list(v.basics, func(b *model.BasicType) (string, model.NamespaceID) { return b.Name, b.Source })
}

func list[T any](m map[string][]*T, key func(*T) (string, model.NamespaceID)) []*T {
	var out []*T
	for _, defs := range m {
		out = append(out, defs...)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, si := key(out[i])
		nj, sj := key(out[j])
		return less(ni, si, nj, sj)
	})
	return out
}

func less(a string, as model.NamespaceID, b string, bs model.NamespaceID) bool {
	if a != b {
		return a < b
	}
	return as.Compare(bs) < 0
}
