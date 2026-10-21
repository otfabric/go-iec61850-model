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
	CDCVariants(name string) ([]*model.CommonDataClass, error)
	ConstructedAttribute(name string) (*model.ConstructedAttribute, error)
	Enumeration(name string) (*model.Enumeration, error)
	BasicType(name string) (*model.BasicType, error)

	// Qualified lookups: the definition of one namespace.
	LogicalNodeIn(ns model.NamespaceID, name string) (*model.LogicalNodeClass, error)
	InheritanceIn(ns model.NamespaceID, name string) ([]*model.LogicalNodeClass, error)
	DataObjectIn(ns model.NamespaceID, lnClass, name string) (catalog.InheritedDataObject, error)
	DataObjectsIn(ns model.NamespaceID, lnClass string) ([]catalog.InheritedDataObject, error)

	// Scoped lookups: a name referenced from a definition of namespace
	// from, resolved in that namespace and its dependencies first.
	CDCVariantsFrom(from model.NamespaceID, name string) ([]*model.CommonDataClass, error)
	ConstructedAttributeFrom(from model.NamespaceID, name string) (*model.ConstructedAttribute, error)
	EnumerationFrom(from model.NamespaceID, name string) (*model.Enumeration, error)
	BasicTypeFrom(from model.NamespaceID, name string) (*model.BasicType, error)
}

// DefaultMaxDepth bounds the nesting depth the resolver follows when it
// expands data objects and constructed types.
const DefaultMaxDepth = 32

// Resolver resolves references and model paths against a [Lookup],
// usually a namespace profile. It is safe for concurrent use.
//
// The logical node class of a path is looked up unqualified, unless the
// resolver is restricted to a namespace with [Resolver.In]. Every other
// reference (CDCs, constructed types, enumerations, basic types, base
// classes) is resolved in the scope of the namespace that declares it,
// so a name that two namespaces define does not make paths through the
// other one ambiguous.
type Resolver struct {
	l        Lookup
	maxDepth int
	ns       *model.NamespaceID
}

// New returns a resolver for the given lookup.
func New(l Lookup) *Resolver { return &Resolver{l: l, maxDepth: DefaultMaxDepth} }

// In returns a resolver that looks up logical node classes in namespace
// ns only, e.g. DBAT as IEC 61850-7-420 defines it when IEC TR 61850-90-9
// defines DBAT too. The ID may be partial ("IEC 61850-7-420").
func (r *Resolver) In(ns model.NamespaceID) *Resolver {
	cp := *r
	cp.ns = &ns
	return &cp
}

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
	// Variant is the CDC variant that declares a data attribute, e.g.
	// "SG" for setVal of SPG/SG.
	Variant string `json:"variant,omitempty"`
	// Variants lists the alternative definitions of a data attribute
	// with different FCs ("SPG/SG.setVal", "RST._setMag"), when no FC
	// was requested; FC is then empty because it depends on the
	// realization. Request an FC to select one.
	Variants []string `json:"variants,omitempty"`
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
	if r.ns != nil {
		return r.l.InheritanceIn(*r.ns, lnClass)
	}
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
		ok, err := r.hasFC(w.cdc.Source, w.cdc.Name, fc, map[string]bool{})
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
	cdcs []*model.CommonDataClass // all variants of cdc
	do   *model.DataObject
	attr *Step
	// doFrom is the namespace that declares the current data object;
	// typeFrom the namespace from which the type of the current
	// attribute is referenced. Lookups are scoped to them.
	doFrom, typeFrom model.NamespaceID
}

func (r *Resolver) start(lnClass string) (*walker, error) {
	var ln *model.LogicalNodeClass
	var err error
	if r.ns != nil {
		ln, err = r.l.LogicalNodeIn(*r.ns, lnClass)
	} else {
		ln, err = r.l.LogicalNode(lnClass)
	}
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
		cdc, da, variants, err := w.selectAttribute(name, fc)
		if err != nil {
			return err
		}
		typ, kind, param, err := w.r.attrType(da, cdc, w.do, w.doFrom)
		if err != nil {
			return w.annotate(err, name)
		}
		w.typeFrom = cdc.Source
		if param {
			w.typeFrom = w.doFrom
		}
		w.cdc = cdc
		w.res.CDC, w.res.Attribute = cdc, da
		step := Step{
			Name: name, Kind: StepDataAttribute, Type: typ, TypeKind: kind, FC: da.FC, Presence: da.Presence,
			IsArray: da.IsArray, Parameterized: param, DeclaredIn: cdc.Key(), Source: cdc.Source, Variant: cdc.Variant,
		}
		if len(variants) > 1 {
			// The FC depends on the variant the device realizes.
			step.FC, step.Variant, step.Variants = "", "", variants
		}
		w.res.Steps = append(w.res.Steps, step)
		w.attr = &w.res.Steps[len(w.res.Steps)-1]
		return nil
	default: // attribute → sub data attribute
		cur := *w.attr
		if cur.TypeKind != model.TypeKindConstructed {
			return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("attribute %s of type %s has no members", cur.Name, cur.Type), "", "")
		}
		ca, err := w.r.l.ConstructedAttributeFrom(w.typeFrom, cur.Type)
		if err != nil {
			return w.annotate(err, name)
		}
		sda, ok := ca.SubDataAttribute(name)
		if !ok {
			return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("type %s has no member %s", ca.Name, name), "", "")
		}
		typ, kind, err := w.r.memberType(sda, ca.Source)
		if err != nil {
			return w.annotate(err, name)
		}
		w.typeFrom = ca.Source
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
	ido, err := r.lookupDataObject(lnClass, name)
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
	base, berr := r.lookupDataObject(lnClass, name[:i])
	if berr != nil || !IsMultiInstance(base.Presence) {
		return ido, "", err
	}
	return base, name[i:], nil
}

