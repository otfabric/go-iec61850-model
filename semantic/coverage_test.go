// SPDX-License-Identifier: MIT

package semantic_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/resolver"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func statuses(c semantic.ClassCoverage) map[string]semantic.DataObjectCoverage {
	out := map[string]semantic.DataObjectCoverage{}
	for _, d := range c.DataObjects {
		out[d.Name] = d
	}
	return out
}

func TestCoverage(t *testing.T) {
	p := fixtures.Profile(t)
	cs, err := semantic.Coverage(p)
	if err != nil {
		t.Fatal(err)
	}
	byClass := map[string]semantic.ClassCoverage{}
	for _, c := range cs {
		if c.Namespace.IsZero() || len(c.DataObjects) != c.Mapped+c.Excluded+c.Unmapped+c.NotMeasured {
			t.Errorf("%s: counts do not add up: %+v", c.LNClass, c)
		}
		byClass[c.LNClass] = c
	}
	if _, abstract := byClass["StatisticsLN"]; abstract {
		t.Error("abstract classes are not reported")
	}
	mmxu := statuses(byClass["MMXU"])
	if d := mmxu["PNV"]; d.Status != semantic.StatusMapped || d.Rule != "mmxu.pnv" || d.CDC != "WYE" || d.DeclaredIn != "MMXU" {
		t.Errorf("MMXU.PNV: %+v", d)
	}
	if d := mmxu["Beh"]; d.Status != semantic.StatusNotMeasured || d.Rule != "" {
		t.Errorf("MMXU.Beh: %+v", d)
	}
	dtst := statuses(byClass["DTST"])
	if d := dtst["OutW"]; d.Status != semantic.StatusUnmapped || d.Reason != "no rule" {
		t.Errorf("DTST.OutW: %+v", d)
	}
	if c := byClass["DTST"]; c.MeasuredValues() != 1 || c.Unmapped != 1 {
		t.Errorf("DTST: %+v", c)
	}

	// A rule whose CDC differs from the model is a gap, with the reason.
	wrong, _ := semantic.NewMapper([]semantic.Rule{{ID: "w", LNClass: "MMXU", DataObject: "Hz", CDC: "CMV", Quantity: semantic.QuantityFrequency}}, nil)
	c, err := wrong.CoverageOf(p, fixtures.ID74, "MMXU")
	if err != nil {
		t.Fatal(err)
	}
	if d := statuses(c)["Hz"]; d.Status != semantic.StatusUnmapped || d.Rule != "w" || !strings.Contains(d.Reason, "expects CDC CMV") {
		t.Errorf("CDC mismatch: %+v", d)
	}
	if _, err := semantic.Default.CoverageOf(p, fixtures.ID74, "XXXX"); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unknown class: %v", err)
	}
	for cdc, measured := range map[string]bool{"MV": true, "HWYE": true, "BCR": true, "SAV": true, "SPS": false, "DPC": false, "ASG": false} {
		if semantic.IsMeasuredCDC(cdc) != measured {
			t.Errorf("IsMeasuredCDC(%s) != %v", cdc, measured)
		}
	}
}

