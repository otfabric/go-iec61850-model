// SPDX-License-Identifier: MIT

package presence

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Evaluate evaluates the presence conditions of the elements of one
// class against an instance and returns one result per element, in the
// order of elems. Children of the instance are not evaluated: they need
// the model ([Evaluator]). A nil instance is an instance with nothing
// present.
func Evaluate(class string, elems []Element, inst *Instance) *Report {
	return evaluate(class, "", elems, inst, Context{})
}

// evaluation is the state of one class evaluation.
type evaluation struct {
	elems []Element
	inst  *Instance
	ctx   Context
	// present per element; instances per element (instance names).
	present   []bool
	instances [][]string
	byName    map[string]int
	unknown   []string
}

func evaluate(class, path string, elems []Element, inst *Instance, parent Context) *Report {
	if inst == nil {
		inst = &Instance{}
	}
	e := &evaluation{elems: elems, inst: inst, ctx: merge(parent, inst.Context)}
	e.match()
	rep := &Report{Path: path, Class: class, Unknown: e.unknown}
	for i := range elems {
		rep.Results = append(rep.Results, e.result(path, i))
	}
	return rep
}

// merge returns the parent context overridden by the fields the child
// sets; facts are merged, the child's win.
func merge(parent, child Context) Context {
	out := parent
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	set(&out.LNClass, child.LNClass)
	set(&out.LDNamespace, child.LDNamespace)
	set(&out.LNNamespace, child.LNNamespace)
	set(&out.DataNamespace, child.DataNamespace)
	set(&out.CtlModel, child.CtlModel)
	if child.RootLD != nil {
		out.RootLD = child.RootLD
	}
	if len(child.Facts) > 0 {
		out.Facts = make(map[string]bool, len(parent.Facts)+len(child.Facts))
		for k, v := range parent.Facts {
			out.Facts[k] = v
		}
		for k, v := range child.Facts {
			out.Facts[k] = v
		}
	}
	return out
}

// match assigns the present names to elements: an exact name first,
// then a numbered instance of a multi-instance element.
func (e *evaluation) match() {
	e.present = make([]bool, len(e.elems))
	e.instances = make([][]string, len(e.elems))
	e.byName = make(map[string]int, len(e.elems))
	for i, el := range e.elems {
		if _, dup := e.byName[baseName(el.Name)]; !dup {
			e.byName[baseName(el.Name)] = i
		}
	}
	seen := map[string]bool{}
	names := append([]string(nil), e.inst.Present...)
	keys := make([]string, 0, len(e.inst.Children)+len(e.inst.Values))
	for k := range e.inst.Children {
		keys = append(keys, k)
	}
	for k := range e.inst.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, p := range append(names, keys...) {
		name, _, _ := strings.Cut(strings.TrimSpace(p), ".")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if !e.assign(name) {
			e.unknown = append(e.unknown, name)
		}
	}
	sort.Strings(e.unknown)
}

func (e *evaluation) assign(name string) bool {
	if _, ok := e.byName[name]; ok {
		// Every per-FC copy with this base name is present.
		for i, el := range e.elems {
			if baseName(el.Name) == name {
				e.present[i] = true
				e.instances[i] = append(e.instances[i], name)
			}
		}
		return true
	}
	stem := strings.TrimRight(name, "0123456789")
	if stem == name || stem == "" {
		return false
	}
	i, ok := e.byName[stem]
	if !ok || !IsMultiInstance(e.elems[i].Presence.Name) {
		return false
	}
	e.present[i] = true
	e.instances[i] = append(e.instances[i], name)
	return true
}

// presentName reports whether the element or instance name is present.
func (e *evaluation) presentName(name string) bool {
	if i, ok := e.byName[name]; ok {
		return e.present[i]
	}
	for _, p := range e.inst.Present {
		if n, _, _ := strings.Cut(strings.TrimSpace(p), "."); n == name {
			return true
		}
	}
	_, child := e.inst.Children[name]
	_, value := e.inst.Values[name]
	return child || value
}

// fact returns the caller's fact for element i, if any.
func (e *evaluation) fact(i int) (holds, ok bool, key string) {
	cond := e.elems[i].Presence.String()
	if v, ok := e.ctx.Facts[baseName(e.elems[i].Name)+":"+cond]; ok {
		return v, true, baseName(e.elems[i].Name) + ":" + cond
	}
	v, ok := e.ctx.Facts[cond]
	return v, ok, cond
}

