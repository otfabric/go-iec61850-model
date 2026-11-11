// SPDX-License-Identifier: MIT

package official

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
)

// Profile is a named selection of the embedded namespaces.
type Profile struct {
	// Name is the profile name, e.g. "default" or "extended".
	Name string
	// Description says what the profile contains.
	Description string
	// Parts are the IEC parts of the profile, e.g. "7-420".
	Parts []string
	// Namespaces are the embedded namespace releases of the parts.
	Namespaces []model.NamespaceID
}

// notice is the attribution and the notices the IEC Code Components
// license requires, per embedded namespace.
//
//go:embed NOTICE.md
var notice string

// Notice returns the attribution, license and copyright notice of every
// embedded namespace (NOTICE.md). Software that redistributes the
// catalog must retain it.
func Notice() string { return notice }

var (
	loadOnce sync.Once
	loaded   *catalog.Catalog
	loadErr  error

	profileMu    sync.Mutex
	profileCache = map[string]*catalog.Profile{}
)

// Catalog returns the catalog of every embedded namespace. It is parsed
// once and shared; it is immutable and safe for concurrent use.
func Catalog() (*catalog.Catalog, error) {
	loadOnce.Do(func() { loaded, loadErr = load(catalogJSON) })
	return loaded, loadErr
}

// load parses the embedded catalog: gzip-compressed JSON, or plain JSON
// as generated before v0.7.0.
func load(data []byte) (*catalog.Catalog, error) {
	nss, err := decodeNamespaces(data)
	if err != nil {
		return nil, err
	}
	return newCatalog(nss)
}

// decodeNamespaces parses the namespaces of the embedded catalog. Each
// call returns fresh values that the caller may modify.
func decodeNamespaces(data []byte) ([]*model.Namespace, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%w: embedded catalog: %v", model.ErrInvalidNSD, err)
		}
		if data, err = io.ReadAll(zr); err != nil {
			return nil, fmt.Errorf("%w: embedded catalog: %v", model.ErrInvalidNSD, err)
		}
	}
	var doc struct {
		Namespaces []*model.Namespace `json:"namespaces"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%w: embedded catalog: %v", model.ErrInvalidNSD, err)
	}
	return doc.Namespaces, nil
}

func newCatalog(nss []*model.Namespace) (*catalog.Catalog, error) {
	srcs := make([]catalog.Source, len(nss))
	for i, ns := range nss {
		srcs[i] = ns
	}
	return catalog.New(srcs...)
}

// Namespaces returns the IDs of the embedded namespaces, in order.
func Namespaces() []model.NamespaceID {
	c, err := Catalog()
	if err != nil {
		return nil
	}
	return c.IDs()
}

// Profiles returns the named profiles, in the order of sources/index.json.
func Profiles() []Profile {
	out := make([]Profile, len(profiles))
	for i, p := range profiles {
		out[i] = Profile{Name: p.Name, Description: p.Description,
			Parts: append([]string(nil), p.Parts...), Namespaces: append([]model.NamespaceID(nil), p.Namespaces...)}
	}
	return out
}

// Lookup returns the definition of the named profile.
func Lookup(name string) (Profile, bool) {
	for _, p := range Profiles() {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// Open returns the catalog profile of the named profile, built once and
// shared.
func Open(name string) (*catalog.Profile, error) {
	profileMu.Lock()
	defer profileMu.Unlock()
	if p, ok := profileCache[name]; ok {
		return p, nil
	}
	def, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%w: no embedded profile %q", model.ErrUnknownNamespace, name)
	}
	c, err := Catalog()
	if err != nil {
		return nil, err
	}
	p, err := c.Profile(def.Namespaces...)
	if err != nil {
		return nil, err
	}
	profileCache[name] = p
	return p, nil
}

// Default returns the default profile: IEC 61850-7-2, 7-3, 7-4, 7-410
// and 7-420. IEC 61850-7-420 is the source for current DER and storage
// models.
func Default() (*catalog.Profile, error) { return Open("default") }

// Extended returns the default profile plus the technical reports. Names
// a report defines again (e.g. DBAT in IEC TR 61850-90-9 and IEC
// 61850-7-420) are ambiguous in it; look them up with their namespace.
func Extended() (*catalog.Profile, error) { return Open("extended") }

// ForNamespace returns the embedded profile to check a device against
// that declares namespace s, as LLN0.NamPlt.ldNs or a logical node's
// lnNs does, e.g. "IEC 61850-7-4:2007B" or "IEC 61850-7-4:2007A".
//
// The profile must hold a release of the declared standard with the
// declared version, the declared revision when there is one, and at
// least the declared release; of several, the highest release wins,
// then the first profile in [Profiles] order. A declaration without a
// revision ("IEC 61850-7-4:2007") selects the oldest embedded revision
// of that version, the edition it was first published as. Standards
// are compared without spaces and technical-report markers, so "IEC TR
// 61850-90-9" finds "(Tr)IEC 61850-90-9".
//
// It fails with [model.ErrUnknownNamespace] when no embedded profile
// holds a matching release, e.g. for "IEC 61850-7-4:2003": IEC publishes
// no NSD for Edition 1 of IEC 61850-7-4.
func ForNamespace(s string) (Profile, error) {
	want, err := model.ParseNamespaceID(s)
	if err != nil {
		return Profile{}, err
	}
	if want.Version == "" {
		return Profile{}, fmt.Errorf("%w: %q declares no version", model.ErrUnknownNamespace, s)
	}
	var best Profile
	var bestID model.NamespaceID
	found := false
	for _, p := range Profiles() {
		for _, id := range p.Namespaces {
			if !matchesDeclared(id, want) {
				continue
			}
			if !found || betterRelease(id, bestID, want.Revision == "") {
				best, bestID, found = p, id, true
			}
		}
	}
	if !found {
		return Profile{}, fmt.Errorf("%w: no embedded release of %s", model.ErrUnknownNamespace, want)
	}
	return best, nil
}

func matchesDeclared(id, want model.NamespaceID) bool {
	if normalizeStandard(id.Standard) != normalizeStandard(want.Standard) || id.Version != want.Version {
		return false
	}
	if want.Revision == "" {
		return true
	}
	return strings.EqualFold(id.Revision, want.Revision) && id.Release >= want.Release
}

// betterRelease reports whether a is preferred over b: the oldest
// revision when the declaration names none, then the highest release.
func betterRelease(a, b model.NamespaceID, oldestRevision bool) bool {
	if oldestRevision && a.Revision != b.Revision {
		return a.Revision < b.Revision
	}
	return a.Release > b.Release
}

// normalizeStandard drops spaces, case and technical-report markers:
// "(Tr)IEC 61850-90-9" and "IEC TR 61850-90-9" both become
// "iec61850-90-9".
func normalizeStandard(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, " ", ""))
	s = strings.ReplaceAll(s, "(tr)", "")
	return strings.Replace(s, "iectr", "iec", 1)
}
