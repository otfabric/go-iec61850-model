// SPDX-License-Identifier: MIT

package presence

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
)

// ModelLookup is the set of model lookups the [Evaluator] needs.
// *catalog.Profile and *catalog.Catalog implement it, and so does every
// resolver.Lookup.
type ModelLookup interface {
	LogicalNode(name string) (*model.LogicalNodeClass, error)
	DataObjects(lnClass string) ([]catalog.InheritedDataObject, error)
	LogicalNodeIn(ns model.NamespaceID, name string) (*model.LogicalNodeClass, error)
	DataObjectsIn(ns model.NamespaceID, lnClass string) ([]catalog.InheritedDataObject, error)
	CDCVariants(name string) ([]*model.CommonDataClass, error)
	CDCVariantsFrom(from model.NamespaceID, name string) ([]*model.CommonDataClass, error)
	ConstructedAttribute(name string) (*model.ConstructedAttribute, error)
	ConstructedAttributeFrom(from model.NamespaceID, name string) (*model.ConstructedAttribute, error)
}

// MaxDepth bounds the nesting of instance children the evaluator
// follows.
const MaxDepth = 32

// Evaluator evaluates instances against a model. It is safe for
// concurrent use.
type Evaluator struct {
	l  ModelLookup
	ns *model.NamespaceID
}

// New returns an evaluator for the given lookup, usually a profile.
func New(l ModelLookup) *Evaluator { return &Evaluator{l: l} }

// In returns an evaluator that looks up logical node classes in
// namespace ns only (see resolver.Resolver.In).
func (ev *Evaluator) In(ns model.NamespaceID) *Evaluator {
	cp := *ev
	cp.ns = &ns
	return &cp
}

// LogicalNode evaluates an instance of a logical node class: its data
// objects, inherited ones included, and the described children against
// their CDCs. The context gets LNClass, and LNNamespace from the
// namespace of the class when the instance does not set it; each data
// object child gets the namespace of its declaring class as
// DataNamespace.
func (ev *Evaluator) LogicalNode(class string, inst *Instance) (*Report, error) {
	var (
		ln  *model.LogicalNodeClass
		dos []catalog.InheritedDataObject
		err error
	)
	if ev.ns != nil {
		if ln, err = ev.l.LogicalNodeIn(*ev.ns, class); err == nil {
			dos, err = ev.l.DataObjectsIn(*ev.ns, class)
		}
	} else if ln, err = ev.l.LogicalNode(class); err == nil {
		dos, err = ev.l.DataObjects(class)
	}
	if err != nil {
		return nil, err
	}
	elems := make([]Element, len(dos))
	decl := make([]model.NamespaceID, len(dos))
	for i, do := range dos {
		decl[i] = do.DeclaredIn.Source
		elems[i] = Element{Name: do.Name, Kind: KindDataObject, Type: do.Type, Presence: do.Presence, Namespace: decl[i]}
	}
	ctx := Context{LNClass: ln.Name, LNNamespace: ln.Source.String()}
	rep := evaluate(ln.Name, "", elems, inst, ctx)
	if inst == nil {
		return rep, nil
	}
	full := merge(ctx, inst.Context)
	err = ev.children(rep, elems, inst, full, 0, func(i int, name string, child *Instance, c Context) (*Report, error) {
		c.DataNamespace = decl[i].String()
		return ev.cdc(decl[i], elems[i].Type, child, joinPath("", name), c, 1)
	})
	return rep, err
}

// CDC evaluates an instance of a data object of the named CDC: its
// data attributes and sub data objects, and the described children. A
// CDC with variants (SPG/SP, SPG/SG) takes the variant whose key the
// instance's Class names, else the one with functional constraint fc,
// else the first.
func (ev *Evaluator) CDC(name string, fc model.FunctionalConstraint, inst *Instance) (*Report, error) {
	vs, err := ev.l.CDCVariants(cdcName(name))
	if err != nil {
		return nil, err
	}
	c := pickVariant(vs, name, fc, inst)
	return ev.cdcOf(c, inst, "", Context{}, 0)
}

// ConstructedAttribute evaluates an instance of a constructed
// attribute type: its members and the described children.
func (ev *Evaluator) ConstructedAttribute(name string, inst *Instance) (*Report, error) {
	ca, err := ev.l.ConstructedAttribute(name)
	if err != nil {
		return nil, err
	}
	return ev.caOf(ca, inst, "", Context{}, 0)
}

// Check evaluates an instance whose Class names a logical node class,
// a CDC (or CDC variant key) or a constructed type, tried in that
// order.
func (ev *Evaluator) Check(inst *Instance) (*Report, error) {
	if inst == nil || inst.Class == "" {
		return nil, fmt.Errorf("%w: the instance names no class", model.ErrInvalidInstance)
	}
	rep, err := ev.LogicalNode(inst.Class, inst)
	if !errors.Is(err, model.ErrUnknownLogicalNode) {
		return rep, err
	}
	rep, err = ev.CDC(inst.Class, "", inst)
	if !errors.Is(err, model.ErrUnknownCDC) {
		return rep, err
	}
	rep, err = ev.ConstructedAttribute(inst.Class, inst)
	if errors.Is(err, model.ErrUnknownType) {
		return nil, fmt.Errorf("%w: %q is no logical node class, CDC or constructed type", model.ErrUnknownType, inst.Class)
	}
	return rep, err
}

