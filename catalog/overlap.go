// SPDX-License-Identifier: MIT

package catalog

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/otfabric/go-iec61850-model/model"
)

// OverlapClass classifies two definitions of the same name.
type OverlapClass string

// Overlap classes.
const (
	// OverlapIdentical: the resolved structures are equal.
	OverlapIdentical OverlapClass = "identical"
	// OverlapCompatible: one definition extends the other; everything
	// the smaller one defines, the larger one defines the same way.
	OverlapCompatible OverlapClass = "compatible-extension"
	// OverlapConflicting: the definitions differ in what both define,
	// or each defines something the other does not.
	OverlapConflicting OverlapClass = "conflicting"
)

// Overlap is a name that two namespaces of the set both define, with
// the comparison of the two definitions.
type Overlap struct {
	// Kind is the kind of definition: logical node class, CDC,
	// constructed attribute or enumeration.
	Kind  Kind              `json:"kind"`
	Name  string            `json:"name"`
	Left  model.NamespaceID `json:"left"`
	Right model.NamespaceID `json:"right"`
	Class OverlapClass      `json:"class"`
	// OnlyLeft and OnlyRight list the members (data objects, data
	// attributes, literals) only one definition has.
	OnlyLeft  []string `json:"onlyLeft,omitempty"`
	OnlyRight []string `json:"onlyRight,omitempty"`
	// Differences describe members both have that differ, and other
	// differences such as the inheritance chain.
	Differences []string `json:"differences,omitempty"`
}

