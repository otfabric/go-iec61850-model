// SPDX-License-Identifier: MIT

package official

import (
	"errors"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// The doc sets Docs and PublicDocs return are the caller's: a change
// reaches neither another caller's set nor the documented profiles.
func TestDocSetsAreOwned(t *testing.T) {
	resetDocs(t)
	const key, mine = "LN:MMXU", "a text of this caller"
	for name, get := range map[string]func() (*nsd.DocSet, error){
		"Docs":       func() (*nsd.DocSet, error) { return Docs("en") },
		"PublicDocs": PublicDocs,
	} {
		first, err := get()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		original, ok := first.Text(key)
		if !ok || original.Title == "" || original.Title == mine {
			t.Fatalf("%s: no text for %s", name, key)
		}
		n := first.Len()
		if err := first.Set(key+"#title", mine); err != nil {
			t.Fatal(err)
		}
		if err := first.Set("LN:ZZZZ", "a class of this caller"); err != nil {
			t.Fatal(err)
		}
		second, err := get()
		if err != nil {
			t.Fatal(err)
		}
		if second == first {
			t.Errorf("%s returned the same set twice", name)
		}
		if got, _ := second.Text(key); got != original || second.Len() != n {
			t.Errorf("%s after a caller changed its set: %+v and %d keys, want %+v and %d", name, got, second.Len(), original, n)
		}
		if _, ok := second.Text("LN:ZZZZ"); ok {
			t.Errorf("%s: a key another caller added is visible", name)
		}
	}
	// The copy is complete: language, keys and texts.
	pub, err := PublicDocs()
	if err != nil {
		t.Fatal(err)
	}
	set, err := Docs("en")
	if err != nil {
		t.Fatal(err)
	}
	if set.Lang() != "en" || pub.Lang() != "en" || pub.Source() == "" || set.Len() != pub.Len() {
		t.Fatalf("Docs: %q, %d keys; PublicDocs: %q from %q, %d keys", set.Lang(), set.Len(), pub.Lang(), pub.Source(), pub.Len())
	}
	for _, k := range pub.Keys() {
		want, _ := pub.Text(k)
		if got, ok := set.Text(k); !ok || got != want {
			t.Fatalf("%s: %+v, want %+v", k, got, want)
		}
	}

	// The documented profiles are built from the package's own texts.
	changed, err := Docs("en")
	if err != nil {
		t.Fatal(err)
	}
	if err := changed.Set(key+"#title", mine); err != nil {
		t.Fatal(err)
	}
	p, err := OpenDocumented("default", "en")
	if err != nil {
		t.Fatal(err)
	}
	if ln, err := p.LogicalNode("MMXU"); err != nil || ln.Doc.Title == mine || ln.Doc.Title == "" {
		t.Errorf("MMXU of the documented profile: %+v, %v", ln.Doc, err)
	}
	// With the caller's set, the profile carries the caller's text.
	own, err := OpenWithDocs("default", changed)
	if err != nil {
		t.Fatal(err)
	}
	if ln, err := own.LogicalNode("MMXU"); err != nil || ln.Doc.Title != mine {
		t.Errorf("MMXU with the caller's set: %+v, %v", ln.Doc, err)
	}
}

// Callers change their own sets while others build profiles from
// theirs and from the package's texts (run with the race detector).
func TestDocSetsConcurrentOwners(t *testing.T) {
	resetDocs(t)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			set, err := Docs("en")
			if err != nil {
				t.Error(err)
				return
			}
			for _, k := range set.Keys()[:200] {
				if err := set.Set(k+"#title", "changed"); err != nil {
					t.Error(err)
					return
				}
			}
			var p interface {
				LogicalNode(string) (*model.LogicalNodeClass, error)
			}
			if i%2 == 0 {
				p, err = OpenWithDocs("default", set)
			} else {
				p, err = OpenDocumented("default", "en")
			}
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := p.LogicalNode("MMXU"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	p, err := OpenDocumented("default", "en")
	if err != nil {
		t.Fatal(err)
	}
	if res, err := resolver.New(p).ResolveString("MMXU.TotW"); err != nil || res.Description() == "" || res.Description() == "changed" {
		t.Errorf("MMXU.TotW of the documented profile: %v", err)
	}
}

// PublicDocs reports the error of the embedded texts.
func TestPublicDocsDecodeError(t *testing.T) {
	saved := publicDocs
	publicDocs = []byte("{")
	defer func() { publicDocs = saved }()
	if s, err := PublicDocs(); s != nil || !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("PublicDocs with unreadable texts: %v, %v", s, err)
	}
}
