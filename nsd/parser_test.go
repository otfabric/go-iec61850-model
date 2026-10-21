// SPDX-License-Identifier: MIT

package nsd_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

const nsOpen = `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" revision="A" release="1">`

func parse(t *testing.T, doc string, opts ...nsd.Option) (*nsd.Document, error) {
	t.Helper()
	return nsd.Parse(strings.NewReader(doc), opts...)
}

func hasDiag(ds model.Diagnostics, code, pathPart string) bool {
	for _, d := range ds {
		if d.Code == code && strings.Contains(d.Path, pathPart) {
			return true
		}
	}
	return false
}

func TestParseFixtures(t *testing.T) {
	doc, err := nsd.Parse(bytes.NewReader(fixtures.Read(t, fixtures.NS73)), nsd.WithSource(fixtures.NS73))
	if err != nil {
		t.Fatal(err)
	}
	ns := doc.Namespace
	if doc.Kind != nsd.KindNS || doc.Kind.String() != "NS" || ns.ID != fixtures.ID73 || doc.ID != fixtures.ID73 {
		t.Fatalf("kind %v id %v", doc.Kind, ns.ID)
	}
	if doc.SchemaNamespace != nsd.SchemaNamespace2016 || len(doc.SHA256) != 64 || ns.Provenance.SHA256 != doc.SHA256 || ns.Provenance.Source != fixtures.NS73 {
		t.Errorf("provenance %+v", ns.Provenance)
	}
	if len(ns.Dependencies) != 1 || ns.Dependencies[0] != (model.NamespaceID{Standard: "IEC 61850-7-2", Version: "2007", Revision: "B"}) {
		t.Errorf("dependencies %v", ns.Dependencies)
	}
	mv, ok := ns.CDC("MV")
	if !ok || !mv.Statistics || len(mv.DataAttributes) != 6 {
		t.Fatalf("MV %+v", mv)
	}
	mag, _ := mv.DataAttribute("mag")
	if mag.Type != "AnalogueValue" || mag.TypeKind != model.TypeKindConstructed || mag.FC != model.FCMX || !mag.Dchg || !mag.Dupd || mag.Qchg || !mag.Presence.IsMandatory() || mag.Position != 1 {
		t.Errorf("mag %+v", mag)
	}
	ens, _ := ns.CDC("ENS")
	if !ens.EnumParameterized || ens.DataAttributes[0].Type != "" {
		t.Errorf("ENS %+v", ens)
	}
	wye, _ := ns.CDC("WYE")
	if len(wye.SubDataObjects) != 5 || wye.DataAttributes[0].Position != 5 {
		t.Errorf("WYE positions: %+v", wye)
	}
	av, _ := ns.ConstructedAttribute("AnalogueValue")
	if f, _ := av.SubDataAttribute("f"); f.Presence.String() != "AtLeastOne(1)" || f.FC != "" {
		t.Errorf("AnalogueValue.f %+v", f)
	}
	mult, _ := ns.Enumeration("MultiplierKind")
	if l, ok := mult.LiteralByValue(-3); !ok || l.Name != "m" {
		t.Errorf("MultiplierKind %+v", mult)
	}
	if !hasDiag(doc.Diagnostics, "empty-literal-name", "MultiplierKind") {
		t.Errorf("expected empty-literal-name info, got %v", doc.Diagnostics)
	}
	if doc.Diagnostics.HasErrors() {
		t.Errorf("unexpected errors %v", doc.Diagnostics)
	}
}

