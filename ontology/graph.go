// SPDX-License-Identifier: MIT

package ontology

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/semantic"
)

// Kind is the kind of a concept.
type Kind string

// Concept kinds. The model kinds mirror IEC 61850 definitions; the
// semantic kinds (measurement, quantity, phase, reference) are
// protocol-neutral OTFabric concepts.
const (
	KindNamespace            Kind = "namespace"
	KindLogicalNode          Kind = "logical-node-class"
	KindDataObject           Kind = "data-object"
	KindCDC                  Kind = "cdc"
	KindSubDataObject        Kind = "sub-data-object"
	KindDataAttribute        Kind = "data-attribute"
	KindConstructedAttribute Kind = "constructed-attribute"
	KindEnumeration          Kind = "enumeration"
	KindBasicType            Kind = "basic-type"
	KindMeasurement          Kind = "measurement"
	KindQuantity             Kind = "quantity"
	KindPhase                Kind = "phase"
	KindReference            Kind = "reference"
)

// Predicate is the kind of a relation.
type Predicate string

// Relation predicates.
const (
	// Contains: a class contains a data object, a CDC a sub data object
	// or data attribute, a constructed type a member.
	Contains Predicate = "contains"
	// Uses: a data object or sub data object uses a CDC.
	Uses Predicate = "uses"
	// HasType: an attribute has a basic, constructed or enumerated
	// type; a data object has an underlying type.
	HasType Predicate = "hasType"
	// InheritsFrom: a class inherits from a base class, an enumeration
	// extends another.
	InheritsFrom Predicate = "inheritsFrom"
	// DefinedIn: a definition belongs to a namespace.
	DefinedIn Predicate = "definedIn"
	// DependsOn: a namespace depends on another.
	DependsOn Predicate = "dependsOn"
	// Represents: a measurement represents a quantity.
	Represents Predicate = "represents"
	// HasPhase: a measurement belongs to a phase.
	HasPhase Predicate = "hasPhase"
	// HasReference: a voltage measurement has a reference.
	HasReference Predicate = "hasReference"
	// MeasuredBy: a measurement is carried by a data object or sub data
	// object.
	MeasuredBy Predicate = "measuredBy"
)

// Property is a named value of a concept.
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Concept is a node of the graph.
type Concept struct {
	// ID is unique in the graph, e.g. "ln:MMXU", "do:MMXU.Hz",
	// "cdc:MV", "quantity:frequency".
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	// Name is the element name, e.g. "Hz".
	Name string `json:"name"`
	// Label is a human-readable label, when one is known.
	Label string `json:"label,omitempty"`
	// Namespace is the defining namespace of model concepts.
	Namespace model.NamespaceID `json:"namespace,omitzero"`
	// Properties carry further facts, e.g. presence, FC, rule.
	Properties []Property `json:"properties,omitempty"`
}

// Property returns the value of the named property.
func (c Concept) Property(name string) (string, bool) {
	for _, p := range c.Properties {
		if p.Name == name {
			return p.Value, true
		}
	}
	return "", false
}

// Relation is a directed edge of the graph.
type Relation struct {
	From      string    `json:"from"`
	Predicate Predicate `json:"predicate"`
	To        string    `json:"to"`
}

// Graph is an immutable concept graph. Every listing and traversal is
// deterministic. It is safe for concurrent use.
type Graph struct {
	conflicts []string
	concepts  map[string]Concept
	ids       []string
	out       map[string][]Relation
	in        map[string][]Relation
	rels      []Relation
}

// Conflicts lists the concepts that more than one namespace of the
// profile defines, and classes whose measurements could not be mapped
// because a name is ambiguous. The graph keeps the first definition in
// namespace order; [catalog.Profile.Diagnostics] reports the same
// conflicts.
func (g *Graph) Conflicts() []string { return append([]string(nil), g.conflicts...) }

// Concept returns the concept with the given ID.
func (g *Graph) Concept(id string) (Concept, bool) {
	c, ok := g.concepts[id]
	return c, ok
}

// Concepts returns all concepts, ordered by ID.
func (g *Graph) Concepts() []Concept {
	out := make([]Concept, len(g.ids))
	for i, id := range g.ids {
		out[i] = g.concepts[id]
	}
	return out
}

// RelationsAll returns all relations, ordered by source, predicate and
// target.
func (g *Graph) RelationsAll() []Relation { return append([]Relation(nil), g.rels...) }

