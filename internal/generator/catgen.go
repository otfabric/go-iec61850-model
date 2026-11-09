// SPDX-License-Identifier: MIT

package generator

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"go/format"
	"go/token"
	"strings"

	"github.com/otfabric/go-iec61850-model/internal/sources"
)

// Catalog file names. The package embeds only CatalogGzipFileName, the
// compact JSON compressed with gzip; CatalogJSONFileName holds the same
// data indented, for review, and is not embedded.
const (
	CatalogGoFileName   = "catalog_gen.go"
	CatalogJSONFileName = "catalog_gen.json"
	CatalogGzipFileName = "catalog_gen.json.gz"
)

// ProfileSpec is a named selection of IEC parts to embed with the
// catalog, e.g. "default" for 7-2, 7-3, 7-4, 7-410 and 7-420.
type ProfileSpec struct {
	Name        string
	Description string
	Parts       []string
}

// Catalog generates the embedded catalog of a Go package: the normalized
// namespaces of the profile as JSON (catalog_gen.json) and a Go file that
// embeds it and declares the named profiles, resolved to the embedded
// namespace releases (catalog_gen.go). The package provides the API
// around them. Local paths are reduced to file names and NSDoc texts are
// removed unless Input.IncludeDocText is set.
func Catalog(in Input, pkg string, profiles []ProfileSpec) (Files, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	if !token.IsIdentifier(pkg) {
		return nil, fmt.Errorf("generator: %q is not a valid package name", pkg)
	}
	nss, err := copyNamespaces(in)
	if err != nil {
		return nil, err
	}
	doc := jsonDoc{Generated: Marker + " generate catalog; DO NOT EDIT.", Generator: "nsdctl " + orDev(in.ToolVersion),
		Format: FormatVersion, Distribution: in.distribution(), Namespaces: nss}
	compact, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	// The same document indented, for review: what json.MarshalIndent
	// writes. Indenting valid JSON cannot fail.
	var indented bytes.Buffer
	_ = json.Indent(&indented, compact, "", "  ")
	indented.WriteByte('\n')
	data := indented.Bytes()
	gz := gzipBest(compact)

	var b bytes.Buffer
	for _, l := range in.headerLines("catalog") {
		b.WriteString(strings.TrimRight("// "+l, " ") + "\n")
	}
	fmt.Fprintf(&b, "\npackage %s\n\nimport (\n\t_ \"embed\"\n\n\t\"github.com/otfabric/go-iec61850-model/model\"\n)\n\n", pkg)
	fmt.Fprintf(&b, "// catalogJSON holds the normalized namespaces: compact JSON, compressed\n// with gzip. %s holds the same data indented, for review.\n//\n//go:embed %s\nvar catalogJSON []byte\n\n", CatalogJSONFileName, CatalogGzipFileName)
	b.WriteString("// profiles are the named profiles of sources/index.json, resolved to the\n// embedded namespace releases.\nvar profiles = []Profile{\n")
	loaded := in.namespaces()
	for _, ps := range profiles {
		ids, err := sources.NamespacesForParts(loaded, ps.Parts)
		if err != nil {
			return nil, fmt.Errorf("generator: profile %s: %w", ps.Name, err)
		}
		fmt.Fprintf(&b, "\t{\n\t\tName:        %q,\n\t\tDescription: %q,\n\t\tParts:       %#v,\n\t\tNamespaces: []model.NamespaceID{\n", ps.Name, ps.Description, ps.Parts)
		for _, id := range ids {
			fmt.Fprintf(&b, "\t\t\t{Standard: %q, Version: %q, Revision: %q, Release: %d},\n", id.Standard, id.Version, id.Revision, id.Release)
		}
		b.WriteString("\t\t},\n\t},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("generator: formatting generated Go: %w", err)
	}
	files := Files{CatalogGoFileName: src, CatalogJSONFileName: data, CatalogGzipFileName: gz}
	in.addNotice(files, "catalog")
	if !in.Publish {
		files["LOCAL-ONLY.md"] = in.localNotice("catalog")
	}
	return files, nil
}

// gzipBest compresses data with gzip at the best compression. The gzip
// header carries no name and no modification time, so the output
// depends on the data and the Go release only. The level is valid and
// writes to a bytes.Buffer do not fail, so there is no error to return.
func gzipBest(data []byte) []byte {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(data)
	_ = zw.Close()
	return buf.Bytes()
}