func TestParseNamespace74(t *testing.T) {
	ns, diags, err := nsd.ParseNamespace(bytes.NewReader(fixtures.Read(t, fixtures.NS74)))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Errorf("diagnostics: %v", diags)
	}
	if len(ns.AbstractLNClasses) != 2 || len(ns.LNClasses) != 5 {
		t.Fatalf("classes: %d abstract, %d concrete", len(ns.AbstractLNClasses), len(ns.LNClasses))
	}
	mmxu, _ := ns.LogicalNodeClass("MMXU")
	if mmxu.Base != "StatisticsLN" || mmxu.Abstract || mmxu.Doc.TitleID != "test.MMXU.title" || mmxu.Source != fixtures.ID74 {
		t.Errorf("MMXU %+v", mmxu)
	}
	dom, _ := ns.LogicalNodeClass("DomainLN")
	if !dom.Abstract {
		t.Error("DomainLN must be abstract")
	}
	beh, _ := dom.DataObject("Beh")
	if beh.UnderlyingType != "BehaviourModeKind" || beh.UnderlyingTypeKind != model.TypeKindEnumerated {
		t.Errorf("Beh %+v", beh)
	}
	lln0, _ := ns.LogicalNodeClass("LLN0")
	if !lln0.CanHaveLOG {
		t.Error("LLN0 canHaveLOG")
	}
}

func TestParseNSDocAndApply(t *testing.T) {
	doc, err := nsd.Parse(bytes.NewReader(fixtures.Read(t, fixtures.NSDoc74)))
	if err != nil {
		t.Fatal(err)
	}
	d := doc.NSDoc
	if doc.Kind != nsd.KindNSDoc || d.Lang != "en" || d.Len() != 6 || len(d.IDs()) != 6 {
		t.Fatalf("NSDoc %+v", d)
	}
	if txt, ok := d.Text("test.MMXU.desc"); !ok || txt != "Test description of a three-phase measurement class." {
		t.Errorf("markup and white space must be flattened: %q", txt)
	}
	if !d.Matches(fixtures.ID74R2) || d.Matches(fixtures.ID73) {
		t.Error("Matches")
	}
	ns, _, err := nsd.ParseNamespace(bytes.NewReader(fixtures.Read(t, fixtures.NS74)))
	if err != nil {
		t.Fatal(err)
	}
	if n := nsd.ApplyDoc(ns, d); n != 6 {
		t.Errorf("applied %d texts, want 6", n)
	}
	mmxu, _ := ns.LogicalNodeClass("MMXU")
	hz, _ := mmxu.DataObject("Hz")
	if mmxu.Doc.Title != "Test measurement class" || hz.Doc.Description != "Test text for frequency." || ns.Doc.Title == "" {
		t.Errorf("docs not applied: %+v %+v", mmxu.Doc, hz.Doc)
	}
	if nsd.ApplyDoc(nil, d) != 0 || nsd.ApplyDoc(ns, nil) != 0 {
		t.Error("nil arguments must apply nothing")
	}
}

