// SPDX-License-Identifier: MIT

package presence_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/presence"
)

func pc(name, args string) model.PresenceCondition {
	return model.PresenceCondition{Name: name, Args: args}
}

// A synthetic namespace with one problem of each kind.
func TestValidateNamespace(t *testing.T) {
	ns := &model.Namespace{
		ID: model.NamespaceID{Standard: "X", Version: "2024", Revision: "A"},
		AbstractLNClasses: []model.LogicalNodeClass{{Name: "BaseLN", Abstract: true, DataObjects: []model.DataObject{
			{Name: "Loc", Presence: pc("O", "")},
		}}},
		LNClasses: []model.LogicalNodeClass{
			{Name: "XAAA", Base: "BaseLN", DataObjects: []model.DataObject{
				{Name: "LocKey", Presence: pc("OF", "Loc")},   // inherited sibling: fine
				{Name: "Bad", Presence: pc("MF", "Nope")},     // unknown sibling
				{Name: "Grp", Presence: pc("AtLeastOne", "")}, // missing group number
				{Name: "Odd", Presence: pc("Mystery", "")},    // no rule
				{Name: "Unset"},
			}},
			{Name: "XBBB", Base: "ForeignLN", DataObjects: []model.DataObject{
				{Name: "Ref", Presence: pc("MF", "Elsewhere")}, // base in another namespace: not checked
			}},
		},
		CDCs: []model.CommonDataClass{{Name: "CDCX", SubDataObjects: []model.DataObject{
			{Name: "c2", Presence: pc("OMSynPh", "x")}, // unexpected argument
		}, DataAttributes: []model.DataAttribute{
			{Name: "__setMag", Presence: pc("O", "")},
			{Name: "ref", Presence: pc("MO", "setMag")},        // base name of a per-FC copy: fine
			{Name: "rng", Presence: pc("OmultiRange", "9, 1")}, // bad range
			{Name: "cond", Presence: pc("MOcond", "")},         // missing condID
		}}},
		ConstructedAttributes: []model.ConstructedAttribute{{Name: "CAX", SubDataAttributes: []model.DataAttribute{
			{Name: "i", Presence: pc("AtLeastOne", "1")},
			{Name: "lim", Presence: pc("MO", "x")}, // unknown sibling
		}}},
	}
	var got []string
	for _, d := range presence.ValidateNamespace(ns) {
		if d.Severity != model.SeverityWarning || d.Namespace != ns.ID || d.Message == "" {
			t.Errorf("diagnostic: %+v", d)
		}
		got = append(got, d.Code+" "+d.Path)
	}
	sort.Strings(got)
	want := []string{
		"presence-invalid-argument /NS/CDCs/CDC[CDCX]/DataAttribute[rng]",
		"presence-invalid-argument /NS/LNClasses/LNClass[XAAA]/DataObject[Grp]",
		"presence-missing-argument /NS/CDCs/CDC[CDCX]/DataAttribute[cond]",
		"presence-unexpected-argument /NS/CDCs/CDC[CDCX]/SubDataObject[c2]",
		"presence-unknown-condition /NS/LNClasses/LNClass[XAAA]/DataObject[Odd]",
		"presence-unknown-sibling /NS/ConstructedAttributes/ConstructedAttribute[CAX]/SubDataAttribute[lim]",
		"presence-unknown-sibling /NS/LNClasses/LNClass[XAAA]/DataObject[Bad]",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if presence.ValidateNamespace(nil) != nil {
		t.Error("nil namespace")
	}
}

// Inheritance cycles and extension classes do not loop or report.
func TestValidateNamespace_CyclesAndExtensions(t *testing.T) {
	ns := &model.Namespace{LNClasses: []model.LogicalNodeClass{
		{Name: "XA", Base: "XB", DataObjects: []model.DataObject{{Name: "a", Presence: pc("MF", "zz")}}},
		{Name: "XB", Base: "XA"},
		{Name: "XC", IsExtension: true, DataObjects: []model.DataObject{{Name: "c", Presence: pc("MF", "zz")}}},
	}}
	if ds := presence.ValidateNamespace(ns); len(ds) != 0 {
		t.Errorf("diagnostics: %v", ds)
	}
}