// Overlaps compares every name that both namespaces define. The IDs may
// be partial ("IEC 61850-7-420"). Zero IDs compare every pair of
// namespaces of the set. Logical node classes are compared with their
// resolved structure: inheritance chain and every data object with its
// CDC and presence, inherited ones included. Results are ordered by
// kind, name and namespaces.
func (v *view) Overlaps(left, right model.NamespaceID) ([]Overlap, error) {
	pairs, err := v.overlapPairs(left, right)
	if err != nil {
		return nil, err
	}
	var out []Overlap
	for _, pr := range pairs {
		a, b := pr[0], pr[1]
		for _, name := range keys(v.lns) {
			la, lb := preferBase(in(v.lns[name], lnSource, a)), preferBase(in(v.lns[name], lnSource, b))
			if len(la) == 0 || len(lb) == 0 || la[0].IsExtension || lb[0].IsExtension {
				continue
			}
			out = append(out, v.compareClasses(name, a, b))
		}
		for _, name := range keys(v.cdcs) {
			ca, cb := in(v.cdcs[name], cdcSource, a), in(v.cdcs[name], cdcSource, b)
			if len(ca) > 0 && len(cb) > 0 {
				out = append(out, compareMembers(KindCDC, name, a, b, cdcMembers(ca), cdcMembers(cb)))
			}
		}
		for _, name := range keys(v.cas) {
			ca, cb := in(v.cas[name], caSource, a), in(v.cas[name], caSource, b)
			if len(ca) > 0 && len(cb) > 0 {
				out = append(out, compareMembers(KindConstructedAttribute, name, a, b, attrMembers(ca[0].SubDataAttributes, ""), attrMembers(cb[0].SubDataAttributes, "")))
			}
		}
		for _, name := range keys(v.enums) {
			ea, eb := in(v.enums[name], enumSource, a), in(v.enums[name], enumSource, b)
			if len(ea) > 0 && len(eb) > 0 {
				out = append(out, compareMembers(KindEnumeration, name, a, b, literalMembers(ea[0]), literalMembers(eb[0])))
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if x.Kind != y.Kind {
			return kindOrder[x.Kind] < kindOrder[y.Kind]
		}
		if x.Name != y.Name {
			return x.Name < y.Name
		}
		if c := x.Left.Compare(y.Left); c != 0 {
			return c < 0
		}
		return x.Right.Compare(y.Right) < 0
	})
	return out, nil
}

func (v *view) overlapPairs(left, right model.NamespaceID) ([][2]model.NamespaceID, error) {
	if left.IsZero() != right.IsZero() {
		return nil, fmt.Errorf("%w: give both namespaces or neither", model.ErrUnknownNamespace)
	}
	if !left.IsZero() {
		a, err := v.ResolveNamespace(left)
		if err != nil {
			return nil, err
		}
		b, err := v.ResolveNamespace(right)
		if err != nil {
			return nil, err
		}
		return [][2]model.NamespaceID{{a, b}}, nil
	}
	var pairs [][2]model.NamespaceID
	for i := range v.namespaces {
		for j := i + 1; j < len(v.namespaces); j++ {
			pairs = append(pairs, [2]model.NamespaceID{v.namespaces[i].ID, v.namespaces[j].ID})
		}
	}
	return pairs, nil
}

// compareClasses compares the resolved structure of a class in two
// namespaces.
func (v *view) compareClasses(name string, a, b model.NamespaceID) Overlap {
	describe := func(id model.NamespaceID) (map[string]string, string, error) {
		chain, err := v.InheritanceIn(id, name)
		if err != nil {
			return nil, "", err
		}
		members := map[string]string{}
		for _, do := range v.dataObjectsOf(chain) {
			members[do.Name] = do.Type + " " + do.Presence.String()
		}
		var names []string
		for _, c := range chain {
			names = append(names, c.Name)
		}
		return members, fmt.Sprint(names), nil
	}
	ma, chainA, errA := describe(a)
	mb, chainB, errB := describe(b)
	o := compareMembers(KindLogicalNode, name, a, b, ma, mb)
	switch {
	case errA != nil || errB != nil:
		o.Class = OverlapConflicting
		for _, e := range []error{errA, errB} {
			if e != nil {
				o.Differences = append(o.Differences, "cannot resolve: "+e.Error())
			}
		}
	case chainA != chainB:
		o.Class = OverlapConflicting
		o.Differences = append(o.Differences, fmt.Sprintf("inheritance %s vs %s", chainA, chainB))
	}
	return o
}

// compareMembers classifies two member maps (name → description).
func compareMembers(kind Kind, name string, a, b model.NamespaceID, ma, mb map[string]string) Overlap {
	o := Overlap{Kind: kind, Name: name, Left: a, Right: b}
	for _, k := range sortedNames(ma) {
		vb, ok := mb[k]
		switch {
		case !ok:
			o.OnlyLeft = append(o.OnlyLeft, k)
		case vb != ma[k]:
			o.Differences = append(o.Differences, fmt.Sprintf("%s: %s vs %s", k, ma[k], vb))
		}
	}
	for _, k := range sortedNames(mb) {
		if _, ok := ma[k]; !ok {
			o.OnlyRight = append(o.OnlyRight, k)
		}
	}
	switch {
	case len(o.Differences) > 0 || (len(o.OnlyLeft) > 0 && len(o.OnlyRight) > 0):
		o.Class = OverlapConflicting
	case len(o.OnlyLeft) > 0 || len(o.OnlyRight) > 0:
		o.Class = OverlapCompatible
	default:
		o.Class = OverlapIdentical
	}
	return o
}

func sortedNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cdcMembers(variants []*model.CommonDataClass) map[string]string {
	out := map[string]string{}
	for _, c := range variants {
		prefix := ""
		if c.Variant != "" {
			prefix = c.Variant + "/"
		}
		for _, s := range c.SubDataObjects {
			out[prefix+s.Name] = s.Type + " " + s.Presence.String()
		}
		for k, v := range attrMembers(c.DataAttributes, prefix) {
			out[k] = v
		}
	}
	return out
}

func attrMembers(attrs []model.DataAttribute, prefix string) map[string]string {
	out := map[string]string{}
	for _, a := range attrs {
		out[prefix+a.Name] = fmt.Sprintf("%s %s %s %s", a.Type, a.TypeKind, a.FC, a.Presence.String())
	}
	return out
}

func literalMembers(e *model.Enumeration) map[string]string {
	out := map[string]string{}
	for _, l := range e.Literals {
		out[l.Name] = strconv.Itoa(l.Value)
	}
	return out
}
