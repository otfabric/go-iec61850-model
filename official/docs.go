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

// DocsEnv names the environment variable that points to a doc set file
// (the JSON form, optionally gzip, or an NSDoc-compatible XML file; see
// nsd.ReadDocSets). When it is set, the file is used instead of any
// embedded doc set, so texts are available without a rebuild. It is
// read once, on first use.
const DocsEnv = "IEC61850_MODEL_DOCS"

// ErrNoDocs reports that no documentation texts are available: the
// binary was built without the iecdocs tag and IEC61850_MODEL_DOCS is
// not set, or the doc set has no texts in the requested language.
var ErrNoDocs = errors.New("iec61850-model: no documentation texts available (build with -tags iecdocs or set " + DocsEnv + ")")

var (
	docsOnce   sync.Once
	docSets    []*nsd.DocSet
	docsSource string
	docsErr    error

	docMu       sync.Mutex
	docCatalogs = map[string]*catalog.Catalog{}
	docProfiles = map[string]*catalog.Profile{}
)

func loadDocs() ([]*nsd.DocSet, error) {
	docsOnce.Do(func() {
		if path := os.Getenv(DocsEnv); path != "" {
			docSets, docsErr = nsd.LoadDocSets(path)
			docsSource = path
			return
		}
		if len(embeddedDocs) == 0 {
			return
		}
		docSets, docsErr = nsd.ReadDocSets(embeddedDocs)
		docsSource = "embedded"
	})
	return docSets, docsErr
}

// DocsSource names where the documentation texts come from: the path
// of IEC61850_MODEL_DOCS, "embedded" for an iecdocs build, or "" when
// there are none.
func DocsSource() string {
	if _, err := loadDocs(); err != nil {
		return ""
	}
	return docsSource
}

// DocLanguages returns the languages of the available documentation
// texts, sorted; none without the iecdocs tag or IEC61850_MODEL_DOCS.
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

// Docs returns the doc set of the language, e.g. "en". It fails with
// [ErrNoDocs] when there is none, or with the error of reading the
// IEC61850_MODEL_DOCS file. Use the set with nsd.WithFallbackDocs to
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
// returns, which stays without texts. It fails with [ErrNoDocs] when no
// texts are available.
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
	c, ok := docCatalogs[lang]
	if !ok {
		if c, err = documentedCatalog(set); err != nil {
			return nil, err
		}
		docCatalogs[lang] = c
	}
	p, err := profileOf(c, name)
	if err != nil {
		return nil, err
	}
	docProfiles[key] = p
	return p, nil
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
