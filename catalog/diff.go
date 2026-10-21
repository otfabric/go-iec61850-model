// SPDX-License-Identifier: MIT

package catalog

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/otfabric/go-iec61850-model/model"
)

// ChangeKind classifies a difference.
type ChangeKind string

// Change kinds.
const (
	Added   ChangeKind = "added"
	Removed ChangeKind = "removed"
	Changed ChangeKind = "changed"
)

// Change is one structural difference between two model versions.
type Change struct {
	Kind ChangeKind `json:"kind"`
	// Element is the kind of element, e.g. "LNClass", "DataObject",
	// "DataAttribute", "Literal" or "Dependency".
	Element string `json:"element"`
	// Path locates the element, e.g. "MMXU.TotPF" or "MV.mag".
	Path string `json:"path"`
	// Field is the changed property for [Changed], e.g. "type".
	Field string `json:"field,omitempty"`
	// Old and New are the property values for [Changed].
	Old string `json:"old,omitempty"`
	New string `json:"new,omitempty"`
	// Description is a one-line summary.
	Description string `json:"description"`
}

// Diff compares the definitions of two sources, for instance two
// releases of a namespace or two profiles, and returns the structural
// differences in a stable order (by path, element and field).
//
// Definitions are matched by name, regardless of the namespace that
// defines them. Each side should define each name once, as a profile
// does; when a side defines a name more than once, the definition of
// the first namespace in ID order is compared.
func Diff(oldSrc, newSrc Source) []Change {
	o, n := collect(oldSrc), collect(newSrc)
	d := &differ{}

	d.deps(o.deps, n.deps)
	diffMap(d, "LNClass", o.lns, n.lns, d.lnClass)
	diffMap(d, "CDC", o.cdcs, n.cdcs, d.cdc)
	diffMap(d, "ConstructedAttribute", o.cas, n.cas, d.constructed)
	diffMap(d, "Enumeration", o.enums, n.enums, d.enumeration)
	diffMap(d, "BasicType", o.basics, n.basics, func(string, *model.BasicType, *model.BasicType) {})
	diffMap(d, "FunctionalConstraint", o.fcs, n.fcs, func(string, *model.FunctionalConstraintDef, *model.FunctionalConstraintDef) {})
	diffMap(d, "PresenceCondition", o.presence, n.presence, func(string, *model.PresenceConditionDef, *model.PresenceConditionDef) {})

	sort.SliceStable(d.out, func(i, j int) bool {
		a, b := d.out[i], d.out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Element != b.Element {
			return a.Element < b.Element
		}
		return a.Field < b.Field
	})
	return d.out
}

type defs struct {
	deps     map[string]model.NamespaceID // "<ns standard> -> <dep standard>"
	lns      map[string]*model.LogicalNodeClass
	cdcs     map[string]*model.CommonDataClass
	cas      map[string]*model.ConstructedAttribute
	enums    map[string]*model.Enumeration
	basics   map[string]*model.BasicType
	fcs      map[string]*model.FunctionalConstraintDef
	presence map[string]*model.PresenceConditionDef
}

func collect(src Source) defs {
	d := defs{
		deps: map[string]model.NamespaceID{}, lns: map[string]*model.LogicalNodeClass{},
		cdcs: map[string]*model.CommonDataClass{}, cas: map[string]*model.ConstructedAttribute{},
		enums: map[string]*model.Enumeration{}, basics: map[string]*model.BasicType{},
		fcs: map[string]*model.FunctionalConstraintDef{}, presence: map[string]*model.PresenceConditionDef{},
	}
	if src == nil {
		return d
	}
	nss := src.Namespaces()
	sort.SliceStable(nss, func(i, j int) bool { return nss[i].ID.Compare(nss[j].ID) < 0 })
	for _, ns := range nss {
		for _, dep := range ns.Dependencies {
			k := ns.ID.Standard + " -> " + dep.Standard
			if _, ok := d.deps[k]; !ok {
				d.deps[k] = dep
			}
		}
		addAll(d.lns, ns.AbstractLNClasses, func(l *model.LogicalNodeClass) string { return l.Name })
		addAll(d.lns, ns.LNClasses, func(l *model.LogicalNodeClass) string {
			if l.IsExtension {
				return l.Name + " (extension in " + ns.ID.Standard + ")"
			}
			return l.Name
		})
		addAll(d.cdcs, ns.CDCs, func(c *model.CommonDataClass) string { return c.Key() })
		addAll(d.cas, ns.ConstructedAttributes, func(c *model.ConstructedAttribute) string { return c.Name })
		addAll(d.enums, ns.Enumerations, func(e *model.Enumeration) string { return e.Name })
		addAll(d.basics, ns.BasicTypes, func(b *model.BasicType) string { return b.Name })
		addAll(d.fcs, ns.FunctionalConstraints, func(f *model.FunctionalConstraintDef) string { return string(f.Abbreviation) })
		addAll(d.presence, ns.PresenceConditions, func(p *model.PresenceConditionDef) string { return p.Name })
	}
	return d
}

