// SPDX-License-Identifier: MIT

package resolver

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/reference"
)

// Lookup is the set of lookups the resolver needs. *catalog.Profile
// implements it, and so does *catalog.Catalog, whose lookups fail on
// names that several loaded namespaces define.
type Lookup interface {
	LogicalNode(name string) (*model.LogicalNodeClass, error)
	Inheritance(name string) ([]*model.LogicalNodeClass, error)
	DataObject(lnClass, name string) (catalog.InheritedDataObject, error)
	DataObjects(lnClass string) ([]catalog.InheritedDataObject, error)
	CDC(name string) (*model.CommonDataClass, error)
	ConstructedAttribute(name string) (*model.ConstructedAttribute, error)
	Enumeration(name string) (*model.Enumeration, error)
	BasicType(name string) (*model.BasicType, error)
}

// DefaultMaxDepth bounds the nesting depth the resolver follows when it
// expands data objects and constructed types.
const DefaultMaxDepth = 32

// Resolver resolves references and model paths against a [Lookup],
// usually a namespace profile. It is safe for concurrent use.
type Resolver struct {
	l        Lookup
	maxDepth int
}

// New returns a resolver for the given lookup.
func New(l Lookup) *Resolver { return &Resolver{l: l, maxDepth: DefaultMaxDepth} }

// StepKind is the kind of one element of a resolved path.
type StepKind string

// Step kinds.
const (
	StepLogicalNode      StepKind = "LogicalNode"
	StepDataObject       StepKind = "DataObject"
	StepSubDataObject    StepKind = "SubDataObject"
	StepDataAttribute    StepKind = "DataAttribute"
	StepSubDataAttribute StepKind = "SubDataAttribute"
)

// Step is one element of a resolved path.
type Step struct {
	Name string   `json:"name"`
	Kind StepKind `json:"kind"`
	// Type is the class of a logical node step, the CDC of a data
	// object step, and the resolved type of an attribute step.
	Type string `json:"type"`
	// TypeKind is the resolved type kind of an attribute step.
	TypeKind model.TypeKind `json:"typeKind,omitempty"`
	// FC is the functional constraint of an attribute step; sub data
	// attributes carry the FC of their data attribute.
	FC       model.FunctionalConstraint `json:"fc,omitempty"`
	Presence model.PresenceCondition    `json:"presence,omitzero"`
	IsArray  bool                       `json:"isArray,omitempty"`
	// Instance is the instance number of a numbered data object, e.g.
	// "1" for Ind1 of the multi-instance data object Ind.
	Instance string `json:"instance,omitempty"`
	// Parameterized reports that the type was taken from the underlying
	// type of the enclosing data object (e.g. ENS.stVal).
	Parameterized bool `json:"parameterized,omitempty"`
	// DeclaredIn is the class, CDC or constructed type that declares
	// the element.
	DeclaredIn string `json:"declaredIn"`
	// Source is the namespace of DeclaredIn.
	Source model.NamespaceID `json:"source"`
}

// Resolution is a resolved model path, from a logical node class down
// to a data object or data attribute.
type Resolution struct {
	// Steps lists every element of the path; the first is the logical
	// node class.
	Steps []Step `json:"steps"`
	// LNClass is the logical node class.
	LNClass *model.LogicalNodeClass `json:"-"`
	// DataObject is the top-level data object, nil for a class.
	DataObject *model.DataObject `json:"-"`
	// CDC is the CDC of the deepest data object on the path.
	CDC *model.CommonDataClass `json:"-"`
	// Attribute is the last attribute on the path, nil when the path
	// ends at a data object.
	Attribute *model.DataAttribute `json:"-"`
	// Enumeration, Constructed and Basic are set when the path ends at
	// an attribute of that kind.
	Enumeration *model.Enumeration          `json:"-"`
	Constructed *model.ConstructedAttribute `json:"-"`
	Basic       *model.BasicType            `json:"-"`
}

// Leaf returns the last step.
func (r *Resolution) Leaf() Step { return r.Steps[len(r.Steps)-1] }

