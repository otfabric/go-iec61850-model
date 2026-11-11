// SPDX-License-Identifier: MIT

package presence

import (
	"fmt"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Requirement is what a presence condition requires of one element in
// a given instance.
type Requirement string

// Requirements.
const (
	// Required: the element shall be present.
	Required Requirement = "required"
	// Optional: the element may be present or absent.
	Optional Requirement = "optional"
	// Forbidden: the element shall be absent.
	Forbidden Requirement = "forbidden"
	// Undetermined: the requirement depends on a fact the instance does
	// not provide; [Result.Missing] names it.
	Undetermined Requirement = "undetermined"
)

// Verdict says whether an instance meets the requirement of an element.
type Verdict string

// Verdicts.
const (
	// Satisfied: the instance meets the requirement.
	Satisfied Verdict = "satisfied"
	// Violated: the instance does not meet the requirement.
	Violated Verdict = "violated"
	// Unresolved: the verdict depends on a missing fact; it is shown as
	// "undetermined".
	Unresolved Verdict = "undetermined"
)

// Kind is the kind of model element a condition applies to.
type Kind string

// Element kinds.
const (
	KindDataObject       Kind = "DataObject"
	KindSubDataObject    Kind = "SubDataObject"
	KindDataAttribute    Kind = "DataAttribute"
	KindSubDataAttribute Kind = "SubDataAttribute"
	KindServiceParameter Kind = "ServiceParameter"
)

// Element is one child of a class, CDC or constructed type together
// with its presence condition: the unit [Evaluate] works on.
type Element struct {
	// Name is the model name. Leading underscores of per-FC attribute
	// copies (_setMag) are ignored when matching instance names.
	Name string
	// Kind is the element kind.
	Kind Kind
	// Type is the CDC of a data object, or the type of an attribute.
	Type string
	// TypeKind is the kind of an attribute type.
	TypeKind model.TypeKind
	// FC is the functional constraint of a data attribute.
	FC model.FunctionalConstraint
	// Presence is the presence condition.
	Presence model.PresenceCondition
	// Namespace is the namespace that declares the element: the
	// namespace of the declaring class for a data object, of the CDC
	// for an attribute. MOdataNs and MOcdcNs compare it.
	Namespace model.NamespaceID
}

// Instance describes what exists of one logical node, data object or
// constructed attribute: the input of the evaluation.
type Instance struct {
	// Class optionally names the logical node class, CDC or
	// constructed type of a top-level instance (nsdctl model check
	// reads it); children take theirs from the model.
	Class string `json:"class,omitempty"`
	// Present names the children that exist, by their instance names:
	// data objects with their instance number ("Ind3"), attributes and
	// sub data objects. A dotted entry ("mag.i") records a member of a
	// present constructed attribute.
	Present []string `json:"present"`
	// Values holds attribute values that conditions read, by name:
	// "ctlModel" (CtlModelKind, name or number) for MOsbo and
	// MOenhanced, "hvRef" (HvReferenceKind) for MOrms. An attribute
	// with a value is present.
	Values map[string]string `json:"values,omitempty"`
	// Context carries the facts about the surroundings of the instance.
	// Children inherit it; their own Context fields override.
	Context Context `json:"context,omitzero"`
	// Children describe present children further, by instance name:
	// the attributes of a data object, the members of a constructed
	// attribute. A described child is present; a present child without
	// an entry is checked only for its own presence.
	Children map[string]*Instance `json:"children,omitempty"`
}

// Context is what the evaluator knows about the surroundings of an
// instance. Empty fields are unknown, never defaulted.
type Context struct {
	// LNClass is the class of the logical node the instance belongs to
	// (MOln0, MFln0: "LLN0" or another class). The logical node
	// evaluation sets it.
	LNClass string `json:"lnClass,omitempty"`
	// RootLD reports whether the logical device is the root logical
	// device (MOrootLD); nil is unknown.
	RootLD *bool `json:"rootLD,omitempty"`
	// LDNamespace is the namespace the logical device declares (LLN0
	// NamPlt.ldNs), e.g. "IEC 61850-7-4:2007B".
	LDNamespace string `json:"ldNs,omitempty"`
	// LNNamespace is the namespace of the logical node: its NamPlt.lnNs,
	// or the namespace that defines its class (MOlnNs, MOdataNs). The
	// logical node evaluation sets it from the class when it is empty.
	LNNamespace string `json:"lnNs,omitempty"`
	// DataNamespace is the namespace of the data object (its dataNs, or
	// the namespace of the class that declares it) for MOdataNs and
	// MOcdcNs. The logical node evaluation sets it per data object.
	DataNamespace string `json:"dataNs,omitempty"`
	// CtlModel is the control model when [Instance.Values] has no
	// ctlModel.
	CtlModel string `json:"ctlModel,omitempty"`
	// Facts state whether a condition holds, keyed by the condition as
	// printed ("MOcond(1)", "MFsubst", "MOoperTm") or by element and
	// condition ("Mir:MOcond(1)"), which wins. A fact overrides what the
	// evaluator would derive.
	Facts map[string]bool `json:"facts,omitempty"`
}

// Result is the evaluation of one element.
type Result struct {
	// Path is the element path relative to the evaluated instance, e.g.
	// "Pos.sboTimeout".
	Path string `json:"path"`
	// Element is the evaluated element.
	Element string `json:"element"`
	// Kind is the element kind.
	Kind Kind `json:"kind"`
	// Condition is the presence condition.
	Condition model.PresenceCondition `json:"condition"`
	// Present reports whether the instance has the element.
	Present bool `json:"present"`
	// Instances are the instance names of a multi-instance element
	// ("Ind1", "Ind2").
	Instances []string `json:"instances,omitempty"`
	// Requirement is what the condition requires in this instance.
	Requirement Requirement `json:"requirement"`
	// Verdict says whether the instance meets it.
	Verdict Verdict `json:"verdict"`
	// Reason explains the requirement and the verdict.
	Reason string `json:"reason"`
	// Missing names the fact an undetermined requirement needs, e.g.
	// "ctlModel" or "fact MOcond(1)".
	Missing string `json:"missing,omitempty"`
}

// String formats the result as "path: condition: requirement, verdict
// (reason)".
func (r Result) String() string {
	return fmt.Sprintf("%s: %s: %s, %s (%s)", r.Path, r.Condition, r.Requirement, r.Verdict, r.Reason)
}

// Report is the evaluation of one instance.
type Report struct {
	// Path is the path of the instance relative to the top level, empty
	// for the top level.
	Path string `json:"path,omitempty"`
	// Class is the logical node class, CDC (Key form, e.g. "SPG/SG")
	// or constructed type.
	Class string `json:"class"`
	// Results has one entry per element of the class, in model order.
	Results []Result `json:"results"`
	// Unknown lists present names that match no element of the class,
	// e.g. a data object from an extension namespace that the profile
	// does not load.
	Unknown []string `json:"unknown,omitempty"`
	// Children are the reports of the described children
	// ([Instance.Children]), in model order.
	Children []*Report `json:"children,omitempty"`
}

// All returns the results of the report and of every child report.
func (r *Report) All() []Result {
	out := append([]Result(nil), r.Results...)
	for _, c := range r.Children {
		out = append(out, c.All()...)
	}
	return out
}

func (r *Report) filter(keep func(Result) bool) []Result {
	var out []Result
	for _, res := range r.All() {
		if keep(res) {
			out = append(out, res)
		}
	}
	return out
}

// Violations returns the violated results, children included.
func (r *Report) Violations() []Result {
	return r.filter(func(x Result) bool { return x.Verdict == Violated })
}

// Undetermined returns the results whose verdict is undetermined,
// children included.
func (r *Report) Undetermined() []Result {
	return r.filter(func(x Result) bool { return x.Verdict == Unresolved })
}

// OK reports whether no result is violated. Undetermined results and
// unknown names do not count.
func (r *Report) OK() bool { return len(r.Violations()) == 0 }

// Err returns a [*model.PresenceError] listing the violations, or nil.
func (r *Report) Err() error {
	vs := r.Violations()
	if len(vs) == 0 {
		return nil
	}
	msgs := make([]string, len(vs))
	for i, v := range vs {
		msgs[i] = fmt.Sprintf("%s: %s: %s", v.Path, v.Condition, v.Reason)
	}
	return &model.PresenceError{Class: r.Class, Violations: msgs}
}

func joinPath(base, name string) string {
	if base == "" {
		return name
	}
	return base + "." + name
}

// baseName strips the leading underscores of per-FC attribute copies.
func baseName(name string) string { return strings.TrimLeft(name, "_") }
