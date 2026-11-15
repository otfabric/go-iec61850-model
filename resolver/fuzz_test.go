// SPDX-License-Identifier: MIT

package resolver_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// FuzzLeaves imports a namespace and expands every class of it. An
// expansion returns leaves or an error of a documented category, never
// more than the limit of 100 000 elements allows, and every leaf is a
// path of the class that ends at an attribute.
func FuzzLeaves(f *testing.F) {
	f.Add(fixtures.Read(f, fixtures.NS73))
	f.Add([]byte(nested(2, 2)))
	f.Add([]byte(nested(3, 4)))
	f.Add([]byte(nested(8, 6))) // beyond the limit
	f.Add([]byte(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="F" version="1"><ConstructedAttributes><ConstructedAttribute name="T"><SubDataAttribute name="t" type="T" typeKind="CONSTRUCTED"/></ConstructedAttribute></ConstructedAttributes>` +
		`<CDCs><CDC name="C"><SubDataObject name="s" type="C"/><DataAttribute name="a" fc="ST" type="T" typeKind="CONSTRUCTED"/></CDC></CDCs><LNClasses><LNClass name="L" base="L"><DataObject name="D" type="C"/></LNClass></LNClasses></NS>`))

	sentinels := []error{
		model.ErrLimitExceeded, model.ErrCycle, model.ErrUnknownLogicalNode, model.ErrUnknownDataObject, model.ErrUnknownCDC,
		model.ErrUnknownAttribute, model.ErrUnknownType, model.ErrAmbiguousReference, model.ErrInvalidReference,
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		ns, _, err := nsd.ParseNamespace(bytes.NewReader(data), nsd.WithMaxDocumentSize(64<<10))
		if err != nil {
			return
		}
		c, err := catalog.New(ns)
		if err != nil {
			return
		}
		p, err := c.Profile(ns.ID)
		if err != nil {
			return
		}
		r := resolver.New(p)
		for _, ln := range p.ListLogicalNodes() {
			leaves, err := r.Leaves(ln.Name, nil, "")
			if err != nil {
				if leaves != nil || !slices.ContainsFunc(sentinels, func(s error) bool { return errors.Is(err, s) }) {
					t.Fatalf("%s: %d leaves, error of no documented category: %v", ln.Name, len(leaves), err)
				}
				continue
			}
			if len(leaves) > 100000 {
				t.Fatalf("%s: %d leaves", ln.Name, len(leaves))
			}
			for _, l := range leaves {
				if len(l.Steps) < 3 || len(l.Steps) > resolver.DefaultMaxDepth || l.Steps[0].Name != ln.Name || !l.IsAttribute() || l.LNClass == nil {
					t.Fatalf("%s: leaf %q of %d steps", ln.Name, l.Path(), len(l.Steps))
				}
			}
		}
	})
}
