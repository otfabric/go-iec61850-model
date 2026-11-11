// SPDX-License-Identifier: MIT

package presence

import (
	"sort"
	"strconv"
	"strings"
)

// Family groups presence conditions by what they depend on.
type Family string

// Condition families.
const (
	// FamilyUnconditional conditions depend on nothing: M, O, F, na and
	// the multi-instance conditions.
	FamilyUnconditional Family = "unconditional"
	// FamilySibling conditions depend on the presence of the element
	// their argument names.
	FamilySibling Family = "sibling"
	// FamilyGroup conditions relate the elements that share the
	// condition and its group number.
	FamilyGroup Family = "group"
	// FamilyContext conditions depend on where the instance sits, on an
	// attribute value or on other elements of the instance.
	FamilyContext Family = "context"
	// FamilyFact conditions depend on a fact the model does not show;
	// they are undetermined unless [Context.Facts] states it.
	FamilyFact Family = "fact"
)

// Condition describes how the evaluator treats one presence condition.
type Condition struct {
	// Name is the NSD condition name, e.g. "MOsbo".
	Name string
	// Family is what the condition depends on.
	Family Family
	// Multi marks a multi-instance condition.
	Multi bool
	// Argument describes the argument: "" (none), "n" (group number),
	// "sibling", "condID" or "min, max".
	Argument string
	// Holds says what makes a two-way condition hold, e.g. "ctlModel
	// selects select-before-operate"; empty for the others.
	Holds string
	// IfHolds and Otherwise are the requirements of a two-way condition
	// (MO: Required and Optional); for the others IfHolds is the fixed
	// requirement, or empty for group conditions.
	IfHolds, Otherwise Requirement
	// Needs names the input the condition reads: an attribute value
	// ("ctlModel"), a context field ("lnClass", "rootLD", "ldNs",
	// "lnNs", "dataNs") or "fact" for [Context.Facts]; empty when the
	// model and the present elements suffice.
	Needs string
}

// rule is the evaluator of one condition name.
type rule struct {
	Condition
	// derive returns whether the condition holds for element i, and
	// why; ok is false when it cannot tell, missing then names the
	// input. Nil for unconditional, sibling and fact-only rules.
	derive func(e *evaluation, i int) (holds, ok bool, why, missing string)
	// group evaluates group conditions instead.
	group func(e *evaluation, i int) (Requirement, string)
}

const (
	argNone    = ""
	argGroup   = "n"
	argSibling = "sibling"
	argCondID  = "condID"
	argRange   = "min, max"
)

var rules = map[string]*rule{}

func add(r *rule) { rules[r.Name] = r }

