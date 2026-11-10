// SPDX-License-Identifier: MIT

package nsd_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// All texts in these tests are written for the tests (OTFabric wording);
// none is taken from the IEC standards.

const condNS = nsOpen + `
<ConstructedAttributes><ConstructedAttribute name="C"><SubDataAttribute name="x" type="B" presCond="MOcond" presCondArgs="3" presCondArgsID="c.sda"/></ConstructedAttribute></ConstructedAttributes>
<CDCs><CDC name="X">
 <SubDataObject name="s" type="X" presCond="MOcond" presCondArgs="1" presCondArgsID="c.sdo"/>
 <DataAttribute name="a" fc="ST" type="B" presCond="MFcond" presCondArgs="2" presCondArgsID="c.da"/>
 <ServiceParameter name="ctlVal" type="B" presCond="OFcond" presCondArgs="4" presCondArgsID="c.sp"/>
</CDC></CDCs>
<LNClasses><LNClass name="TEST">
 <DataObject name="D" type="X" presCond="MOcond" presCondArgs="1" presCondArgsID="c.do" dsPresCond="MOcond" dsPresCondArgs="1" dsPresCondArgsID="c.ds"/>
</LNClass></LNClasses>
</NS>`

const condDoc = `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" revision="A" lang="en">
<Doc id="c.sda">test condition for x</Doc><Doc id="c.sdo">test condition for s</Doc>
<Doc id="c.da">test condition for a</Doc><Doc id="c.sp">test condition for ctlVal</Doc>
<Doc id="c.do">test condition for D</Doc><Doc id="c.ds">test data set condition for D</Doc>
</NSDoc>`