// Path returns the class path, e.g. "MMXU.Hz.mag.f".
func (r *Resolution) Path() string { return joinSteps(r.Steps, nil) }

// DataObjectPath returns the data object part of the path, e.g. "Hz"
// or "PNV.phsA".
func (r *Resolution) DataObjectPath() string {
	return joinSteps(r.Steps, func(s Step) bool { return s.Kind == StepDataObject || s.Kind == StepSubDataObject })
}

// AttributePath returns the attribute part of the path, e.g. "mag.f";
// empty when the path ends at a data object.
func (r *Resolution) AttributePath() string {
	return joinSteps(r.Steps, func(s Step) bool { return s.Kind == StepDataAttribute || s.Kind == StepSubDataAttribute })
}

// Type returns the type of the leaf: the CDC name for a data object, or
// the attribute type, e.g. "FLOAT32".
func (r *Resolution) Type() string { return r.Leaf().Type }

// TypeKind returns the type kind of the leaf; empty for a data object.
func (r *Resolution) TypeKind() model.TypeKind { return r.Leaf().TypeKind }

// FC returns the functional constraint of the leaf; empty for a data
// object.
func (r *Resolution) FC() model.FunctionalConstraint { return r.Leaf().FC }

// IsAttribute reports whether the path ends at an attribute.
func (r *Resolution) IsAttribute() bool {
	k := r.Leaf().Kind
	return k == StepDataAttribute || k == StepSubDataAttribute
}

// ClassRef returns the class reference of the resolved path, with the
// leaf FC for attribute paths.
func (r *Resolution) ClassRef() reference.ClassRef {
	ref := reference.ClassRef{LNClass: r.Steps[0].Name}
	for _, s := range r.Steps[1:] {
		ref.Path = append(ref.Path, s.Name)
	}
	if r.IsAttribute() {
		ref.FC = r.FC()
	}
	return ref
}

func joinSteps(steps []Step, keep func(Step) bool) string {
	var parts []string
	for _, s := range steps {
		if keep == nil || keep(s) {
			parts = append(parts, s.Name)
		}
	}
	return strings.Join(parts, ".")
}

// Resolve resolves an instance reference, e.g. "LD0/MMXU1.Hz.mag.f[MX]",
// through the logical node class of its instance name. The logical
// device is not checked: the catalog describes classes, not devices.
func (r *Resolver) Resolve(ref reference.Ref) (*Resolution, error) {
	cls, err := ref.Class()
	if err != nil {
		return nil, err
	}
	return r.ResolveClass(cls)
}

// ResolveString parses s as an instance reference when it contains
// "/" and as a class reference otherwise, and resolves it.
func (r *Resolver) ResolveString(s string) (*Resolution, error) {
	if strings.Contains(s, "/") {
		ref, err := reference.Parse(s)
		if err != nil {
			return nil, err
		}
		return r.Resolve(ref)
	}
	ref, err := reference.ParseClass(s)
	if err != nil {
		return nil, err
	}
	return r.ResolveClass(ref)
}

// ResolveClass resolves a class reference, e.g. "MMXU.Hz.mag.f[MX]".
func (r *Resolver) ResolveClass(ref reference.ClassRef) (*Resolution, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	return r.ResolvePath(ref.LNClass, ref.Path, ref.FC)
}

// ResolveAttribute resolves the attribute path below a data object of
// a logical node class, e.g. ("MMXU", "Hz", "mag", "f").
func (r *Resolver) ResolveAttribute(lnClass, dataObject string, attrPath ...string) (*Resolution, error) {
	return r.ResolvePath(lnClass, append([]string{dataObject}, attrPath...), "")
}

// ResolveCDC returns the CDC with the given name.
func (r *Resolver) ResolveCDC(name string) (*model.CommonDataClass, error) { return r.l.CDC(name) }

