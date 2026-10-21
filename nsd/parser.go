// SPDX-License-Identifier: MIT

package nsd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/otfabric/go-iec61850-model/model"
)

// DocumentKind is the kind of an NSD XML document, given by its root
// element.
type DocumentKind int

// Document kinds.
const (
	// KindNS is a namespace definition (root element NS).
	KindNS DocumentKind = iota + 1
	// KindNSDoc is a documentation file (root element NSDoc).
	KindNSDoc
	// KindServiceNS is a service namespace definition (root element
	// ServiceNS). Its content is recognized but not modeled yet.
	KindServiceNS
	// KindApplicableServiceNS is an applicable services definition
	// (root element ApplicableServiceNS). Its content is recognized but
	// not modeled yet.
	KindApplicableServiceNS
)

// String returns the root element name of the kind.
func (k DocumentKind) String() string {
	switch k {
	case KindNS:
		return "NS"
	case KindNSDoc:
		return "NSDoc"
	case KindServiceNS:
		return "ServiceNS"
	case KindApplicableServiceNS:
		return "ApplicableServiceNS"
	default:
		return fmt.Sprintf("DocumentKind(%d)", int(k))
	}
}

var rootKinds = map[string]DocumentKind{
	"NS":                  KindNS,
	"NSDoc":               KindNSDoc,
	"ServiceNS":           KindServiceNS,
	"ApplicableServiceNS": KindApplicableServiceNS,
}

// Document is one parsed NSD XML document.
type Document struct {
	// Kind is the document kind.
	Kind DocumentKind
	// SchemaNamespace is the XML namespace URI of the root element.
	SchemaNamespace string
	// SHA256 is the hex-encoded SHA-256 digest of the document bytes.
	SHA256 string
	// Namespace is set for [KindNS] documents.
	Namespace *model.Namespace
	// NSDoc is set for [KindNSDoc] documents.
	NSDoc *NSDoc
	// ID identifies the namespace the document belongs to, for every
	// kind.
	ID model.NamespaceID
	// Diagnostics lists warnings and informational findings. A
	// document with error diagnostics is not returned; the error is a
	// [*model.ValidationError] carrying them.
	Diagnostics model.Diagnostics
}

// Parse reads and parses one NSD XML document: a namespace definition
// (NS), a documentation file (NSDoc), or a service namespace, which is
// recognized but not modeled.
//
// The input is bounded by the configured size, depth and token limits;
// DOCTYPE declarations are rejected. A document whose structure has
// errors, or warnings in strict mode, is rejected with a
// [*model.ValidationError] that lists every finding.
func Parse(r io.Reader, opts ...Option) (*Document, error) {
	o := buildOptions(opts)
	data, err := readBounded(r, o.MaxDocumentSize, o.Source)
	if err != nil {
		return nil, err
	}
	return parseBytes(data, &o)
}

// ParseNamespace parses an NS document and returns its namespace. Use
// [Parse] to accept other document kinds.
func ParseNamespace(r io.Reader, opts ...Option) (*model.Namespace, model.Diagnostics, error) {
	doc, err := Parse(r, opts...)
	if err != nil {
		return nil, nil, err
	}
	if doc.Kind != KindNS {
		return nil, doc.Diagnostics, &model.SourceError{
			Source: sourceName(buildOptions(opts).Source),
			Reason: fmt.Sprintf("expected an NS document, got %s", doc.Kind),
			Err:    model.ErrUnsupportedSchema,
		}
	}
	return doc.Namespace, doc.Diagnostics, nil
}