func addAll[T any](m map[string]*T, items []T, key func(*T) string) {
	for i := range items {
		k := key(&items[i])
		if _, ok := m[k]; !ok {
			m[k] = &items[i]
		}
	}
}

type differ struct{ out []Change }

func (d *differ) add(kind ChangeKind, elem, path, desc string) {
	d.out = append(d.out, Change{Kind: kind, Element: elem, Path: path, Description: desc})
}

func (d *differ) field(elem, path, field, oldV, newV string) {
	if oldV == newV {
		return
	}
	d.out = append(d.out, Change{
		Kind: Changed, Element: elem, Path: path, Field: field, Old: oldV, New: newV,
		Description: fmt.Sprintf("%s %s: %s changed from %q to %q", elem, path, field, oldV, newV),
	})
}

func diffMap[T any](d *differ, elem string, o, n map[string]*T, changed func(string, *T, *T)) {
	for _, k := range sortedKeys(o, n) {
		a, b := o[k], n[k]
		switch {
		case a == nil:
			d.add(Added, elem, k, fmt.Sprintf("%s %s added", elem, k))
		case b == nil:
			d.add(Removed, elem, k, fmt.Sprintf("%s %s removed", elem, k))
		default:
			changed(k, a, b)
		}
	}
}

func sortedKeys[T any](o, n map[string]*T) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range []map[string]*T{o, n} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (d *differ) deps(o, n map[string]model.NamespaceID) {
	seen := map[string]bool{}
	var ks []string
	for _, m := range []map[string]model.NamespaceID{o, n} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				ks = append(ks, k)
			}
		}
	}
	sort.Strings(ks)
	for _, k := range ks {
		a, aok := o[k]
		b, bok := n[k]
		switch {
		case !aok:
			d.add(Added, "Dependency", k, fmt.Sprintf("dependency %s added (%s)", k, b))
		case !bok:
			d.add(Removed, "Dependency", k, fmt.Sprintf("dependency %s removed (was %s)", k, a))
		default:
			d.field("Dependency", k, "version", a.String(), b.String())
		}
	}
}

func (d *differ) lnClass(name string, a, b *model.LogicalNodeClass) {
	d.field("LNClass", name, "base", a.Base, b.Base)
	d.field("LNClass", name, "abstract", fmtBool(a.Abstract), fmtBool(b.Abstract))
	d.field("LNClass", name, "canHaveLOG", fmtBool(a.CanHaveLOG), fmtBool(b.CanHaveLOG))
	d.field("LNClass", name, "deprecated", fmtBool(a.Deprecated), fmtBool(b.Deprecated))
	d.dataObjects("DataObject", name, a.DataObjects, b.DataObjects)
}

