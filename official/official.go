// SPDX-License-Identifier: MIT

package official

import (
	_ "embed"
	"encoding/json"
	"fmt"
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

func load(data []byte) (*catalog.Catalog, error) {
	var doc struct {
		Namespaces []*model.Namespace `json:"namespaces"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%w: embedded catalog: %v", model.ErrInvalidNSD, err)
	}
	srcs := make([]catalog.Source, len(doc.Namespaces))
	for i, ns := range doc.Namespaces {
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
