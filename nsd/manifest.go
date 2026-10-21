// SPDX-License-Identifier: MIT

package nsd

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// ManifestFileName is the file name of the package manifest inside
// official IEC code component archives. Some packages name it
// IECManifest.xml instead; both are recognized.
const ManifestFileName = "Manifest.xml"

// ManifestNamespace is the XML namespace of IEC code component
// manifests.
const ManifestNamespace = "http://www.iec.ch/CC/2017/IECManifest"

// isManifestFile reports whether a file name is a package manifest.
func isManifestFile(name string) bool {
	b := path.Base(name)
	return strings.EqualFold(b, ManifestFileName) || strings.EqualFold(b, "IECManifest.xml")
}

// License returns the uri and kind attributes of the manifest's License
// element, e.g. "www.iec.ch/CCv1" and "Standard".
func (m *Manifest) License() (uri, kind string, ok bool) {
	for i := range m.Elements {
		e := &m.Elements[i]
		if path.Base(e.Path) == "License" {
			uri, _ = e.Attr("uri")
			kind, _ = e.Attr("kind")
			return uri, kind, true
		}
	}
	return "", "", false
}

// Notice returns the text of the manifest's copyright notice, which
// must accompany redistributed code components.
func (m *Manifest) Notice() string {
	for i := range m.Elements {
		if path.Base(m.Elements[i].Path) == "Notice" {
			return normalizeSpace(m.Elements[i].Text)
		}
	}
	return ""
}

// Manifest is the content of an IECManifest.xml package manifest.
//
// The manifest format is not modeled field by field yet: the root
// attributes and every element below the root are kept generically,
// in document order, so no information is lost until the format has
// been verified against official packages.
type Manifest struct {
	// Source names the input, e.g. "package.zip!IECManifest.xml".
	Source string `json:"source,omitempty"`
	// Root is the local name of the root element.
	Root string `json:"root"`
	// Attrs are the attributes of the root element.
	Attrs []Attr `json:"attrs,omitempty"`
	// Elements are all elements below the root, depth first.
	Elements []ManifestElement `json:"elements,omitempty"`
	// SHA256 is the hex-encoded SHA-256 digest of the manifest bytes.
	SHA256 string `json:"sha256"`
}

// Attr is an XML attribute.
type Attr struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ManifestElement is one element of a manifest.
type ManifestElement struct {
	// Path is the slash-separated element path below the root, e.g.
	// "Files/File".
	Path  string `json:"path"`
	Attrs []Attr `json:"attrs,omitempty"`
	// Text is the trimmed character data directly inside the element.
	Text string `json:"text,omitempty"`
}

// Attr returns the value of the root attribute with the given local
// name.
func (m *Manifest) Attr(name string) (string, bool) { return findAttr(m.Attrs, name) }

// Attr returns the value of the attribute with the given local name.
func (e *ManifestElement) Attr(name string) (string, bool) { return findAttr(e.Attrs, name) }

func findAttr(attrs []Attr, name string) (string, bool) {
	for _, a := range attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// ReadManifest reads an IECManifest.xml document. It applies the same
// size, depth and DTD restrictions as [Parse].
func ReadManifest(r io.Reader, opts ...Option) (*Manifest, error) {
	o := buildOptions(opts)
	data, err := readBounded(r, o.MaxDocumentSize, o.Source)
	if err != nil {
		return nil, err
	}
	return parseManifest(data, &o)
}

func parseManifest(data []byte, o *Options) (*Manifest, error) {
	if _, err := prescan(data, o); err != nil {
		return nil, err
	}
	src := sourceName(o.Source)
	m := &Manifest{Source: o.Source, SHA256: sha256Hex(data)}
	d := xml.NewDecoder(bytes.NewReader(data))
	var stack []string
	var open []int // index into m.Elements of each open element, -1 for the root
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &model.SourceError{Source: src, Reason: "malformed manifest", Err: errors.Join(model.ErrInvalidNSD, err)}
		}
		switch t := tok.(type) {
		case xml.StartElement:
			attrs := convertAttrs(t.Attr)
			if len(stack) == 0 {
				m.Root, m.Attrs = t.Name.Local, attrs
				open = append(open, -1)
			} else {
				path := strings.Join(append(append([]string(nil), stack[1:]...), t.Name.Local), "/")
				m.Elements = append(m.Elements, ManifestElement{Path: path, Attrs: attrs})
				open = append(open, len(m.Elements)-1)
			}
			stack = append(stack, t.Name.Local)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
			open = open[:len(open)-1]
		case xml.CharData:
			if len(open) > 0 && open[len(open)-1] >= 0 {
				e := &m.Elements[open[len(open)-1]]
				e.Text = strings.TrimSpace(e.Text + string(t))
			}
		}
	}
	return m, nil
}

func convertAttrs(in []xml.Attr) []Attr {
	var out []Attr
	for _, a := range in {
		if isMetaAttr(a.Name) {
			continue
		}
		out = append(out, Attr{Name: a.Name.Local, Value: a.Value})
	}
	return out
}
