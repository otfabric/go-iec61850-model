// SPDX-License-Identifier: MIT

package ontology_test

import (
	"bytes"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850-model/ontology"
)

// What the graph returns is the caller's: changing it does not change
// the graph, also not the Properties of a concept, which a copy of the
// struct alone would share.
func TestGraphResultsAreOwned(t *testing.T) {
	g := build(t, true)
	var before bytes.Buffer
	if err := g.ExportJSON(&before); err != nil {
		t.Fatal(err)
	}
	const id = "ln:MMXU"
	scribble := func(c ontology.Concept) {
		if len(c.Properties) == 0 {
			t.Fatalf("%s has no property to change", c.ID)
		}
		for i := range c.Properties {
			c.Properties[i] = ontology.Property{Name: "changed", Value: "changed"}
		}
		// Appending within the capacity must not reach the graph either.
		_ = append(c.Properties[:0], ontology.Property{Name: "appended"})
	}

	c, ok := g.Concept(id)
	if !ok {
		t.Fatal("no concept " + id)
	}
	scribble(c)
	for _, c := range g.Concepts() {
		if len(c.Properties) > 0 {
			scribble(c)
		}
	}
	for _, c := range g.FindConcepts(ontology.Query{Name: "MMXU"}) {
		if len(c.Properties) > 0 {
			scribble(c)
		}
	}
	if err := g.Walk(id, func(c ontology.Concept, _ int) bool {
		if len(c.Properties) > 0 {
			scribble(c)
		}
		return true
	}); err != nil {
		t.Fatal(err)
	}
	for _, rs := range [][]ontology.Relation{g.RelationsAll(), g.Relations(id), g.Incoming("cdc:MV")} {
		if len(rs) == 0 {
			t.Fatal("no relation to change")
		}
		for i := range rs {
			rs[i] = ontology.Relation{From: "changed", Predicate: "changed", To: "changed"}
		}
	}
	if cs := g.Concepts(); len(cs) > 0 {
		cs[0] = ontology.Concept{ID: "changed"}
	}

	again, _ := g.Concept(id)
	if v, ok := again.Property("group"); !ok || v != "M Metering and measurement" {
		t.Errorf("%s after a caller changed its copy: properties %v", id, again.Properties)
	}
	var after bytes.Buffer
	if err := g.ExportJSON(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Error("the graph changed through a value it returned")
	}
	data, err := g.MarshalJSON()
	if err != nil || bytes.Contains(data, []byte("changed")) || bytes.Contains(data, []byte("appended")) {
		t.Errorf("MarshalJSON after the changes: %v", err)
	}
}

// Readers that change their own results do not disturb each other: the
// graph is safe for concurrent use (run with the race detector).
func TestGraphConcurrentReadersOwnTheirResults(t *testing.T) {
	g := build(t, false)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				c, _ := g.Concept("ln:MMXU")
				for i := range c.Properties {
					c.Properties[i].Value = "mine"
				}
				for _, c := range g.FindConcepts(ontology.Query{Kinds: []ontology.Kind{ontology.KindLogicalNode}}) {
					for i := range c.Properties {
						c.Properties[i].Value = "mine"
					}
				}
				if again, _ := g.Concept("ln:MMXU"); len(again.Properties) == 0 || again.Properties[0].Value == "mine" {
					t.Errorf("another reader's change is visible: %v", again.Properties)
					return
				}
			}
		}()
	}
	wg.Wait()
}
