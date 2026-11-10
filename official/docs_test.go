// SPDX-License-Identifier: MIT

package official

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// resetDocs forgets the loaded doc sets and documented profiles.
func resetDocs(t *testing.T) {
	t.Helper()
	docMu.Lock()
	defer docMu.Unlock()
	docsOnce = sync.Once{}
	docSets, docsSource, docsErr = nil, "", nil
	docCatalogs = map[string]*catalog.Catalog{}
	docProfiles = map[string]*catalog.Profile{}
	t.Cleanup(func() {
		docsOnce = sync.Once{}
		docSets, docsSource, docsErr = nil, "", nil
		docCatalogs = map[string]*catalog.Catalog{}
		docProfiles = map[string]*catalog.Profile{}
	})
}

// fixtureDocs writes a doc set in OTFabric test wording and points
// IEC61850_MODEL_DOCS at it.
func fixtureDocs(t *testing.T) string {
	t.Helper()
	s := nsd.NewDocSet("en")
	for k, v := range map[string]string{
		"LN:MMXU":         "OTFabric test title for MMXU",
		"DO:MMXU.TotW":    "OTFabric test text for TotW",
		"DO:DomainLN.Beh": "OTFabric test text for Beh",
		"DA:MV.mag":       "OTFabric test text for mag",
		"CDC:MV":          "OTFabric test title for MV",
	} {
		if err := s.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	data, err := nsd.EncodeDocSets(s)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "docs.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(DocsEnv, path)
	return path
}

func TestNoDocs(t *testing.T) {
	if len(embeddedDocs) > 0 {
		t.Skip("built with the iecdocs tag")
	}
	resetDocs(t)
	t.Setenv(DocsEnv, "")
	if DocLanguages() != nil || DocsSource() != "" {
		t.Errorf("languages %v source %q", DocLanguages(), DocsSource())
	}
	if _, err := Docs("en"); !errors.Is(err, ErrNoDocs) {
		t.Errorf("Docs: %v", err)
	}
	if _, err := DefaultDocumented("en"); !errors.Is(err, ErrNoDocs) {
		t.Errorf("DefaultDocumented: %v", err)
	}
	if _, err := DocumentedCatalog("en"); !errors.Is(err, ErrNoDocs) {
		t.Errorf("DocumentedCatalog: %v", err)
	}
	if _, err := CatalogWithDocs(nil); !errors.Is(err, ErrNoDocs) {
		t.Errorf("CatalogWithDocs(nil): %v", err)
	}
	if _, err := OpenWithDocs("default", nil); !errors.Is(err, ErrNoDocs) {
		t.Errorf("OpenWithDocs(nil): %v", err)
	}
}

func TestDocumentedProfilesFromEnv(t *testing.T) {
	resetDocs(t)
	path := fixtureDocs(t)
	if l := DocLanguages(); len(l) != 1 || l[0] != "en" || DocsSource() != path {
		t.Fatalf("languages %v source %q", l, DocsSource())
	}
	if _, err := Docs("de"); !errors.Is(err, ErrNoDocs) {
		t.Errorf("Docs(de): %v", err)
	}
	doc, err := DefaultDocumented("en")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if doc == plain {
		t.Fatal("documented and plain profiles must be distinct")
	}
	res, err := resolver.New(doc).ResolveString("LD0/MMXU1.TotW.mag.f[MX]")
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps[0].Doc.Title != "OTFabric test title for MMXU" || res.Steps[1].Doc.Description != "OTFabric test text for TotW" ||
		res.Steps[2].Doc.Description != "OTFabric test text for mag" {
		t.Errorf("texts: %+v", res.Steps[:3])
	}
	res, err = resolver.New(plain).ResolveString("LD0/MMXU1.TotW.mag.f[MX]")
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps[1].Doc.HasText() {
		t.Error("Default() must stay without texts")
	}
	if beh, err := resolver.New(doc).ResolveString("XCBR.Beh"); err != nil || beh.Description() != "OTFabric test text for Beh" {
		t.Errorf("inherited data object: %v %v", beh, err)
	}
	if again, _ := DefaultDocumented("en"); again != doc {
		t.Error("documented profiles are cached")
	}
	ext, err := ExtendedDocumented("en")
	if err != nil || ext == doc {
		t.Fatalf("extended: %v", err)
	}
	if r := doc.Search(catalog.Query{Text: "OTFabric test title", Kinds: []catalog.Kind{catalog.KindLogicalNode}}); len(r) == 0 || r[0].Path != "MMXU" { // the 7-420 extension of MMXU shares the key
		t.Errorf("text search: %v", r)
	}
	if _, err := OpenDocumented("nope", "en"); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("unknown profile: %v", err)
	}
	set, _ := Docs("en")
	dc, err := DocumentedCatalog("en")
	if err != nil || len(dc.IDs()) != len(Namespaces()) {
		t.Fatalf("DocumentedCatalog: %v", err)
	}
	if again, _ := DocumentedCatalog("en"); again != dc {
		t.Error("documented catalogs are cached")
	}
	if c, err := CatalogWithDocs(set); err != nil || c == dc {
		t.Errorf("CatalogWithDocs: %v", err)
	}
	if _, err := DocumentedCatalog("de"); !errors.Is(err, ErrNoDocs) {
		t.Errorf("DocumentedCatalog(de): %v", err)
	}
	if p, err := OpenWithDocs("edition2", set); err != nil || p == nil {
		t.Errorf("OpenWithDocs: %v", err)
	}
	if _, err := OpenWithDocs("nope", set); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("OpenWithDocs unknown: %v", err)
	}
}

// Documented profiles are built once under concurrent first use.
func TestDocumentedProfilesRace(t *testing.T) {
	resetDocs(t)
	fixtureDocs(t)
	var wg sync.WaitGroup
	got := make([]*catalog.Profile, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "default"
			if i%2 == 1 {
				name = "extended"
			}
			p, err := OpenDocumented(name, "en")
			if err != nil {
				t.Error(err)
				return
			}
			_, _ = resolver.New(p).ResolveString("MMXU.TotW")
			got[i] = p
		}(i)
	}
	wg.Wait()
	for i := 2; i < len(got); i++ {
		if got[i] != got[i%2] {
			t.Errorf("profile %d is a second instance", i)
		}
	}
}

func TestDocsEnvErrors(t *testing.T) {
	resetDocs(t)
	t.Setenv(DocsEnv, filepath.Join(t.TempDir(), "missing.json"))
	if _, err := Docs("en"); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("missing file: %v", err)
	}
	if DocLanguages() != nil || DocsSource() != "" {
		t.Error("no languages from a failed load")
	}
	if _, err := decodeNamespaces([]byte("{")); err == nil {
		t.Error("decode")
	}
	saved := catalogJSON
	catalogJSON = []byte("{")
	defer func() { catalogJSON = saved }()
	if _, err := OpenWithDocs("default", nsd.NewDocSet("en")); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("broken catalog: %v", err)
	}
}
