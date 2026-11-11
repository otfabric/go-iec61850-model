// SPDX-License-Identifier: MIT

package official

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// DocsEnv names the environment variable that points to a local doc set
// file (the JSON form, optionally gzip, or an NSDoc-compatible XML file;
// see nsd.ReadDocSets). Its texts take precedence over the public OT
// Fabric texts, which fill every key it leaves out. When it is set, it is
// used instead of a doc set embedded with the iecdocs build tag. It is
// read once, on first use.
const DocsEnv = "IEC61850_MODEL_DOCS"

// PublicDocsSource is what [DocsSource] reports when only the public OT
// Fabric texts are in use.
const PublicDocsSource = "otfabric"

// ErrNoDocs reports that no documentation texts are available in the
// requested language: the public texts are English, and other languages
// need a local doc set (IEC61850_MODEL_DOCS or -tags iecdocs).
var ErrNoDocs = errors.New("iec61850-model: no documentation texts available in this language (the public texts are English; set " + DocsEnv + " for others)")

var (
	docsOnce   sync.Once
	docSets    []*nsd.DocSet // public texts with the local doc set applied
	localSets  []*nsd.DocSet // the local doc set alone, if any
	publicSets []*nsd.DocSet
	docsSource string
	docsErr    error

	docMu       sync.Mutex
	docCatalogs = map[string]*catalog.Catalog{}
	docProfiles = map[string]*catalog.Profile{}
)

func loadDocs() ([]*nsd.DocSet, error) {
	docsOnce.Do(func() {
		publicSets, docsErr = nsd.ReadDocSets(publicDocs)
		if docsErr != nil {
			return
		}
		docsSource = PublicDocsSource
		switch path := os.Getenv(DocsEnv); {
		case path != "":
			localSets, docsErr = nsd.LoadDocSets(path)
			docsSource = path
		case len(embeddedDocs) > 0:
			localSets, docsErr = nsd.ReadDocSets(embeddedDocs)
			docsSource = "embedded"
		}
		if docsErr != nil {
			docsSource = ""
			return
		}
		docSets = mergeDocSets(publicSets, localSets)
	})
	return docSets, docsErr
}

// mergeDocSets returns one set per language: the texts of over, and the
// texts of base for the fields over leaves empty.
func mergeDocSets(base, over []*nsd.DocSet) []*nsd.DocSet {
	var langs []string
	seen := map[string]bool{}
	for _, s := range append(append([]*nsd.DocSet(nil), base...), over...) {
		if !seen[s.Lang()] {
			seen[s.Lang()] = true
			langs = append(langs, s.Lang())
		}
	}
	out := make([]*nsd.DocSet, 0, len(langs))
	for _, lang := range langs {
		m := nsd.NewDocSet(lang)
		for _, src := range []*nsd.DocSet{nsd.SelectDocSet(base, lang), nsd.SelectDocSet(over, lang)} {
			if src == nil {
				continue
			}
			for _, k := range src.Keys() {
				t, _ := src.Text(k)
				if t.Title != "" {
					_ = m.Set(k+"#title", t.Title) // keys come from a valid set
				}
				if t.Description != "" {
					_ = m.Set(k+"#desc", t.Description)
				}
			}
		}
		out = append(out, m)
	}
	return out
}

// DocsSource names where the documentation texts come from: the path
// of IEC61850_MODEL_DOCS or "embedded" (an iecdocs build) when a local
// doc set is applied on top of the public texts, [PublicDocsSource] when
// only the public texts are in use, or "" when the local doc set cannot
// be read.
func DocsSource() string {
	if _, err := loadDocs(); err != nil {
		return ""
	}
	return docsSource
}

// DocLanguages returns the languages of the available documentation
// texts, sorted: "en" for the public texts, plus those of a local doc
// set.
func DocLanguages() []string {
	sets, err := loadDocs()
	if err != nil {
		return nil
	}
	var out []string
	for _, s := range sets {
		if s.Len() > 0 {
			out = append(out, s.Lang())
		}
	}
	sort.Strings(out)
	return out
}

// PublicDocs returns the public OT Fabric doc set: English titles and
// descriptions for every element of every embedded namespace, written by
// OT Fabric (MIT). Most data object descriptions are composed from the
// expansions of the terms their names are built from. Unlike [Docs] it
// ignores any local doc set.
func PublicDocs() (*nsd.DocSet, error) {
	if _, err := loadDocs(); err != nil && publicSets == nil {
		return nil, err
	}
	return nsd.SelectDocSet(publicSets, "en"), nil
}