// Relations returns the outgoing relations of a concept, restricted to
// the given predicates when any are given, ordered by predicate and
// target.
func (g *Graph) Relations(id string, preds ...Predicate) []Relation { return filter(g.out[id], preds) }

// Incoming returns the incoming relations of a concept, restricted to
// the given predicates when any are given, ordered by predicate and
// source.
func (g *Graph) Incoming(id string, preds ...Predicate) []Relation { return filter(g.in[id], preds) }

func filter(rs []Relation, preds []Predicate) []Relation {
	var out []Relation
	for _, r := range rs {
		if len(preds) == 0 || containsPred(preds, r.Predicate) {
			out = append(out, r)
		}
	}
	return out
}

func containsPred(ps []Predicate, p Predicate) bool {
	for _, x := range ps {
		if x == p {
			return true
		}
	}
	return false
}

// Query selects concepts in [Graph.FindConcepts]. Empty fields do not
// filter.
type Query struct {
	Kinds []Kind
	// Name matches the concept name case-insensitively as a substring,
	// or exactly when Exact is set.
	Name  string
	Exact bool
	// Standard restricts the result to concepts of one standard, e.g.
	// "IEC 61850-7-4".
	Standard string
}

// FindConcepts returns the concepts matching q, ordered by ID.
func (g *Graph) FindConcepts(q Query) []Concept {
	var out []Concept
	for _, id := range g.ids {
		c := g.concepts[id]
		if len(q.Kinds) > 0 && !containsKind(q.Kinds, c.Kind) {
			continue
		}
		if q.Standard != "" && c.Namespace.Standard != q.Standard {
			continue
		}
		if q.Name != "" {
			if q.Exact && c.Name != q.Name {
				continue
			}
			if !q.Exact && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(q.Name)) {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

func containsKind(ks []Kind, k Kind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

// Walk traverses the graph breadth-first from start along outgoing
// relations with the given predicates (all when none are given),
// visiting each concept once. fn receives the concept and its distance
// from start; returning false stops the walk below that concept.
// Neighbors are visited in [Graph.Relations] order.
func (g *Graph) Walk(start string, fn func(c Concept, depth int) bool, preds ...Predicate) error {
	if _, ok := g.concepts[start]; !ok {
		return fmt.Errorf("%w: %q", model.ErrUnknownConcept, start)
	}
	type item struct {
		id    string
		depth int
	}
	seen := map[string]bool{start: true}
	queue := []item{{start, 0}}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if !fn(g.concepts[it.id], it.depth) {
			continue
		}
		for _, r := range g.Relations(it.id, preds...) {
			if !seen[r.To] {
				seen[r.To] = true
				queue = append(queue, item{r.To, it.depth + 1})
			}
		}
	}
	return nil
}

type export struct {
	Concepts  []Concept  `json:"concepts"`
	Relations []Relation `json:"relations"`
}

// MarshalJSON encodes the graph as {"concepts": [...], "relations":
// [...]}, both in their deterministic order.
func (g *Graph) MarshalJSON() ([]byte, error) {
	return json.Marshal(export{Concepts: g.Concepts(), Relations: g.RelationsAll()})
}

// ExportJSON writes the indented JSON form of the graph to w. The
// output is byte-identical for identical input.
func (g *Graph) ExportJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(export{Concepts: g.Concepts(), Relations: g.RelationsAll()})
}

// Options configures [BuildGraph].
type Options struct {
	// Semantics adds measurement concepts, with their quantities,
	// phases and references, using this mapper. Nil leaves them out.
	Semantics *semantic.Mapper
}

// builder collects concepts and relations.
type builder struct {
	concepts  map[string]Concept
	rels      map[Relation]bool
	conflicts []string
}

// add adds a concept. A concept ID defined twice keeps the first
// definition and is recorded as a conflict.
func (b *builder) add(c Concept) error {
	if prev, dup := b.concepts[c.ID]; dup {
		b.conflicts = append(b.conflicts, fmt.Sprintf("%s is defined by %s and %s", c.ID, prev.Namespace, c.Namespace))
		return nil
	}
	b.concepts[c.ID] = c
	return nil
}

func (b *builder) ensure(c Concept) {
	if _, ok := b.concepts[c.ID]; !ok {
		b.concepts[c.ID] = c
	}
}

func (b *builder) relate(from string, p Predicate, to string) {
	b.rels[Relation{From: from, Predicate: p, To: to}] = true
}

// IDs of model concepts.
func nsID(id model.NamespaceID) string { return "ns:" + id.String() }

// LogicalNodeID returns the concept ID of a logical node class.
func LogicalNodeID(name string) string { return "ln:" + name }

// DataObjectID returns the concept ID of a data object of a class.
func DataObjectID(lnClass, name string) string { return "do:" + lnClass + "." + name }

// CDCID returns the concept ID of a CDC.
func CDCID(name string) string { return "cdc:" + name }

// MeasurementID returns the concept ID of a measurement, e.g.
// "measurement:MMXU.PNV.phsA".
func MeasurementID(path string) string { return "measurement:" + path }

// QuantityID returns the concept ID of a quantity.
func QuantityID(q semantic.Quantity) string { return "quantity:" + string(q) }

// PhaseID returns the concept ID of a phase.
func PhaseID(p semantic.Phase) string { return "phase:" + string(p) }

// BuildGraph builds the concept graph of a profile: namespaces and
// their dependencies; logical node classes with inheritance and data
// objects; CDCs with sub data objects and data attributes; constructed
// types with members; enumerations and basic types. With
// Options.Semantics, measurements are added for every mapped data
// object and phase, linked to quantities, phases and references.
//
// Relations whose target is not defined in the profile are left out;
// [catalog.Profile.Diagnostics] reports such references. Names defined
// by more than one namespace keep their first definition and are listed
// by [Graph.Conflicts].
func BuildGraph(p *catalog.Profile, opts Options) (*Graph, error) {
	b := &builder{concepts: map[string]Concept{}, rels: map[Relation]bool{}}
	nss := p.Namespaces()
	for _, ns := range nss {
		if err := b.add(Concept{ID: nsID(ns.ID), Kind: KindNamespace, Name: ns.ID.String(), Label: ns.Doc.Title, Namespace: ns.ID}); err != nil {
			return nil, err
		}
	}
	for _, ns := range nss {
		if err := b.addNamespace(ns); err != nil {
			return nil, err
		}
	}
	for _, ns := range nss {
		b.relateNamespace(ns)
	}
	if opts.Semantics != nil {
		if err := b.addMeasurements(p, opts.Semantics); err != nil {
			return nil, err
		}
	}
	return b.graph(), nil
}

func presenceProps(p model.PresenceCondition) []Property {
	if p.IsZero() {
		return nil
	}
	return []Property{{Name: "presence", Value: p.String()}}
}

func (b *builder) addNamespace(ns *model.Namespace) error {
	var err error
	add := func(c Concept) {
		if err == nil {
			c.Namespace = ns.ID
			err = b.add(c)
		}
	}
	for _, lns := range [][]model.LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
		for i := range lns {
			ln := &lns[i]
			c := Concept{ID: LogicalNodeID(ln.Name), Kind: KindLogicalNode, Name: ln.Name, Label: ln.Doc.Title}
			if ln.Abstract {
				c.Properties = append(c.Properties, Property{Name: "abstract", Value: "true"})
			} else if g := ln.Group(); g.Label() != "" {
				c.Properties = append(c.Properties, Property{Name: "group", Value: g.String() + " " + g.Label()})
			}
			add(c)
			for j := range ln.DataObjects {
				do := &ln.DataObjects[j]
				add(Concept{ID: DataObjectID(ln.Name, do.Name), Kind: KindDataObject, Name: do.Name, Properties: presenceProps(do.Presence)})
			}
		}
	}
	for i := range ns.CDCs {
		c := &ns.CDCs[i]
		add(Concept{ID: CDCID(c.Name), Kind: KindCDC, Name: c.Name, Label: c.Doc.Title})
		for j := range c.SubDataObjects {
			s := &c.SubDataObjects[j]
			add(Concept{ID: "sdo:" + c.Name + "." + s.Name, Kind: KindSubDataObject, Name: s.Name, Properties: presenceProps(s.Presence)})
		}
		for j := range c.DataAttributes {
			a := &c.DataAttributes[j]
			props := append([]Property{{Name: "fc", Value: string(a.FC)}}, presenceProps(a.Presence)...)
			add(Concept{ID: "da:" + c.Name + "." + a.Name, Kind: KindDataAttribute, Name: a.Name, Properties: props})
		}
	}
	for i := range ns.ConstructedAttributes {
		c := &ns.ConstructedAttributes[i]
		add(Concept{ID: "ca:" + c.Name, Kind: KindConstructedAttribute, Name: c.Name, Label: c.Doc.Title})
		for j := range c.SubDataAttributes {
			a := &c.SubDataAttributes[j]
			add(Concept{ID: "da:" + c.Name + "." + a.Name, Kind: KindDataAttribute, Name: a.Name, Properties: presenceProps(a.Presence)})
		}
	}
	for i := range ns.Enumerations {
		e := &ns.Enumerations[i]
		add(Concept{ID: "enum:" + e.Name, Kind: KindEnumeration, Name: e.Name, Label: e.Doc.Title})
	}
	for i := range ns.BasicTypes {
		t := &ns.BasicTypes[i]
		add(Concept{ID: "basic:" + t.Name, Kind: KindBasicType, Name: t.Name})
	}
	return err
}

// typeTarget returns the concept ID of a type name, by declared kind
// or, without one, by the definition that exists.
func (b *builder) typeTarget(name string, kind model.TypeKind) string {
	candidates := []string{"basic:" + name, "ca:" + name, "enum:" + name}
	switch kind {
	case model.TypeKindBasic:
		candidates = candidates[:1]
	case model.TypeKindConstructed:
		candidates = candidates[1:2]
	case model.TypeKindEnumerated:
		candidates = candidates[2:]
	default:
		// Undefined or other kinds: whichever definition exists.
	}
	for _, id := range candidates {
		if _, ok := b.concepts[id]; ok {
			return id
		}
	}
	return ""
}

func (b *builder) relateIfExists(from string, p Predicate, to string) {
	if to == "" {
		return
	}
	if _, ok := b.concepts[to]; ok {
		b.relate(from, p, to)
	}
}

func (b *builder) relateNamespace(ns *model.Namespace) {
	nid := nsID(ns.ID)
	for _, dep := range ns.Dependencies {
		for id, c := range b.concepts {
			if c.Kind == KindNamespace && c.Namespace.Satisfies(dep) {
				b.relate(nid, DependsOn, id)
			}
		}
	}
	for _, lns := range [][]model.LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
		for i := range lns {
			ln := &lns[i]
			id := LogicalNodeID(ln.Name)
			b.relate(id, DefinedIn, nid)
			b.relateIfExists(id, InheritsFrom, LogicalNodeID(ln.Base))
			for j := range ln.DataObjects {
				do := &ln.DataObjects[j]
				did := DataObjectID(ln.Name, do.Name)
				b.relate(id, Contains, did)
				b.relateIfExists(did, Uses, CDCID(do.Type))
				if do.UnderlyingType != "" {
					b.relateIfExists(did, HasType, b.typeTarget(do.UnderlyingType, do.UnderlyingTypeKind))
				}
			}
		}
	}
	for i := range ns.CDCs {
		c := &ns.CDCs[i]
		id := CDCID(c.Name)
		b.relate(id, DefinedIn, nid)
		for j := range c.SubDataObjects {
			s := &c.SubDataObjects[j]
			sid := "sdo:" + c.Name + "." + s.Name
			b.relate(id, Contains, sid)
			b.relateIfExists(sid, Uses, CDCID(s.Type))
		}
		for j := range c.DataAttributes {
			a := &c.DataAttributes[j]
			aid := "da:" + c.Name + "." + a.Name
			b.relate(id, Contains, aid)
			if a.Type != "" {
				b.relateIfExists(aid, HasType, b.typeTarget(a.Type, a.TypeKind))
			}
		}
	}
	for i := range ns.ConstructedAttributes {
		c := &ns.ConstructedAttributes[i]
		id := "ca:" + c.Name
		b.relate(id, DefinedIn, nid)
		for j := range c.SubDataAttributes {
			a := &c.SubDataAttributes[j]
			aid := "da:" + c.Name + "." + a.Name
			b.relate(id, Contains, aid)
			b.relateIfExists(aid, HasType, b.typeTarget(a.Type, a.TypeKind))
		}
	}
	for i := range ns.Enumerations {
		e := &ns.Enumerations[i]
		id := "enum:" + e.Name
		b.relate(id, DefinedIn, nid)
		if e.InheritedFrom != "" {
			b.relateIfExists(id, InheritsFrom, "enum:"+e.InheritedFrom)
		}
	}
	for i := range ns.BasicTypes {
		b.relate("basic:"+ns.BasicTypes[i].Name, DefinedIn, nid)
	}
}