func (r *Resolver) lookupDataObject(lnClass, name string) (catalog.InheritedDataObject, error) {
	if r.ns != nil {
		return r.l.DataObjectIn(*r.ns, lnClass, name)
	}
	return r.l.DataObject(lnClass, name)
}

// IsMultiInstance reports whether a presence condition allows numbered
// instances of a data object (Omulti, Mmulti and their variants).
func IsMultiInstance(p model.PresenceCondition) bool {
	return strings.Contains(strings.ToLower(p.Name), "multi")
}

// selectAttribute finds a data attribute among the variants of the
// current CDC, by exact name or by base name (setMag also matches the
// per-FC copies _setMag and __setMag). With an FC, the definition with
// that FC is chosen. Without one, the first is used, and when the
// candidates differ in FC all of them are returned as alternatives
// ("CDC/variant.attribute").
func (w *walker) selectAttribute(name string, fc model.FunctionalConstraint) (*model.CommonDataClass, *model.DataAttribute, []string, error) {
	type cand struct {
		cdc *model.CommonDataClass
		da  *model.DataAttribute
	}
	var all, found []cand
	for _, c := range w.cdcs {
		for i := range c.DataAttributes {
			da := &c.DataAttributes[i]
			if da.Name == name || (!strings.HasPrefix(name, "_") && da.BaseName() == name) {
				all = append(all, cand{c, da})
				if fc == "" || da.FC == fc {
					found = append(found, cand{c, da})
				}
			}
		}
	}
	switch {
	case len(all) == 0:
		return nil, nil, nil, w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("CDC %s has no member %s", w.cdc.Name, name), "", "")
	case len(found) == 0:
		fcs := make([]string, len(all))
		for i, c := range all {
			fcs[i] = string(c.da.FC)
		}
		return nil, nil, nil, w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("attribute %s has functional constraint %s", name, strings.Join(fcs, "/")), string(fc), strings.Join(fcs, "/"))
	}
	var alternatives []string
	for _, c := range found[1:] {
		if c.da.FC != found[0].da.FC {
			for _, a := range found {
				alternatives = append(alternatives, a.cdc.Key()+"."+a.da.Name)
			}
			break
		}
	}
	return found[0].cdc, found[0].da, alternatives, nil
}

