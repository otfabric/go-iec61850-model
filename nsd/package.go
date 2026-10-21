// SPDX-License-Identifier: MIT

package nsd

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// maxNesting bounds how deep archives inside archives are opened.
const maxNesting = 2

// Package is the content of one NSD source: a ZIP archive, a directory
// or a single document. It holds the parsed namespaces, documentation
// files, the manifest when present, and the diagnostics of the import.
//
// Package implements the source interface of the catalog package
// through its Namespaces method.
type Package struct {
	source      string
	sha256      string
	content     model.ContentKind
	manifests   []*Manifest
	namespaces  []*model.Namespace
	docs        []*NSDoc
	schemas     []*Schema
	services    []model.NamespaceID
	diagnostics model.Diagnostics
}

// Source names the input the package was loaded from.
func (p *Package) Source() string { return p.source }

// SHA256 is the hex-encoded SHA-256 digest of the archive or single
// document; empty for directories.
func (p *Package) SHA256() string { return p.sha256 }

// Content returns the package kind (light or full) as given by the
// package name, e.g. "IEC_61850-7-4.NSD.2007B6.light.zip"; unknown when
// the name does not say.
func (p *Package) Content() model.ContentKind { return p.content }

// Schemas returns the inventories of the XSD files of the package,
// ordered by source.
func (p *Package) Schemas() []*Schema { return append([]*Schema(nil), p.schemas...) }

// Manifest returns the manifest of the package, or nil when there is
// none. For a directory of packages it is the first manifest in source
// order; see [Package.Manifests].
func (p *Package) Manifest() *Manifest {
	if len(p.manifests) == 0 {
		return nil
	}
	return p.manifests[0]
}

// ServiceNamespaces returns the IDs of the service namespace documents
// (ServiceNS, e.g. IEC 61850-8-1) of the package, ordered by ID. Their
// content is recognized but not modeled yet.
func (p *Package) ServiceNamespaces() []model.NamespaceID {
	return append([]model.NamespaceID(nil), p.services...)
}

// Manifests returns every manifest found, one per archive, ordered by
// source.
func (p *Package) Manifests() []*Manifest { return append([]*Manifest(nil), p.manifests...) }

// Namespaces returns the parsed namespaces, ordered by ID.
func (p *Package) Namespaces() []*model.Namespace {
	return append([]*model.Namespace(nil), p.namespaces...)
}

// Docs returns the parsed NSDoc files, ordered by namespace ID and
// language.
func (p *Package) Docs() []*NSDoc { return append([]*NSDoc(nil), p.docs...) }

// Diagnostics returns the warnings and informational findings of the
// import.
func (p *Package) Diagnostics() model.Diagnostics {
	return append(model.Diagnostics(nil), p.diagnostics...)
}

// LoadFile loads an NSD source from a file: a ZIP archive, recognized
// by its content whatever its extension (official package names such
// as "IEC_61850-7-2.NSD.2007B6.light" may have none), an XSD schema, or
// a single XML document.
func LoadFile(name string, opts ...Option) (*Package, error) {
	o := buildOptions(opts)
	if o.Source == "" {
		o.Source = name
	}
	data, err := readFileBounded(name, o.MaxArchiveSize)
	if err != nil {
		return nil, err
	}
	if isZIP(data) {
		return readZIPBytes(data, &o)
	}
	b := newBuilder(o.Source, &o)
	b.pkg.sha256 = sha256Hex(data)
	if err := b.addFile(path.Base(filepath.ToSlash(name)), o.Source, data, 0); err != nil {
		return nil, err
	}
	return b.finish()
}

// isZIP reports whether data starts with a ZIP local file header or is
// an empty archive.
func isZIP(data []byte) bool {
	return bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06"))
}

// contentKind infers light or full from a package name.
func contentKind(name string) model.ContentKind {
	n := strings.ToLower(path.Base(filepath.ToSlash(name)))
	switch {
	case strings.Contains(n, ".light"):
		return model.ContentLight
	case strings.Contains(n, ".full"):
		return model.ContentFull
	default:
		return model.ContentUnknown
	}
}

