// SPDX-License-Identifier: MIT

package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
)

// names are the model strings of the namespace built by named: every
// string that the Go generator writes into a comment or a header line.
type names struct {
	standard, version, revision      string
	class, base, dataObject, cdcType string
	presence, presenceArgs           string
	enum, literal                    string
	digest, license, toolVersion     string
}

func plainNames() names {
	return names{
		standard: "OTF 61850-99-1", version: "2026", revision: "A",
		class: "XTST", base: "XBase", dataObject: "Val", cdcType: "XMV",
		presence: "Omulti", presenceArgs: "1, 2",
		enum: "XKind", literal: "on",
		digest: "00ff", license: "example.org/license", toolVersion: "v0.0.0-test",
	}
}

// fields returns the fields of n by name, for tests that change one at
// a time.
func (n *names) fields() map[string]*string {
	return map[string]*string{
		"namespace id": &n.standard, "namespace version": &n.version, "namespace revision": &n.revision,
		"class name": &n.class, "base class": &n.base, "data object name": &n.dataObject, "data object type": &n.cdcType,
		"presence condition": &n.presence, "presence arguments": &n.presenceArgs,
		"enumeration name": &n.enum, "literal name": &n.literal,
		"document digest": &n.digest, "license URI": &n.license, "tool version": &n.toolVersion,
	}
}

// named builds the input of a namespace with one class (which names a
// base class that is not defined, so that the header carries a note),
// one data object and one enumeration, from the given strings.
func named(t testing.TB, n names) Input {
	t.Helper()
	id := model.NamespaceID{Standard: n.standard, Version: n.version, Revision: n.revision}
	ns := &model.Namespace{
		ID:         id,
		Provenance: model.Provenance{SHA256: n.digest},
		Copyright:  model.Copyright{Notice: "Synthetic notice.", LicenseURI: n.license, LicenseKind: "Test"},
		LNClasses: []model.LogicalNodeClass{{Name: n.class, Base: n.base, Source: id, DataObjects: []model.DataObject{{
			Name: n.dataObject, Type: n.cdcType, Presence: model.PresenceCondition{Name: n.presence, Args: n.presenceArgs},
		}}}},
		Enumerations: []model.Enumeration{{Name: n.enum, Source: id, Literals: []model.Literal{{Name: n.literal, Value: 1}}}},
	}
	c, err := catalog.New(ns)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Profile(id)
	if err != nil {
		t.Fatal(err)
	}
	return Input{Profile: p, ToolVersion: n.toolVersion}
}

// declarations lists the top-level declarations of a Go file: the kind
// of each, with the number of its specifications or the name of the
// function.
func declarations(t testing.TB, name string, src []byte) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("%s does not parse: %v\n%s", name, err, src)
	}
	var out []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			out = append(out, fmt.Sprintf("%s(%d)", d.Tok, len(d.Specs)))
		case *ast.FuncDecl:
			recv := ""
			if d.Recv != nil {
				recv = "method "
			}
			out = append(out, "func "+recv+d.Name.Name)
		}
	}
	return out
}

// The declarations of the files generated from named: the listings of
// classes and CDCs, the data objects, the enumeration with its String
// method, and the test.
var (
	wantDeclarations = []string{
		"import(1)", "type(1)", "const(1)", "var(1)", // strconv, LNClass and its listing
		"type(1)", "const(0)", "var(1)", // CDC and its listing: the namespace defines none
		"type(1)", "const(1)", // DataObjectRef and the data object of the class
		"type(1)", "const(1)", "func method String", // the enumeration
	}
	wantTestDeclarations = []string{"import(1)", "func TestGeneratedNames"}
)