// ResolveInheritance returns the class chain of a logical node class,
// from the class itself to its root class.
func (r *Resolver) ResolveInheritance(lnClass string) ([]*model.LogicalNodeClass, error) {
	return r.l.Inheritance(lnClass)
}

// ResolvePath resolves a path of data object and attribute names below
// a logical node class. When fc is set, every data attribute on the
// path must have that functional constraint, and a path that ends at a
// data object must contain at least one attribute with it.
//
// Errors wrap [model.ErrUnknownLogicalNode], [model.ErrUnknownDataObject],
// [model.ErrUnknownCDC], [model.ErrUnknownAttribute],
// [model.ErrUnknownType], [model.ErrCycle] or
// [model.ErrAmbiguousReference].
func (r *Resolver) ResolvePath(lnClass string, path []string, fc model.FunctionalConstraint) (*Resolution, error) {
	w, err := r.start(lnClass)
	if err != nil {
		return nil, err
	}
	for _, name := range path {
		if err := w.step(name, fc); err != nil {
			return nil, err
		}
	}
	if fc != "" && !w.res.IsAttribute() && len(w.res.Steps) > 1 {
		ok, err := r.hasFC(w.cdc, fc, map[string]bool{})
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, w.fail(model.ErrUnknownAttribute, "", fmt.Sprintf("no data attribute with functional constraint %s", fc), string(fc), "")
		}
	}
	if err := w.finish(); err != nil {
		return nil, err
	}
	return w.res, nil
}

// walker carries the state of one resolution.
type walker struct {
	r   *Resolver
	res *Resolution
	// cdc and do are the CDC and data object of the current data
	// object context; attr is set once the path enters an attribute.
	cdc  *model.CommonDataClass
	do   *model.DataObject
	attr *Step
}

func (r *Resolver) start(lnClass string) (*walker, error) {
	ln, err := r.l.LogicalNode(lnClass)
	if err != nil {
		return nil, err
	}
	w := &walker{r: r, res: &Resolution{LNClass: ln}}
	w.res.Steps = []Step{{Name: ln.Name, Kind: StepLogicalNode, Type: ln.Name, DeclaredIn: ln.Name, Source: ln.Source}}
	return w, nil
}

func (w *walker) path() string { return w.res.Path() }

func (w *walker) fail(sentinel error, element, reason, expected, actual string) error {
	return &model.ResolutionError{Namespace: w.source(), Path: joinPath(w.path(), element), Element: element, Expected: expected, Actual: actual, Reason: reason, Err: sentinel}
}

// source returns the namespace of the definition being searched: the
// CDC in a data object context, otherwise the last step.
func (w *walker) source() model.NamespaceID {
	if w.attr == nil && w.cdc != nil {
		return w.cdc.Source
	}
	return w.res.Leaf().Source
}

func joinPath(base, elem string) string {
	if elem == "" {
		return base
	}
	return base + "." + elem
}

// annotate adds the path to errors of the lookup, leaving ambiguity
// errors as they are.
func (w *walker) annotate(err error, element string) error {
	var re *model.ResolutionError
	if errors.As(err, &re) && re.Path == "" {
		cp := *re
		cp.Path = joinPath(w.path(), element)
		if cp.Namespace.IsZero() {
			cp.Namespace = w.source()
		}
		return &cp
	}
	return err
}

