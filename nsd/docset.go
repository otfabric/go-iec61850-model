// SPDX-License-Identifier: MIT

package nsd

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// Doc set keys name a model element by kind and path, independently of
// the namespace release, e.g. "LN:MMXU", "DO:MMXU.TotW", "CDC:MV",
// "DA:MV.mag" or "FC:ST". Documentation IDs (titleID, descID) differ
// between releases of a namespace; a path key does not, so one doc set
// serves every release that defines the element.
//
// A key may end in "#title" or "#desc" to name the field explicitly;
// without it the kind's default field is used (see [DocKind]).
const (
	KeyLogicalNode          = "LN"   // LN:<class>, abstract or concrete; title
	KeyDataObject           = "DO"   // DO:<class>.<data object>; description
	KeyCDC                  = "CDC"  // CDC:<cdc>, all variants; title
	KeyDataAttribute        = "DA"   // DA:<cdc>.<attribute>, device name (setMag for _setMag); description
	KeySubDataObject        = "SDO"  // SDO:<cdc>.<sub data object>; description
	KeyServiceParameter     = "SP"   // SP:<cdc>.<service parameter>; description
	KeyConstructedAttribute = "CA"   // CA:<type>; title
	KeySubDataAttribute     = "SDA"  // SDA:<type>.<member>; description
	KeyEnumeration          = "ENUM" // ENUM:<enumeration>; title
	KeyLiteral              = "LIT"  // LIT:<enumeration>.<literal>; description
	KeyFunctionalConstraint = "FC"   // FC:<abbreviation>; title
	KeyBasicType            = "BT"   // BT:<type>; title
	KeyPresenceCondition    = "PC"   // PC:<condition>; description
	KeyTriggerOption        = "TRG"  // TRG:<abbreviation>; title
	KeyAbbreviation         = "ABBR" // ABBR:<name part>; description
)

// DocKind describes a doc set key kind: its prefix and the field a key
// without "#title" or "#desc" fills.
type DocKind struct {
	Prefix string
	// Title reports whether the default field is the title; otherwise
	// it is the description.
	Title bool
}

var docKinds = map[string]bool{ // prefix → default field is the title
	KeyLogicalNode: true, KeyDataObject: false, KeyCDC: true, KeyDataAttribute: false,
	KeySubDataObject: false, KeyServiceParameter: false, KeyConstructedAttribute: true,
	KeySubDataAttribute: false, KeyEnumeration: true, KeyLiteral: false,
	KeyFunctionalConstraint: true, KeyBasicType: true, KeyPresenceCondition: false,
	KeyTriggerOption: true, KeyAbbreviation: false,
}