func TestApplyDocAllElements(t *testing.T) {
	src := nsOpen + `
<BasicTypes><BasicType name="B" titleID="t"/></BasicTypes>
<FunctionalConstraints><FunctionalConstraint abbreviation="ST" titleID="t"/></FunctionalConstraints>
<TriggerOptions><TriggerOption abbreviation="dchg" titleID="t"/></TriggerOptions>
<PresenceConditions><PresenceCondition name="M" titleID="t"/></PresenceConditions>
<Abbreviations><Abbreviation name="Tot" descID="t"/></Abbreviations>
<Enumerations><Enumeration name="E" titleID="t"><Literal name="a" literalVal="1" descID="t"/></Enumeration></Enumerations>
<ConstructedAttributes><ConstructedAttribute name="C" titleID="t"><SubDataAttribute name="x" type="B" descID="t"/></ConstructedAttribute></ConstructedAttributes>
<CDCs><CDC name="X" titleID="t"><SubDataObject name="s" type="X" descID="t"/><DataAttribute name="a" fc="ST" type="B" descID="t"/><ServiceParameter name="ctlVal" type="B" descID="t"/></CDC></CDCs>
<AbstractLNClasses><AbstractLNClass name="A" titleID="t"><DataObject name="D" type="X" descID="t"/></AbstractLNClass></AbstractLNClasses>
</NS>`
	ns, _, err := nsd.ParseNamespace(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(ns.CDCs[0].ServiceParameters) != 1 || ns.CDCs[0].ServiceParameters[0].Position != 2 {
		t.Errorf("service parameters %+v", ns.CDCs[0].ServiceParameters)
	}
	docSrc := `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" revision="A" lang="en"><Doc id="t">text</Doc></NSDoc>`
	doc, err := nsd.Parse(strings.NewReader(docSrc))
	if err != nil {
		t.Fatal(err)
	}
	if n := nsd.ApplyDoc(ns, doc.NSDoc); n != 15 {
		t.Errorf("applied %d texts, want 15", n)
	}
}

func TestParseServiceNS(t *testing.T) {
	doc, err := parse(t, `<ServiceNS xmlns="http://www.iec.ch/61850/2016/NSD" id="IEC 61850-7-2" version="2007" revision="B" release="3"><Services/></ServiceNS>`)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Kind != nsd.KindServiceNS || doc.Namespace != nil || doc.ID.Release != 3 || !hasDiag(doc.Diagnostics, "not-modeled", "ServiceNS") {
		t.Errorf("doc %+v", doc)
	}
	if nsd.KindApplicableServiceNS.String() != "ApplicableServiceNS" || nsd.DocumentKind(99).String() != "DocumentKind(99)" {
		t.Error("kind strings")
	}
	_, _, err = nsd.ParseNamespace(strings.NewReader(`<ServiceNS xmlns="http://www.iec.ch/61850/2016/NSD" id="X" version="1"/>`))
	if !errors.Is(err, model.ErrUnsupportedSchema) {
		t.Errorf("ParseNamespace(ServiceNS) error = %v", err)
	}
}

func TestParseUnknownContentIsReported(t *testing.T) {
	src := nsOpen + `<Future x="1"/>
<CDCs foo="bar"><CDC name="X" extra="1"><DataAttribute name="a" fc="ST" type="BOOLEAN" newAttr="y"><Nested/></DataAttribute><Other/></CDC></CDCs>
<LNClasses><LNClass name="ABCD"><DataObject name="D" type="X" fc="ST" typeKind="BASIC" dchg="true"/><Unknown/></LNClass></LNClasses>
<ConstructedAttributes><ConstructedAttribute name="C"><Weird/><SubDataAttribute name="s" type="X" fc="MX" underlyingType="Q" transient="true"/></ConstructedAttribute></ConstructedAttributes>
</NS>`
	doc, err := parse(t, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ code, path string }{
		{"unknown-element", "/NS"},
		{"unknown-attribute", "/NS/CDCs"},
		{"unknown-attribute", "CDC[X]"},
		{"unknown-attribute", "DataAttribute[a]"},
		{"unknown-element", "DataAttribute[a]"},
		{"unknown-element", "CDC[X]"},
		{"unexpected-attribute", "DataObject[D]"},
		{"unknown-element", "LNClass[ABCD]"},
		{"unknown-element", "ConstructedAttribute[C]"},
		{"unexpected-attribute", "SubDataAttribute[s]"},
	} {
		if !hasDiag(doc.Diagnostics, want.code, want.path) {
			t.Errorf("missing %s at %s in %v", want.code, want.path, doc.Diagnostics)
		}
	}
	for _, d := range doc.Diagnostics {
		if d.Source != "<input>" || d.Namespace.Standard != "T" {
			t.Errorf("diagnostic without source or namespace: %+v", d)
		}
	}
	_, err = parse(t, src, nsd.WithStrict())
	var ve *model.ValidationError
	if !errors.As(err, &ve) || !errors.Is(err, model.ErrInvalidNSD) {
		t.Fatalf("strict parse error = %v", err)
	}
}

func TestParseStructuralErrors(t *testing.T) {
	tests := []struct {
		name, body, code, path string
	}{
		{"missing name", `<CDCs><CDC/></CDCs>`, "missing-attribute", "/NS/CDCs/CDC"},
		{"missing fc", `<CDCs><CDC name="X"><DataAttribute name="a" type="B"/></CDC></CDCs>`, "missing-attribute", "DataAttribute[a]"},
		{"missing DO type", `<LNClasses><LNClass name="ABCD"><DataObject name="D"/></LNClass></LNClasses>`, "missing-attribute", "DataObject[D]"},
		{"bad boolean", `<CDCs><CDC name="X" statistics="yes"/></CDCs>`, "invalid-boolean", "CDC[X]"},
		{"bad literal", `<Enumerations><Enumeration name="E"><Literal name="a" literalVal="x"/></Enumeration></Enumerations>`, "invalid-literal", "Literal[a]"},
		{"duplicate CDC", `<CDCs><CDC name="X"/><CDC name="X"/></CDCs>`, "duplicate-definition", "CDC[X]"},
		{"duplicate LN across abstract", `<AbstractLNClasses><AbstractLNClass name="A"/></AbstractLNClasses><LNClasses><LNClass name="A"/></LNClasses>`, "duplicate-definition", "LNClass[A]"},
		{"duplicate DO", `<LNClasses><LNClass name="ABCD"><DataObject name="D" type="X"/><DataObject name="D" type="X"/></LNClass></LNClasses>`, "duplicate-definition", "DataObject[D]"},
		{"duplicate member", `<CDCs><CDC name="X"><SubDataObject name="a" type="X"/><DataAttribute name="a" fc="ST" type="B"/></CDC></CDCs>`, "duplicate-definition", "member[a]"},
		{"duplicate literal value", `<Enumerations><Enumeration name="E"><Literal name="a" literalVal="1"/><Literal name="b" literalVal="1"/></Enumeration></Enumerations>`, "duplicate-literal-value", "Literal[b]"},
		{"duplicate sub attribute", `<ConstructedAttributes><ConstructedAttribute name="C"><SubDataAttribute name="a" type="B"/><SubDataAttribute name="a" type="B"/></ConstructedAttribute></ConstructedAttributes>`, "duplicate-definition", "SubDataAttribute[a]"},
		{"self inheritance", `<LNClasses><LNClass name="ABCD" base="ABCD"/></LNClasses>`, "self-inheritance", "LNClass[ABCD]"},
		{"enum self inheritance", `<Enumerations><Enumeration name="E" inheritedFrom="E"/></Enumerations>`, "self-inheritance", "Enumeration[E]"},
		{"self dependency", `<DependsOn id="T" version="2026" revision="A"/>`, "self-dependency", "DependsOn[T]"},
		{"bad release", `<DependsOn id="X" version="1" release="x"/>`, "invalid-release", "DependsOn[X]"},
		{"duplicate basic", `<BasicTypes><BasicType name="B"/><BasicType name="B"/></BasicTypes>`, "duplicate-definition", "BasicType[B]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(t, nsOpen+tc.body+`</NS>`)
			var ve *model.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("error = %v, want *ValidationError", err)
			}
			if !hasDiag(ve.Diagnostics, tc.code, tc.path) {
				t.Errorf("missing %s at %s in %v", tc.code, tc.path, ve.Diagnostics)
			}
		})
	}
}