func (b *builder) addMeasurements(p *catalog.Profile, m *semantic.Mapper) error {
	for _, ln := range p.ListLogicalNodes() {
		ds, err := m.FindMeasurements(p, ln.Name)
		if errors.Is(err, model.ErrAmbiguousReference) {
			b.conflicts = append(b.conflicts, fmt.Sprintf("measurements of %s not mapped: %v", ln.Name, err))
			continue
		}
		if err != nil {
			return err
		}
		for _, d := range ds {
			path := d.LNClass + "." + d.DataObject
			carrier := DataObjectID(d.LNClass, d.DataObject)
			if _, ok := b.concepts[carrier]; !ok {
				// Inherited data object: the carrier is declared by a base class.
				if ido, err := p.DataObject(d.LNClass, d.DataObject); err == nil {
					carrier = DataObjectID(ido.DeclaredIn.Name, d.DataObject)
				}
			}
			if d.SubDataObject != "" {
				path += "." + d.SubDataObject
				carrier = "sdo:" + d.CDC + "." + d.SubDataObject
			}
			id := MeasurementID(path)
			label := d.Quantity.Label()
			if d.Phase != "" {
				label += ", phase " + string(d.Phase)
			}
			if d.Reference != "" {
				label += ", " + string(d.Reference)
			}
			if d.Direction != "" {
				label += ", " + string(d.Direction)
			}
			props := []Property{
				{Name: "lnClass", Value: d.LNClass},
				{Name: "rule", Value: d.Rule},
				{Name: "source", Value: d.Source},
				{Name: "confidence", Value: string(d.Confidence)},
				{Name: "basis", Value: string(d.Basis)},
			}
			if !d.UnitHint.IsZero() {
				props = append(props, Property{Name: "unitHint", Value: d.UnitHint.String()})
			}
			if err := b.add(Concept{ID: id, Kind: KindMeasurement, Name: path, Label: label, Properties: props}); err != nil {
				return err
			}
			b.ensure(Concept{ID: QuantityID(d.Quantity), Kind: KindQuantity, Name: string(d.Quantity), Label: d.Quantity.Label()})
			b.relate(id, Represents, QuantityID(d.Quantity))
			b.relateIfExists(id, MeasuredBy, carrier)
			if d.Phase != "" {
				b.ensure(Concept{ID: PhaseID(d.Phase), Kind: KindPhase, Name: string(d.Phase)})
				b.relate(id, HasPhase, PhaseID(d.Phase))
			}
			if d.Reference != "" {
				rid := "reference:" + string(d.Reference)
				b.ensure(Concept{ID: rid, Kind: KindReference, Name: string(d.Reference)})
				b.relate(id, HasReference, rid)
			}
		}
	}
	return nil
}

func (b *builder) graph() *Graph {
	sort.Strings(b.conflicts)
	b.conflicts = slices.Compact(b.conflicts)
	g := &Graph{concepts: b.concepts, conflicts: b.conflicts, out: map[string][]Relation{}, in: map[string][]Relation{}}
	for id := range b.concepts {
		g.ids = append(g.ids, id)
	}
	sort.Strings(g.ids)
	for r := range b.rels {
		g.rels = append(g.rels, r)
	}
	sort.Slice(g.rels, func(i, j int) bool {
		a, c := g.rels[i], g.rels[j]
		if a.From != c.From {
			return a.From < c.From
		}
		if a.Predicate != c.Predicate {
			return a.Predicate < c.Predicate
		}
		return a.To < c.To
	})
	for _, r := range g.rels {
		g.out[r.From] = append(g.out[r.From], r)
	}
	for _, r := range g.rels {
		g.in[r.To] = append(g.in[r.To], r)
	}
	for id, rs := range g.in {
		sort.SliceStable(rs, func(i, j int) bool {
			if rs[i].Predicate != rs[j].Predicate {
				return rs[i].Predicate < rs[j].Predicate
			}
			return rs[i].From < rs[j].From
		})
		g.in[id] = rs
	}
	return g
}
