// SPDX-License-Identifier: MIT

package nsd_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

func readManifest(data []byte) error {
	_, err := nsd.ReadManifest(bytes.NewReader(data))
	return err
}

// A manifest holds at most 10 000 elements below its root.
func TestManifestElementLimit(t *testing.T) {
	build := func(n int) []byte {
		return []byte(`<IECManifest>` + strings.Repeat(`<File/>`, n) + `</IECManifest>`)
	}
	for _, tc := range []struct {
		n  int
		ok bool
	}{{0, true}, {1, true}, {9999, true}, {10000, true}, {10001, false}, {200000, false}} {
		m, err := nsd.ReadManifest(bytes.NewReader(build(tc.n)), nsd.WithSource("Manifest.xml"))
		switch {
		case tc.ok && (err != nil || len(m.Elements) != tc.n):
			t.Errorf("%d elements: error = %v", tc.n, err)
		case !tc.ok && (!errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "Manifest.xml: manifest has more than 10000 elements")):
			t.Errorf("%d elements: error = %v, want ErrLimitExceeded", tc.n, err)
		}
	}
	// The same limit applies to a manifest inside an archive.
	data := buildZIP(t, entry{name: "IECManifest.xml", data: build(10001)})
	if _, err := nsd.ReadZIP(bytes.NewReader(data)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("in an archive: error = %v, want ErrLimitExceeded", err)
	}
	checkLinear(t, 20000, 120, build, readManifest, model.ErrLimitExceeded)
}

// The element paths of a manifest take at most 1 MiB together: a path
// repeats the names of the elements around it.
func TestManifestPathLimit(t *testing.T) {
	// One element with a name of n bytes and c children. The element
	// counts n+1 bytes and each child n+2: a path with the separator
	// before it.
	build := func(n, c int) []byte {
		name := strings.Repeat("N", n)
		return []byte(`<IECManifest><` + name + `>` + strings.Repeat(`<f/>`, c) + `</` + name + `></IECManifest>`)
	}
	for _, tc := range []struct {
		n, c int
		ok   bool
	}{
		{1000, 0, true},
		{1000, 1045, true},   // 1 048 091 bytes
		{1000, 1046, false},  // 1 049 093 bytes
		{1<<20 - 2, 0, true}, // one byte below the limit
		{1<<20 - 1, 0, true}, // the limit
		{1 << 20, 0, false},  // one byte more
		{524286, 1, true},    // 2n+3 bytes: one byte below the limit
		{524287, 1, false},   // one byte above it
		{40000, 2000, false}, // 80 MiB of paths in a document of 88 KiB
		{100, 9999, true},    // short names: 10 000 elements, within both limits
		{100, 10000, false},  // one element more than the element limit
	} {
		m, err := nsd.ReadManifest(bytes.NewReader(build(tc.n, tc.c)))
		switch {
		case tc.ok && (err != nil || len(m.Elements) != tc.c+1 || len(m.Elements[0].Path) != tc.n):
			t.Errorf("name of %d bytes, %d children: error = %v", tc.n, tc.c, err)
		case !tc.ok && !errors.Is(err, model.ErrLimitExceeded):
			t.Errorf("name of %d bytes, %d children: error = %v, want ErrLimitExceeded", tc.n, tc.c, err)
		}
	}
	// What a refused manifest cost: the document, not its paths.
	checkLinear(t, 1000, 96, func(n int) []byte { return build(40*n, n) }, readManifest, model.ErrLimitExceeded)

	// Nested elements: every level repeats the levels above it.
	nested := func(depth int) []byte {
		name := strings.Repeat("N", 1000)
		return []byte(`<IECManifest>` + strings.Repeat(`<`+name+`>`, depth) + strings.Repeat(`</`+name+`>`, depth) + `</IECManifest>`)
	}
	// Level k counts 1001k-1 bytes (1001 at the first): 1 035 991 for 45
	// levels, 1 082 036 for 46.
	if _, err := nsd.ReadManifest(bytes.NewReader(nested(45))); err != nil {
		t.Errorf("45 levels: %v", err)
	}
	if _, err := nsd.ReadManifest(bytes.NewReader(nested(46))); !errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "element paths exceed 1048576 bytes") {
		t.Errorf("46 levels: error = %v, want ErrLimitExceeded", err)
	}
}

// The text of a manifest element is collected once, whatever the number
// of chunks it comes in, and reads as before.
func TestManifestText(t *testing.T) {
	for doc, want := range map[string]string{
		`<m><e>  plain  </e></m>`:                            "plain",
		`<m><e> a <!-- c --> b </e></m>`:                     "a b",
		`<m><e>a <!-- c -->b</e></m>`:                        "ab",
		`<m><e>a<!-- c --> b</e></m>`:                        "a b",
		`<m><e> <!-- c --> a</e></m>`:                        "a",
		`<m><e>a <f>inner</f> b <f/> c </e></m>`:             "a b c",
		`<m><e>a<![CDATA[ b ]]>c</e></m>`:                    "a bc",
		`<m><e><!-- c --></e></m>`:                           "",
		"<m><e>\u00a0a\u2003<!-- c -->\u00a0b\u00a0</e></m>": "a\u00a0b",
	} {
		m, err := nsd.ReadManifest(strings.NewReader(doc))
		if err != nil || len(m.Elements) == 0 || m.Elements[0].Text != want {
			t.Errorf("%s: text %q, error %v, want %q", doc, m.Elements[0].Text, err, want)
		}
	}
	chunks := func(n int) []byte {
		return []byte(`<m><e>` + strings.Repeat(`x<!---->`, n) + `</e></m>`)
	}
	m, err := nsd.ReadManifest(bytes.NewReader(chunks(5000)))
	if err != nil || m.Elements[0].Text != strings.Repeat("x", 5000) {
		t.Fatalf("5000 chunks: error = %v", err)
	}
	checkLinear(t, 20000, 96, chunks, readManifest, nil)
}

// A manifest at the limits of a document: the limits of Parse apply.
func TestManifestDocumentLimits(t *testing.T) {
	doc := `<IECManifest a="1"><File name="x"><Sub/></File></IECManifest>` // depth 3, 6 tokens and 2 attributes
	for _, tc := range []struct {
		name string
		opt  nsd.Option
		ok   bool
	}{
		{"size-1", nsd.WithMaxDocumentSize(int64(len(doc)) - 1), false},
		{"size", nsd.WithMaxDocumentSize(int64(len(doc))), true},
		{"depth-1", nsd.WithMaxDepth(2), false},
		{"depth", nsd.WithMaxDepth(3), true},
		{"tokens-1", nsd.WithMaxTokens(7), false},
		{"tokens", nsd.WithMaxTokens(8), true},
	} {
		_, err := nsd.ReadManifest(strings.NewReader(doc), tc.opt)
		if tc.ok && err != nil || !tc.ok && !errors.Is(err, model.ErrLimitExceeded) {
			t.Errorf("%s: error = %v", tc.name, err)
		}
	}
	if _, err := nsd.ReadManifest(strings.NewReader(doc), nsd.WithMaxDepth(0)); !errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "option WithMaxDepth") {
		t.Errorf("depth 0: error = %v", err)
	}
	if _, err := nsd.ParseSchema(strings.NewReader(doc), nsd.WithMaxTokens(-1)); !errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "option WithMaxTokens") {
		t.Errorf("schema, tokens -1: error = %v", err)
	}
}