func TestParseRejectsUnsafeOrInvalidInput(t *testing.T) {
	deep := `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1">` + strings.Repeat("<a>", 70) + strings.Repeat("</a>", 70) + `</NS>`
	tests := []struct {
		name     string
		doc      string
		opts     []nsd.Option
		sentinel error
	}{
		{"doctype", `<?xml version="1.0"?><!DOCTYPE NS [<!ENTITY x "y">]>` + nsOpen + `</NS>`, nil, model.ErrInvalidNSD},
		{"malformed", nsOpen + `<CDCs>`, nil, model.ErrInvalidNSD},
		{"empty", ``, nil, model.ErrInvalidNSD},
		{"text outside root", `junk` + nsOpen + `</NS>`, nil, model.ErrInvalidNSD},
		{"two roots", nsOpen + `</NS>` + nsOpen + `</NS>`, nil, model.ErrInvalidNSD},
		{"too deep", deep, nil, model.ErrLimitExceeded},
		{"too large", nsOpen + `</NS>`, []nsd.Option{nsd.WithMaxDocumentSize(10)}, model.ErrLimitExceeded},
		{"too many tokens", nsOpen + `<CDCs/><CDCs/></NS>`, []nsd.Option{nsd.WithMaxTokens(3)}, model.ErrLimitExceeded},
		{"depth option", nsOpen + `<CDCs><CDC name="X"/></CDCs></NS>`, []nsd.Option{nsd.WithMaxDepth(2)}, model.ErrLimitExceeded},
		{"unknown root", `<SCL xmlns="http://www.iec.ch/61850/2003/SCL"/>`, nil, model.ErrUnsupportedSchema},
		{"unknown namespace", `<NS xmlns="urn:other" id="T" version="1"/>`, nil, model.ErrUnsupportedSchema},
		{"no namespace", `<NS id="T" version="1"/>`, nil, model.ErrUnsupportedSchema},
		{"charset", `<?xml version="1.0" encoding="ISO-8859-1"?>` + nsOpen + `</NS>`, nil, model.ErrInvalidNSD},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(t, tc.doc, tc.opts...)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("error = %v, want %v", err, tc.sentinel)
			}
			var se *model.SourceError
			if !errors.As(err, &se) {
				t.Errorf("error %T is not a *SourceError", err)
			}
		})
	}
	if _, err := nsd.Parse(nil); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("nil reader error = %v", err)
	}
	if _, err := nsd.Parse(errReader{}); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("failing reader error = %v", err)
	}
}

