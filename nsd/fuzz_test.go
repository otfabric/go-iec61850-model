// SPDX-License-Identifier: MIT

package nsd_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// The fuzz targets of the importer. Each asserts more than "it
// returns": a result or an error, never both; an error of a documented
// category; the fixed bounds on what a result carries; and, where the
// cost is a function of the input alone, an allocation budget.

// fuzzDocumentSize bounds the documents of the fuzz targets, so that
// one execution stays fast.
const fuzzDocumentSize = 1 << 20

// documentBudget is the allocation budget of one imported document:
// 2000 bytes per byte plus a fixed part for the smallest inputs. The
// most expensive document is one of the shortest elements that decode
// into the largest structure ("<a/>" inside a CDC): about 870 bytes per
// byte. A cost that grows with the square of the document is above the
// budget at the size of the seeds below.
func documentBudget(data []byte) uint64 { return 2000*uint64(len(data)) + 1<<20 }

// oneOf reports whether err wraps one of the sentinels.
func oneOf(err error, sentinels ...error) bool {
	return slices.ContainsFunc(sentinels, func(s error) bool { return errors.Is(err, s) })
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(nsOpen + `<CDCs><CDC name="X"><DataAttribute name="a" fc="ST" type="B"/></CDC></CDCs></NS>`))
	f.Add([]byte(`<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1"><Doc id="a">x</Doc></NSDoc>`))
	f.Add([]byte(`<ServiceNS xmlns="http://www.iec.ch/61850/2016/NSD" id="IEC 61850-8-1" version="2003"/>`))
	f.Add([]byte(`<!DOCTYPE x>`))
	f.Add(fixtures.Read(f, fixtures.NS73))
	f.Add(fixtures.Read(f, fixtures.NS74))
	f.Add(fixtures.Read(f, fixtures.NSDoc74))
	f.Add([]byte(OfficialShaped))
	// The shapes of the budget tests: a long name with many children, a
	// long namespace URI with many attributes, many definitions of one
	// CDC, many attributes, a value at the end of the 32-bit range.
	var long, uri, cdcs, attrs strings.Builder
	long.WriteString(nsOpen + `<Enumerations><Enumeration name="` + strings.Repeat("N", 320000) + `">`)
	uri.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" xmlns:x="urn:` + strings.Repeat("u", 80000) + `" id="T" version="2026"`)
	cdcs.WriteString(nsOpen + `<CDCs>`)
	attrs.WriteString(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026"`)
	for i := range 8000 {
		fmt.Fprintf(&long, `<Literal name="l%d" literalVal="%d" extra="1"/>`, i, i)
		if i < 2000 {
			fmt.Fprintf(&uri, ` x:a%d="1"`, i)
		}
		if i < 4000 {
			fmt.Fprintf(&cdcs, `<CDC name="X" variant="v%d"/>`, i)
		}
	}
	for i := range 12000 {
		fmt.Fprintf(&attrs, ` a%d="1"`, i)
	}
	f.Add([]byte(long.String() + `</Enumeration></Enumerations></NS>`))
	f.Add([]byte(uri.String() + `/>`))
	f.Add([]byte(cdcs.String() + `</CDCs></NS>`))
	f.Add([]byte(attrs.String() + `/>`))
	f.Add([]byte(nsOpen + `<Enumerations><Enumeration name="E"><Literal name="a" literalVal="2147483648"/></Enumeration></Enumerations></NS>`))
	f.Add([]byte(nsOpen + "<Enumerations><Enumeration name=\"E&#10;F\"/></Enumerations></NS>"))
	f.Add([]byte(nsOpen + `<CDCs><CDC name="C">` + strings.Repeat(`<a/>`, 50000) + `</CDC></CDCs></NS>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var doc *nsd.Document
		var err error
		cost := allocated(func() { doc, err = nsd.Parse(bytes.NewReader(data), nsd.WithMaxDocumentSize(fuzzDocumentSize)) })
		if budget := documentBudget(data); cost > budget && len(data) <= fuzzDocumentSize {
			t.Errorf("a document of %d bytes allocated %d bytes, budget %d", len(data), cost, budget)
		}
		if err != nil {
			if doc != nil {
				t.Fatal("document with error")
			}
			if !oneOf(err, model.ErrInvalidNSD, model.ErrUnsupportedSchema, model.ErrLimitExceeded) {
				t.Fatalf("error of no documented category: %v", err)
			}
			var se *model.SourceError
			var ve *model.ValidationError
			if !errors.As(err, &se) && !errors.As(err, &ve) {
				t.Fatalf("error %T is neither a *SourceError nor a *ValidationError", err)
			}
			if ve != nil && (len(ve.Diagnostics) > 10000 || !model.Diagnostics(ve.Diagnostics).HasErrors()) {
				t.Fatalf("validation error with %d findings", len(ve.Diagnostics))
			}
			return
		}
		if doc == nil {
			t.Fatal("nil document without error")
		}
		if len(doc.SHA256) != 64 || doc.SchemaNamespace != nsd.SchemaNamespace2016 {
			t.Fatalf("document %q in %q", doc.SHA256, doc.SchemaNamespace)
		}
		if len(doc.Diagnostics) > 10000 || doc.Diagnostics.HasErrors() {
			t.Fatalf("document with %d findings, errors: %v", len(doc.Diagnostics), doc.Diagnostics.HasErrors())
		}
		for _, d := range doc.Diagnostics {
			// A path has at most four elements, each with a name of at
			// most 256 bytes; a message quotes at most two names.
			if len(d.Path) > 4*(2*259+4) || len(d.Message) > 2*259+128 {
				t.Fatalf("finding of %d and %d bytes: %.100s", len(d.Path), len(d.Message), d.Message)
			}
		}
		switch doc.Kind {
		case nsd.KindNS:
			ns := doc.Namespace
			if ns == nil || doc.NSDoc != nil || ns.ID != doc.ID || ns.ID.Standard == "" || ns.ID.Version == "" || ns.Provenance.SHA256 != doc.SHA256 {
				t.Fatalf("NS document %+v", doc.ID)
			}
			// What Parse accepted has no structural error.
			if ds := nsd.Validate(ns); ds.HasErrors() {
				t.Fatalf("accepted namespace does not validate: %v", ds.Errors()[0])
			}
			for _, e := range ns.Enumerations {
				for _, l := range e.Literals {
					if int(int32(l.Value)) != l.Value {
						t.Fatalf("literal %s.%s = %d is not a 32-bit integer", e.Name, l.Name, l.Value)
					}
				}
			}
		case nsd.KindNSDoc:
			if doc.NSDoc == nil || doc.Namespace != nil || doc.NSDoc.ID != doc.ID || len(doc.NSDoc.IDs()) != doc.NSDoc.Len() {
				t.Fatalf("NSDoc document %+v", doc.ID)
			}
		case nsd.KindServiceNS, nsd.KindApplicableServiceNS:
			if doc.Namespace != nil || doc.NSDoc != nil {
				t.Fatalf("%s document with content", doc.Kind)
			}
		default:
			t.Fatalf("document kind %v", doc.Kind)
		}
	})
}

func FuzzReadManifest(f *testing.F) {
	f.Add([]byte(`<IECManifest xmlns="http://www.iec.ch/CC/2017/IECManifest" version="1"><Copyright><Notice> a <b>c</b> d </Notice><License uri="u" kind="k"/></Copyright><Files><File name="x"/></Files></IECManifest>`))
	f.Add([]byte(`<m/>`))
	f.Add([]byte(`<!DOCTYPE m><m/>`))
	f.Add([]byte(`<m><e>` + strings.Repeat(`x<!---->`, 60000) + `</e></m>`))
	f.Add([]byte(`<m>` + strings.Repeat(`<f/>`, 10001) + `</m>`))
	name := strings.Repeat("N", 60000)
	f.Add([]byte(`<m><` + name + `>` + strings.Repeat(`<f/>`, 2000) + `</` + name + `></m>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var m *nsd.Manifest
		var err error
		cost := allocated(func() { m, err = nsd.ReadManifest(bytes.NewReader(data), nsd.WithMaxDocumentSize(fuzzDocumentSize)) })
		if budget := documentBudget(data); cost > budget && len(data) <= fuzzDocumentSize {
			t.Errorf("a manifest of %d bytes allocated %d bytes, budget %d", len(data), cost, budget)
		}
		if err != nil {
			if m != nil || !oneOf(err, model.ErrInvalidNSD, model.ErrLimitExceeded) {
				t.Fatalf("manifest %v, error %v", m != nil, err)
			}
			return
		}
		if m == nil || m.Root == "" || len(m.SHA256) != 64 {
			t.Fatalf("manifest %+v", m)
		}
		paths, text := 0, 0
		for _, e := range m.Elements {
			if e.Path == "" || strings.TrimSpace(e.Text) != e.Text {
				t.Fatalf("element %q with text %q", e.Path, e.Text)
			}
			paths += len(e.Path)
			text += len(e.Text)
		}
		if len(m.Elements) > 10000 || paths > 1<<20 || text > len(data) {
			t.Fatalf("%d elements, %d bytes of paths, %d bytes of text from %d bytes", len(m.Elements), paths, text, len(data))
		}
		_, _, _ = m.License()
		_ = m.Notice()
	})
}