func (d *differ) dataObjects(elem, parent string, a, b []model.DataObject) {
	om, nm := index(a, func(x *model.DataObject) string { return x.Name }), index(b, func(x *model.DataObject) string { return x.Name })
	diffMap(d, elem, prefixKeys(parent, om), prefixKeys(parent, nm), func(path string, x, y *model.DataObject) {
		d.field(elem, path, "type", x.Type, y.Type)
		d.field(elem, path, "underlyingType", x.UnderlyingType, y.UnderlyingType)
		d.field(elem, path, "presence", x.Presence.String(), y.Presence.String())
		d.field(elem, path, "dsPresence", x.DSPresence.String(), y.DSPresence.String())
		d.field(elem, path, "isArray", fmtBool(x.IsArray), fmtBool(y.IsArray))
		d.field(elem, path, "transient", fmtBool(x.Transient), fmtBool(y.Transient))
		d.field(elem, path, "deprecated", fmtBool(x.Deprecated), fmtBool(y.Deprecated))
	})
}

func (d *differ) attributes(elem, parent string, a, b []model.DataAttribute) {
	om, nm := index(a, func(x *model.DataAttribute) string { return x.Name }), index(b, func(x *model.DataAttribute) string { return x.Name })
	diffMap(d, elem, prefixKeys(parent, om), prefixKeys(parent, nm), func(path string, x, y *model.DataAttribute) {
		d.field(elem, path, "type", x.Type, y.Type)
		d.field(elem, path, "typeKind", string(x.TypeKind), string(y.TypeKind))
		d.field(elem, path, "fc", string(x.FC), string(y.FC))
		d.field(elem, path, "presence", x.Presence.String(), y.Presence.String())
		d.field(elem, path, "isArray", fmtBool(x.IsArray), fmtBool(y.IsArray))
		d.field(elem, path, "dchg", fmtBool(x.Dchg), fmtBool(y.Dchg))
		d.field(elem, path, "qchg", fmtBool(x.Qchg), fmtBool(y.Qchg))
		d.field(elem, path, "dupd", fmtBool(x.Dupd), fmtBool(y.Dupd))
		d.field(elem, path, "deprecated", fmtBool(x.Deprecated), fmtBool(y.Deprecated))
	})
}

func (d *differ) cdc(name string, a, b *model.CommonDataClass) {
	d.field("CDC", name, "enumParameterized", fmtBool(a.EnumParameterized), fmtBool(b.EnumParameterized))
	d.field("CDC", name, "typeKindParameterized", fmtBool(a.TypeKindParameterized), fmtBool(b.TypeKindParameterized))
	d.field("CDC", name, "deprecated", fmtBool(a.Deprecated), fmtBool(b.Deprecated))
	d.dataObjects("SubDataObject", name, a.SubDataObjects, b.SubDataObjects)
	d.attributes("DataAttribute", name, a.DataAttributes, b.DataAttributes)
}

func (d *differ) constructed(name string, a, b *model.ConstructedAttribute) {
	d.field("ConstructedAttribute", name, "deprecated", fmtBool(a.Deprecated), fmtBool(b.Deprecated))
	d.attributes("SubDataAttribute", name, a.SubDataAttributes, b.SubDataAttributes)
}

func (d *differ) enumeration(name string, a, b *model.Enumeration) {
	d.field("Enumeration", name, "inheritedFrom", a.InheritedFrom, b.InheritedFrom)
	d.field("Enumeration", name, "deprecated", fmtBool(a.Deprecated), fmtBool(b.Deprecated))
	om, nm := index(a.Literals, func(x *model.Literal) string { return x.Name }), index(b.Literals, func(x *model.Literal) string { return x.Name })
	diffMap(d, "Literal", prefixKeys(name, om), prefixKeys(name, nm), func(path string, x, y *model.Literal) {
		d.field("Literal", path, "value", strconv.Itoa(x.Value), strconv.Itoa(y.Value))
		d.field("Literal", path, "deprecated", fmtBool(x.Deprecated), fmtBool(y.Deprecated))
	})
}

func index[T any](items []T, key func(*T) string) map[string]*T {
	m := make(map[string]*T, len(items))
	for i := range items {
		k := key(&items[i])
		if _, ok := m[k]; !ok {
			m[k] = &items[i]
		}
	}
	return m
}

func prefixKeys[T any](parent string, m map[string]*T) map[string]*T {
	out := make(map[string]*T, len(m))
	for k, v := range m {
		out[model.Path(parent, k)] = v
	}
	return out
}

func fmtBool(b bool) string { return strconv.FormatBool(b) }
