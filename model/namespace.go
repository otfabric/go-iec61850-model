// SPDX-License-Identifier: MIT

package model

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// NamespaceID identifies one release of an IEC 61850 namespace, as
// declared by the id, version, revision and release attributes of an
// NSD document.
//
// The canonical string form is "<Standard>:<Version><Revision><Release>",
// e.g. "IEC 61850-7-4:2007B5". The release is omitted when zero, e.g.
// "IEC 61850-7-4:2007B".
type NamespaceID struct {
	// Standard is the namespace name, e.g. "IEC 61850-7-4".
	Standard string `json:"standard"`
	// Version is the namespace version year, e.g. "2007".
	Version string `json:"version"`
	// Revision is the namespace revision letter, e.g. "B".
	Revision string `json:"revision"`
	// Release is the namespace release number; 0 when not declared.
	Release int `json:"release,omitempty"`
}

// ParseNamespaceID parses the canonical string form produced by
// [NamespaceID.String]. A bare standard without ":" yields an ID with
// only Standard set.
func ParseNamespaceID(s string) (NamespaceID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return NamespaceID{}, fmt.Errorf("%w: empty namespace ID", ErrUnknownNamespace)
	}
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return NamespaceID{Standard: s}, nil
	}
	std, ver := strings.TrimSpace(s[:idx]), s[idx+1:]
	if std == "" {
		return NamespaceID{}, fmt.Errorf("%w: empty standard in %q", ErrUnknownNamespace, s)
	}
	i := 0
	for i < len(ver) && ver[i] >= '0' && ver[i] <= '9' {
		i++
	}
	j := i
	for j < len(ver) && ((ver[j] >= 'A' && ver[j] <= 'Z') || (ver[j] >= 'a' && ver[j] <= 'z')) {
		j++
	}
	id := NamespaceID{Standard: std, Version: ver[:i], Revision: ver[i:j]}
	if id.Version == "" {
		return NamespaceID{}, fmt.Errorf("%w: missing version in %q", ErrUnknownNamespace, s)
	}
	if j < len(ver) {
		rel, err := strconv.Atoi(ver[j:])
		if err != nil || rel < 0 {
			return NamespaceID{}, fmt.Errorf("%w: invalid release in %q", ErrUnknownNamespace, s)
		}
		id.Release = rel
	}
	return id, nil
}

// String returns the canonical form, e.g. "IEC 61850-7-4:2007B5".
func (id NamespaceID) String() string {
	if id.Version == "" && id.Revision == "" && id.Release == 0 {
		return id.Standard
	}
	s := id.Standard + ":" + id.Version + id.Revision
	if id.Release > 0 {
		s += strconv.Itoa(id.Release)
	}
	return s
}

// IsZero reports whether id is the zero value.
func (id NamespaceID) IsZero() bool { return id == NamespaceID{} }

// SameVersion reports whether id and other name the same standard,
// version and revision, ignoring the release.
func (id NamespaceID) SameVersion(other NamespaceID) bool {
	return id.Standard == other.Standard && id.Version == other.Version && id.Revision == other.Revision
}

// Satisfies reports whether id satisfies a dependency declaration. The
// standard, version and revision must be equal; a dependency that
// declares a release is satisfied by that release or a later one, since
// releases of one version and revision only add corrections.
func (id NamespaceID) Satisfies(dep NamespaceID) bool {
	if !id.SameVersion(dep) {
		return false
	}
	return id.Release >= dep.Release
}

// Compare orders IDs by standard, version, revision and release. It
// returns -1, 0 or +1 and gives catalogs a stable order; it does not
// imply that a higher ID is preferable or compatible.
func (id NamespaceID) Compare(other NamespaceID) int {
	if c := cmp.Compare(id.Standard, other.Standard); c != 0 {
		return c
	}
	if c := cmp.Compare(id.Version, other.Version); c != 0 {
		return c
	}
	if c := cmp.Compare(id.Revision, other.Revision); c != 0 {
		return c
	}
	return cmp.Compare(id.Release, other.Release)
}