func (e *evaluation) result(path string, i int) Result {
	el := e.elems[i]
	res := Result{
		Path: joinPath(path, baseName(el.Name)), Element: el.Name, Kind: el.Kind, Condition: el.Presence,
		Present: e.present[i],
	}
	if IsMultiInstance(el.Presence.Name) {
		res.Instances = e.instances[i]
	}
	r, ok := rules[el.Presence.Name]
	if !ok {
		res.Requirement, res.Verdict = Undetermined, Unresolved
		res.Reason = fmt.Sprintf("no rule for presence condition %q", el.Presence.String())
		res.Missing = "rule for " + el.Presence.Name
		if el.Presence.IsZero() {
			res.Reason = "no presence condition declared"
			res.Missing = "presence condition"
		}
		return res
	}
	switch {
	case r.group != nil && r.Needs == "":
		req, why := r.group(e, i)
		e.decide(&res, req, why)
	case r.group != nil:
		// MAllOrNonePerGroup: the group decides when a member is present.
		req, why := r.group(e, i)
		if req != Undetermined {
			e.decide(&res, req, why)
			break
		}
		e.twoWay(&res, r, i)
	case r.Family == FamilyUnconditional:
		e.decide(&res, r.IfHolds, unconditionalWhy(r))
	default:
		e.twoWay(&res, r, i)
	}
	if r.Multi {
		e.checkInstances(&res, i)
	}
	return res
}

func unconditionalWhy(r *rule) string {
	switch r.IfHolds {
	case Required:
		return "mandatory"
	case Forbidden:
		return "forbidden"
	default:
		if r.Name == "na" {
			return "not applicable"
		}
		return "optional"
	}
}

// twoWay evaluates a condition that selects one of two requirements.
func (e *evaluation) twoWay(res *Result, r *rule, i int) {
	holds, ok, key := e.fact(i)
	why := ""
	missing := ""
	if ok {
		why = "fact " + key
	} else {
		switch {
		case r.Family == FamilySibling:
			holds, ok, why, missing = e.sibling(i)
		case r.derive != nil:
			holds, ok, why, missing = r.derive(e, i)
		default:
			missing = "fact " + e.elems[i].Presence.String()
			why = "whether " + r.Holds + " is not stated"
			if t := e.elems[i].Presence.ArgsText; t != "" {
				why += " (" + t + ")"
			}
		}
	}
	if ok {
		req, verb := r.Otherwise, "so not: "
		if holds {
			req, verb = r.IfHolds, "so "
		}
		if r.Holds != "" {
			why += ", " + verb + r.Holds
		}
		e.decide(res, req, why)
		return
	}
	res.Requirement, res.Missing = Undetermined, missing
	vt, vf := verdict(r.IfHolds, res.Present), verdict(r.Otherwise, res.Present)
	if vt == vf {
		res.Verdict = vt
		res.Reason = fmt.Sprintf("%s; %s either way", why, presenceWord(res.Present))
		return
	}
	res.Verdict = Unresolved
	res.Reason = fmt.Sprintf("%s; %s if it holds, %s otherwise", why, r.IfHolds, r.Otherwise)
}

func presenceWord(present bool) string {
	if present {
		return "present, which is allowed"
	}
	return "absent, which is allowed"
}

// decide sets a determined requirement and its verdict.
func (e *evaluation) decide(res *Result, req Requirement, why string) {
	res.Requirement = req
	res.Verdict = verdict(req, res.Present)
	state := "absent"
	if res.Present {
		state = "present"
	}
	res.Reason = why + "; " + state
}

func verdict(req Requirement, present bool) Verdict {
	switch {
	case req == Required && !present, req == Forbidden && present:
		return Violated
	default:
		return Satisfied
	}
}

// checkInstances checks the instance numbers of a multi-instance
// element: within the range of MmultiRange/OmultiRange, and of one
// width.
func (e *evaluation) checkInstances(res *Result, i int) {
	el := e.elems[i]
	stem := baseName(el.Name)
	lo, hi, ranged := 0, 0, false
	if strings.HasSuffix(el.Presence.Name, "Range") {
		lo, hi, ranged = ParseRange(el.Presence.Args)
	}
	width := -1
	for _, inst := range e.instances[i] {
		digits := strings.TrimPrefix(inst, stem)
		if digits == "" {
			continue
		}
		n, err := strconv.Atoi(digits)
		if err != nil {
			continue
		}
		if ranged && (n < lo || n > hi) {
			res.Verdict = Violated
			res.Reason += fmt.Sprintf("; instance %s is outside %d..%d", inst, lo, hi)
		}
		if width >= 0 && len(digits) != width {
			res.Verdict = Violated
			res.Reason += fmt.Sprintf("; instance %s has %d digits, another instance %d (7-2 §22.2: one width per data object)", inst, len(digits), width)
		}
		if width < 0 {
			width = len(digits)
		}
	}
}

// sibling decides the sibling conditions (MF, MO, OF, OM, FM, MmultiF).
func (e *evaluation) sibling(i int) (holds, ok bool, why, missing string) {
	s := strings.TrimSpace(e.elems[i].Presence.Args)
	if s == "" {
		return false, false, "the condition names no sibling", "sibling argument"
	}
	if e.presentName(s) {
		return true, true, s + " is present", ""
	}
	return false, true, s + " is absent", ""
}

