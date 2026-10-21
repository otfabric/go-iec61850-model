// SPDX-License-Identifier: MIT

package nsd_test

import (
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// OfficialShaped is an independently authored NS document with the
// constructs found in the official packages (copyright block, changes,
// NSD version, dependency types, FC services, CDC variants, per-FC
// attribute copies, value bounds, abstract classes listed among the
// concrete ones, extension classes). It contains no IEC text.
const OfficialShaped = `<?xml version="1.0" encoding="UTF-8"?>
<NS xmlns="http://www.iec.ch/61850/2016/NSD" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="http://www.iec.ch/61850/2016/NSD NSD.xsd"
    id="(Tr)OTF 61850-99-1" version="2030" revision="A" release="2"
    namespaceType="TR" appVersion="1.0" nsdVersion="2017" nsdRevision="B" nsdRelease="6">
  <Copyright>
    <Notice>Synthetic   notice
      for tests.</Notice>
    <License uri="example.org/license" kind="Test"/>
  </Copyright>
  <Changes version="2029" revision="A" release="1" tissues="1, 2" changesID="c1" date="2030-01-01"/>
  <DependsOn id="OTF 61850-99-0" version="2030" revision="A" dependencyType="includes"/>
  <FunctionalConstraints>
    <FunctionalConstraint abbreviation="SG">
      <ApplicableServices>
        <Service name="GetEditSGValue"/>
        <DataSetMemberOf cb="GoCB"/>
      </ApplicableServices>
    </FunctionalConstraint>
  </FunctionalConstraints>
  <CDCs>
    <CDC name="XSG" variant="SP"><DataAttribute name="setVal" fc="SP" type="INT32" typeKind="BASIC" presCond="M"/></CDC>
    <CDC name="XSG" variant="SG"><DataAttribute name="setVal" fc="SG" type="INT32" typeKind="BASIC" presCond="M"/></CDC>
    <CDC name="XRS">
      <DataAttribute name="setMag" fc="SP" type="INT32" typeKind="BASIC" presCond="M" minValue="0" maxValue="10"/>
      <DataAttribute name="_setMag" fc="SG" type="INT32" typeKind="BASIC" presCond="M" minIndex="1"/>
    </CDC>
  </CDCs>
  <LNClasses>
    <AbstractLNClass name="XBaseLN"><DataObject name="Beh" type="XRS" presCond="M"/></AbstractLNClass>
    <LNClass name="XTST" base="XBaseLN"><DataObject name="Set" type="XSG" presCond="O"/></LNClass>
    <LNClass name="XEXT" isExtension="true"><DataObject name="Added" type="XRS" presCond="O"/></LNClass>
  </LNClasses>
</NS>`

func TestOfficialShapedDocument(t *testing.T) {
	doc, err := nsd.Parse(strings.NewReader(OfficialShaped), nsd.WithStrict())
	if err != nil {
		t.Fatalf("strict import of the official shape: %v", err)
	}
	ns := doc.Namespace
	if ns.ID.Standard != "(Tr)OTF 61850-99-1" || ns.NamespaceType != "TR" || ns.AppVersion != "1.0" || ns.NSDVersion != "2017B6" {
		t.Errorf("namespace attributes %+v", ns)
	}
	if c := ns.Copyright; c.Notice != "Synthetic notice for tests." || c.LicenseURI != "example.org/license" || c.LicenseKind != "Test" || c.IsZero() {
		t.Errorf("copyright %+v", c)
	}
	if len(ns.Changes) != 1 || ns.Changes[0] != (model.NamespaceChange{Version: "2029", Revision: "A", Release: "1", Tissues: "1, 2", ChangesID: "c1", Date: "2030-01-01"}) {
		t.Errorf("changes %+v", ns.Changes)
	}
	if ns.DependencyTypes["OTF 61850-99-0"] != "includes" {
		t.Errorf("dependency types %v", ns.DependencyTypes)
	}
	fc := ns.FunctionalConstraints[0]
	if len(fc.Services) != 1 || fc.Services[0] != "GetEditSGValue" || len(fc.DataSetMemberOf) != 1 || fc.DataSetMemberOf[0] != "GoCB" {
		t.Errorf("FC services %+v", fc)
	}
	if vs := ns.CDCVariants("XSG"); len(vs) != 2 || vs[1].Key() != "XSG/SG" {
		t.Errorf("variants %v", vs)
	}
	rs, _ := ns.CDC("XRS")
	if rs.Key() != "XRS" || rs.DataAttributes[0].MinValue != "0" || rs.DataAttributes[0].MaxValue != "10" || rs.DataAttributes[1].MinIndex != "1" || rs.DataAttributes[1].BaseName() != "setMag" {
		t.Errorf("value bounds and copies %+v", rs.DataAttributes)
	}
	if len(ns.AbstractLNClasses) != 1 || ns.AbstractLNClasses[0].Name != "XBaseLN" || len(ns.LNClasses) != 2 || !ns.LNClasses[1].IsExtension {
		t.Errorf("classes: %d abstract, %d concrete", len(ns.AbstractLNClasses), len(ns.LNClasses))
	}
	if (model.Copyright{}).IsZero() != true {
		t.Error("zero copyright")
	}
}

func TestOfficialShapedDiagnostics(t *testing.T) {
	missingVariant := strings.Replace(OfficialShaped, `<CDC name="XSG" variant="SP">`, `<CDC name="XSG">`, 1)
	_, err := nsd.Parse(strings.NewReader(missingVariant))
	if err == nil || !strings.Contains(err.Error(), "has no variant") {
		t.Errorf("a CDC defined twice needs variants: %v", err)
	}
	badCB := strings.Replace(OfficialShaped, `<DataSetMemberOf cb="GoCB"/>`, `<DataSetMemberOf other="x"/>`, 1)
	_, err = nsd.Parse(strings.NewReader(badCB))
	if err == nil || !strings.Contains(err.Error(), `required attribute "cb"`) {
		t.Errorf("DataSetMemberOf without cb: %v", err)
	}
	doc, err := nsd.Parse(strings.NewReader(strings.Replace(OfficialShaped, `<Changes `, `<Changes extra="1" `, 1)))
	if err != nil || !hasDiag(doc.Diagnostics, "unknown-attribute", "/NS/Changes") {
		t.Errorf("unknown attribute on Changes: %v", err)
	}
	trigger := strings.Replace(OfficialShaped, `<CDCs>`, `<TriggerOptions><TriggerOption abbreviation="dchg"><ApplicableServices/></TriggerOption></TriggerOptions><CDCs>`, 1)
	doc, err = nsd.Parse(strings.NewReader(trigger))
	if err != nil || !hasDiag(doc.Diagnostics, "unknown-element", "TriggerOption[dchg]") {
		t.Errorf("ApplicableServices on a trigger option is reported: %v", err)
	}
}
