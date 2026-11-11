// SPDX-License-Identifier: MIT

package presence

import (
	"fmt"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

func isLN0(e *evaluation, _ int) (bool, bool, string, string) {
	if e.ctx.LNClass == "" {
		return false, false, "the logical node class is not known", "lnClass"
	}
	return e.ctx.LNClass == "LLN0", true, "logical node class " + e.ctx.LNClass, ""
}

func isRootLD(e *evaluation, _ int) (bool, bool, string, string) {
	if e.ctx.RootLD == nil {
		return false, false, "whether the logical device is the root logical device is not known", "rootLD"
	}
	if *e.ctx.RootLD {
		return true, true, "root logical device", ""
	}
	return false, true, "not the root logical device", ""
}

// sameNamespace compares two namespace declarations by standard,
// version and revision: ldNs "IEC 61850-7-4:2007B" names every release
// of 2007B. Strings that do not parse are compared verbatim.
func sameNamespace(a, b string) bool {
	ia, errA := model.ParseNamespaceID(a)
	ib, errB := model.ParseNamespaceID(b)
	if errA != nil || errB != nil || ia.Version == "" || ib.Version == "" {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return ia.SameVersion(ib)
}

func nsWhy(what, a, b string) string {
	return fmt.Sprintf("%s %s vs %s", what, a, b)
}

func lnNsDiffers(e *evaluation, _ int) (bool, bool, string, string) {
	switch {
	case e.ctx.LNNamespace == "" && e.ctx.LDNamespace == "":
		return false, false, "the logical node and logical device namespaces are not known", "lnNs, ldNs"
	case e.ctx.LNNamespace == "":
		return false, false, "the logical node namespace is not known", "lnNs"
	case e.ctx.LDNamespace == "":
		return false, false, "the logical device namespace is not known", "ldNs"
	}
	return !sameNamespace(e.ctx.LNNamespace, e.ctx.LDNamespace), true,
		nsWhy("lnNs/ldNs", e.ctx.LNNamespace, e.ctx.LDNamespace), ""
}

// lnRef is the namespace a data object's namespace is compared with:
// that of the logical node, else that of the logical device.
func (e *evaluation) lnRef() string {
	if e.ctx.LNNamespace != "" {
		return e.ctx.LNNamespace
	}
	return e.ctx.LDNamespace
}

func dataNsDiffers(e *evaluation, _ int) (bool, bool, string, string) {
	switch {
	case e.ctx.DataNamespace == "":
		return false, false, "the data object namespace is not known", "dataNs"
	case e.lnRef() == "":
		return false, false, "the logical node and logical device namespaces are not known", "lnNs or ldNs"
	}
	return !sameNamespace(e.ctx.DataNamespace, e.lnRef()), true,
		nsWhy("data/logical node namespace", e.ctx.DataNamespace, e.lnRef()), ""
}

func cdcNsDiffers(e *evaluation, i int) (bool, bool, string, string) {
	cdc := e.elems[i].Namespace
	ref := e.ctx.DataNamespace
	if ref == "" {
		ref = e.lnRef()
	}
	switch {
	case cdc.IsZero():
		return false, false, "the CDC namespace is not known", "CDC namespace"
	case ref == "":
		return false, false, "the data object, logical node and logical device namespaces are not known", "dataNs, lnNs or ldNs"
	}
	return !sameNamespace(cdc.String(), ref), true, nsWhy("CDC/data namespace", cdc.String(), ref), ""
}

// Control models (CtlModelKind, IEC 61850-7-3).
const (
	ctlStatusOnly = iota
	ctlDirectNormal
	ctlSBONormal
	ctlDirectEnhanced
	ctlSBOEnhanced
)

var ctlModels = map[string]int{
	"status-only":                   ctlStatusOnly,
	"direct-with-normal-security":   ctlDirectNormal,
	"sbo-with-normal-security":      ctlSBONormal,
	"direct-with-enhanced-security": ctlDirectEnhanced,
	"sbo-with-enhanced-security":    ctlSBOEnhanced,
	"0":                             ctlStatusOnly,
	"1":                             ctlDirectNormal,
	"2":                             ctlSBONormal,
	"3":                             ctlDirectEnhanced,
	"4":                             ctlSBOEnhanced,
}

func normalizeCtlModel(s string) (int, bool) {
	m, ok := ctlModels[strings.TrimSpace(s)]
	return m, ok
}

func (e *evaluation) ctlModel() string {
	if v, ok := e.inst.Values["ctlModel"]; ok {
		return v
	}
	return e.ctx.CtlModel
}

func isSBO(m int) bool      { return m == ctlSBONormal || m == ctlSBOEnhanced }
func isEnhanced(m int) bool { return m == ctlDirectEnhanced || m == ctlSBOEnhanced }

func ctlModelIs(pred func(int) bool) func(e *evaluation, i int) (bool, bool, string, string) {
	return func(e *evaluation, _ int) (bool, bool, string, string) {
		v := e.ctlModel()
		if v == "" {
			return false, false, "ctlModel is not known", "ctlModel"
		}
		m, ok := normalizeCtlModel(v)
		if !ok {
			return false, false, fmt.Sprintf("ctlModel %q is not a CtlModelKind value", v), "ctlModel"
		}
		return pred(m), true, "ctlModel " + v, ""
	}
}

func hvRefIsRMS(e *evaluation, _ int) (bool, bool, string, string) {
	v, ok := e.inst.Values["hvRef"]
	if !ok || v == "" {
		return false, false, "hvRef is not known", "hvRef"
	}
	switch strings.TrimSpace(v) {
	case "rms", "2":
		return true, true, "hvRef " + v, ""
	case "fundamental", "1", "absolute", "3":
		return false, true, "hvRef " + v, ""
	}
	return false, false, fmt.Sprintf("hvRef %q is not a HvReferenceKind value", v), "hvRef"
}

// substitution holds when another substitution attribute (MFsubst) is
// present: the data object then supports substitution.
func substitution(e *evaluation, i int) (bool, bool, string, string) {
	if o := e.othersPresent(-1, e.members(i, false)); len(o) > 0 {
		return true, true, e.names(o) + " present", ""
	}
	return false, false, "no substitution attribute is present and substitution support is not stated", "fact MFsubst"
}

// members of a present attribute: (names, known).
func (e *evaluation) memberNames(path ...string) ([]string, bool) {
	prefix := strings.Join(path, ".") + "."
	var out []string
	known := false
	for _, p := range e.inst.Present {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(p), prefix); ok {
			known = true
			if n, _, _ := strings.Cut(rest, "."); n != "" {
				out = append(out, n)
			}
		}
	}
	cur := e.inst
	for _, name := range path {
		if cur == nil {
			break
		}
		cur = cur.Children[name]
	}
	if cur != nil && cur != e.inst {
		known = true
		for _, p := range cur.Present {
			n, _, _ := strings.Cut(strings.TrimSpace(p), ".")
			out = append(out, n)
		}
	}
	return out, known
}

// scaled decides the MFscaled* conditions from the integer member i of
// the present attributes of the given type (AnalogueValue, or the mag
// or ang member of a Vector).
func scaled(typ, sub string) func(e *evaluation, i int) (bool, bool, string, string) {
	return func(e *evaluation, _ int) (bool, bool, string, string) {
		anyKnown, anyAttr := false, false
		for j, el := range e.elems {
			if !strings.HasPrefix(el.Type, typ) || !e.present[j] {
				continue
			}
			anyAttr = true
			path := []string{baseName(el.Name)}
			if sub != "" {
				path = append(path, sub)
			}
			ms, known := e.memberNames(path...)
			if !known {
				continue
			}
			anyKnown = true
			for _, m := range ms {
				if m == "i" {
					return true, true, strings.Join(path, ".") + ".i is present", ""
				}
			}
		}
		switch {
		case !anyAttr:
			return false, true, "no " + typ + " attribute is present", ""
		case !anyKnown:
			return false, false, "the members of the " + typ + " attributes are not known", "members of the " + typ + " attributes"
		}
		return false, true, "no " + typ + " attribute has the integer member", ""
	}
}