func (w *walker) step(name string, fc model.FunctionalConstraint) error {
	if len(w.res.Steps) > w.r.maxDepth {
		return w.fail(model.ErrLimitExceeded, name, fmt.Sprintf("path deeper than %d elements", w.r.maxDepth), "", "")
	}
	switch {
	case len(w.res.Steps) == 1: // logical node → data object
		ido, inst, err := w.r.dataObject(w.res.LNClass.Name, name)
		if err != nil {
			return w.annotate(err, name)
		}
		if err := w.enterDataObject(ido.DataObject, StepDataObject, ido.DeclaredIn.Name, ido.DeclaredIn.Source); err != nil {
			return err
		}
		last := &w.res.Steps[len(w.res.Steps)-1]
		last.Name, last.Instance = name, inst
		return nil
	case w.attr == nil: // data object → sub data object or data attribute
		if sdo, ok := w.cdc.SubDataObject(name); ok {
			return w.enterDataObject(sdo, StepSubDataObject, w.cdc.Name, w.cdc.Source)
		}
		da, ok := w.cdc.DataAttribute(name)
		if !ok {
			return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("CDC %s has no member %s", w.cdc.Name, name), "", "")
		}
		if fc != "" && da.FC != fc {
			return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("attribute %s has functional constraint %s", name, da.FC), string(fc), string(da.FC))
		}
		typ, kind, param, err := w.r.attrType(da, w.cdc, w.do)
		if err != nil {
			return w.annotate(err, name)
		}
		w.res.Attribute = da
		w.res.Steps = append(w.res.Steps, Step{
			Name: name, Kind: StepDataAttribute, Type: typ, TypeKind: kind, FC: da.FC, Presence: da.Presence,
			IsArray: da.IsArray, Parameterized: param, DeclaredIn: w.cdc.Name, Source: w.cdc.Source,
		})
		w.attr = &w.res.Steps[len(w.res.Steps)-1]
		return nil
	default: // attribute → sub data attribute
		cur := *w.attr
		if cur.TypeKind != model.TypeKindConstructed {
			return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("attribute %s of type %s has no members", cur.Name, cur.Type), "", "")
		}
		ca, err := w.r.l.ConstructedAttribute(cur.Type)
		if err != nil {
			return w.annotate(err, name)
		}
		sda, ok := ca.SubDataAttribute(name)
		if !ok {
			return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("type %s has no member %s", ca.Name, name), "", "")
		}
		typ, kind, err := w.r.memberType(sda)
		if err != nil {
			return w.annotate(err, name)
		}
		w.res.Attribute = sda
		w.res.Steps = append(w.res.Steps, Step{
			Name: name, Kind: StepSubDataAttribute, Type: typ, TypeKind: kind, FC: cur.FC, Presence: sda.Presence,
			IsArray: sda.IsArray, DeclaredIn: ca.Name, Source: ca.Source,
		})
		w.attr = &w.res.Steps[len(w.res.Steps)-1]
		return nil
	}
}

// dataObject looks up a data object of a class. A name with a trailing
// instance number, e.g. "Ind1", matches a multi-instance data object
// ("Ind" with a presence condition such as Omulti) when the class has
// no data object with the full name.
func (r *Resolver) dataObject(lnClass, name string) (catalog.InheritedDataObject, string, error) {
	ido, err := r.l.DataObject(lnClass, name)
	if err == nil || !errors.Is(err, model.ErrUnknownDataObject) {
		return ido, "", err
	}
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	if i == len(name) || i == 0 {
		return ido, "", err
	}
	base, berr := r.l.DataObject(lnClass, name[:i])
	if berr != nil || !IsMultiInstance(base.Presence) {
		return ido, "", err
	}
	return base, name[i:], nil
}

// IsMultiInstance reports whether a presence condition allows numbered
// instances of a data object (Omulti, Mmulti and their variants).
func IsMultiInstance(p model.PresenceCondition) bool {
	return strings.Contains(strings.ToLower(p.Name), "multi")
}

func (w *walker) enterDataObject(do *model.DataObject, kind StepKind, declaredIn string, src model.NamespaceID) error {
	cdc, err := w.r.l.CDC(do.Type)
	if err != nil {
		return w.annotate(err, do.Name)
	}
	if kind == StepDataObject {
		w.res.DataObject = do
	}
	w.cdc, w.do = cdc, do
	w.res.CDC = cdc
	w.res.Steps = append(w.res.Steps, Step{
		Name: do.Name, Kind: kind, Type: do.Type, Presence: do.Presence, IsArray: do.IsArray, DeclaredIn: declaredIn, Source: src,
	})
	return nil
}

