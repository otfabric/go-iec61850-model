// SPDX-License-Identifier: MIT

package official

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// These tests check a locally generated doc set (make docs-extract):
// embedded with -tags iecdocs, or named by IEC61850_MODEL_DOCS. Public
// CI has none, so they skip; REQUIRE_IEC_DOCS=1 turns the skip into a
// failure (make test-docs). They assert keys and counts only, never a
// text: the texts are IEC content and must not appear in the
// repository.

func realDocs(t *testing.T) *nsd.DocSet {
	t.Helper()
	resetDocs(t)
	_, err := Docs("en")
	set := nsd.SelectDocSet(localSets, "en")
	if err == nil && set == nil {
		err = errors.New("only the public texts")
	}
	if err != nil {
		if os.Getenv("REQUIRE_IEC_DOCS") == "1" {
			t.Fatalf("REQUIRE_IEC_DOCS=1 but no doc set: %v", err)
		}
		t.Skip("no local doc set (build with -tags iecdocs or set IEC61850_MODEL_DOCS)")
	}
	return set
}

func TestRealDocsKeysMatchElements(t *testing.T) {
	set := realDocs(t)
	c, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, ns := range c.Namespaces() {
		for _, k := range nsd.DocKeys(ns) {
			keys[k] = true
		}
	}
	orphans := 0
	for _, k := range set.Keys() {
		if !keys[k] {
			orphans++
			if orphans <= 10 {
				t.Errorf("key %s names no element of an embedded namespace", k)
			}
		}
		if txt, _ := set.Text(k); txt.Title == "" && txt.Description == "" {
			t.Errorf("key %s has no text", k)
		}
	}
	if orphans > 0 {
		t.Errorf("%d orphan keys", orphans)
	}
}

// Coverage floors of the current releases, per kind: a regression of
// the extractor fails here.
func TestRealDocsCoverage(t *testing.T) {
	set := realDocs(t)
	c, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	floors := map[string]map[string]float64{ // namespace → kind → minimum share
		"IEC 61850-7-2:2007B6":   {"FC": 1.0},
		"IEC 61850-7-3:2007B6":   {"CDC": 0.5, "DA": 0.7, "SDO": 0.9, "SP": 0.9},
		"IEC 61850-7-4:2007B6":   {"LN": 0.75, "DO": 0.8, "ABBR": 0.4}, // LN includes the abstract classes, new in Ed2.1,
		"IEC 61850-7-410:2024A4": {"DO": 0.25},
		"IEC 61850-7-420:2019A4": {"LN": 0.2},
	}
	for _, ns := range c.Namespaces() {
		want, ok := floors[ns.ID.String()]
		if !ok {
			continue
		}
		total, have := map[string]int{}, map[string]int{}
		for _, k := range nsd.DocKeys(ns) {
			kind, _, _ := strings.Cut(k, ":")
			total[kind]++
			if _, ok := set.Text(k); ok {
				have[kind]++
			}
		}
		for kind, min := range want {
			share := float64(have[kind]) / float64(total[kind])
			t.Logf("%s %s: %d/%d", ns.ID, kind, have[kind], total[kind])
			if total[kind] == 0 || share < min {
				t.Errorf("%s %s coverage %.2f < %.2f", ns.ID, kind, share, min)
			}
		}
	}
}

// Golden keys: present and non-empty, the text itself is not asserted.
func TestRealDocsGoldenKeys(t *testing.T) {
	set := realDocs(t)
	for key, title := range map[string]bool{
		"LN:MMXU": true, "LN:XCBR": true, "CDC:MV": true, "CDC:SPS": true, "FC:ST": true, "FC:MX": true,
		"DO:MMXU.TotW": false, "DO:MMXU.Hz": false, "DO:XCBR.Pos": false, "SDO:WYE.phsA": false, "DA:MV.mag": false,
		"SP:SPC.ctlVal": false,
	} {
		txt, ok := set.Text(key)
		if !ok || (title && txt.Title == "") || (!title && txt.Description == "") {
			t.Errorf("%s: missing or in the wrong field: %+v", key, txt)
		}
	}
	p, err := DefaultDocumented("en")
	if err != nil {
		t.Fatal(err)
	}
	res, err := resolver.New(p).ResolveString("LD0/MMXU1.TotW.mag.f[MX]")
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps[0].Doc.Title == "" || res.Steps[1].Doc.Description == "" {
		t.Error("documented profile carries no texts")
	}
	plain, _ := Default()
	if r, _ := resolver.New(plain).ResolveString("LD0/MMXU1.TotW"); r.Steps[1].Doc.HasText() {
		t.Error("Default() must stay without texts")
	}
}

func TestRealDocsProfilesRace(t *testing.T) {
	realDocs(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			open := DefaultDocumented
			if i%2 == 1 {
				open = ExtendedDocumented
			}
			p, err := open("en")
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := resolver.New(p).ResolveString("XCBR.Pos.Oper.ctlVal[CO]"); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}

// The public texts are OT Fabric's own wording: no public text of ten
// words or more shares most of its words with the local text of the same
// key. Shorter texts are names and term expansions ("Total active
// power"), which necessarily match.
func TestPublicTextsAreOwnWording(t *testing.T) {
	local := realDocs(t)
	pub, err := PublicDocs()
	if err != nil {
		t.Fatal(err)
	}
	words := func(t nsd.DocText) map[string]bool {
		m := map[string]bool{}
		for _, w := range strings.FieldsFunc(strings.ToLower(t.Title+" "+t.Description), func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9')
		}) {
			m[w] = true
		}
		return m
	}
	checked := 0
	for _, k := range pub.Keys() {
		lt, ok := local.Text(k)
		if !ok {
			continue
		}
		pt, _ := pub.Text(k)
		a, b := words(pt), words(lt)
		if len(a) < 10 || len(b) == 0 {
			continue
		}
		checked++
		inter := 0
		for w := range a {
			if b[w] {
				inter++
			}
		}
		if j := float64(inter) / float64(len(a)+len(b)-inter); j >= 0.8 {
			t.Errorf("%s: public text shares %.0f%% of its words with the local text", k, 100*j)
		}
	}
	t.Logf("%d long public texts compared", checked)
}