// DocKinds returns the key kinds, sorted by prefix.
func DocKinds() []DocKind {
	out := make([]DocKind, 0, len(docKinds))
	for p, t := range docKinds {
		out = append(out, DocKind{Prefix: p, Title: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix < out[j].Prefix })
	return out
}

// DocText is the title and description of one doc set entry.
type DocText struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// DocSet holds documentation texts keyed by model path, in one
// language. Load one with [ReadDocSets] or [LoadDocSets], build one with
// [NewDocSet] and [DocSet.Set], or convert an NSDoc file whose Doc IDs
// are path keys with [DocSetFromNSDoc]. Apply it with [ApplyDocSet] or,
// as a fallback behind a package's own NSDoc, with [WithFallbackDocs].
//
// A DocSet is not safe for concurrent modification; once built it may
// be read and applied concurrently.
type DocSet struct {
	lang   string
	source string
	texts  map[string]DocText
}

// NewDocSet returns an empty doc set for the language, e.g. "en".
func NewDocSet(lang string) *DocSet {
	return &DocSet{lang: lang, texts: map[string]DocText{}}
}

// Lang returns the language code of the set.
func (s *DocSet) Lang() string { return s.lang }

// Source names where the set was loaded from, if known.
func (s *DocSet) Source() string { return s.source }

// Len returns the number of keys.
func (s *DocSet) Len() int { return len(s.texts) }

// Keys returns the keys (without field suffix) in sorted order.
func (s *DocSet) Keys() []string {
	out := make([]string, 0, len(s.texts))
	for k := range s.texts {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Text returns the texts of a key, e.g. "DO:MMXU.TotW".
func (s *DocSet) Text(key string) (DocText, bool) {
	t, ok := s.texts[key]
	return t, ok
}

// Set stores text under key. A key with "#title" or "#desc" sets that
// field; otherwise the default field of the key kind. White space is
// normalized. It fails with [model.ErrInvalidNSD] for a malformed key.
func (s *DocSet) Set(key, text string) error {
	k, title, err := parseDocKey(key)
	if err != nil {
		return err
	}
	t := s.texts[k]
	if title {
		t.Title = normalizeSpace(text)
	} else {
		t.Description = normalizeSpace(text)
	}
	s.texts[k] = t
	return nil
}

// parseDocKey splits the field suffix off a key and validates it,
// returning the bare key and whether the field is the title.
func parseDocKey(key string) (string, bool, error) {
	k, field, explicit := strings.Cut(key, "#")
	prefix, path, ok := strings.Cut(k, ":")
	title, known := docKinds[prefix]
	switch {
	case !ok || !known:
		return "", false, fmt.Errorf("%w: doc set key %q has no known kind prefix (LN:, DO:, CDC:, ...)", model.ErrInvalidNSD, key)
	case path == "":
		return "", false, fmt.Errorf("%w: doc set key %q has an empty path", model.ErrInvalidNSD, key)
	}
	if explicit {
		switch field {
		case "title":
			title = true
		case "desc":
			title = false
		default:
			return "", false, fmt.Errorf("%w: doc set key %q: field must be #title or #desc", model.ErrInvalidNSD, key)
		}
	}
	return k, title, nil
}

// DocSetFromNSDoc converts an NSDoc file whose Doc IDs are doc set keys
// (an NSDoc-compatible doc set, e.g. <Doc id="DO:MMXU.TotW">) into a
// DocSet. It fails with [model.ErrInvalidNSD] on the first ID that is
// not a key, e.g. for an IEC NSDoc keyed by documentation IDs: apply
// those with [ApplyDoc].
func DocSetFromNSDoc(d *NSDoc) (*DocSet, error) {
	s := NewDocSet(d.Lang)
	for _, id := range d.IDs() {
		if err := s.Set(id, d.texts[id]); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// docSetFormat identifies the JSON form of doc sets.
const docSetFormat = "otfabric-docset/1"

type docSetsJSON struct {
	Format string       `json:"format"`
	Sets   []docSetJSON `json:"sets"`
}

type docSetJSON struct {
	Lang   string             `json:"lang"`
	Source string             `json:"source,omitempty"`
	Texts  map[string]DocText `json:"texts"`
}

// ReadDocSets parses doc sets: the JSON form (optionally gzip
// compressed), {"format": "otfabric-docset/1", "sets": [{"lang", "texts":
// {key: {"title", "description"}}}]}, or an NSDoc-compatible XML file
// with path keys as Doc IDs. Input is bounded by
// [DefaultMaxDocumentSize] after decompression. Errors wrap
// [model.ErrInvalidNSD] or [model.ErrLimitExceeded].
func ReadDocSets(data []byte) ([]*DocSet, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%w: doc set: %v", model.ErrInvalidNSD, err)
		}
		if data, err = io.ReadAll(io.LimitReader(zr, DefaultMaxDocumentSize+1)); err != nil {
			return nil, fmt.Errorf("%w: doc set: %v", model.ErrInvalidNSD, err)
		}
	}
	if len(data) > DefaultMaxDocumentSize {
		return nil, fmt.Errorf("%w: doc set exceeds %d bytes", model.ErrLimitExceeded, DefaultMaxDocumentSize)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return decodeDocSets(trimmed)
	}
	doc, err := Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if doc.Kind != KindNSDoc {
		return nil, fmt.Errorf("%w: doc set: expected an NSDoc document, got %s", model.ErrInvalidNSD, doc.Kind)
	}
	s, err := DocSetFromNSDoc(doc.NSDoc)
	if err != nil {
		return nil, err
	}
	return []*DocSet{s}, nil
}

func decodeDocSets(data []byte) ([]*DocSet, error) {
	var in docSetsJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("%w: doc set: %v", model.ErrInvalidNSD, err)
	}
	if in.Format != docSetFormat {
		return nil, fmt.Errorf("%w: doc set format %q, want %q", model.ErrInvalidNSD, in.Format, docSetFormat)
	}
	out := make([]*DocSet, 0, len(in.Sets))
	for _, js := range in.Sets {
		s := NewDocSet(js.Lang)
		s.source = js.Source
		for k, t := range js.Texts {
			bare, _, err := parseDocKey(k)
			if err != nil {
				return nil, err
			}
			if bare != k {
				return nil, fmt.Errorf("%w: doc set key %q: the JSON form takes no field suffix", model.ErrInvalidNSD, k)
			}
			s.texts[k] = DocText{Title: normalizeSpace(t.Title), Description: normalizeSpace(t.Description)}
		}
		out = append(out, s)
	}
	return out, nil
}

// LoadDocSets reads doc sets from a file; see [ReadDocSets]. Errors
// wrap [model.ErrSourceUnavailable] when the file cannot be read.
func LoadDocSets(path string) ([]*DocSet, error) {
	data, err := readFileBounded(path, DefaultMaxDocumentSize)
	if err != nil {
		return nil, err
	}
	sets, err := ReadDocSets(data)
	if err != nil {
		return nil, &model.SourceError{Source: path, Reason: "cannot read doc set", Err: err}
	}
	for _, s := range sets {
		if s.source == "" {
			s.source = path
		}
	}
	return sets, nil
}

// EncodeDocSets returns the JSON form of the sets, with sorted keys.
func EncodeDocSets(sets ...*DocSet) ([]byte, error) {
	out := docSetsJSON{Format: docSetFormat}
	for _, s := range sets {
		out.Sets = append(out.Sets, docSetJSON{Lang: s.lang, Source: s.source, Texts: s.texts})
	}
	return json.MarshalIndent(out, "", " ")
}

// SelectDocSet returns the set of the language among sets, or nil.
func SelectDocSet(sets []*DocSet, lang string) *DocSet {
	for _, s := range sets {
		if s.lang == lang {
			return s
		}
	}
	return nil
}

// ApplyDocSet fills the Title and Description fields that are empty in
// ns from set, by element path, and returns the number of texts
// applied. Texts an NSDoc already applied are kept, so applying a set
// after [ApplyDoc] only fills the gaps. It modifies ns, so call it
// before the namespace is added to a catalog.
func ApplyDocSet(ns *model.Namespace, set *DocSet) int {
	if ns == nil || set == nil {
		return 0
	}
	n := 0
	walkDocKeys(ns, func(key string, d *model.Doc) {
		t, ok := set.texts[key]
		if !ok {
			return
		}
		if d.Title == "" && t.Title != "" {
			d.Title = t.Title
			n++
		}
		if d.Description == "" && t.Description != "" {
			d.Description = t.Description
			n++
		}
	})
	return n
}

// DocKeys returns the doc set keys of every element of ns, sorted and
// without duplicates.
func DocKeys(ns *model.Namespace) []string {
	seen := map[string]bool{}
	walkDocKeys(ns, func(key string, _ *model.Doc) { seen[key] = true })
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// DocKey returns the doc set key of a kind and path elements, e.g.
// DocKey(KeyDataObject, "MMXU", "TotW") = "DO:MMXU.TotW".
func DocKey(kind string, path ...string) string { return kind + ":" + strings.Join(path, ".") }

// walkDocKeys calls fn for every documented element of ns with its key.
func walkDocKeys(ns *model.Namespace, fn func(key string, d *model.Doc)) {
	for i := range ns.BasicTypes {
		fn(DocKey(KeyBasicType, ns.BasicTypes[i].Name), &ns.BasicTypes[i].Doc)
	}
	for i := range ns.FunctionalConstraints {
		fn(DocKey(KeyFunctionalConstraint, string(ns.FunctionalConstraints[i].Abbreviation)), &ns.FunctionalConstraints[i].Doc)
	}
	for i := range ns.TriggerOptions {
		fn(DocKey(KeyTriggerOption, ns.TriggerOptions[i].Abbreviation), &ns.TriggerOptions[i].Doc)
	}
	for i := range ns.PresenceConditions {
		fn(DocKey(KeyPresenceCondition, ns.PresenceConditions[i].Name), &ns.PresenceConditions[i].Doc)
	}
	for i := range ns.Abbreviations {
		fn(DocKey(KeyAbbreviation, ns.Abbreviations[i].Name), &ns.Abbreviations[i].Doc)
	}
	for i := range ns.Enumerations {
		e := &ns.Enumerations[i]
		fn(DocKey(KeyEnumeration, e.Name), &e.Doc)
		for j := range e.Literals {
			fn(DocKey(KeyLiteral, e.Name, e.Literals[j].Name), &e.Literals[j].Doc)
		}
	}
	for i := range ns.ConstructedAttributes {
		c := &ns.ConstructedAttributes[i]
		fn(DocKey(KeyConstructedAttribute, c.Name), &c.Doc)
		for j := range c.SubDataAttributes {
			fn(DocKey(KeySubDataAttribute, c.Name, c.SubDataAttributes[j].Name), &c.SubDataAttributes[j].Doc)
		}
	}
	for i := range ns.CDCs {
		c := &ns.CDCs[i]
		fn(DocKey(KeyCDC, c.Name), &c.Doc)
		for j := range c.SubDataObjects {
			fn(DocKey(KeySubDataObject, c.Name, c.SubDataObjects[j].Name), &c.SubDataObjects[j].Doc)
		}
		for j := range c.DataAttributes {
			fn(DocKey(KeyDataAttribute, c.Name, c.DataAttributes[j].BaseName()), &c.DataAttributes[j].Doc)
		}
		for j := range c.ServiceParameters {
			fn(DocKey(KeyServiceParameter, c.Name, c.ServiceParameters[j].Name), &c.ServiceParameters[j].Doc)
		}
	}
	for _, lns := range [][]model.LogicalNodeClass{ns.AbstractLNClasses, ns.LNClasses} {
		for i := range lns {
			fn(DocKey(KeyLogicalNode, lns[i].Name), &lns[i].Doc)
			for j := range lns[i].DataObjects {
				fn(DocKey(KeyDataObject, lns[i].Name, lns[i].DataObjects[j].Name), &lns[i].DataObjects[j].Doc)
			}
		}
	}
}