func FuzzParseSchema(f *testing.F) {
	f.Add([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.iec.ch/61850/2016/NSD"><xs:element name="NS"/><xs:element name="Future"/><xs:attribute name="name"/><xs:attribute name="future"/></xs:schema>`))
	f.Add([]byte(`<schema/>`))
	f.Add([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="a"/><xs:element name="a"/></xs:schema>`))
	f.Add([]byte(`<!DOCTYPE s><s/>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var s *nsd.Schema
		var err error
		cost := allocated(func() { s, err = nsd.ParseSchema(bytes.NewReader(data), nsd.WithMaxDocumentSize(fuzzDocumentSize)) })
		if budget := documentBudget(data); cost > budget && len(data) <= fuzzDocumentSize {
			t.Errorf("a schema of %d bytes allocated %d bytes, budget %d", len(data), cost, budget)
		}
		if err != nil {
			if s != nil || !oneOf(err, model.ErrInvalidNSD, model.ErrUnsupportedSchema, model.ErrLimitExceeded) {
				t.Fatalf("schema %v, error %v", s != nil, err)
			}
			return
		}
		if s == nil || len(s.SHA256) != 64 {
			t.Fatalf("schema %+v", s)
		}
		for _, names := range [][]string{s.Elements, s.Attributes} {
			if !slices.IsSorted(names) || len(slices.Compact(slices.Clone(names))) != len(names) {
				t.Fatalf("names are not sorted and distinct: %.10q", names)
			}
		}
		// The gaps are names the schema declares.
		elements, attributes := s.Gaps()
		for _, e := range elements {
			if !slices.Contains(s.Elements, e) {
				t.Fatalf("gap %q is not an element of the schema", e)
			}
		}
		for _, a := range attributes {
			if !slices.Contains(s.Attributes, a) {
				t.Fatalf("gap %q is not an attribute of the schema", a)
			}
		}
	})
}

func FuzzReadZIP(f *testing.F) {
	f.Add(buildZIP(f, fixtureEntries(f, "pkg/")...))
	f.Add(buildZIP(f, entry{name: "inner.zip", data: buildZIP(f, fixtureEntries(f, "")...)}))
	f.Add(buildZIP(f, entry{name: "../a.nsd", data: []byte(nsOpen + `</NS>`)}))
	f.Add(buildZIP(f, entry{name: "NSD.xsd", data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)}, entry{name: "Manifest.xml", data: []byte(`<m/>`)}))
	f.Add(directoryOnly(3))
	f.Add(directoryOnly(1025))
	f.Add(stored(f, "inner.zip", directoryOnly(1024)))
	content := []byte(nsOpen + strings.Repeat(" ", 2<<20) + `</NS>`)
	compressed := deflated(f, content)
	f.Add(oneEntry(content, compressed, uint64(len(compressed)), false))
	f.Add(oneEntry(content, compressed, 1<<40, true))
	f.Add([]byte("PK\x05\x06" + strings.Repeat("\x00", 18)))
	f.Add([]byte("not a zip"))

	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := nsd.ReadZIP(bytes.NewReader(data), nsd.WithSource("fuzz.zip"),
			nsd.WithMaxArchiveSize(4<<20), nsd.WithMaxDocumentSize(fuzzDocumentSize))
		if err != nil {
			if p != nil {
				t.Fatal("package with error")
			}
			if !oneOf(err, model.ErrInvalidNSD, model.ErrUnsupportedSchema, model.ErrLimitExceeded, model.ErrUnsafeArchive, model.ErrNamespaceConflict) {
				t.Fatalf("error of no documented category: %v", err)
			}
			return
		}
		if p == nil || p.Source() != "fuzz.zip" || len(p.SHA256()) != 64 {
			t.Fatal("package without error is incomplete")
		}
		nss := p.Namespaces()
		for i, ns := range nss {
			if ns.Provenance.PackageSHA256 != p.SHA256() || !strings.HasPrefix(ns.Provenance.Source, "fuzz.zip!") {
				t.Fatalf("provenance %+v", ns.Provenance)
			}
			if i > 0 && nss[i-1].ID.Compare(ns.ID) >= 0 {
				t.Fatalf("namespaces are not ordered and distinct: %s, %s", nss[i-1].ID, ns.ID)
			}
		}
		if len(p.Manifests()) > 1025 || len(p.Diagnostics()) > 100000 || p.Diagnostics().HasErrors() {
			t.Fatalf("%d manifests, %d findings", len(p.Manifests()), len(p.Diagnostics()))
		}
		if (p.Manifest() == nil) != (len(p.Manifests()) == 0) {
			t.Fatal("Manifest and Manifests disagree")
		}
	})
}