// MDL-08: NSDoc holds the texts of presCondArgsID and dsPresCondArgsID
// (IEC 61850-7-7 NSDoc); ApplyDoc fills PresenceCondition.ArgsText.
func TestSpec_7_7_PresCondArgsTexts(t *testing.T) {
	ns, _, err := nsd.ParseNamespace(strings.NewReader(condNS))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := nsd.Parse(strings.NewReader(condDoc))
	if err != nil {
		t.Fatal(err)
	}
	if n := nsd.ApplyDoc(ns, doc.NSDoc); n != 6 {
		t.Errorf("applied %d texts, want 6", n)
	}
	ln, _ := ns.LogicalNodeClass("TEST")
	d := ln.DataObjects[0]
	cdc := ns.CDCs[0]
	got := []string{
		d.Presence.ArgsText, d.DSPresence.ArgsText, cdc.SubDataObjects[0].Presence.ArgsText,
		cdc.DataAttributes[0].Presence.ArgsText, cdc.ServiceParameters[0].Presence.ArgsText,
		ns.ConstructedAttributes[0].SubDataAttributes[0].Presence.ArgsText,
	}
	want := []string{"test condition for D", "test data set condition for D", "test condition for s",
		"test condition for a", "test condition for ctlVal", "test condition for x"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ArgsText[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(ns.Presences()) != 7 {
		t.Errorf("Presences = %d", len(ns.Presences()))
	}
}

func docZIP(t *testing.T, docs ...string) []byte {
	es := []entry{{name: "t.nsd", data: []byte(strings.Replace(condNS, `release="1"`, `release="6"`, 1))}}
	for i, d := range docs {
		es = append(es, entry{name: "d" + string(rune('a'+i)) + ".nsdoc", data: []byte(d)})
	}
	return buildZIP(t, es...)
}

// MDL-09: documentation IDs change between releases; an NSDoc that
// matches the namespace version but applies nothing is reported, and
// the NSDoc of the exact release wins.
func TestNSDocReleaseMatching(t *testing.T) {
	other := `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" revision="A" release="5" lang="en"><Doc id="other.prefix.c.do">x</Doc></NSDoc>`
	p, err := nsd.ReadZIP(bytes.NewReader(docZIP(t, other)))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiag(p.Diagnostics(), "doc-ids-unmatched", "") {
		t.Errorf("diagnostics %v", p.Diagnostics())
	}
	if d := p.Namespaces()[0].LNClasses[0].DataObjects[0]; d.Presence.ArgsText != "" {
		t.Errorf("text applied: %+v", d.Presence)
	}

	older := `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" revision="A" release="5" lang="en"><Doc id="c.do">older text</Doc></NSDoc>`
	exact := `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" revision="A" release="6" lang="en"><Doc id="c.do">exact text</Doc></NSDoc>`
	for _, order := range [][]string{{older, exact}, {exact, older}} {
		p, err := nsd.ReadZIP(bytes.NewReader(docZIP(t, order...)))
		if err != nil {
			t.Fatal(err)
		}
		if d := p.Namespaces()[0].LNClasses[0].DataObjects[0]; d.Presence.ArgsText != "exact text" {
			t.Errorf("exact release must win: %q", d.Presence.ArgsText)
		}
		if hasDiag(p.Diagnostics(), "doc-ids-unmatched", "") {
			t.Errorf("unexpected diagnostic: %v", p.Diagnostics())
		}
	}
}

func testSet(t *testing.T) *nsd.DocSet {
	t.Helper()
	s := nsd.NewDocSet("en")
	for k, v := range map[string]string{
		"LN:MMXU":            "Set title for the measurement class",
		"LN:MMXU#desc":       "Set description for the measurement class",
		"DO:MMXU.TotW":       "Set text for total power",
		"DO:MMXU.TotVAr":     "Set text for total reactive power",
		"DO:DomainLN.Beh":    "Set text for behaviour",
		"CDC:MV":             "Set title for measured values",
		"DA:MV.mag":          "Set text for the magnitude",
		"SDO:WYE.phsA":       "Set text for phase A",
		"CA:AnalogueValue":   "Set title for analogue values",
		"SDA:Vector.mag":     "Set text for a vector magnitude",
		"FC:MX":              "Set title for measurands",
		"BT:FLOAT32":         "Set title for a float",
		"PC:Omulti":          "Set text for multi-instance",
		"TRG:dchg":           "Set title for data change",
		"ENUM:HealthKind":    "Set title for health",
		"LIT:HealthKind.Ok":  "Set text for ok",
		"DO:MMXU.Hz#title":   "Set title for frequency",
		"SP:SPC.ctlVal":      "Set text for a control value",
		"ABBR:Tot":           "Set text for total",
		"DO:MMXU.NoSuchName": "Set text for an orphan",
	} {
		if err := s.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestDocSetKeysAndApply(t *testing.T) {
	s := testSet(t)
	if s.Lang() != "en" || s.Len() != 19 || s.Keys()[0] != "ABBR:Tot" {
		t.Fatalf("set: %s %d %v", s.Lang(), s.Len(), s.Keys())
	}
	if txt, ok := s.Text("LN:MMXU"); !ok || txt.Title == "" || txt.Description == "" {
		t.Errorf("LN:MMXU = %+v", txt)
	}
	if txt, _ := s.Text("DO:MMXU.Hz"); txt.Title != "Set title for frequency" || txt.Description != "" {
		t.Errorf("#title suffix: %+v", txt)
	}
	for _, bad := range []string{"", "MMXU", "XX:MMXU", "LN:", "LN:MMXU#body"} {
		if err := s.Set(bad, "x"); !errors.Is(err, model.ErrInvalidNSD) {
			t.Errorf("Set(%q) = %v", bad, err)
		}
	}
	if len(nsd.DocKinds()) != 15 || nsd.DocKinds()[0].Prefix != "ABBR" {
		t.Errorf("DocKinds = %v", nsd.DocKinds())
	}
	if nsd.DocKey(nsd.KeyDataObject, "MMXU", "TotW") != "DO:MMXU.TotW" {
		t.Error("DocKey")
	}

	p := fixtures.Package(t)
	var ns74 *model.Namespace
	for _, ns := range p.Namespaces() {
		if ns.ID == fixtures.ID74 {
			ns74 = ns
		}
	}
	keys := nsd.DocKeys(ns74)
	for _, k := range []string{"LN:MMXU", "DO:MMXU.TotW", "DO:DomainLN.Beh", "LN:StatisticsLN"} {
		if !contains(keys, k) {
			t.Errorf("DocKeys lacks %s", k)
		}
	}
	// The package's NSDoc applied first; the set fills only the gaps.
	n := nsd.ApplyDocSet(ns74, s)
	mmxu, _ := ns74.LogicalNodeClass("MMXU")
	totW, _ := mmxu.DataObject("TotW")
	totVAr, _ := mmxu.DataObject("TotVAr")
	if mmxu.Doc.Title != "Test measurement class" || totW.Doc.Description != "Test text for total power." {
		t.Errorf("NSDoc texts must win: %+v %+v", mmxu.Doc, totW.Doc)
	}
	if totVAr.Doc.Description != "Set text for total reactive power" || n != 3 {
		t.Errorf("set texts: %+v, applied %d", totVAr.Doc, n)
	}
	if nsd.ApplyDocSet(nil, s) != 0 || nsd.ApplyDocSet(ns74, nil) != 0 {
		t.Error("nil arguments apply nothing")
	}

	all := &model.Namespace{}
	if _, _, err := nsd.ParseNamespace(strings.NewReader(condNS)); err != nil {
		t.Fatal(err)
	}
	if len(nsd.DocKeys(all)) != 0 {
		t.Error("empty namespace has no keys")
	}
}

func TestApplyDocSetAllKinds(t *testing.T) {
	c, _, err := nsd.ParseNamespace(bytes.NewReader(fixtures.Read(t, fixtures.NS73)))
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := nsd.ParseNamespace(bytes.NewReader(fixtures.Read(t, fixtures.NS72)))
	if err != nil {
		t.Fatal(err)
	}
	s := testSet(t)
	if n := nsd.ApplyDocSet(c, s) + nsd.ApplyDocSet(b, s); n != 11 {
		t.Errorf("applied %d", n)
	}
	mv, _ := c.CDC("MV")
	mag, _ := mv.DataAttribute("mag")
	health, _ := c.Enumeration("HealthKind")
	if mv.Doc.Title == "" || mag.Doc.Description == "" || health.Doc.Title == "" || health.Literals[0].Doc.Description == "" {
		t.Errorf("not applied: %+v %+v %+v", mv.Doc, mag.Doc, health)
	}
	if b.FunctionalConstraints[1].Doc.Title == "" || b.BasicTypes[5].Doc.Title == "" || b.TriggerOptions[0].Doc.Title == "" || b.PresenceConditions[3].Doc.Description == "" {
		t.Errorf("7-2 kinds not applied")
	}
	sp := `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1" revision="A"><Abbreviations><Abbreviation name="Tot"/></Abbreviations><CDCs><CDC name="SPC"><ServiceParameter name="ctlVal" type="BOOLEAN"/></CDC></CDCs></NS>`
	x, _, err := nsd.ParseNamespace(strings.NewReader(sp))
	if err != nil {
		t.Fatal(err)
	}
	if n := nsd.ApplyDocSet(x, s); n != 2 || x.Abbreviations[0].Doc.Description == "" || x.CDCs[0].ServiceParameters[0].Doc.Description == "" {
		t.Errorf("abbreviation and service parameter: %d", n)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestDocSetFormats(t *testing.T) {
	s := testSet(t)
	data, err := nsd.EncodeDocSets(s)
	if err != nil {
		t.Fatal(err)
	}
	sets, err := nsd.ReadDocSets(data)
	if err != nil || len(sets) != 1 || sets[0].Len() != s.Len() || nsd.SelectDocSet(sets, "en") == nil || nsd.SelectDocSet(sets, "fr") != nil {
		t.Fatalf("JSON round trip: %v %v", sets, err)
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write(data)
	_ = zw.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "docs.json.gz")
	if err := os.WriteFile(path, gz.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	sets, err = nsd.LoadDocSets(path)
	if err != nil || sets[0].Source() != path {
		t.Fatalf("gzip file: %v %v", sets, err)
	}
	if txt, _ := sets[0].Text("DO:MMXU.TotW"); txt.Description != "Set text for total power" {
		t.Errorf("text = %+v", txt)
	}

	xmlSet := `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="OTFabric docs" version="1" revision="A" lang="en">
 <Doc id="DO:MMXU.TotW">XML set text for <b>total</b>
   power</Doc><Doc id="LN:MMXU#desc">XML set description</Doc></NSDoc>`
	xpath := filepath.Join(dir, "docs.nsdoc")
	if err := os.WriteFile(xpath, []byte(xmlSet), 0o600); err != nil {
		t.Fatal(err)
	}
	sets, err = nsd.LoadDocSets(xpath)
	if err != nil {
		t.Fatal(err)
	}
	if txt, _ := sets[0].Text("DO:MMXU.TotW"); txt.Description != "XML set text for total power" || sets[0].Lang() != "en" {
		t.Errorf("NSDoc-compatible set: %+v", txt)
	}

	for name, bad := range map[string]string{
		"format":    `{"format":"other","sets":[]}`,
		"json":      `{"format":`,
		"key":       `{"format":"otfabric-docset/1","sets":[{"lang":"en","texts":{"titleID.x":{"title":"x"}}}]}`,
		"suffix":    `{"format":"otfabric-docset/1","sets":[{"lang":"en","texts":{"LN:X#title":{"title":"x"}}}]}`,
		"idkeys":    `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1" revision="A" lang="en"><Doc id="IEC.x.desc">x</Doc></NSDoc>`,
		"ns":        nsOpen + `</NS>`,
		"gzip":      "\x1f\x8bnot gzip",
		"malformed": `<NSDoc`,
	} {
		if _, err := nsd.ReadDocSets([]byte(bad)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := nsd.ReadDocSets(bytes.Repeat([]byte(" "), nsd.DefaultMaxDocumentSize+1)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("oversize: %v", err)
	}
	if _, err := nsd.LoadDocSets(filepath.Join(dir, "missing.json")); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("missing file: %v", err)
	}
	badPath := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(badPath, []byte(`{"format":"x"}`), 0o600)
	var se *model.SourceError
	if _, err := nsd.LoadDocSets(badPath); !errors.As(err, &se) || !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("bad file: %v", err)
	}
}

// WithFallbackDocs: the package's own NSDoc wins; the set fills the
// rest, and only for the selected language.
func TestWithFallbackDocs(t *testing.T) {
	s := testSet(t)
	find := func(p *nsd.Package) *model.LogicalNodeClass {
		for _, ns := range p.Namespaces() {
			if ns.ID == fixtures.ID74 {
				ln, _ := ns.LogicalNodeClass("MMXU")
				return ln
			}
		}
		t.Fatal("no 7-4 fixture")
		return nil
	}
	p, err := nsd.LoadDir(fixtures.Dir(), nsd.WithFallbackDocs(s))
	if err != nil {
		t.Fatal(err)
	}
	mmxu := find(p)
	totW, _ := mmxu.DataObject("TotW")
	totVAr, _ := mmxu.DataObject("TotVAr")
	if mmxu.Doc.Title != "Test measurement class" || totW.Doc.Description != "Test text for total power." {
		t.Errorf("NSDoc must win: %+v %+v", mmxu.Doc, totW.Doc)
	}
	if totVAr.Doc.Description != "Set text for total reactive power" {
		t.Errorf("fallback not applied: %+v", totVAr.Doc)
	}
	p, err = nsd.LoadDir(fixtures.Dir(), nsd.WithFallbackDocs(s), nsd.WithLanguage("de"))
	if err != nil {
		t.Fatal(err)
	}
	if totVAr, _ := find(p).DataObject("TotVAr"); totVAr.Doc.Description != "" {
		t.Error("fallback of another language applied")
	}
}

func FuzzReadDocSets(f *testing.F) {
	for _, s := range []string{
		`{"format":"otfabric-docset/1","sets":[{"lang":"en","texts":{"LN:X":{"title":"t"}}}]}`,
		`<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1" revision="A" lang="en"><Doc id="DO:X.Y">t</Doc></NSDoc>`,
		`{"format":"otfabric-docset/1","sets":[{"lang":"en","texts":{"LN:X#x":{}}}]}`, "\x1f\x8b", "{",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		sets, err := nsd.ReadDocSets(data)
		if err != nil {
			return
		}
		out, err := nsd.EncodeDocSets(sets...)
		if err != nil {
			t.Fatal(err)
		}
		again, err := nsd.ReadDocSets(out)
		if err != nil || len(again) != len(sets) {
			t.Fatalf("round trip: %v", err)
		}
		for i := range sets {
			if again[i].Len() != sets[i].Len() {
				t.Fatalf("round trip changed set %d", i)
			}
		}
	})
}