func init() {
	fixed := func(name string, req Requirement, multi bool, arg string) {
		add(&rule{Condition: Condition{Name: name, Family: FamilyUnconditional, Multi: multi, Argument: arg, IfHolds: req}})
	}
	fixed("M", Required, false, argNone)
	fixed("O", Optional, false, argNone)
	fixed("F", Forbidden, false, argNone)
	fixed("na", Optional, false, argNone)
	fixed("Mmulti", Required, true, argNone)
	fixed("Omulti", Optional, true, argNone)
	fixed("MmultiRange", Required, true, argRange)
	fixed("OmultiRange", Optional, true, argRange)

	sibling := func(name string, t, f Requirement, multi bool) {
		add(&rule{Condition: Condition{Name: name, Family: FamilySibling, Multi: multi, Argument: argSibling,
			Holds: "the named sibling is present", IfHolds: t, Otherwise: f}})
	}
	sibling("MF", Required, Forbidden, false)
	sibling("MO", Required, Optional, false)
	sibling("OF", Optional, Forbidden, false)
	sibling("OM", Optional, Required, false)
	sibling("FM", Forbidden, Required, false)
	sibling("MmultiF", Required, Forbidden, true)

	group := func(name, arg string, fn func(e *evaluation, i int) (Requirement, string)) {
		add(&rule{Condition: Condition{Name: name, Family: FamilyGroup, Argument: arg}, group: fn})
	}
	group("AtLeastOne", argGroup, atLeastOne)
	group("AtMostOne", argNone, atMostOne)
	group("AllOrNonePerGroup", argGroup, allOrNone)
	group("AllOnlyOneGroup", argGroup, allOnlyOne)
	group("AllAtLeastOneGroup", argGroup, allAtLeastOne)
	add(&rule{Condition: Condition{Name: "MAllOrNonePerGroup", Family: FamilyGroup, Argument: argGroup,
		Holds: "the controllable data object has status information", IfHolds: Required, Otherwise: Optional,
		Needs: "fact"}, group: mAllOrNone})

	// Every context condition is mandatory when it holds.
	ctx := func(name, holds string, f Requirement, needs string,
		fn func(e *evaluation, i int) (bool, bool, string, string)) {
		add(&rule{Condition: Condition{Name: name, Family: FamilyContext, Holds: holds,
			IfHolds: Required, Otherwise: f, Needs: needs}, derive: fn})
	}
	ctx("MOln0", "the logical node is LLN0", Optional, "lnClass", isLN0)
	ctx("MFln0", "the logical node is LLN0", Forbidden, "lnClass", isLN0)
	ctx("MOrootLD", "the logical device is the root logical device", Optional, "rootLD", isRootLD)
	ctx("MOlnNs", "the logical node namespace differs from the logical device namespace", Optional, "lnNs, ldNs", lnNsDiffers)
	ctx("MOdataNs", "the data object namespace differs from the logical node namespace", Optional, "dataNs, lnNs or ldNs", dataNsDiffers)
	ctx("MOcdcNs", "the CDC namespace differs from the data object namespace", Optional, "dataNs, lnNs or ldNs", cdcNsDiffers)
	ctx("MOsbo", "ctlModel is a select-before-operate model", Optional, "ctlModel", ctlModelIs(isSBO))
	ctx("MOenhanced", "ctlModel is an enhanced-security model", Optional, "ctlModel", ctlModelIs(isEnhanced))
	ctx("MOrms", "hvRef is rms", Optional, "hvRef", hvRefIsRMS)
	ctx("MFsubst", "substitution is supported", Forbidden, "fact", substitution)
	ctx("MFscaledAV", "an AnalogueValue attribute has its integer member i", Forbidden, "members of the AnalogueValue attributes", scaled("AnalogueValue", ""))
	ctx("MFscaledMagV", "a Vector attribute has the integer member mag.i", Forbidden, "members of the Vector attributes", scaled("Vector", "mag"))
	ctx("MFscaledAngV", "a Vector attribute has the integer member ang.i", Forbidden, "members of the Vector attributes", scaled("Vector", "ang"))

	fact := func(name, arg, holds string, t, f Requirement) {
		add(&rule{Condition: Condition{Name: name, Family: FamilyFact, Argument: arg, Holds: holds,
			IfHolds: t, Otherwise: f, Needs: "fact"}})
	}
	fact("MOcond", argCondID, "the condition the argument identifies holds", Required, Optional)
	fact("MFcond", argCondID, "the condition the argument identifies holds", Required, Forbidden)
	fact("OFcond", argCondID, "the condition the argument identifies holds", Optional, Forbidden)
	fact("MOoperTm", argNone, "time-activated operation is supported", Required, Optional)
	fact("MONamPlt", argNone, "the name plate differs from that of the logical device", Required, Optional)
	fact("MORange", argNone, "range supervision is used", Required, Optional)
	fact("OMSynPh", argNone, "the logical node provides synchrophasors", Required, Optional)
	fact("MFsg", argNone, "the data object is a member of a setting group", Required, Forbidden)
	fact("MFnsg", argNone, "the data object is a setting outside a setting group", Required, Forbidden)
}

// Conditions returns how the evaluator treats every condition it
// knows, sorted by name.
func Conditions() []Condition {
	out := make([]Condition, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.Condition)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Describe returns how the evaluator treats the named condition.
func Describe(name string) (Condition, bool) {
	r, ok := rules[name]
	if !ok {
		return Condition{}, false
	}
	return r.Condition, true
}

// Known reports whether the evaluator has a rule for the condition.
func Known(name string) bool {
	_, ok := rules[name]
	return ok
}

// IsMultiInstance reports whether the condition allows numbered
// instances of its data object (Mmulti, Omulti, MmultiRange,
// OmultiRange, MmultiF).
func IsMultiInstance(name string) bool {
	r, ok := rules[name]
	return ok && r.Multi
}

// ParseRange parses the "min, max" argument of MmultiRange and
// OmultiRange.
func ParseRange(args string) (lo, hi int, ok bool) {
	a, b, found := strings.Cut(args, ",")
	if !found {
		return 0, 0, false
	}
	lo, err1 := strconv.Atoi(strings.TrimSpace(a))
	hi, err2 := strconv.Atoi(strings.TrimSpace(b))
	if err1 != nil || err2 != nil || lo < 0 || hi < lo {
		return 0, 0, false
	}
	return lo, hi, true
}
