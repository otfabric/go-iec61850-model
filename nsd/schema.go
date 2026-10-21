// SPDX-License-Identifier: MIT

package nsd

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Schema is the inventory of an XSD file of the NSD format (IEC
// 61850-7-7): its target namespace and the element and attribute names
// it declares. It is used to find constructs of an official schema that
// the importer does not model yet; it is not an XSD validator.
type Schema struct {
	// Source names the input, e.g. "package.zip!NSD.xsd".
	Source string `json:"source"`
	// SHA256 is the hex digest of the XSD document.
	SHA256 string `json:"sha256"`
	// TargetNamespace is the targetNamespace of the schema.
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// Elements and Attributes are the declared names, sorted.
	Elements   []string `json:"elements,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}

// ParseSchema reads the inventory of one XSD document, under the same
// size, depth and DTD restrictions as [Parse].
func ParseSchema(r io.Reader, opts ...Option) (*Schema, error) {
	o := buildOptions(opts)
	data, err := readBounded(r, o.MaxDocumentSize, o.Source)
	if err != nil {
		return nil, err
	}
	return parseSchema(data, &o)
}

const xsdNamespace = "http://www.w3.org/2001/XMLSchema"

func parseSchema(data []byte, o *Options) (*Schema, error) {
	src := sourceName(o.Source)
	root, err := prescan(data, o)
	if err != nil {
		return nil, err
	}
	if root.Local != "schema" || root.Space != xsdNamespace {
		return nil, &model.SourceError{Source: src, Reason: "not an XML schema document", Err: model.ErrUnsupportedSchema}
	}
	s := &Schema{Source: o.Source, SHA256: sha256Hex(data)}
	elems, attrs := map[string]bool{}, map[string]bool{}
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &model.SourceError{Source: src, Reason: "malformed schema", Err: errors.Join(model.ErrInvalidNSD, err)}
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Space != xsdNamespace {
			continue
		}
		for _, a := range se.Attr {
			switch {
			case se.Name.Local == "schema" && a.Name.Local == "targetNamespace":
				s.TargetNamespace = a.Value
			case se.Name.Local == "element" && a.Name.Local == "name":
				elems[a.Value] = true
			case se.Name.Local == "attribute" && a.Name.Local == "name":
				attrs[a.Value] = true
			}
		}
	}
	s.Elements, s.Attributes = sortedKeys(elems), sortedKeys(attrs)
	return s, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// memberElements are the element names decoded through rawCDCMember,
// which collects them with an ",any" field.
var memberElements = []string{"SubDataObject", "DataAttribute", "ServiceParameter", "SubDataAttribute"}

// Supported returns the element and attribute names the importer
// models, derived from its decoding structures, sorted. ServiceNS and
// ApplicableServiceNS content is recognized but not modeled and is not
// included.
func Supported() (elements, attributes []string) {
	elems, attrs := map[string]bool{"NS": true, "NSDoc": true, "Doc": true}, map[string]bool{"id": true, "uri": true, "kind": true}
	for _, e := range memberElements {
		elems[e] = true
	}
	seen := map[reflect.Type]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("xml")
			name, opts, _ := strings.Cut(tag, ",")
			switch {
			case f.Anonymous:
				walk(f.Type)
			case strings.Contains(opts, "any") || name == "" || name == "-":
				walk(f.Type)
			case strings.Contains(opts, "attr"):
				attrs[name] = true
			default:
				elems[name] = true
				walk(f.Type)
			}
		}
	}
	walk(reflect.TypeOf(rawNS{}))
	walk(reflect.TypeOf(rawNSDoc{}))
	return sortedKeys(elems), sortedKeys(attrs)
}

// Gaps returns the element and attribute names the schema declares and
// the importer does not model. Content of such constructs is reported
// as warnings when it occurs in NSD documents.
func (s *Schema) Gaps() (elements, attributes []string) {
	se, sa := Supported()
	for _, e := range s.Elements {
		if !slices.Contains(se, e) {
			elements = append(elements, e)
		}
	}
	for _, a := range s.Attributes {
		if !slices.Contains(sa, a) {
			attributes = append(attributes, a)
		}
	}
	return elements, attributes
}