func TestWithSchemaNamespaces(t *testing.T) {
	doc := `<NS xmlns="urn:future" id="T" version="1"/>`
	if _, err := parse(t, doc, nsd.WithSchemaNamespaces("urn:future")); err != nil {
		t.Fatalf("custom schema namespace rejected: %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestReadManifest(t *testing.T) {
	src := `<?xml version="1.0"?>
<IECManifest xmlns="urn:x" xmlns:x="urn:y" version="1" x:other="2">
  <Package name="P">
    <File name="a.nsd" sha256="abc">note</File>
    <File name="b.nsdoc"/>
  </Package>
</IECManifest>`
	m, err := nsd.ReadManifest(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.Root != "IECManifest" || len(m.Attrs) != 2 || len(m.Elements) != 3 || len(m.SHA256) != 64 {
		t.Fatalf("manifest %+v", m)
	}
	if v, ok := m.Attr("version"); !ok || v != "1" {
		t.Error("root attr")
	}
	if _, ok := m.Attr("missing"); ok {
		t.Error("missing attr found")
	}
	f := m.Elements[1]
	if f.Path != "Package/File" || f.Text != "note" {
		t.Errorf("element %+v", f)
	}
	if v, ok := f.Attr("sha256"); !ok || v != "abc" {
		t.Error("element attr")
	}
	if _, err := nsd.ReadManifest(strings.NewReader(`<!DOCTYPE x><x/>`)); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("DOCTYPE manifest error = %v", err)
	}
	if _, err := nsd.ReadManifest(strings.NewReader(`<x/>`), nsd.WithMaxDocumentSize(2)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("oversized manifest error = %v", err)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(nsOpen + `<CDCs><CDC name="X"><DataAttribute name="a" fc="ST" type="B"/></CDC></CDCs></NS>`))
	f.Add([]byte(`<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1"><Doc id="a">x</Doc></NSDoc>`))
	f.Add([]byte(`<!DOCTYPE x>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := nsd.Parse(bytes.NewReader(data), nsd.WithMaxDocumentSize(1<<16))
		if err == nil && doc == nil {
			t.Fatal("nil document without error")
		}
		if err != nil && doc != nil {
			t.Fatal("document with error")
		}
	})
}