// Namespace is one parsed NSD namespace: the normalized content of one
// NS document. Definitions keep their document order.
//
// A Namespace held by a catalog must be treated as read-only.
type Namespace struct {
	// ID identifies the namespace release.
	ID NamespaceID `json:"id"`
	// PublicationStage is the declared stage, e.g. "IS" or "FDIS".
	PublicationStage string `json:"publicationStage,omitempty"`
	// UMLVersion and UMLDate identify the UML model the NSD was
	// generated from, when declared.
	UMLVersion string `json:"umlVersion,omitempty"`
	UMLDate    string `json:"umlDate,omitempty"`
	// TissuesApplied lists the technical issues applied, when declared.
	TissuesApplied string `json:"tissuesApplied,omitempty"`
	// NamespaceType and AppVersion are declared by some namespaces,
	// e.g. technical reports, verbatim.
	NamespaceType string `json:"namespaceType,omitempty"`
	AppVersion    string `json:"appVersion,omitempty"`
	// NSDVersion is the version of the NSD format the document was
	// written for, e.g. "2017B6" (nsdVersion, nsdRevision, nsdRelease).
	NSDVersion string `json:"nsdVersion,omitempty"`
	// Changes lists the change records of the namespace, in order.
	Changes []NamespaceChange `json:"changes,omitempty"`
	// Copyright is the copyright notice and license the namespace
	// document declares. Redistributed code components must retain it.
	Copyright Copyright `json:"copyright,omitzero"`
	// Doc holds the namespace-level documentation references.
	Doc Doc `json:"doc,omitzero"`

	// Dependencies are the namespaces this namespace depends on, in
	// declaration order.
	Dependencies []NamespaceID `json:"dependencies,omitempty"`
	// DependencyTypes holds the declared dependencyType of a
	// dependency, by dependency standard, verbatim.
	DependencyTypes map[string]string `json:"dependencyTypes,omitempty"`

	BasicTypes            []BasicType               `json:"basicTypes,omitempty"`
	FunctionalConstraints []FunctionalConstraintDef `json:"functionalConstraints,omitempty"`
	TriggerOptions        []TriggerOption           `json:"triggerOptions,omitempty"`
	PresenceConditions    []PresenceConditionDef    `json:"presenceConditions,omitempty"`
	Abbreviations         []Abbreviation            `json:"abbreviations,omitempty"`
	Enumerations          []Enumeration             `json:"enumerations,omitempty"`
	ConstructedAttributes []ConstructedAttribute    `json:"constructedAttributes,omitempty"`
	CDCs                  []CommonDataClass         `json:"cdcs,omitempty"`
	// AbstractLNClasses are the abstract logical node classes that
	// concrete classes inherit from (e.g. DomainLN, StatisticsLN).
	AbstractLNClasses []LogicalNodeClass `json:"abstractLNClasses,omitempty"`
	// LNClasses are the concrete logical node classes (e.g. MMXU).
	LNClasses []LogicalNodeClass `json:"lnClasses,omitempty"`

	// Provenance records where the namespace was loaded from.
	Provenance Provenance `json:"provenance,omitzero"`
}

// Copyright is the copyright block of an NSD document or package
// manifest.
type Copyright struct {
	// Notice is the copyright notice, with white space normalized.
	Notice string `json:"notice,omitempty"`
	// LicenseURI and LicenseKind identify the license, e.g.
	// "www.iec.ch/CCv1" and "Standard".
	LicenseURI  string `json:"licenseURI,omitempty"`
	LicenseKind string `json:"licenseKind,omitempty"`
}

// IsZero reports whether no copyright is declared.
func (c Copyright) IsZero() bool { return c == Copyright{} }

