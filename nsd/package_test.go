// SPDX-License-Identifier: MIT

package nsd_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

type entry struct {
	name string
	data []byte
	mode fs.FileMode
}

func buildZIP(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func fixtureEntries(t *testing.T, dir string) []entry {
	es := []entry{
		{name: dir + fixtures.NS72, data: fixtures.Read(t, fixtures.NS72)},
		{name: dir + fixtures.NS73, data: fixtures.Read(t, fixtures.NS73)},
		{name: dir + fixtures.NS74, data: fixtures.Read(t, fixtures.NS74)},
		{name: dir + fixtures.NSDoc74, data: fixtures.Read(t, fixtures.NSDoc74)},
		{name: dir + "IECManifest.xml", data: []byte(`<IECManifest version="1"><File name="x"/></IECManifest>`)},
		{name: dir + "LICENSE.txt", data: []byte("license")},
	}
	if dir != "" {
		es = append(es, entry{name: dir, mode: fs.ModeDir | 0o755})
	}
	return es
}

func TestReadZIP(t *testing.T) {
	data := buildZIP(t, fixtureEntries(t, "pkg/")...)
	p, err := nsd.ReadZIP(bytes.NewReader(data), nsd.WithSource("pkg.zip"))
	if err != nil {
		t.Fatal(err)
	}
	nss := p.Namespaces()
	if len(nss) != 3 || nss[0].ID != fixtures.ID72 || nss[2].ID != fixtures.ID74 {
		t.Fatalf("namespaces %v", p.Namespaces())
	}
	if p.Source() != "pkg.zip" || len(p.SHA256()) != 64 || p.Manifest() == nil || len(p.Docs()) != 1 {
		t.Errorf("package %+v", p)
	}
	ns74 := nss[2]
	if ns74.Provenance.Source != "pkg.zip!pkg/"+fixtures.NS74 || ns74.Provenance.PackageSHA256 != p.SHA256() {
		t.Errorf("provenance %+v", ns74.Provenance)
	}
	if mmxu, _ := ns74.LogicalNodeClass("MMXU"); mmxu.Doc.Title != "Test measurement class" {
		t.Errorf("documentation not applied: %+v", mmxu.Doc)
	}
	if !hasDiagSource(p.Diagnostics(), "not-processed", "LICENSE.txt") || !hasDiagSource(p.Diagnostics(), "no-documentation", fixtures.NS72) {
		t.Errorf("diagnostics %v", p.Diagnostics())
	}

	p, err = nsd.ReadZIP(bytes.NewReader(data), nsd.WithLanguage(""))
	if err != nil {
		t.Fatal(err)
	}
	if mmxu, _ := p.Namespaces()[2].LogicalNodeClass("MMXU"); mmxu.Doc.Title != "" {
		t.Error("documentation applied although disabled")
	}
}

func hasDiagSource(ds model.Diagnostics, code, src string) bool {
	for _, d := range ds {
		if d.Code == code && strings.Contains(d.Source, src) {
			return true
		}
	}
	return false
}

func TestReadZIPNested(t *testing.T) {
	inner := buildZIP(t, fixtureEntries(t, "")...)
	outer := buildZIP(t, entry{name: "inner.zip", data: inner})
	p, err := nsd.ReadZIP(bytes.NewReader(outer), nsd.WithSource("outer.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Namespaces()) != 3 || !strings.HasPrefix(p.Namespaces()[0].Provenance.Source, "outer.zip!inner.zip!") {
		t.Errorf("nested package %v", p.Namespaces()[0].Provenance)
	}
	tooDeep := buildZIP(t, entry{name: "outer.zip", data: outer})
	if _, err := nsd.ReadZIP(bytes.NewReader(tooDeep)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("nesting error = %v", err)
	}
}

func TestReadZIPRejectsUnsafeArchives(t *testing.T) {
	ns := fixtures.Read(t, fixtures.NS72)
	big := bytes.Repeat([]byte(" "), 4<<20)
	bomb := append(append([]byte(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1">`), big...), []byte(`</NS>`)...)
	tests := []struct {
		name     string
		entries  []entry
		opts     []nsd.Option
		sentinel error
	}{
		{"traversal", []entry{{name: "../evil.nsd", data: ns}}, nil, model.ErrUnsafeArchive},
		{"inner traversal", []entry{{name: "a/../../evil.nsd", data: ns}}, nil, model.ErrUnsafeArchive},
		{"absolute", []entry{{name: "/etc/evil.nsd", data: ns}}, nil, model.ErrUnsafeArchive},
		{"backslash", []entry{{name: `a\evil.nsd`, data: ns}}, nil, model.ErrUnsafeArchive},
		{"drive letter", []entry{{name: "C:evil.nsd", data: ns}}, nil, model.ErrUnsafeArchive},
		{"nul", []entry{{name: "a\x00.nsd", data: ns}}, nil, model.ErrUnsafeArchive},
		{"dot", []entry{{name: "./", data: nil}}, nil, model.ErrUnsafeArchive},
		{"duplicate", []entry{{name: "a.nsd", data: ns}, {name: "a.nsd", data: ns}}, nil, model.ErrUnsafeArchive},
		{"case duplicate", []entry{{name: "A.nsd", data: ns}, {name: "a.NSD", data: ns}}, nil, model.ErrUnsafeArchive},
		{"symlink", []entry{{name: "link.nsd", data: []byte("/etc/passwd"), mode: fs.ModeSymlink | 0o777}}, nil, model.ErrUnsafeArchive},
		{"two manifests", []entry{{name: "IECManifest.xml", data: []byte("<m/>")}, {name: "x/IECManifest.xml", data: []byte("<m/>")}}, nil, model.ErrUnsafeArchive},
		{"too many entries", []entry{{name: "a.txt"}, {name: "b.txt"}, {name: "c.txt"}}, []nsd.Option{nsd.WithMaxArchiveEntries(2)}, model.ErrLimitExceeded},
		{"compression ratio", []entry{{name: "bomb.nsd", data: bomb}}, nil, model.ErrLimitExceeded},
		{"ratio option", []entry{{name: "bomb.nsd", data: bomb}}, []nsd.Option{nsd.WithMaxCompressionRatio(1 << 20), nsd.WithMaxDocumentSize(1 << 10)}, model.ErrLimitExceeded},
		{"archive budget", []entry{{name: "a.nsd", data: ns}, {name: "b.nsd", data: ns}}, []nsd.Option{nsd.WithMaxArchiveSize(int64(len(ns)) + 500)}, model.ErrLimitExceeded},
		{"conflicting namespace", []entry{{name: "a.nsd", data: ns}, {name: "b.nsd", data: append(append([]byte(nil), ns...), "<!-- other -->"...)}}, nil, model.ErrNamespaceConflict},
		{"invalid document", []entry{{name: "a.nsd", data: []byte("<NS")}}, nil, model.ErrInvalidNSD},
		{"bad manifest", []entry{{name: "IECManifest.xml", data: []byte("<m>")}}, nil, model.ErrInvalidNSD},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := buildZIP(t, tc.entries...)
			_, err := nsd.ReadZIP(bytes.NewReader(data), tc.opts...)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("error = %v, want %v", err, tc.sentinel)
			}
		})
	}
	if _, err := nsd.ReadZIP(strings.NewReader("not a zip")); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("non-zip error = %v", err)
	}
	if _, err := nsd.ReadZIP(strings.NewReader("xxxx"), nsd.WithMaxArchiveSize(2)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("oversized archive error = %v", err)
	}
}

func TestIdenticalDocumentsAreDeduplicated(t *testing.T) {
	ns := fixtures.Read(t, fixtures.NS72)
	inner := buildZIP(t, entry{name: "x.nsd", data: ns}, entry{name: "Manifest.xml", data: []byte(`<IECManifest><Copyright><Notice> Notice  text </Notice><License uri="www.iec.ch/CCv1" kind="Standard"/></Copyright></IECManifest>`)})
	outer := buildZIP(t, entry{name: "pkg/x.nsd", data: ns}, entry{name: "pkg/inner.zip", data: inner},
		entry{name: "pkg/s.snsd", data: []byte(`<ServiceNS xmlns="http://www.iec.ch/61850/2016/NSD" id="IEC 61850-8-1" version="2003" revision="A" release="4"/>`)},
		entry{name: "pkg/a.AppNS", data: []byte(`<ApplicableServiceNS xmlns="http://www.iec.ch/61850/2016/NSD" version="2"/>`)})
	p, err := nsd.ReadZIP(bytes.NewReader(outer))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Namespaces()) != 1 || !hasDiagSource(p.Diagnostics(), "duplicate-document", "inner.zip") {
		t.Errorf("dedupe: %d namespaces %v", len(p.Namespaces()), p.Diagnostics())
	}
	if s := p.ServiceNamespaces(); len(s) != 1 || s[0].String() != "IEC 61850-8-1:2003A4" {
		t.Errorf("service namespaces %v", s)
	}
	m := p.Manifest()
	if m == nil {
		t.Fatal("Manifest.xml not recognized")
	}
	if uri, kind, ok := m.License(); !ok || uri != "www.iec.ch/CCv1" || kind != "Standard" || m.Notice() != "Notice text" {
		t.Errorf("license %q %q %v, notice %q", uri, kind, ok, m.Notice())
	}
	empty := &nsd.Manifest{}
	if _, _, ok := empty.License(); ok || empty.Notice() != "" {
		t.Error("empty manifest")
	}
}

