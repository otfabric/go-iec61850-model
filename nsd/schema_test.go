// SPDX-License-Identifier: MIT

package nsd_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

const testXSD = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.iec.ch/61850/2016/NSD">
  <xs:element name="NS"/><xs:element name="Extra"/>
  <xs:complexType name="T"><xs:attribute name="name"/><xs:attribute name="newOne"/><xs:attribute ref="x"/></xs:complexType>
</xs:schema>`

func TestParseSchemaAndGaps(t *testing.T) {
	s, err := nsd.ParseSchema(strings.NewReader(testXSD), nsd.WithSource("t.xsd"))
	if err != nil {
		t.Fatal(err)
	}
	if s.TargetNamespace != nsd.SchemaNamespace2016 || len(s.Elements) != 2 || len(s.Attributes) != 2 || len(s.SHA256) != 64 || s.Source != "t.xsd" {
		t.Fatalf("schema %+v", s)
	}
	els, attrs := s.Gaps()
	if !slices.Equal(els, []string{"Extra"}) || !slices.Equal(attrs, []string{"newOne"}) {
		t.Errorf("gaps %v %v", els, attrs)
	}
	se, sa := nsd.Supported()
	for _, want := range []string{"LNClass", "DataAttribute", "SubDataAttribute", "Literal", "Doc"} {
		if !slices.Contains(se, want) {
			t.Errorf("Supported elements lack %s", want)
		}
	}
	for _, want := range []string{"presCond", "typeKind", "underlyingType", "literalVal", "lang", "fc"} {
		if !slices.Contains(sa, want) {
			t.Errorf("Supported attributes lack %s", want)
		}
	}
	if _, err := nsd.ParseSchema(strings.NewReader(`<NS/>`)); !errors.Is(err, model.ErrUnsupportedSchema) {
		t.Errorf("non-schema: %v", err)
	}
	if _, err := nsd.ParseSchema(strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="a"></xs:schema>`)); err == nil {
		t.Error("malformed schema accepted")
	}
	if _, err := nsd.ParseSchema(strings.NewReader(testXSD), nsd.WithMaxDocumentSize(5)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("oversized: %v", err)
	}
}

func TestPackageNamesWithoutZIPExtension(t *testing.T) {
	dir := t.TempDir()
	data := buildZIP(t,
		entry{name: "a.nsd", data: fixtures.Read(t, fixtures.NS72)},
		entry{name: "NSD.xsd", data: []byte(testXSD)},
		entry{name: "IECManifest.xml", data: []byte("<m/>")},
	)
	light := filepath.Join(dir, "IEC_61850-7-2.NSD.2007B6.light")
	full := filepath.Join(dir, "IEC_61850-7-3.NSD.2007B6.full")
	if err := os.WriteFile(light, data, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := nsd.LoadFile(light)
	if err != nil {
		t.Fatal(err)
	}
	if p.Content() != model.ContentLight || len(p.Schemas()) != 1 || len(p.Namespaces()) != 1 || p.Namespaces()[0].Provenance.Content != model.ContentLight || p.Manifest() == nil {
		t.Errorf("light package %+v", p)
	}
	if err := os.WriteFile(full, buildZIP(t, entry{name: "b.nsd", data: fixtures.Read(t, fixtures.NS73)}, entry{name: "IECManifest.xml", data: []byte("<m/>")}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme"), []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.xsd"), []byte(testXSD), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := nsd.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Namespaces()) != 2 || len(d.Manifests()) != 2 || len(d.Schemas()) != 2 {
		t.Fatalf("directory of packages: %d namespaces, %d manifests, %d schemas", len(d.Namespaces()), len(d.Manifests()), len(d.Schemas()))
	}
	if d.Namespaces()[1].Provenance.Content != model.ContentFull {
		t.Errorf("content from the package name: %+v", d.Namespaces()[1].Provenance)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.xsd"), []byte("<x/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := nsd.LoadDir(dir); !errors.Is(err, model.ErrUnsupportedSchema) {
		t.Errorf("invalid schema file: %v", err)
	}
	var none *nsd.Package
	if p, _ := nsd.LoadFile(fixtures.Path(fixtures.NS72)); p.Manifest() != nil || p.Content() != model.ContentUnknown {
		t.Errorf("plain document %v", none)
	}
}
