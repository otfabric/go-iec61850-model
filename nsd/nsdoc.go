// SPDX-License-Identifier: MIT

package nsd

import (
	"sort"

	"github.com/otfabric/go-iec61850-model/model"
)

// NSDoc is a parsed NSDoc documentation file: the texts referenced by
// the titleID, descID and presCondArgsID attributes of one namespace,
// in one language.
//
// The texts are IEC content. Use them at run time; do not redistribute
// them (see docs/licensing.md).
type NSDoc struct {
	// ID identifies the namespace the texts belong to.
	ID model.NamespaceID
	// Lang is the language code, e.g. "en".
	Lang  string
	texts map[string]string
}

// Text returns the text with the given documentation ID.
func (d *NSDoc) Text(id string) (string, bool) {
	t, ok := d.texts[id]
	return t, ok
}

// Len returns the number of texts.
func (d *NSDoc) Len() int { return len(d.texts) }

// IDs returns the documentation IDs in sorted order.
func (d *NSDoc) IDs() []string {
	ids := make([]string, 0, len(d.texts))
	for id := range d.texts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Matches reports whether the documentation belongs to namespace id:
// same standard, version and revision. The release is not compared,
// because documentation files are commonly shared across releases.
func (d *NSDoc) Matches(id model.NamespaceID) bool { return d.ID.SameVersion(id) }

// ApplyDoc fills the Title and Description fields of every definition
// in ns from doc, and the ArgsText of every presence condition whose
// presCondArgsID or dsPresCondArgsID doc names, and returns the number
// of texts applied. It modifies ns, so call it before the namespace is
// added to a catalog.
func ApplyDoc(ns *model.Namespace, doc *NSDoc) int {
	if ns == nil || doc == nil {
		return 0
	}
	n := 0
	for _, d := range ns.Docs() {
		if t, ok := doc.texts[d.TitleID]; ok && d.TitleID != "" {
			d.Title = t
			n++
		}
		if t, ok := doc.texts[d.DescID]; ok && d.DescID != "" {
			d.Description = t
			n++
		}
	}
	for _, p := range ns.Presences() {
		if t, ok := doc.texts[p.ArgsID]; ok && p.ArgsID != "" {
			p.ArgsText = t
			n++
		}
	}
	return n
}

// docIDs returns the number of distinct documentation IDs that ns
// references: titleID, descID and presence argument IDs.
func docIDs(ns *model.Namespace) int {
	seen := map[string]bool{}
	for _, d := range ns.Docs() {
		seen[d.TitleID], seen[d.DescID] = true, true
	}
	for _, p := range ns.Presences() {
		seen[p.ArgsID] = true
	}
	delete(seen, "")
	return len(seen)
}