// finish attaches the type definition of an attribute leaf.
func (w *walker) finish() error {
	if !w.res.IsAttribute() {
		return nil
	}
	leaf := w.res.Leaf()
	var err error
	switch leaf.TypeKind {
	case model.TypeKindEnumerated:
		w.res.Enumeration, err = w.r.l.Enumeration(leaf.Type)
	case model.TypeKindConstructed:
		w.res.Constructed, err = w.r.l.ConstructedAttribute(leaf.Type)
	case model.TypeKindBasic:
		w.res.Basic, err = w.r.l.BasicType(leaf.Type)
	default:
		if b, e := w.r.l.BasicType(leaf.Type); e == nil {
			w.res.Basic = b
		}
	}
	if err != nil {
		return w.annotate(err, "")
	}
	return nil
}

// attrType determines the type of a CDC data attribute, taking the
// data object's underlying type for parameterized CDCs.
func (r *Resolver) attrType(da *model.DataAttribute, cdc *model.CommonDataClass, do *model.DataObject) (string, model.TypeKind, bool, error) {
	typ, kind := da.Type, da.TypeKind
	param := false
	if typ == "" && (cdc.EnumParameterized || cdc.TypeKindParameterized || kind == model.TypeKindUndefined || kind == model.TypeKindEnumerated) {
		if do == nil || do.UnderlyingType == "" {
			return "", "", false, &model.ResolutionError{
				Element: da.Name, Reason: fmt.Sprintf("attribute type is parameterized but data object declares no underlying type (CDC %s)", cdc.Name),
				Err: model.ErrUnknownType,
			}
		}
		typ, param = do.UnderlyingType, true
		if kind == "" || kind == model.TypeKindUndefined {
			kind = do.UnderlyingTypeKind
		}
	}
	if typ == "" {
		return "", "", false, &model.ResolutionError{Element: da.Name, Reason: "attribute has no type", Err: model.ErrUnknownType}
	}
	kind, err := r.kindOf(da.Name, typ, kind)
	return typ, kind, param, err
}

// memberType determines the type of a sub data attribute.
func (r *Resolver) memberType(sda *model.DataAttribute) (string, model.TypeKind, error) {
	if sda.Type == "" {
		return "", "", &model.ResolutionError{Element: sda.Name, Reason: "attribute has no type", Err: model.ErrUnknownType}
	}
	kind, err := r.kindOf(sda.Name, sda.Type, sda.TypeKind)
	return sda.Type, kind, err
}

// kindOf checks a declared type kind, or infers it when it is missing
// or "undefined", from the definitions of the profile.
func (r *Resolver) kindOf(element, typ string, kind model.TypeKind) (model.TypeKind, error) {
	switch kind {
	case model.TypeKindBasic, model.TypeKindEnumerated, model.TypeKindConstructed:
		return kind, nil
	case "", model.TypeKindUndefined:
	default:
		// Other kinds (e.g. PACKED_LIST) are leaves; keep them verbatim.
		return kind, nil
	}
	var found []model.TypeKind
	if _, err := r.l.BasicType(typ); err == nil {
		found = append(found, model.TypeKindBasic)
	}
	if _, err := r.l.Enumeration(typ); err == nil {
		found = append(found, model.TypeKindEnumerated)
	}
	if _, err := r.l.ConstructedAttribute(typ); err == nil {
		found = append(found, model.TypeKindConstructed)
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", &model.ResolutionError{Element: element, Reason: fmt.Sprintf("type %s is not defined", typ), Err: model.ErrUnknownType}
	default:
		return "", &model.ResolutionError{Element: element, Reason: fmt.Sprintf("type %s is defined as more than one kind", typ), Err: model.ErrAmbiguousReference}
	}
}

