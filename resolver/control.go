// SPDX-License-Identifier: MIT

package resolver

import (
	"fmt"

	"github.com/otfabric/go-iec61850-model/model"
)

// The control model of IEC 61850-8-1:2011 clause 20. The service
// parameters of the control services are mapped within the controllable
// data object with FC CO (§20.3): SBO (a VISIBLE STRING129, Table 83)
// and the structures SBOw (Table 85), Oper (Table 90) and Cancel
// (Table 88). Their members are the parameters of Table 80. ctlVal is
// typed by the CDC (IEC 61850-7-3 "parameters for control services",
// the ServiceParameters of the NSD); the others have fixed types.

// controlServiceOrder is the order of the control services in Table 81.
var controlServiceOrder = []string{"SBO", "SBOw", "Oper", "Cancel"}

// controlServices lists the members of each control service structure;
// SBO is a leaf and has none. Cancel has no Check (Table 88).
var controlServices = map[string][]string{
	"SBO":    nil,
	"SBOw":   {"ctlVal", "operTm", "origin", "ctlNum", "T", "Test", "Check"},
	"Oper":   {"ctlVal", "operTm", "origin", "ctlNum", "T", "Test", "Check"},
	"Cancel": {"ctlVal", "operTm", "origin", "ctlNum", "T", "Test"},
}

// controlParameter is a fixed-type parameter of Table 80.
type controlParameter struct {
	typ      string
	kind     model.TypeKind
	presence string
}

// controlParameters are the Table 80 parameters other than ctlVal, with
// the IEC 61850-7-2 NSD type names. operTm is conditional: it is
// present only when the control object supports TimeActivatedOperate
// (Table 90 NOTE 2), which the model reports as optional.
var controlParameters = map[string]controlParameter{
	"operTm": {typ: "Timestamp", presence: "O"},
	"origin": {typ: "Originator", kind: model.TypeKindConstructed, presence: "M"},
	"ctlNum": {typ: "INT8U", presence: "M"},
	"T":      {typ: "Timestamp", presence: "M"},
	"Test":   {typ: "BOOLEAN", presence: "M"},
	"Check":  {typ: "Check", presence: "M"},
}

// sboType is the type of SBO, VISIBLE STRING129 (Table 83).
const sboType = "VisString129"

// controllable returns the variant of a CDC that declares service
// parameters, or nil when the CDC is not controllable.
func controllable(variants []*model.CommonDataClass) *model.CommonDataClass {
	for _, c := range variants {
		if len(c.ServiceParameters) > 0 {
			return c
		}
	}
	return nil
}

// isControlParameter reports whether name is a parameter of a control
// service structure of the CDC.
func isControlParameter(ctl *model.CommonDataClass, name string) bool {
	if _, ok := controlParameters[name]; ok {
		return true
	}
	for i := range ctl.ServiceParameters {
		if ctl.ServiceParameters[i].Name == name {
			return true
		}
	}
	return false
}

// enterControlService adds a control service step (SBO, SBOw, Oper,
// Cancel) below a controllable data object.
func (w *walker) enterControlService(ctl *model.CommonDataClass, name string, fc model.FunctionalConstraint) error {
	if fc != "" && fc != model.FCCO {
		return w.fail(model.ErrUnknownAttribute, name,
			fmt.Sprintf("control service %s has functional constraint CO (IEC 61850-8-1 §20.3)", name), string(fc), string(model.FCCO))
	}
	// Which services a device has depends on its ctlModel (Table 81);
	// Oper exists for every control model other than status-only.
	presence := "O"
	if name == "Oper" {
		presence = "M"
	}
	step := Step{
		Name: name, Kind: StepControlService, Type: name, TypeKind: model.TypeKindConstructed, FC: model.FCCO,
		Presence: model.PresenceCondition{Name: presence}, DeclaredIn: ctl.Key(), Source: ctl.Source,
	}
	if name == "SBO" {
		kind, err := w.r.kindOf(ctl.Source, name, sboType, "")
		if err != nil {
			return w.annotate(err, name)
		}
		step.Type, step.TypeKind = sboType, kind
		w.typeFrom = ctl.Source
	}
	w.ctlCDC = ctl
	w.res.CDC = ctl
	w.res.Steps = append(w.res.Steps, step)
	w.attr = &w.res.Steps[len(w.res.Steps)-1]
	return nil
}

// enterServiceParameter adds a parameter step below a control service
// structure.
func (w *walker) enterServiceParameter(name string) error {
	svc := w.attr.Name
	if controlServices[svc] == nil {
		return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("attribute %s of type %s has no members", svc, w.attr.Type), "", "")
	}
	if !hasMember(controlServices[svc], name) {
		return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("control service %s has no parameter %s (IEC 61850-8-1 Table 80)", svc, name), "", "")
	}
	ctl := w.ctlCDC
	step := Step{Name: name, Kind: StepServiceParameter, FC: model.FCCO, DeclaredIn: ctl.Key(), Source: ctl.Source}
	if name == "ctlVal" {
		sp := ctlVal(ctl)
		if sp == nil {
			return w.fail(model.ErrUnknownAttribute, name, fmt.Sprintf("CDC %s declares no service parameter ctlVal", ctl.Name), "", "")
		}
		da := &model.DataAttribute{Name: sp.Name, Type: sp.Type, TypeKind: sp.TypeKind, FC: model.FCCO, Presence: sp.Presence}
		typ, kind, param, err := w.r.attrType(da, ctl, w.do, w.doFrom)
		if err != nil {
			return w.annotate(err, name)
		}
		w.typeFrom = ctl.Source
		if param {
			w.typeFrom = w.doFrom
		}
		presence := sp.Presence
		if presence.IsZero() {
			presence = model.PresenceCondition{Name: "M"} // Table 90: ctlVal is mandatory
		}
		step.Type, step.TypeKind, step.Parameterized, step.Presence = typ, kind, param, presence
	} else {
		p := controlParameters[name]
		kind, err := w.r.kindOf(ctl.Source, name, p.typ, p.kind)
		if err != nil {
			return w.annotate(err, name)
		}
		w.typeFrom = ctl.Source
		step.Type, step.TypeKind, step.Presence = p.typ, kind, model.PresenceCondition{Name: p.presence}
	}
	w.res.Attribute = nil
	w.res.Steps = append(w.res.Steps, step)
	w.attr = &w.res.Steps[len(w.res.Steps)-1]
	return nil
}

// ctlVal returns the ctlVal service parameter of a CDC.
func ctlVal(c *model.CommonDataClass) *model.ServiceParameter {
	for i := range c.ServiceParameters {
		if c.ServiceParameters[i].Name == "ctlVal" {
			return &c.ServiceParameters[i]
		}
	}
	return nil
}

func hasMember(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}