// A model string is written into the source as an identifier, as a
// string constant or as a comment. In a comment it stays on its line,
// whatever characters it holds: the declarations of the file are the
// same as for the string with spaces in their place.
func TestModelStringsStayInTheirComments(t *testing.T) {
	clean, err := Go(named(t, plainNames()), "names")
	if err != nil {
		t.Fatal(err)
	}
	if got := declarations(t, GoFileName, clean[GoFileName]); strings.Join(got, " ") != strings.Join(wantDeclarations, " ") {
		t.Fatalf("declarations %v, want %v", got, wantDeclarations)
	}
	if got := declarations(t, GoTestFileName, clean[GoTestFileName]); strings.Join(got, " ") != strings.Join(wantTestDeclarations, " ") {
		t.Fatalf("test declarations %v, want %v", got, wantTestDeclarations)
	}
	typeCheck(t, clean)

	// Text that reads as a declaration when it starts a line, and leaves
	// the rest of its line a comment.
	const text = "var Added = 1 //"
	separators := map[string]string{
		"line feed": "\n", "carriage return and line feed": "\r\n", "carriage return": "\r",
		"vertical tab": "\v", "form feed": "\f", "next line": "\u0085",
		"line separator": "\u2028", "paragraph separator": "\u2029",
		"file separator": "\x1c", "NUL": "\x00", "escape": "\x1b", "delete": "\x7f",
		"invalid UTF-8": "\xff\xfe", "byte order mark": "\ufeff",
		"comment end": "*/", "comment start": "/*", "quote": "\"", "backquote": "`",
	}
	for field := range (&names{}).fields() {
		for what, sep := range separators {
			n := plainNames()
			f := n.fields()[field]
			*f += sep + text
			files, err := Go(named(t, n), "names")
			if err != nil {
				t.Errorf("%s with a %s: %v", field, what, err)
				continue
			}
			if got := declarations(t, GoFileName, files[GoFileName]); strings.Join(got, " ") != strings.Join(wantDeclarations, " ") {
				t.Errorf("%s with a %s: declarations %v, want %v", field, what, got, wantDeclarations)
			}
			if got := declarations(t, GoTestFileName, files[GoTestFileName]); strings.Join(got, " ") != strings.Join(wantTestDeclarations, " ") {
				t.Errorf("%s with a %s: test declarations %v, want %v", field, what, got, wantTestDeclarations)
			}
			for name, src := range map[string][]byte{GoFileName: files[GoFileName], "NOTICE.md": files["NOTICE.md"], "LOCAL-ONLY.md": files["LOCAL-ONLY.md"]} {
				// The header and the notices keep one line per item.
				if lines, cleanLines := strings.Count(string(src), "\n"), strings.Count(string(clean[name]), "\n"); lines != cleanLines && name != GoFileName {
					t.Errorf("%s with a %s: %s has %d lines, want %d", field, what, name, lines, cleanLines)
				}
			}
		}
	}

	// The catalog generator writes the same header into its Go file.
	n := plainNames()
	n.standard += "\n" + text
	n.toolVersion += "\n" + text
	files, err := Catalog(named(t, n), "official", []ProfileSpec{{Name: "p\n" + text, Description: "d\n" + text}})
	if err != nil {
		t.Fatal(err)
	}
	if got := declarations(t, CatalogGoFileName, files[CatalogGoFileName]); strings.Join(got, " ") != "import(2) var(1) var(1)" {
		t.Errorf("catalog declarations %v", got)
	}
}

func TestComment(t *testing.T) {
	for in, want := range map[string]string{
		"":                 "",
		"MMXU":             "MMXU",
		"IEC 61850-7-4":    "IEC 61850-7-4",
		"a\nb":             "a b",
		"a\r\nb":           "a  b",
		"a\tb":             "a b",
		"a\x00b\x7fc":      "a b c",
		"a\u0085b":         "a b",
		"a\u2028b\u2029c":  "a b c",
		"\u00e9\u4e16 ok":  "\u00e9\u4e16 ok",
		"a\xffb":           "a\ufffdb",
		"// not a comment": "// not a comment",
	} {
		if got := comment(in); got != want {
			t.Errorf("comment(%q) = %q, want %q", in, got, want)
		}
	}
}