// hasFC reports whether a CDC, or a CDC of one of its sub data
// objects, has a data attribute with the functional constraint.
func (r *Resolver) hasFC(cdc *model.CommonDataClass, fc model.FunctionalConstraint, visiting map[string]bool) (bool, error) {
	if visiting[cdc.Name] {
		return false, &model.ResolutionError{Element: cdc.Name, Reason: "CDC contains itself", Err: model.ErrCycle}
	}
	visiting[cdc.Name] = true
	defer delete(visiting, cdc.Name)
	for i := range cdc.DataAttributes {
		if cdc.DataAttributes[i].FC == fc {
			return true, nil
		}
	}
	for i := range cdc.SubDataObjects {
		sub, err := r.l.CDC(cdc.SubDataObjects[i].Type)
		if err != nil {
			return false, err
		}
		ok, err := r.hasFC(sub, fc, visiting)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// Leaves expands a model path into every attribute leaf below it: the
// basic, enumerated and other non-constructed attributes, with
// constructed types expanded. An empty path expands every data object
// of the class, including inherited ones. When fc is set, only data
// attributes with that functional constraint are followed.
//
// Leaves are returned in model order: data objects in class order,
// members of a CDC or constructed type in document order.
func (r *Resolver) Leaves(lnClass string, path []string, fc model.FunctionalConstraint) ([]*Resolution, error) {
	base, err := r.ResolvePath(lnClass, path, fc)
	if err != nil {
		return nil, err
	}
	var out []*Resolution
	if len(base.Steps) == 1 {
		dos, err := r.l.DataObjects(lnClass)
		if err != nil {
			return nil, err
		}
		for _, ido := range dos {
			sub, err := r.ResolvePath(lnClass, []string{ido.Name}, "")
			if err != nil {
				return nil, err
			}
			if out, err = r.expand(sub, fc, out, map[string]bool{}); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return r.expand(base, fc, out, map[string]bool{})
}

func (r *Resolver) expand(res *Resolution, fc model.FunctionalConstraint, out []*Resolution, visiting map[string]bool) ([]*Resolution, error) {
	if len(res.Steps) > r.maxDepth {
		return nil, &model.ResolutionError{Path: res.Path(), Reason: fmt.Sprintf("expansion deeper than %d elements", r.maxDepth), Err: model.ErrLimitExceeded}
	}
	leaf := res.Leaf()
	var typeKey string
	var children []string
	switch {
	case !res.IsAttribute():
		typeKey = "cdc:" + res.CDC.Name
		children = memberOrder(res.CDC)
	case leaf.TypeKind == model.TypeKindConstructed:
		typeKey = "ca:" + leaf.Type
		for _, m := range res.Constructed.SubDataAttributes {
			children = append(children, m.Name)
		}
	default:
		return append(out, res), nil
	}
	if visiting[typeKey] {
		return nil, &model.ResolutionError{Path: res.Path(), Element: leaf.Type, Reason: "type contains itself", Err: model.ErrCycle}
	}
	visiting[typeKey] = true
	defer delete(visiting, typeKey)

	for _, name := range children {
		if !res.IsAttribute() && fc != "" {
			if da, ok := res.CDC.DataAttribute(name); ok && da.FC != fc {
				continue
			}
		}
		child, err := r.extend(res, name)
		if err != nil {
			return nil, err
		}
		if out, err = r.expand(child, fc, out, visiting); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// extend resolves one more path element below res.
func (r *Resolver) extend(res *Resolution, name string) (*Resolution, error) {
	path := make([]string, 0, len(res.Steps))
	for _, s := range res.Steps[1:] {
		path = append(path, s.Name)
	}
	return r.ResolvePath(res.Steps[0].Name, append(path, name), "")
}

// memberOrder lists the sub data objects and data attributes of a CDC
// in document order.
func memberOrder(c *model.CommonDataClass) []string {
	type member struct {
		name string
		pos  int
	}
	ms := make([]member, 0, len(c.SubDataObjects)+len(c.DataAttributes))
	for _, d := range c.SubDataObjects {
		ms = append(ms, member{d.Name, d.Position})
	}
	for _, a := range c.DataAttributes {
		ms = append(ms, member{a.Name, a.Position})
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].pos < ms[j].pos })
	names := make([]string, len(ms))
	for i, m := range ms {
		names[i] = m.name
	}
	return names
}