// Docs returns the doc set of the language, e.g. "en": the public
// texts with a local doc set applied, if there is one. It fails with
// [ErrNoDocs] for a language without texts, or with the error of reading
// the IEC61850_MODEL_DOCS file. Use the set with nsd.WithFallbackDocs to
// document namespaces loaded from packages.
func Docs(lang string) (*nsd.DocSet, error) {
	sets, err := loadDocs()
	if err != nil {
		return nil, err
	}
	s := nsd.SelectDocSet(sets, lang)
	if s == nil || s.Len() == 0 {
		return nil, fmt.Errorf("%w: no %q texts", ErrNoDocs, lang)
	}
	return s, nil
}

// OpenDocumented is like [Open], with the titles and descriptions of
// the doc set of the language applied: resolutions, descriptors and
// catalog text search then carry texts. The profile is built once per
// name and language and shared; it is separate from the one [Open]
// returns, which stays without texts. It fails with [ErrNoDocs] for a
// language without texts.
func OpenDocumented(name, lang string) (*catalog.Profile, error) {
	set, err := Docs(lang)
	if err != nil {
		return nil, err
	}
	key := name + "\x00" + lang
	docMu.Lock()
	defer docMu.Unlock()
	if p, ok := docProfiles[key]; ok {
		return p, nil
	}
	c, err := cachedDocumentedCatalog(lang, set)
	if err != nil {
		return nil, err
	}
	p, err := profileOf(c, name)
	if err != nil {
		return nil, err
	}
	docProfiles[key] = p
	return p, nil
}

// cachedDocumentedCatalog returns the documented catalog of the
// language, building it once; docMu must be held.
func cachedDocumentedCatalog(lang string, set *nsd.DocSet) (*catalog.Catalog, error) {
	if c, ok := docCatalogs[lang]; ok {
		return c, nil
	}
	c, err := documentedCatalog(set)
	if err != nil {
		return nil, err
	}
	docCatalogs[lang] = c
	return c, nil
}

// DocumentedCatalog is like [Catalog], with the texts of the doc set of
// the language applied to every embedded namespace. It is built once
// per language and shared; it fails with [ErrNoDocs] for a language
// without texts.
func DocumentedCatalog(lang string) (*catalog.Catalog, error) {
	set, err := Docs(lang)
	if err != nil {
		return nil, err
	}
	docMu.Lock()
	defer docMu.Unlock()
	return cachedDocumentedCatalog(lang, set)
}

// CatalogWithDocs returns the catalog of every embedded namespace with
// the texts of set applied. It is built on every call.
func CatalogWithDocs(set *nsd.DocSet) (*catalog.Catalog, error) {
	if set == nil {
		return nil, ErrNoDocs
	}
	return documentedCatalog(set)
}

// DefaultDocumented returns the default profile with texts; see
// [OpenDocumented].
func DefaultDocumented(lang string) (*catalog.Profile, error) {
	return OpenDocumented("default", lang)
}

// ExtendedDocumented returns the extended profile with texts; see
// [OpenDocumented].
func ExtendedDocumented(lang string) (*catalog.Profile, error) {
	return OpenDocumented("extended", lang)
}

// OpenWithDocs returns the named profile with the texts of set applied,
// e.g. a doc set read at run time with nsd.LoadDocSets. Unlike
// [OpenDocumented] the result is built on every call and not cached.
func OpenWithDocs(name string, set *nsd.DocSet) (*catalog.Profile, error) {
	if set == nil {
		return nil, ErrNoDocs
	}
	if _, ok := Lookup(name); !ok {
		return nil, fmt.Errorf("%w: no embedded profile %q", model.ErrUnknownNamespace, name)
	}
	c, err := documentedCatalog(set)
	if err != nil {
		return nil, err
	}
	return profileOf(c, name)
}

// documentedCatalog decodes a fresh copy of the embedded namespaces and
// applies set before the catalog is built, so that catalogs stay
// immutable.
func documentedCatalog(set *nsd.DocSet) (*catalog.Catalog, error) {
	nss, err := decodeNamespaces(catalogJSON)
	if err != nil {
		return nil, err
	}
	for _, ns := range nss {
		nsd.ApplyDocSet(ns, set)
	}
	return newCatalog(nss)
}

func profileOf(c *catalog.Catalog, name string) (*catalog.Profile, error) {
	def, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%w: no embedded profile %q", model.ErrUnknownNamespace, name)
	}
	return c.Profile(def.Namespaces...)
}