// LoadDir loads every NSD document (.nsd, .snsd, .nsdoc), XSD schema,
// manifest and ZIP archive (by extension or content) directly inside
// dir, in name order. Subdirectories are not searched.
func LoadDir(dir string, opts ...Option) (*Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &model.SourceError{Source: dir, Reason: "cannot read directory", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	o := buildOptions(opts)
	b := newBuilder(dir, &o)
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		known := isNSDFile(e.Name()) || isSchemaFile(e.Name()) || strings.EqualFold(filepath.Ext(e.Name()), ".zip") || isManifestFile(e.Name())
		if !known && !startsWithZIP(full) {
			b.info("not-processed", full, "file is not an NSD document and was not processed")
			continue
		}
		data, err := readFileBounded(full, o.MaxArchiveSize)
		if err != nil {
			return nil, err
		}
		if err := b.addFile(e.Name(), full, data, 0); err != nil {
			return nil, err
		}
	}
	return b.finish()
}

// LoadZIP loads an NSD package from a ZIP archive on disk.
func LoadZIP(name string, opts ...Option) (*Package, error) {
	o := buildOptions(opts)
	if o.Source == "" {
		o.Source = name
	}
	data, err := readFileBounded(name, o.MaxArchiveSize)
	if err != nil {
		return nil, err
	}
	return readZIPBytes(data, &o)
}

// ReadZIP loads an NSD package from a ZIP archive held by r.
func ReadZIP(r io.Reader, opts ...Option) (*Package, error) {
	o := buildOptions(opts)
	data, err := readBounded(r, o.MaxArchiveSize, o.Source)
	if err != nil {
		return nil, err
	}
	return readZIPBytes(data, &o)
}

func readZIPBytes(data []byte, o *Options) (*Package, error) {
	b := newBuilder(sourceName(o.Source), o)
	b.pkg.sha256 = sha256Hex(data)
	b.pkg.content = contentKind(o.Source)
	if err := b.addZIP(sourceName(o.Source), data, 0); err != nil {
		return nil, err
	}
	return b.finish()
}

func readFileBounded(name string, limit int64) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, &model.SourceError{Source: name, Reason: "cannot open", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	defer func() { _ = f.Close() }()
	return readBounded(f, limit, name)
}

type builder struct {
	o       *Options
	pkg     *Package
	budget  int64 // remaining uncompressed bytes for archive entries
	entries int
	// manifestIn records the containers (archive sources) that already
	// have a manifest: one manifest per archive.
	manifestIn map[string]bool
}

func newBuilder(source string, o *Options) *builder {
	return &builder{o: o, pkg: &Package{source: source}, budget: o.MaxArchiveSize, manifestIn: map[string]bool{}}
}

func (b *builder) info(code, source, msg string) {
	b.pkg.diagnostics = append(b.pkg.diagnostics, model.Diagnostic{Severity: model.SeverityInfo, Code: code, Source: source, Message: msg})
}

func isNSDFile(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".nsd", ".snsd", ".nsdoc", ".appns":
		return true
	}
	return false
}

func isSchemaFile(name string) bool { return strings.EqualFold(path.Ext(name), ".xsd") }

// startsWithZIP reports whether the file at name starts like a ZIP
// archive.
func startsWithZIP(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 4)
	n, _ := io.ReadFull(f, head)
	return isZIP(head[:n])
}

// addFile dispatches one file by name and content: archive, manifest,
// schema or document.
func (b *builder) addFile(name, source string, data []byte, depth int) error {
	switch {
	case strings.EqualFold(path.Ext(name), ".zip") || isZIP(data):
		return b.addZIP(source, data, depth)
	case isSchemaFile(name):
		o := *b.o
		o.Source = source
		s, err := parseSchema(data, &o)
		if err != nil {
			return err
		}
		b.pkg.schemas = append(b.pkg.schemas, s)
		return nil
	case isManifestFile(name):
		o := *b.o
		o.Source = source
		m, err := parseManifest(data, &o)
		if err != nil {
			return err
		}
		container := ""
		if i := strings.LastIndex(source, "!"); i >= 0 {
			container = source[:i]
		}
		if b.manifestIn[container] {
			return &model.SourceError{Source: source, Reason: "more than one manifest in package", Err: model.ErrUnsafeArchive}
		}
		b.manifestIn[container] = true
		m.Source = source
		b.pkg.manifests = append(b.pkg.manifests, m)
		return nil
	default:
		o := *b.o
		o.Source = source
		if int64(len(data)) > o.MaxDocumentSize {
			return &model.SourceError{Source: source, Reason: fmt.Sprintf("document exceeds %d bytes", o.MaxDocumentSize), Err: model.ErrLimitExceeded}
		}
		doc, err := parseBytes(data, &o)
		if err != nil {
			return err
		}
		b.pkg.diagnostics = append(b.pkg.diagnostics, doc.Diagnostics...)
		switch doc.Kind {
		case KindNS:
			b.pkg.namespaces = append(b.pkg.namespaces, doc.Namespace)
		case KindNSDoc:
			b.pkg.docs = append(b.pkg.docs, doc.NSDoc)
		case KindServiceNS:
			if !doc.ID.IsZero() {
				b.pkg.services = append(b.pkg.services, doc.ID)
			}
		case KindApplicableServiceNS:
			// Recognized, reported by parseBytes, not modeled.
		}
		return nil
	}
}

