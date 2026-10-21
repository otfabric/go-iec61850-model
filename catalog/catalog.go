// SPDX-License-Identifier: MIT

package catalog

import (
	"fmt"
	"sort"

	"github.com/otfabric/go-iec61850-model/model"
)

// Source provides namespaces. *nsd.Package, *model.Namespace, *Catalog
// and *Profile implement it.
type Source interface {
	Namespaces() []*model.Namespace
}

// Catalog is an immutable index of namespaces. It may hold several
// releases of the same namespace; no release is preferred over another.
//
// The lookups of a Catalog (LogicalNode, CDC, ...) search every loaded
// namespace and fail with a [*model.AmbiguityError] when a name is
// defined more than once, for instance by two releases of IEC
// 61850-7-4. Build a [Profile] to select a consistent set of releases.
//
// A Catalog is safe for concurrent use. The namespaces it holds, and
// every definition returned by its lookups, are shared and must not be
// modified.
type Catalog struct {
	view
	byID map[model.NamespaceID]*model.Namespace
}

// New builds a catalog from the namespaces of the given sources. Two
// namespaces with the same ID are rejected with
// [model.ErrNamespaceConflict]. Namespaces are not copied: do not
// modify them after New returns.
func New(sources ...Source) (*Catalog, error) {
	byID := map[model.NamespaceID]*model.Namespace{}
	var nss []*model.Namespace
	for i, src := range sources {
		if src == nil {
			return nil, fmt.Errorf("%w: source %d is nil", model.ErrSourceUnavailable, i)
		}
		for _, ns := range src.Namespaces() {
			if ns == nil {
				return nil, fmt.Errorf("%w: source %d contains a nil namespace", model.ErrSourceUnavailable, i)
			}
			if ns.ID.Standard == "" {
				return nil, fmt.Errorf("%w: namespace without ID (source %q)", model.ErrInvalidNSD, ns.Provenance.Source)
			}
			if prev, dup := byID[ns.ID]; dup {
				if prev == ns {
					continue
				}
				return nil, fmt.Errorf("%w: %s is loaded twice (%s and %s)", model.ErrNamespaceConflict, ns.ID,
					sourceOf(prev), sourceOf(ns))
			}
			byID[ns.ID] = ns
			nss = append(nss, ns)
		}
	}
	sort.Slice(nss, func(i, j int) bool { return nss[i].ID.Compare(nss[j].ID) < 0 })
	return &Catalog{view: newView(nss), byID: byID}, nil
}

func sourceOf(ns *model.Namespace) string {
	if ns.Provenance.Source == "" {
		return "<unknown source>"
	}
	return ns.Provenance.Source
}

// Namespace returns the namespace with exactly the given ID.
func (c *Catalog) Namespace(id model.NamespaceID) (*model.Namespace, error) {
	ns, ok := c.byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrUnknownNamespace, id)
	}
	return ns, nil
}

// Releases returns the IDs of the loaded releases of a standard, e.g.
// every loaded release of "IEC 61850-7-4", in order.
func (c *Catalog) Releases(standard string) []model.NamespaceID {
	var out []model.NamespaceID
	for _, ns := range c.namespaces {
		if ns.ID.Standard == standard {
			out = append(out, ns.ID)
		}
	}
	return out
}

// Standards returns the distinct standards of the loaded namespaces,
// in order.
func (c *Catalog) Standards() []string {
	var out []string
	for _, ns := range c.namespaces {
		if len(out) == 0 || out[len(out)-1] != ns.ID.Standard {
			out = append(out, ns.ID.Standard)
		}
	}
	return out
}

// match finds the loaded namespace a possibly partial ID designates.
func (c *Catalog) match(id model.NamespaceID) (*model.Namespace, error) {
	resolved, err := c.ResolveNamespace(id)
	if err != nil {
		return nil, err
	}
	return c.byID[resolved], nil
}

// DefaultProfile returns a profile of every loaded namespace when the
// catalog holds exactly one release of each standard. Otherwise it
// fails with a [*model.AmbiguityError]: no release is preferred
// implicitly, so select releases with [Catalog.Profile].
func (c *Catalog) DefaultProfile() (*Profile, error) {
	seen := map[string]model.NamespaceID{}
	for _, ns := range c.namespaces {
		if prev, ok := seen[ns.ID.Standard]; ok {
			return nil, &model.AmbiguityError{Kind: "namespace", Name: ns.ID.Standard, Candidates: c.Releases(prev.Standard)}
		}
		seen[ns.ID.Standard] = ns.ID
	}
	return c.Profile(c.IDs()...)
}