// members returns the indices of the elements with the same condition
// name, and the same argument when sameArgs is set.
func (e *evaluation) members(i int, sameArgs bool) []int {
	p := e.elems[i].Presence
	var out []int
	for j, el := range e.elems {
		if el.Presence.Name == p.Name && (!sameArgs || el.Presence.Args == p.Args) {
			out = append(out, j)
		}
	}
	return out
}

func (e *evaluation) names(idx []int) string {
	ns := make([]string, len(idx))
	for k, j := range idx {
		ns[k] = baseName(e.elems[j].Name)
	}
	return strings.Join(ns, ", ")
}

// othersPresent returns the members other than i that are present
// (all present members for i < 0).
func (e *evaluation) othersPresent(i int, idx []int) []int {
	var out []int
	for _, j := range idx {
		if j != i && e.present[j] {
			out = append(out, j)
		}
	}
	return out
}

func atLeastOne(e *evaluation, i int) (Requirement, string) {
	g := e.members(i, true)
	if o := e.othersPresent(i, g); len(o) > 0 {
		return Optional, fmt.Sprintf("group has %s present", e.names(o))
	}
	return Required, fmt.Sprintf("at least one of %s is required and no other member is present", e.names(g))
}

func atMostOne(e *evaluation, i int) (Requirement, string) {
	g := e.members(i, false)
	if o := e.othersPresent(i, g); len(o) > 0 {
		return Forbidden, fmt.Sprintf("at most one of %s, and %s is present", e.names(g), e.names(o))
	}
	return Optional, fmt.Sprintf("at most one of %s, and no other is present", e.names(g))
}

func allOrNone(e *evaluation, i int) (Requirement, string) {
	g := e.members(i, true)
	if o := e.othersPresent(-1, g); len(o) > 0 {
		return Required, fmt.Sprintf("all or none of %s, and %s is present", e.names(g), e.names(o))
	}
	return Optional, fmt.Sprintf("all or none of %s, and no other member is present", e.names(g))
}

// groups returns the members of every group of the condition of i, by
// argument, and the argument of i.
func (e *evaluation) groups(i int) (map[string][]int, []string) {
	gs := map[string][]int{}
	var keys []string
	for _, j := range e.members(i, false) {
		a := e.elems[j].Presence.Args
		if _, ok := gs[a]; !ok {
			keys = append(keys, a)
		}
		gs[a] = append(gs[a], j)
	}
	return gs, keys
}

func allOnlyOne(e *evaluation, i int) (Requirement, string) {
	gs, keys := e.groups(i)
	mine := e.elems[i].Presence.Args
	for _, k := range keys {
		if k == mine {
			continue
		}
		if o := e.othersPresent(i, gs[k]); len(o) > 0 {
			return Forbidden, fmt.Sprintf("only one group may be present, and group %s has %s present", k, e.names(o))
		}
	}
	if o := e.othersPresent(i, gs[mine]); len(o) > 0 {
		return Required, fmt.Sprintf("group %s (%s) is present and must be complete", mine, e.names(gs[mine]))
	}
	return Required, fmt.Sprintf("exactly one of the groups %s shall be present in full, and none is", strings.Join(keys, ", "))
}

func allAtLeastOne(e *evaluation, i int) (Requirement, string) {
	gs, keys := e.groups(i)
	mine := e.elems[i].Presence.Args
	if o := e.othersPresent(i, gs[mine]); len(o) > 0 {
		return Required, fmt.Sprintf("group %s (%s) is present and must be complete", mine, e.names(gs[mine]))
	}
	for _, k := range keys {
		if k != mine && len(e.othersPresent(i, gs[k])) > 0 {
			return Optional, fmt.Sprintf("group %s is present; at least one complete group is required", k)
		}
	}
	return Required, fmt.Sprintf("at least one of the groups %s shall be present in full, and none is", strings.Join(keys, ", "))
}

// mAllOrNone is all-or-none per group; when no member is present the
// group is required only if the data object has status information,
// which ctlModel status-only implies.
func mAllOrNone(e *evaluation, i int) (Requirement, string) {
	g := e.members(i, true)
	if o := e.othersPresent(-1, g); len(o) > 0 {
		return Required, fmt.Sprintf("all or none of %s, and %s is present", e.names(g), e.names(o))
	}
	if _, ok, _ := e.fact(i); ok {
		return Undetermined, ""
	}
	if m, ok := normalizeCtlModel(e.ctlModel()); ok && m == ctlStatusOnly {
		return Required, fmt.Sprintf("ctlModel status-only has status information, so %s is required", e.names(g))
	}
	return Undetermined, ""
}
