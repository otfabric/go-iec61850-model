// SPDX-License-Identifier: MIT

package semantic

import (
	"fmt"

	"github.com/otfabric/go-iec61850-model/model"
)

// CoverageStatus says how the rules treat a data object.
type CoverageStatus string

// Coverage statuses.
const (
	// StatusMapped: a rule (or override) maps the data object and its
	// CDC matches the model.
	StatusMapped CoverageStatus = "mapped"
	// StatusExcluded: the data object is deliberately not mapped; the
	// reason is reported.
	StatusExcluded CoverageStatus = "excluded"
	// StatusUnmapped: a measured value (MV, CMV, WYE, DEL, SEQ, HMV,
	// HWYE, HDEL, HST, SAV or BCR) without a rule, or with a rule
	// whose CDC differs from the model. These are the gaps.
	StatusUnmapped CoverageStatus = "unmapped"
	// StatusNotMeasured: a status, control, setting or description
	// data object without a rule, e.g. Beh, an alarm or a set point.
	StatusNotMeasured CoverageStatus = "not-measured"
)

// measuredCDCs are the CDCs of measured values (IEC 61850-7-3 §7.4,
// plus the binary counter reading BCR).
var measuredCDCs = map[string]bool{
	"MV": true, "CMV": true, "WYE": true, "DEL": true, "SEQ": true,
	"HMV": true, "HWYE": true, "HDEL": true, "HST": true, "SAV": true, "BCR": true,
}

// IsMeasuredCDC reports whether a CDC is one of measured values: MV,
// CMV, WYE, DEL, SEQ, HMV, HWYE, HDEL, HST, SAV or BCR.
func IsMeasuredCDC(cdc string) bool { return measuredCDCs[cdc] }

// DataObjectCoverage is the coverage of one data object of a class.
type DataObjectCoverage struct {
	Name string `json:"name"`
	CDC  string `json:"cdc"`
	// DeclaredIn is the class that declares the data object.
	DeclaredIn string         `json:"declaredIn"`
	Status     CoverageStatus `json:"status"`
	// Rule is the ID of the rule or override that maps it.
	Rule string `json:"rule,omitempty"`
	// Reason says why an excluded or unmapped data object is not
	// mapped.
	Reason string `json:"reason,omitempty"`
}

// ClassCoverage counts how the rules cover the data objects of a
// logical node class, inherited ones included.
type ClassCoverage struct {
	LNClass     string               `json:"lnClass"`
	Namespace   model.NamespaceID    `json:"namespace"`
	DataObjects []DataObjectCoverage `json:"dataObjects"`
	Mapped      int                  `json:"mapped"`
	Excluded    int                  `json:"excluded"`
	Unmapped    int                  `json:"unmapped"`
	NotMeasured int                  `json:"notMeasured"`
}

// MeasuredValues returns the number of data objects that are measured
// values or mapped: mapped, excluded and unmapped together.
func (c ClassCoverage) MeasuredValues() int { return c.Mapped + c.Excluded + c.Unmapped }

// Coverage returns the coverage of every concrete logical node class of
// the model by the default rules, ordered as the model lists the
// classes. A class that several namespaces define is reported per
// definition.
func Coverage(l Lookup) ([]ClassCoverage, error) { return Default.Coverage(l) }

// Coverage is like the package-level [Coverage], with the rules,
// overrides and exclusions of m.
func (m *Mapper) Coverage(l Lookup) ([]ClassCoverage, error) {
	var out []ClassCoverage
	for _, ln := range l.ListLogicalNodes() {
		if ln.Abstract {
			continue
		}
		c, err := m.CoverageOf(l, ln.Source, ln.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// CoverageOf returns the coverage of the class lnClass as namespace ns
// defines it. It fails with the catalog's errors when the class is not
// defined there.
func (m *Mapper) CoverageOf(l Lookup, ns model.NamespaceID, lnClass string) (ClassCoverage, error) {
	dos, err := l.DataObjectsIn(ns, lnClass)
	if err != nil {
		return ClassCoverage{}, err
	}
	chain, err := l.InheritanceIn(ns, lnClass)
	if err != nil {
		return ClassCoverage{}, err
	}
	names := make([]string, len(chain))
	for i, c := range chain {
		names[i] = c.Name
	}
	c := ClassCoverage{LNClass: lnClass, Namespace: ns, DataObjects: make([]DataObjectCoverage, 0, len(dos))}
	for _, ido := range dos {
		dc := DataObjectCoverage{Name: ido.Name, CDC: ido.Type}
		if ido.DeclaredIn != nil {
			dc.DeclaredIn = ido.DeclaredIn.Name
		}
		rule, basis, mapped := m.find(names, ido.Name)
		excl, excluded := m.excluded(names, ido.Name)
		switch {
		case mapped && (basis == BasisOverride || !excluded) && rule.CDC == ido.Type:
			dc.Status, dc.Rule = StatusMapped, rule.ID
			c.Mapped++
		case excluded && (!mapped || basis == BasisRule):
			dc.Status, dc.Reason = StatusExcluded, excl.Reason
			c.Excluded++
		case mapped:
			dc.Status, dc.Rule = StatusUnmapped, rule.ID
			dc.Reason = fmt.Sprintf("rule %s expects CDC %s, the model has %s", rule.ID, rule.CDC, ido.Type)
			c.Unmapped++
		case IsMeasuredCDC(ido.Type):
			dc.Status, dc.Reason = StatusUnmapped, "no rule"
			c.Unmapped++
		default:
			dc.Status = StatusNotMeasured
			c.NotMeasured++
		}
		c.DataObjects = append(c.DataObjects, dc)
	}
	return c, nil
}
