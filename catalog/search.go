// SPDX-License-Identifier: MIT

package catalog

import (
	"slices"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Kind is the kind of a catalog element.
type Kind string

// Element kinds, in the order search results list them.
const (
	KindLogicalNode          Kind = "ln"
	KindDataObject           Kind = "do"
	KindCDC                  Kind = "cdc"
	KindDataAttribute        Kind = "da"
	KindConstructedAttribute Kind = "ca"
	KindEnumeration          Kind = "enum"
	KindBasicType            Kind = "basic"
)

var kindOrder = map[Kind]int{
	KindLogicalNode: 0, KindDataObject: 1, KindCDC: 2, KindDataAttribute: 3,
	KindConstructedAttribute: 4, KindEnumeration: 5, KindBasicType: 6,
}

// Kinds returns every element kind, in result order.
func Kinds() []Kind {
	return []Kind{KindLogicalNode, KindDataObject, KindCDC, KindDataAttribute, KindConstructedAttribute, KindEnumeration, KindBasicType}
}

// ParseKind parses a kind name as used by [Kind] values.
func ParseKind(s string) (Kind, bool) {
	k := Kind(strings.ToLower(strings.TrimSpace(s)))
	_, ok := kindOrder[k]
	return k, ok
}

// Query selects catalog elements. Empty fields do not filter. A filter
// that does not apply to a kind excludes that kind: with FC set, only
// data attributes are returned.
type Query struct {
	// Kinds restricts the result to these kinds.
	Kinds []Kind
	// Name matches element names case-insensitively as a substring, or
	// exactly (case-sensitive) when Exact is set.
	Name  string
	Exact bool
	// CDC restricts data objects to those of this CDC and logical node
	// classes to those declaring such a data object.
	CDC string
	// Type restricts data attributes and sub data attributes to those
	// of this type.
	Type string
	// FC restricts data attributes to those with this functional
	// constraint.
	FC model.FunctionalConstraint
	// Standard restricts the result to definitions of namespaces of
	// this standard, e.g. "IEC 61850-7-4".
	Standard string
	// Group restricts logical node classes, and their data objects, to
	// this group.
	Group model.LNGroup
	// IncludeAbstract includes abstract logical node classes and their
	// data objects.
	IncludeAbstract bool
}

// Result is one search hit.
type Result struct {
	Kind Kind `json:"kind"`
	// Name is the element name.
	Name string `json:"name"`
	// Path is the element path: the name for top-level definitions,
	// "MMXU.Hz" for a data object, "MV.mag" for a data attribute and
	// "Vector.mag" for a member of a constructed attribute.
	Path string `json:"path"`
	// Namespace is the namespace that defines the element.
	Namespace model.NamespaceID `json:"namespace"`
	// Type is the CDC of a data object or the type of an attribute.
	Type string `json:"type,omitempty"`
	// FC is the functional constraint of a data attribute.
	FC model.FunctionalConstraint `json:"fc,omitempty"`
	// Presence is the presence condition of a data object or attribute.
	Presence model.PresenceCondition `json:"presence,omitzero"`
	// Abstract marks abstract logical node classes and their data
	// objects.
	Abstract bool `json:"abstract,omitempty"`
}

// Search returns the elements matching q in a stable order: by kind,
// path and namespace. Data objects are listed where they are declared;
// inherited data objects are not repeated for each subclass.
func (v *view) Search(q Query) []Result {
	want := func(k Kind) bool { return len(q.Kinds) == 0 || slices.Contains(q.Kinds, k) }
	nameOK := func(n string) bool {
		if q.Name == "" {
			return true
		}
		if q.Exact {
			return n == q.Name
		}
		return strings.Contains(strings.ToLower(n), strings.ToLower(q.Name))
	}
	// Filters that do not apply to a kind exclude that kind.
	lnFilters := q.Type == "" && q.FC == ""
	cdcFilters := q.Group == 0 && q.Type == "" && q.FC == ""
	daFilters := q.Group == 0
	memberFilters := q.Group == 0 && q.CDC == "" && q.FC == ""
	typeFilters := q.Group == 0 && q.CDC == "" && q.Type == "" && q.FC == ""

	var out []Result
	for _, ns := range v.namespaces {
		if q.Standard != "" && ns.ID.Standard != q.Standard {
			continue
		}
		for _, lns := range [][]model.LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
			if !lnFilters {
				break
			}
			for i := range lns {
				ln := &lns[i]
				if ln.Abstract && !q.IncludeAbstract {
					continue
				}
				if q.Group != 0 && ln.Group() != q.Group {
					continue
				}
				if want(KindLogicalNode) && nameOK(ln.Name) && (q.CDC == "" || declaresCDC(ln, q.CDC)) {
					out = append(out, Result{Kind: KindLogicalNode, Name: ln.Name, Path: ln.Name, Namespace: ns.ID, Abstract: ln.Abstract})
				}
				if !want(KindDataObject) {
					continue
				}
				for j := range ln.DataObjects {
					do := &ln.DataObjects[j]
					if nameOK(do.Name) && (q.CDC == "" || do.Type == q.CDC) {
						out = append(out, Result{
							Kind: KindDataObject, Name: do.Name, Path: model.Path(ln.Name, do.Name), Namespace: ns.ID,
							Type: do.Type, Presence: do.Presence, Abstract: ln.Abstract,
						})
					}
				}
			}
		}
		attrOK := func(a *model.DataAttribute) bool {
			return nameOK(a.Name) && (q.Type == "" || a.Type == q.Type) && (q.FC == "" || a.FC == q.FC)
		}
		for i := range ns.CDCs {
			c := &ns.CDCs[i]
			if q.CDC != "" && c.Name != q.CDC {
				continue
			}
			if want(KindCDC) && cdcFilters && nameOK(c.Name) {
				out = append(out, Result{Kind: KindCDC, Name: c.Name, Path: c.Name, Namespace: ns.ID})
			}
			if !want(KindDataAttribute) || !daFilters {
				continue
			}
			for j := range c.DataAttributes {
				a := &c.DataAttributes[j]
				if attrOK(a) {
					out = append(out, Result{Kind: KindDataAttribute, Name: a.Name, Path: model.Path(c.Name, a.Name), Namespace: ns.ID, Type: a.Type, FC: a.FC, Presence: a.Presence})
				}
			}
		}
		for i := range ns.ConstructedAttributes {
			c := &ns.ConstructedAttributes[i]
			if want(KindConstructedAttribute) && typeFilters && nameOK(c.Name) {
				out = append(out, Result{Kind: KindConstructedAttribute, Name: c.Name, Path: c.Name, Namespace: ns.ID})
			}
			if !want(KindDataAttribute) || !memberFilters {
				continue
			}
			for j := range c.SubDataAttributes {
				a := &c.SubDataAttributes[j]
				if attrOK(a) {
					out = append(out, Result{Kind: KindDataAttribute, Name: a.Name, Path: model.Path(c.Name, a.Name), Namespace: ns.ID, Type: a.Type, Presence: a.Presence})
				}
			}
		}
		if !typeFilters {
			continue
		}
		for i := range ns.Enumerations {
			e := &ns.Enumerations[i]
			if want(KindEnumeration) && nameOK(e.Name) {
				out = append(out, Result{Kind: KindEnumeration, Name: e.Name, Path: e.Name, Namespace: ns.ID})
			}
		}
		for i := range ns.BasicTypes {
			b := &ns.BasicTypes[i]
			if want(KindBasicType) && nameOK(b.Name) {
				out = append(out, Result{Kind: KindBasicType, Name: b.Name, Path: b.Name, Namespace: ns.ID})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return kindOrder[a.Kind] < kindOrder[b.Kind]
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Namespace.Compare(b.Namespace) < 0
	})
	return out
}

func declaresCDC(ln *model.LogicalNodeClass, cdc string) bool {
	for i := range ln.DataObjects {
		if ln.DataObjects[i].Type == cdc {
			return true
		}
	}
	return false
}