func (w *walker) enterDataObject(do *model.DataObject, kind StepKind, declaredIn string, src model.NamespaceID) error {
	cdcs, err := w.r.l.CDCVariantsFrom(src, do.Type)
	if err != nil {
		return w.annotate(err, do.Name)
	}
	cdc := cdcs[0]
	if kind == StepDataObject {
		w.res.DataObject = do
	}
	w.cdc, w.cdcs, w.do, w.doFrom = cdc, cdcs, do, src
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
		w.res.Enumeration, err = w.r.l.EnumerationFrom(w.typeFrom, leaf.Type)
	case model.TypeKindConstructed:
		w.res.Constructed, err = w.r.l.ConstructedAttributeFrom(w.typeFrom, leaf.Type)
	case model.TypeKindBasic:
		w.res.Basic, err = w.r.l.BasicTypeFrom(w.typeFrom, leaf.Type)
	default:
		if b, e := w.r.l.BasicTypeFrom(w.typeFrom, leaf.Type); e == nil {
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
func (r *Resolver) attrType(da *model.DataAttribute, cdc *model.CommonDataClass, do *model.DataObject, doFrom model.NamespaceID) (string, model.TypeKind, bool, error) {
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
	from := cdc.Source
	if param {
		from = doFrom
	}
	kind, err := r.kindOf(from, da.Name, typ, kind)
	return typ, kind, param, err
}

// memberType determines the type of a sub data attribute.
func (r *Resolver) memberType(sda *model.DataAttribute, from model.NamespaceID) (string, model.TypeKind, error) {
	if sda.Type == "" {
		return "", "", &model.ResolutionError{Element: sda.Name, Reason: "attribute has no type", Err: model.ErrUnknownType}
	}
	kind, err := r.kindOf(from, sda.Name, sda.Type, sda.TypeKind)
	return sda.Type, kind, err
}

// kindOf checks a declared type kind, or infers it when it is missing
// or "undefined", from the definitions of the profile.
func (r *Resolver) kindOf(from model.NamespaceID, element, typ string, kind model.TypeKind) (model.TypeKind, error) {
	switch kind {
	case model.TypeKindBasic, model.TypeKindEnumerated, model.TypeKindConstructed:
		return kind, nil
	case "", model.TypeKindUndefined:
	default:
		// Other kinds (e.g. PACKED_LIST) are leaves; keep them verbatim.
		return kind, nil
	}
	var found []model.TypeKind
	if _, err := r.l.BasicTypeFrom(from, typ); err == nil {
		found = append(found, model.TypeKindBasic)
	}
	if _, err := r.l.EnumerationFrom(from, typ); err == nil {
		found = append(found, model.TypeKindEnumerated)
	}
	if _, err := r.l.ConstructedAttributeFrom(from, typ); err == nil {
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

// hasFC reports whether a CDC (any of its variants), or a CDC of one of
// its sub data objects, has a data attribute with the functional
// constraint.
func (r *Resolver) hasFC(from model.NamespaceID, cdcName string, fc model.FunctionalConstraint, visiting map[string]bool) (bool, error) {
	if visiting[cdcName] {
		return false, &model.ResolutionError{Element: cdcName, Reason: "CDC contains itself", Err: model.ErrCycle}
	}
	visiting[cdcName] = true
	defer delete(visiting, cdcName)
	variants, err := r.l.CDCVariantsFrom(from, cdcName)
	if err != nil {
		return false, err
	}
	for _, cdc := range variants {
		for i := range cdc.DataAttributes {
			if cdc.DataAttributes[i].FC == fc {
				return true, nil
			}
		}
	}
	for i := range variants[0].SubDataObjects {
		ok, err := r.hasFC(variants[0].Source, variants[0].SubDataObjects[i].Type, fc, visiting)
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
		var dos []catalog.InheritedDataObject
		var err error
		if r.ns != nil {
			dos, err = r.l.DataObjectsIn(*r.ns, lnClass)
		} else {
			dos, err = r.l.DataObjects(lnClass)
		}
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
	var members []member
	switch {
	case !res.IsAttribute():
		typeKey = "cdc:" + res.CDC.Name
		ms, err := r.cdcMembers(res.CDC.Source, res.CDC.Name, fc)
		if err != nil {
			return nil, err
		}
		members = ms
	case leaf.TypeKind == model.TypeKindConstructed:
		typeKey = "ca:" + leaf.Type
		for _, m := range res.Constructed.SubDataAttributes {
			members = append(members, member{name: m.Name})
		}
	default:
		return append(out, res), nil
	}
	if visiting[typeKey] {
		return nil, &model.ResolutionError{Path: res.Path(), Element: leaf.Type, Reason: "type contains itself", Err: model.ErrCycle}
	}
	visiting[typeKey] = true
	defer delete(visiting, typeKey)

	for _, m := range members {
		child, err := r.extend(res, m.name, m.fc)
		if err != nil {
			return nil, err
		}
		if out, err = r.expand(child, fc, out, visiting); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// member is a child of a CDC or constructed type to expand: its device
// name and, for data attributes, the FC that selects its definition.
type member struct {
	name string
	fc   model.FunctionalConstraint
	pos  int
}

// cdcMembers lists the sub data objects and data attributes of a CDC
// in document order, over all its variants, by device name and FC
// (setMag[SP], setMag[SG], ...). With fc, only data attributes with that
// FC are included.
func (r *Resolver) cdcMembers(from model.NamespaceID, name string, fc model.FunctionalConstraint) ([]member, error) {
	variants, err := r.l.CDCVariantsFrom(from, name)
	if err != nil {
		return nil, err
	}
	var out []member
	seen := map[string]bool{}
	for _, s := range variants[0].SubDataObjects {
		out = append(out, member{name: s.Name, pos: s.Position})
	}
	for vi, c := range variants {
		for i := range c.DataAttributes {
			da := &c.DataAttributes[i]
			if fc != "" && da.FC != fc {
				continue
			}
			key := da.BaseName() + "[" + string(da.FC) + "]"
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, member{name: da.BaseName(), fc: da.FC, pos: da.Position*len(variants) + vi})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].pos < out[j].pos })
	return out, nil
}

// extend resolves one more path element below res. The FC selects the
// definition of a data attribute among variants and per-FC copies.
func (r *Resolver) extend(res *Resolution, name string, fc model.FunctionalConstraint) (*Resolution, error) {
	path := make([]string, 0, len(res.Steps))
	for _, s := range res.Steps[1:] {
		path = append(path, s.Name)
	}
	return r.ResolvePath(res.Steps[0].Name, append(path, name), fc)
}