func TestLoadFileDirZIP(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "pkg.zip")
	if err := os.WriteFile(zipPath, buildZIP(t, fixtureEntries(t, "")...), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := nsd.LoadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Namespaces()) != 3 || p.Source() != zipPath {
		t.Errorf("LoadFile(zip) %v", p.Namespaces())
	}
	p, err = nsd.LoadFile(fixtures.Path(fixtures.NS74))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Namespaces()) != 1 || p.Namespaces()[0].Provenance.PackageSHA256 != "" || p.SHA256() == "" {
		t.Errorf("LoadFile(nsd) %+v", p.Namespaces()[0].Provenance)
	}
	inDir, err := nsd.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	zipDigest := sha256.Sum256(must(os.ReadFile(zipPath)))
	for _, ns := range inDir.Namespaces() {
		if ns.Provenance.PackageSHA256 != hex.EncodeToString(zipDigest[:]) {
			t.Errorf("%s loaded from a directory must record the archive digest: %+v", ns.ID, ns.Provenance)
		}
	}
	p, err = nsd.LoadDir(fixtures.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Namespaces()) != 5 || len(p.Docs()) != 1 {
		t.Errorf("LoadDir: %d namespaces", len(p.Namespaces()))
	}
	if _, err := nsd.LoadFile(filepath.Join(dir, "missing.nsd")); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("missing file error = %v", err)
	}
	if _, err := nsd.LoadZIP(filepath.Join(dir, "missing.zip")); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("missing zip error = %v", err)
	}
	if _, err := nsd.LoadDir(filepath.Join(dir, "missing")); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("missing dir error = %v", err)
	}
	bad := filepath.Join(dir, "bad.nsd")
	if err := os.WriteFile(bad, []byte("<NS"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := nsd.LoadFile(bad); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("bad file error = %v", err)
	}
	if _, err := nsd.LoadDir(dir); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("bad dir error = %v", err)
	}
	if _, err := nsd.LoadFile(fixtures.Path(fixtures.NS74), nsd.WithMaxDocumentSize(10)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("oversized file error = %v", err)
	}
	if _, err := nsd.LoadDir(fixtures.Dir(), nsd.WithMaxDocumentSize(10)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("oversized dir entry error = %v", err)
	}
}

func must(data []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return data
}

func TestValidateNil(t *testing.T) {
	if ds := nsd.Validate(nil); !ds.HasErrors() {
		t.Error("Validate(nil) must report an error")
	}
}