func (b *builder) addZIP(source string, data []byte, depth int) error {
	if depth >= maxNesting {
		return &model.SourceError{Source: source, Reason: fmt.Sprintf("archives nested deeper than %d levels", maxNesting), Err: model.ErrLimitExceeded}
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return &model.SourceError{Source: source, Reason: "not a valid ZIP archive", Err: errors.Join(model.ErrInvalidNSD, err)}
	}
	seen := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		b.entries++
		if b.entries > b.o.MaxArchiveEntries {
			return &model.SourceError{Source: source, Reason: fmt.Sprintf("archive has more than %d entries", b.o.MaxArchiveEntries), Err: model.ErrLimitExceeded}
		}
		name, err := safeEntryName(f.Name)
		if err != nil {
			return &model.SourceError{Source: source + "!" + f.Name, Reason: err.Error(), Err: model.ErrUnsafeArchive}
		}
		key := strings.ToLower(name)
		if prev, dup := seen[key]; dup {
			return &model.SourceError{Source: source + "!" + f.Name, Reason: fmt.Sprintf("duplicate entry (also %q)", prev), Err: model.ErrUnsafeArchive}
		}
		seen[key] = f.Name
		mode := f.Mode()
		if mode.IsDir() {
			continue
		}
		entrySource := source + "!" + name
		if !mode.IsRegular() {
			return &model.SourceError{Source: entrySource, Reason: "entry is not a regular file", Err: model.ErrUnsafeArchive}
		}
		base := path.Base(name)
		if !isNSDFile(base) && !isSchemaFile(base) && !strings.EqualFold(path.Ext(base), ".zip") && !isManifestFile(base) {
			b.info("not-processed", entrySource, "entry is not an NSD document and was not processed")
			continue
		}
		content, err := b.readEntry(f, entrySource)
		if err != nil {
			return err
		}
		if err := b.addFile(base, entrySource, content, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// readEntry decompresses one entry within the remaining size budget and
// the compression-ratio bound. Declared sizes are checked first but not
// trusted: the actual decompressed size is enforced while reading.
func (b *builder) readEntry(f *zip.File, source string) ([]byte, error) {
	limit := b.budget
	if f.UncompressedSize64 > uint64(limit) {
		return nil, &model.SourceError{Source: source, Reason: fmt.Sprintf("entry declares %d bytes, more than the remaining archive budget", f.UncompressedSize64), Err: model.ErrLimitExceeded}
	}
	rc, err := f.Open()
	if err != nil {
		return nil, &model.SourceError{Source: source, Reason: "cannot open entry", Err: errors.Join(model.ErrInvalidNSD, err)}
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, &model.SourceError{Source: source, Reason: "cannot read entry", Err: errors.Join(model.ErrInvalidNSD, err)}
	}
	n := int64(len(data))
	if n > limit {
		return nil, &model.SourceError{Source: source, Reason: fmt.Sprintf("archive exceeds %d uncompressed bytes", b.o.MaxArchiveSize), Err: model.ErrLimitExceeded}
	}
	if c := int64(f.CompressedSize64); c > 0 && n > 1<<20 && n/c > b.o.MaxCompressionRatio {
		return nil, &model.SourceError{Source: source, Reason: fmt.Sprintf("compression ratio %d exceeds %d", n/c, b.o.MaxCompressionRatio), Err: model.ErrLimitExceeded}
	}
	b.budget -= n
	return data, nil
}

// safeEntryName returns the cleaned entry name, or an error for names
// that are absolute, escape the archive root, or are ambiguous across
// platforms.
func safeEntryName(name string) (string, error) {
	switch {
	case name == "":
		return "", errors.New("empty entry name")
	case strings.ContainsRune(name, 0):
		return "", errors.New("entry name contains NUL")
	case strings.Contains(name, `\`):
		return "", errors.New("entry name contains a backslash")
	case strings.HasPrefix(name, "/"):
		return "", errors.New("absolute entry name")
	case len(name) >= 2 && name[1] == ':':
		return "", errors.New("entry name has a drive letter")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", errors.New("entry name escapes the archive root")
		}
	}
	clean := path.Clean(name)
	if clean == "." {
		return "", errors.New("empty entry name")
	}
	return clean, nil
}

// finish orders the package content, checks for duplicate namespaces
// and applies documentation in the configured language.
func (b *builder) finish() (*Package, error) {
	p := b.pkg
	sort.SliceStable(p.namespaces, func(i, j int) bool { return p.namespaces[i].ID.Compare(p.namespaces[j].ID) < 0 })
	sort.SliceStable(p.docs, func(i, j int) bool {
		if c := p.docs[i].ID.Compare(p.docs[j].ID); c != 0 {
			return c < 0
		}
		return p.docs[i].Lang < p.docs[j].Lang
	})
	sort.SliceStable(p.services, func(i, j int) bool { return p.services[i].Compare(p.services[j]) < 0 })
	p.services = slices.Compact(p.services)
	// Packages may ship the same document twice, e.g. directly and in a
	// nested copy of the package: keep one when the bytes are identical.
	kept := p.namespaces[:0]
	for _, ns := range p.namespaces {
		if n := len(kept); n > 0 && kept[n-1].ID == ns.ID && kept[n-1].Provenance.SHA256 == ns.Provenance.SHA256 {
			p.diagnostics = append(p.diagnostics, model.Diagnostic{
				Severity: model.SeverityInfo, Code: "duplicate-document", Namespace: ns.ID, Source: ns.Provenance.Source,
				Message: "identical copy of " + kept[n-1].Provenance.Source + "; ignored",
			})
			continue
		}
		kept = append(kept, ns)
	}
	p.namespaces = kept
	for i := 1; i < len(p.namespaces); i++ {
		if p.namespaces[i].ID == p.namespaces[i-1].ID {
			return nil, &model.SourceError{
				Source: p.source,
				Reason: fmt.Sprintf("namespace %s is defined by %s and %s", p.namespaces[i].ID,
					p.namespaces[i-1].Provenance.Source, p.namespaces[i].Provenance.Source),
				Err: model.ErrNamespaceConflict,
			}
		}
	}
	sort.SliceStable(p.schemas, func(i, j int) bool { return p.schemas[i].Source < p.schemas[j].Source })
	sort.SliceStable(p.manifests, func(i, j int) bool { return p.manifests[i].Source < p.manifests[j].Source })
	for _, ns := range p.namespaces {
		if p.sha256 != "" && ns.Provenance.Source != p.source {
			ns.Provenance.PackageSHA256 = p.sha256
		}
		if ns.Provenance.Content == model.ContentUnknown {
			if p.content != model.ContentUnknown {
				ns.Provenance.Content = p.content
			} else {
				ns.Provenance.Content = contentKind(ns.Provenance.Source)
			}
		}
		if b.o.Language == "" {
			continue
		}
		applied := false
		for _, d := range p.docs {
			if d.Lang == b.o.Language && d.Matches(ns.ID) {
				ApplyDoc(ns, d)
				applied = true
			}
		}
		if !applied && len(p.docs) > 0 {
			p.diagnostics = append(p.diagnostics, model.Diagnostic{
				Severity: model.SeverityInfo, Code: "no-documentation", Namespace: ns.ID, Source: ns.Provenance.Source,
				Message: fmt.Sprintf("no %q documentation file for this namespace", b.o.Language),
			})
		}
	}
	return p, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