// NamespaceChange is a change record of a namespace: the namespace
// version it refers to and the technical issues it applies.
type NamespaceChange struct {
	Version   string `json:"version,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Release   string `json:"release,omitempty"`
	Tissues   string `json:"tissues,omitempty"`
	ChangesID string `json:"changesID,omitempty"`
	Date      string `json:"date,omitempty"`
}

// LogicalNodeClass returns the concrete or abstract logical node class
// with the given name.
func (ns *Namespace) LogicalNodeClass(name string) (*LogicalNodeClass, bool) {
	if ln := find(ns.LNClasses, name, func(l *LogicalNodeClass) string { return l.Name }); ln != nil {
		return ln, true
	}
	ln := find(ns.AbstractLNClasses, name, func(l *LogicalNodeClass) string { return l.Name })
	return ln, ln != nil
}

// CDC returns the common data class with the given name. For a CDC
// defined in variants it returns the first variant; see
// [Namespace.CDCVariants].
func (ns *Namespace) CDC(name string) (*CommonDataClass, bool) {
	c := find(ns.CDCs, name, func(c *CommonDataClass) string { return c.Name })
	return c, c != nil
}

// CDCVariants returns every definition of the CDC with the given name:
// one, or one per variant (e.g. SPG for SP, SG and SE).
func (ns *Namespace) CDCVariants(name string) []*CommonDataClass {
	var out []*CommonDataClass
	for i := range ns.CDCs {
		if ns.CDCs[i].Name == name {
			out = append(out, &ns.CDCs[i])
		}
	}
	return out
}

// ConstructedAttribute returns the constructed attribute type with the
// given name.
func (ns *Namespace) ConstructedAttribute(name string) (*ConstructedAttribute, bool) {
	c := find(ns.ConstructedAttributes, name, func(c *ConstructedAttribute) string { return c.Name })
	return c, c != nil
}

// Enumeration returns the enumeration with the given name.
func (ns *Namespace) Enumeration(name string) (*Enumeration, bool) {
	e := find(ns.Enumerations, name, func(e *Enumeration) string { return e.Name })
	return e, e != nil
}

// BasicType returns the basic type with the given name.
func (ns *Namespace) BasicType(name string) (*BasicType, bool) {
	b := find(ns.BasicTypes, name, func(b *BasicType) string { return b.Name })
	return b, b != nil
}

// Namespaces returns ns as a one-element slice. It lets a single
// namespace be passed wherever a source of namespaces is accepted.
func (ns *Namespace) Namespaces() []*Namespace { return []*Namespace{ns} }

func find[T any](items []T, name string, key func(*T) string) *T {
	for i := range items {
		if key(&items[i]) == name {
			return &items[i]
		}
	}
	return nil
}

// Doc holds the documentation identifiers of a definition and, when an
// NSDoc file was applied, the resolved texts.
//
// The texts originate from IEC NSDoc files. They are available at run
// time but are not redistributable; see docs/licensing.md.
type Doc struct {
	// TitleID references the title text in an NSDoc file.
	TitleID string `json:"titleID,omitempty"`
	// DescID references the description text in an NSDoc file.
	DescID string `json:"descID,omitempty"`
	// Title is the resolved title, empty when no NSDoc was applied.
	Title string `json:"title,omitempty"`
	// Description is the resolved description, empty when no NSDoc
	// was applied.
	Description string `json:"description,omitempty"`
}

// BasicType is a basic (primitive) attribute type, e.g. BOOLEAN,
// INT32 or FLOAT32.
type BasicType struct {
	Name   string      `json:"name"`
	Doc    Doc         `json:"doc,omitzero"`
	Source NamespaceID `json:"source"`
}

// FunctionalConstraintDef is the namespace definition of a functional
// constraint.
type FunctionalConstraintDef struct {
	Abbreviation FunctionalConstraint `json:"abbreviation"`
	// Services lists the services applicable to data of this FC.
	Services []string `json:"services,omitempty"`
	// DataSetMemberOf lists the control blocks whose data sets may
	// contain data of this FC, e.g. GoCB.
	DataSetMemberOf []string    `json:"dataSetMemberOf,omitempty"`
	Doc             Doc         `json:"doc,omitzero"`
	Source          NamespaceID `json:"source"`
}

// TriggerOption is the namespace definition of a trigger option, e.g.
// dchg, qchg or dupd.
type TriggerOption struct {
	Abbreviation string      `json:"abbreviation"`
	Doc          Doc         `json:"doc,omitzero"`
	Source       NamespaceID `json:"source"`
}

// PresenceConditionDef is the namespace definition of a presence
// condition, e.g. M, O or AtLeastOne.
type PresenceConditionDef struct {
	Name string `json:"name"`
	// Argument describes the argument the condition takes, if any.
	Argument string      `json:"argument,omitempty"`
	Doc      Doc         `json:"doc,omitzero"`
	Source   NamespaceID `json:"source"`
}

// Abbreviation is a name-part abbreviation used to build data object
// names, e.g. "Tot" or "Hz".
type Abbreviation struct {
	Name   string      `json:"name"`
	Doc    Doc         `json:"doc,omitzero"`
	Source NamespaceID `json:"source"`
}

// Docs returns pointers to the documentation of the namespace and of
// every definition and member in it, in a stable order. Use it to fill
// or clear documentation texts.
func (ns *Namespace) Docs() []*Doc {
	out := []*Doc{&ns.Doc}
	for i := range ns.BasicTypes {
		out = append(out, &ns.BasicTypes[i].Doc)
	}
	for i := range ns.FunctionalConstraints {
		out = append(out, &ns.FunctionalConstraints[i].Doc)
	}
	for i := range ns.TriggerOptions {
		out = append(out, &ns.TriggerOptions[i].Doc)
	}
	for i := range ns.PresenceConditions {
		out = append(out, &ns.PresenceConditions[i].Doc)
	}
	for i := range ns.Abbreviations {
		out = append(out, &ns.Abbreviations[i].Doc)
	}
	for i := range ns.Enumerations {
		e := &ns.Enumerations[i]
		out = append(out, &e.Doc)
		for j := range e.Literals {
			out = append(out, &e.Literals[j].Doc)
		}
	}
	for i := range ns.ConstructedAttributes {
		c := &ns.ConstructedAttributes[i]
		out = append(out, &c.Doc)
		for j := range c.SubDataAttributes {
			out = append(out, &c.SubDataAttributes[j].Doc)
		}
	}
	for i := range ns.CDCs {
		c := &ns.CDCs[i]
		out = append(out, &c.Doc)
		for j := range c.SubDataObjects {
			out = append(out, &c.SubDataObjects[j].Doc)
		}
		for j := range c.DataAttributes {
			out = append(out, &c.DataAttributes[j].Doc)
		}
		for j := range c.ServiceParameters {
			out = append(out, &c.ServiceParameters[j].Doc)
		}
	}
	for _, lns := range [][]LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
		for i := range lns {
			out = append(out, &lns[i].Doc)
			for j := range lns[i].DataObjects {
				out = append(out, &lns[i].DataObjects[j].Doc)
			}
		}
	}
	return out
}