func readBounded(r io.Reader, limit int64, source string) ([]byte, error) {
	if r == nil {
		return nil, &model.SourceError{Source: sourceName(source), Reason: "nil reader", Err: model.ErrSourceUnavailable}
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, &model.SourceError{Source: sourceName(source), Reason: "read failed", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	if int64(len(data)) > limit {
		return nil, &model.SourceError{
			Source: sourceName(source),
			Reason: fmt.Sprintf("document exceeds %d bytes", limit),
			Err:    model.ErrLimitExceeded,
		}
	}
	return data, nil
}

func parseBytes(data []byte, o *Options) (*Document, error) {
	src := sourceName(o.Source)
	root, err := prescan(data, o)
	if err != nil {
		return nil, err
	}
	kind, ok := rootKinds[root.Local]
	if !ok {
		return nil, &model.SourceError{Source: src, Reason: fmt.Sprintf("unsupported root element %q", root.Local), Err: model.ErrUnsupportedSchema}
	}
	if !slices.Contains(o.SchemaNamespaces, root.Space) {
		return nil, &model.SourceError{Source: src, Reason: fmt.Sprintf("unsupported XML namespace %q", root.Space), Err: model.ErrUnsupportedSchema}
	}
	sum := sha256.Sum256(data)
	doc := &Document{Kind: kind, SchemaNamespace: root.Space, SHA256: hex.EncodeToString(sum[:])}
	n := &normalizer{source: src, strict: o.Strict}

	switch kind {
	case KindNS:
		var raw rawNS
		if err := decode(data, &raw); err != nil {
			return nil, &model.SourceError{Source: src, Reason: "malformed NS document", Err: errors.Join(model.ErrInvalidNSD, err)}
		}
		ns := n.namespace(&raw)
		ns.Provenance = model.Provenance{Source: o.Source, SHA256: doc.SHA256, SchemaNamespace: root.Space}
		n.diags = append(n.diags, Validate(ns)...)
		doc.Namespace, doc.ID = ns, ns.ID
	case KindNSDoc:
		var raw rawNSDoc
		if err := decode(data, &raw); err != nil {
			return nil, &model.SourceError{Source: src, Reason: "malformed NSDoc document", Err: errors.Join(model.ErrInvalidNSD, err)}
		}
		doc.NSDoc = n.nsdoc(&raw)
		doc.ID = doc.NSDoc.ID
	case KindServiceNS, KindApplicableServiceNS:
		var raw rawAny
		if err := decode(data, &raw); err != nil {
			return nil, &model.SourceError{Source: src, Reason: "malformed document", Err: errors.Join(model.ErrInvalidNSD, err)}
		}
		doc.ID = n.idFromAttrs(raw.Attrs, "/"+root.Local)
		n.add(model.SeverityInfo, "not-modeled", "/"+root.Local, fmt.Sprintf("%s documents are recognized but their content is not modeled yet", root.Local))
	}

	doc.Diagnostics = n.diags
	for i := range doc.Diagnostics {
		doc.Diagnostics[i].Source = src
		if doc.Diagnostics[i].Namespace.IsZero() {
			doc.Diagnostics[i].Namespace = doc.ID
		}
	}
	if o.Strict {
		for i := range doc.Diagnostics {
			if doc.Diagnostics[i].Severity == model.SeverityWarning {
				doc.Diagnostics[i].Severity = model.SeverityError
			}
		}
	}
	if err := doc.Diagnostics.Err(); err != nil {
		return nil, err
	}
	return doc, nil
}

func decode(data []byte, v any) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	return d.Decode(v)
}

// prescan walks the token stream once to enforce depth and token
// limits, reject DTDs and entity declarations, and find the root
// element, before the document is decoded into structures.
func prescan(data []byte, o *Options) (xml.Name, error) {
	src := sourceName(o.Source)
	d := xml.NewDecoder(bytes.NewReader(data))
	var root xml.Name
	depth, tokens := 0, 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return root, &model.SourceError{Source: src, Reason: "malformed XML", Err: errors.Join(model.ErrInvalidNSD, err)}
		}
		tokens++
		if tokens > o.MaxTokens {
			return root, &model.SourceError{Source: src, Reason: fmt.Sprintf("document exceeds %d XML tokens", o.MaxTokens), Err: model.ErrLimitExceeded}
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if root.Local != "" {
					return root, &model.SourceError{Source: src, Reason: "multiple root elements", Err: model.ErrInvalidNSD}
				}
				root = t.Name
			}
			depth++
			if depth > o.MaxDepth {
				line, _ := d.InputPos()
				return root, &model.SourceError{Source: src, Reason: fmt.Sprintf("element nesting exceeds depth %d at line %d", o.MaxDepth, line), Err: model.ErrLimitExceeded}
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			line, _ := d.InputPos()
			return root, &model.SourceError{Source: src, Reason: fmt.Sprintf("DTD and entity declarations are not allowed (line %d)", line), Err: model.ErrInvalidNSD}
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(t)) > 0 {
				return root, &model.SourceError{Source: src, Reason: "text outside the root element", Err: model.ErrInvalidNSD}
			}
		}
	}
	if root.Local == "" {
		return root, &model.SourceError{Source: src, Reason: "no root element", Err: model.ErrInvalidNSD}
	}
	return root, nil
}

func sourceName(s string) string {
	if s == "" {
		return "<input>"
	}
	return s
}