func TestExclusions(t *testing.T) {
	p := fixtures.Profile(t)
	m, err := semantic.Default.WithExclusions(semantic.Exclusion{LNClass: "DTST", DataObject: "OutW", Reason: "vendor-specific"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Describe(p, "DTST.OutW"); !errors.Is(err, model.ErrNoMapping) || !strings.Contains(err.Error(), "vendor-specific") {
		t.Errorf("excluded: %v", err)
	}
	c, _ := m.CoverageOf(p, fixtures.IDExt, "DTST")
	if d := statuses(c)["OutW"]; d.Status != semantic.StatusExcluded || d.Reason != "vendor-specific" || c.Excluded != 1 {
		t.Errorf("coverage of an exclusion: %+v", c)
	}
	if n := len(m.Exclusions()); n != len(semantic.DefaultExclusions())+1 {
		t.Errorf("Exclusions() = %d", n)
	}

	// An exclusion on a subclass wins over a default rule on a base class.
	base, _ := semantic.NewMapper([]semantic.Rule{{ID: "b", LNClass: "StatisticsLN", DataObject: "ClcExp", CDC: "SPS", Quantity: semantic.QuantityCount}}, nil)
	bx, err := base.WithExclusions(semantic.Exclusion{LNClass: "MMXU", DataObject: "ClcExp", Reason: "not here"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bx.Describe(p, "MMXU.ClcExp"); !errors.Is(err, model.ErrNoMapping) {
		t.Errorf("exclusion over a base-class rule: %v", err)
	}
	if d, err := bx.Describe(p, "MMTR.ClcExp"); err != nil || d.Rule != "b" {
		t.Errorf("other subclasses keep the rule: %+v %v", d, err)
	}
	// An override wins over an exclusion, and WithOverrides keeps the
	// exclusions.
	o, err := m.WithOverrides(semantic.Rule{ID: "v", LNClass: "DTST", DataObject: "OutW", CDC: "MV", Quantity: semantic.QuantityActivePower, Source: "vendor:x"})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := o.Describe(p, "DTST.OutW"); err != nil || d.Basis != semantic.BasisOverride {
		t.Errorf("override over exclusion: %+v %v", d, err)
	}
	if c, _ := o.CoverageOf(p, fixtures.IDExt, "DTST"); c.Mapped != 1 || c.Excluded != 0 {
		t.Errorf("coverage of an override over an exclusion: %+v", c)
	}
	if len(o.Exclusions()) != len(m.Exclusions()) {
		t.Error("WithOverrides drops exclusions")
	}

	for i, bad := range [][]semantic.Exclusion{
		{{LNClass: "X", DataObject: "D"}},
		{{DataObject: "D", Reason: "r"}},
		{{LNClass: "MMXU", DataObject: "Hz", Reason: "mapped by mmxu.hz"}},
		{{LNClass: "X", DataObject: "D", Reason: "r"}, {LNClass: "X", DataObject: "D", Reason: "again"}},
	} {
		if _, err := semantic.Default.WithExclusions(bad...); !errors.Is(err, model.ErrInvalidRule) {
			t.Errorf("case %d: %v", i, err)
		}
	}
	if _, err := semantic.Default.WithOverrides(semantic.Rule{ID: "dup", LNClass: "MMXU", DataObject: "Hz", CDC: "MV", Quantity: semantic.QuantityFrequency}); !errors.Is(err, model.ErrInvalidRule) {
		t.Errorf("WithOverrides keeps validating: %v", err)
	}
}

func TestDefaultRulesAndExclusions(t *testing.T) {
	m, err := semantic.NewMapper(semantic.DefaultRules(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.WithExclusions(semantic.DefaultExclusions()...); err != nil {
		t.Fatal(err)
	}
	for _, e := range semantic.DefaultExclusions() {
		if e.Reason == "" || e.LNClass == "" || e.DataObject == "" {
			t.Errorf("exclusion %+v", e)
		}
	}
	// DefaultRules returns a copy.
	rs := semantic.DefaultRules()
	rs[0].ID = "changed"
	if semantic.DefaultRules()[0].ID == "changed" {
		t.Error("DefaultRules is not a copy")
	}
	if _, err := semantic.NewMapper([]semantic.Rule{{ID: "of", LNClass: "X", DataObject: "D", CDC: "MV", Quantity: semantic.QuantityHarmonicDistortion, Of: "furlongs"}}, nil); !errors.Is(err, model.ErrInvalidRule) {
		t.Errorf("unknown Of: %v", err)
	}
	if _, err := semantic.NewMapper([]semantic.Rule{{ID: "h", LNClass: "X", DataObject: "D", CDC: "HMV", Quantity: semantic.QuantityCurrent, Phase: semantic.PhaseA}}, nil); !errors.Is(err, model.ErrInvalidRule) {
		t.Errorf("phase on HMV: %v", err)
	}
}

func TestCriteriaOfAndSequence(t *testing.T) {
	d := &semantic.Descriptor{Quantity: semantic.QuantityImbalance, Of: semantic.QuantityCurrent, Sequence: semantic.SequenceNegative, Confidence: semantic.ConfidenceHigh}
	for i, tc := range []struct {
		c    semantic.Criteria
		want bool
	}{
		{semantic.Criteria{Of: semantic.QuantityCurrent}, true},
		{semantic.Criteria{Of: semantic.QuantityVoltage}, false},
		{semantic.Criteria{Sequence: semantic.SequenceNegative}, true},
		{semantic.Criteria{Sequence: semantic.SequenceZero}, false},
		{semantic.Criteria{Quantity: semantic.QuantityImbalance, Of: semantic.QuantityCurrent, Sequence: semantic.SequenceNegative}, true},
	} {
		if tc.c.Matches(d) != tc.want {
			t.Errorf("case %d: Matches = %v", i, !tc.want)
		}
	}
}

func TestDescribeMalformedSubDataObject(t *testing.T) {
	p := fixtures.Profile(t)
	res, err := resolver.New(p).ResolveString("MMXU.A.phsB")
	if err != nil {
		t.Fatal(err)
	}
	res.Steps[2].Name = "phsB(1" // only a hand-made resolution has this
	if _, err := semantic.Default.DescribeResolution(p, res); !errors.Is(err, model.ErrNoMapping) {
		t.Errorf("malformed sub data object: %v", err)
	}
}

// lookup wraps a profile: it can list an abstract class and make
// InheritanceIn fail.
type lookup struct {
	*catalog.Profile
	extra       []*model.LogicalNodeClass
	inheritance error
}

func (l lookup) ListLogicalNodes() []*model.LogicalNodeClass {
	return append(l.extra, l.Profile.ListLogicalNodes()...)
}

func (l lookup) InheritanceIn(ns model.NamespaceID, name string) ([]*model.LogicalNodeClass, error) {
	if l.inheritance != nil {
		return nil, l.inheritance
	}
	return l.Profile.InheritanceIn(ns, name)
}

func TestCoverageErrors(t *testing.T) {
	p := fixtures.Profile(t)
	// An abstract class a Lookup lists is skipped.
	cs, err := semantic.Coverage(lookup{Profile: p, extra: []*model.LogicalNodeClass{{Name: "StatisticsLN", Abstract: true, Source: fixtures.ID74}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.LNClass == "StatisticsLN" {
			t.Error("abstract class reported")
		}
	}
	// A class the Lookup lists but cannot resolve fails the report.
	if _, err := semantic.Coverage(lookup{Profile: p, extra: []*model.LogicalNodeClass{{Name: "XXXX", Source: fixtures.ID74}}}); !errors.Is(err, model.ErrUnknownLogicalNode) {
		t.Errorf("unresolvable class: %v", err)
	}
	broken := errors.New("broken inheritance")
	if _, err := semantic.Default.CoverageOf(lookup{Profile: p, inheritance: broken}, fixtures.ID74, "MMXU"); !errors.Is(err, broken) {
		t.Errorf("InheritanceIn error: %v", err)
	}
}