// The value the generated test uses for "no such literal" is a 32-bit
// integer that no literal has, also at the ends of the range, and the
// generated files type-check for every value a literal may have.
func TestEnumerationValuesAtTheEndsOfInt32(t *testing.T) {
	for _, tc := range []struct {
		values []int
		unused int
	}{
		{[]int{0}, 1},
		{[]int{1, 2, 3}, 4},
		{[]int{-5, -1}, 0},
		{[]int{math.MaxInt32 - 1}, math.MaxInt32},
		{[]int{math.MaxInt32}, math.MaxInt32 - 1},
		{[]int{0, math.MaxInt32}, -1},
		{[]int{math.MinInt32}, math.MinInt32 + 1},
		{[]int{math.MinInt32, math.MaxInt32}, math.MinInt32 + 1},
		{[]int{math.MinInt32, math.MinInt32 + 1, math.MinInt32 + 3, math.MaxInt32}, math.MinInt32 + 2},
	} {
		used := map[int]bool{}
		for _, v := range tc.values {
			used[v] = true
		}
		if got := unusedValue(used); got != tc.unused || used[got] {
			t.Errorf("unusedValue(%v) = %d, want %d", tc.values, got, tc.unused)
		}
		id := model.NamespaceID{Standard: "E", Version: "1"}
		e := model.Enumeration{Name: "EKind", Source: id}
		for i, v := range tc.values {
			e.Literals = append(e.Literals, model.Literal{Name: fmt.Sprintf("l%d", i), Value: v})
		}
		c, err := catalog.New(&model.Namespace{ID: id, Enumerations: []model.Enumeration{e}})
		if err != nil {
			t.Fatal(err)
		}
		p, err := c.Profile(id)
		if err != nil {
			t.Fatal(err)
		}
		files, err := Go(Input{Profile: p}, "names")
		if err != nil {
			t.Fatal(err)
		}
		pkg := typeCheck(t, files)
		if pkg.Scope().Lookup("EKind") == nil {
			t.Errorf("values %v: no type EKind", tc.values)
		}
		if want := fmt.Sprintf("EKind(%d).String()", tc.unused); !strings.Contains(string(files[GoTestFileName]), want) {
			t.Errorf("values %v: the generated test lacks %s", tc.values, want)
		}
	}
}

// The header names the generator revision: the version of this library
// in the program that generates, when it has one.
func TestHeaderNamesTheLibraryVersion(t *testing.T) {
	defer func(saved func() (*debug.BuildInfo, bool)) { readBuildInfo = saved }(readBuildInfo)
	library := func(version string, replace *debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{
			Main: debug.Module{Path: modulePath + "/cmd/nsdctl", Version: "(devel)"},
			Deps: []*debug.Module{nil, {Path: "github.com/spf13/cobra", Version: "v1.10.2"}, {Path: modulePath, Version: version, Replace: replace}},
		}
	}
	for _, tc := range []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"a released library", library("v0.10.2", nil), true, "Generator: nsdctl v1 (go-iec61850-model v0.10.2, output format 7)"},
		{"a pseudo-version", library("v0.10.2-0.20261115101500-0123456789ab", nil), true,
			"Generator: nsdctl v1 (go-iec61850-model v0.10.2-0.20261115101500-0123456789ab, output format 7)"},
		{"a replaced library", library("v0.10.2", &debug.Module{Path: "../.."}), true, "Generator: nsdctl v1 (output format 7)"},
		{"a library without a version", library("(devel)", nil), true, "Generator: nsdctl v1 (output format 7)"},
		{"the library's own tree", &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: "v0.10.2"}}, true, "Generator: nsdctl v1 (output format 7)"},
		{"no dependency on the library", &debug.BuildInfo{Deps: []*debug.Module{{Path: "example.org/x", Version: "v1.0.0"}}}, true, "Generator: nsdctl v1 (output format 7)"},
		{"no build information", nil, false, "Generator: nsdctl v1 (output format 7)"},
		{"nil build information", nil, true, "Generator: nsdctl v1 (output format 7)"},
	} {
		readBuildInfo = func() (*debug.BuildInfo, bool) { return tc.info, tc.ok }
		n := plainNames()
		n.base, n.toolVersion = "", "v1"
		in := named(t, n)
		lines := in.headerLines("go")
		if lines[2] != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, lines[2], tc.want)
		}
		for kind, gen := range map[string]func(Input) (Files, error){
			"go":      func(in Input) (Files, error) { return Go(in, "names") },
			"catalog": func(in Input) (Files, error) { return Catalog(in, "official", nil) },
			"docs":    Docs,
		} {
			files, err := gen(in)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{GoFileName, CatalogGoFileName, "index.md"} {
				if src, ok := files[name]; ok && !strings.Contains(string(src), tc.want+"\n") {
					t.Errorf("%s: %s of generate %s lacks %q", tc.name, name, kind, tc.want)
				}
			}
		}
	}
	// The JSON forms carry the output format, as before.
	readBuildInfo = func() (*debug.BuildInfo, bool) { return library("v0.10.2", nil), true }
	files, err := Catalog(named(t, plainNames()), "official", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files[CatalogJSONFileName]), "\"generator\": \"nsdctl v0.0.0-test\",\n  \"format\": 7,") {
		t.Errorf("catalog JSON header: %.200s", files[CatalogJSONFileName])
	}
}