// Validate evaluates an instance of a logical node class and returns a
// [*model.PresenceError] when a condition is violated. Undetermined
// conditions do not fail.
func (ev *Evaluator) Validate(class string, inst *Instance) error {
	rep, err := ev.LogicalNode(class, inst)
	if err != nil {
		return err
	}
	return rep.Err()
}

func cdcName(key string) string {
	n, _, _ := strings.Cut(key, "/")
	return n
}

func pickVariant(vs []*model.CommonDataClass, key string, fc model.FunctionalConstraint, inst *Instance) *model.CommonDataClass {
	want := []string{key}
	if inst != nil && inst.Class != "" {
		want = append([]string{inst.Class}, want...)
	}
	for _, w := range want {
		for _, v := range vs {
			if v.Key() == w {
				return v
			}
		}
	}
	if fc != "" {
		for _, v := range vs {
			if v.Variant == string(fc) {
				return v
			}
		}
	}
	return vs[0]
}

func (ev *Evaluator) cdc(from model.NamespaceID, typ string, inst *Instance, path string, ctx Context, depth int) (*Report, error) {
	vs, err := ev.l.CDCVariantsFrom(from, cdcName(typ))
	if err != nil {
		return nil, err
	}
	return ev.cdcOf(pickVariant(vs, typ, "", inst), inst, path, ctx, depth)
}

func (ev *Evaluator) cdcOf(c *model.CommonDataClass, inst *Instance, path string, ctx Context, depth int) (*Report, error) {
	type item struct {
		el  Element
		pos int
	}
	items := make([]item, 0, len(c.SubDataObjects)+len(c.DataAttributes))
	for _, s := range c.SubDataObjects {
		items = append(items, item{Element{Name: s.Name, Kind: KindSubDataObject, Type: s.Type, Presence: s.Presence, Namespace: c.Source}, s.Position})
	}
	for _, a := range c.DataAttributes {
		items = append(items, item{Element{Name: a.Name, Kind: KindDataAttribute, Type: a.Type, TypeKind: a.TypeKind,
			FC: a.FC, Presence: a.Presence, Namespace: c.Source}, a.Position})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].pos < items[j].pos })
	elems := make([]Element, len(items))
	for i, it := range items {
		elems[i] = it.el
	}
	rep := evaluate(c.Key(), path, elems, inst, ctx)
	if inst == nil {
		return rep, nil
	}
	full := merge(ctx, inst.Context)
	err := ev.children(rep, elems, inst, full, depth, func(i int, name string, child *Instance, cc Context) (*Report, error) {
		el := elems[i]
		cc.CtlModel = ""
		switch {
		case el.Kind == KindSubDataObject:
			return ev.cdc(c.Source, el.Type, child, joinPath(path, name), cc, depth+1)
		case el.TypeKind == model.TypeKindConstructed:
			ca, err := ev.l.ConstructedAttributeFrom(c.Source, el.Type)
			if err != nil {
				return nil, err
			}
			return ev.caOf(ca, child, joinPath(path, name), cc, depth+1)
		}
		return nil, nil
	})
	return rep, err
}

func (ev *Evaluator) caOf(ca *model.ConstructedAttribute, inst *Instance, path string, ctx Context, depth int) (*Report, error) {
	elems := make([]Element, len(ca.SubDataAttributes))
	for i, a := range ca.SubDataAttributes {
		elems[i] = Element{Name: a.Name, Kind: KindSubDataAttribute, Type: a.Type, TypeKind: a.TypeKind,
			Presence: a.Presence, Namespace: ca.Source}
	}
	rep := evaluate(ca.Name, path, elems, inst, ctx)
	if inst == nil {
		return rep, nil
	}
	full := merge(ctx, inst.Context)
	err := ev.children(rep, elems, inst, full, depth, func(i int, name string, child *Instance, cc Context) (*Report, error) {
		if elems[i].TypeKind != model.TypeKindConstructed {
			return nil, nil
		}
		sub, err := ev.l.ConstructedAttributeFrom(ca.Source, elems[i].Type)
		if err != nil {
			return nil, err
		}
		return ev.caOf(sub, child, joinPath(path, name), cc, depth+1)
	})
	return rep, err
}

// children evaluates the described children of inst that match an
// element, in model order, and appends their reports.
func (ev *Evaluator) children(rep *Report, elems []Element, inst *Instance, ctx Context, depth int,
	eval func(i int, name string, child *Instance, ctx Context) (*Report, error)) error {
	if len(inst.Children) == 0 {
		return nil
	}
	if depth >= MaxDepth {
		return fmt.Errorf("%w: instance nesting deeper than %d at %s", model.ErrLimitExceeded, MaxDepth, rep.Class)
	}
	e := &evaluation{elems: elems, inst: inst}
	e.match()
	done := map[string]bool{}
	for i := range elems {
		names := append([]string(nil), e.instances[i]...)
		sort.Strings(names)
		for _, name := range names {
			child := inst.Children[name]
			if child == nil || done[name] {
				continue
			}
			done[name] = true
			cr, err := eval(i, name, child, ctx)
			if err != nil {
				return err
			}
			if cr != nil {
				rep.Children = append(rep.Children, cr)
			}
		}
	}
	return nil
}
