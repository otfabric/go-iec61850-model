// SPDX-License-Identifier: MIT

package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/internal/sources"
	"github.com/otfabric/go-iec61850-model/model"
)

func input(t *testing.T) Input {
	t.Helper()
	return Input{Profile: fixtures.Profile(t), ToolVersion: "v0.0.0-test"}
}

func all(t *testing.T, in Input) map[string]Files {
	t.Helper()
	g, err := Go(in, "names")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Docs(in)
	if err != nil {
		t.Fatal(err)
	}
	j, err := JSON(in)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]Files{"go": g, "docs": d, "json": j}
}

func TestDeterministic(t *testing.T) {
	a, b := all(t, input(t)), all(t, input(t))
	for kind, files := range a {
		for _, p := range files.Paths() {
			if !bytes.Equal(files[p], b[kind][p]) {
				t.Errorf("%s/%s differs between runs", kind, p)
			}
			if !isGenerated(files[p]) {
				t.Errorf("%s/%s lacks the marker", kind, p)
			}
			if bytes.Contains(files[p], []byte(fixtures.Dir())) {
				t.Errorf("%s/%s contains a local path", kind, p)
			}
		}
		if _, ok := files["LOCAL-ONLY.md"]; !ok {
			t.Errorf("%s output has no local-only notice", kind)
		}
	}
}

func TestGoOutputTypeChecks(t *testing.T) {
	files, err := Go(input(t), "names")
	if err != nil {
		t.Fatal(err)
	}
	src := files[GoFileName]
	pkg := typeCheck(t, files)
	for _, name := range []string{"LN_MMXU", "CDC_WYE", "MMXU_Hz", "MMXU_Beh", "BehaviourModeKind_test_blocked", "MultiplierKind_Empty", "LNClasses", "CDCs"} {
		if pkg.Scope().Lookup(name) == nil {
			t.Errorf("missing identifier %s", name)
		}
	}
	text := string(src)
	for _, want := range []string{"Renamed identifiers", `"BehaviourModeKind_test/blocked" -> BehaviourModeKind_test_blocked`, `DataObjectRef = "MMXU.Hz"`, "sha256:"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
	if _, err := Go(input(t), "not a package"); err == nil {
		t.Error("invalid package name accepted")
	}
	if _, err := Go(Input{}, "x"); !errors.Is(err, model.ErrUnknownNamespace) {
		t.Errorf("nil profile: %v", err)
	}
}

func TestIdents(t *testing.T) {
	id := newIdents("CDC")
	for _, tc := range []struct{ prefix, name, want string }{
		{"", "CDC", "CDC_2"}, {"", "lower", "Xlower"}, {"", "a-b", "Xa_b"}, {"T_", "", "T_Empty"},
		{"", "Ünïcode", "X_n_code"}, {"E_", "1x", "E_1x"}, {"E_", "1x", "E_1x_2"},
	} {
		if got := id.assign(tc.prefix, tc.name); got != tc.want {
			t.Errorf("assign(%q, %q) = %s, want %s", tc.prefix, tc.name, got, tc.want)
		}
	}
	if len(id.renames) != 6 {
		t.Errorf("renames %v", id.renames)
	}
}

func TestDocsContent(t *testing.T) {
	files, err := Docs(input(t))
	if err != nil {
		t.Fatal(err)
	}
	ln := string(files["logical-nodes.md"])
	for _, want := range []string{"## M — Metering and measurement", "### MMXU", "MMXU → StatisticsLN → DomainLN",
		"| PNV | WYE | O | MMXU | voltage, line-neutral |", "| TotW | MV | O | MMXU | active-power (total) |", "## D — Distributed energy resources"} {
		if !strings.Contains(ln, want) {
			t.Errorf("logical-nodes.md lacks %q", want)
		}
	}
	if strings.Contains(ln, "Test measurement class") {
		t.Error("NSDoc text included without IncludeDocText")
	}
	if !strings.Contains(string(files["cdcs.md"]), "| stVal | data attribute | (ENUMERATED, from data object) | ST | M | dchg |") {
		t.Error("cdcs.md parameterized attribute")
	}
	if !strings.Contains(string(files["types.md"]), "| 4 | test/blocked |") {
		t.Error("types.md literals")
	}
	if !strings.Contains(string(files["coverage.md"]), "| IEC 61850-7-4:2007B1 | 5 (2) | 17 / 17 |") {
		t.Errorf("coverage.md:\n%s", files["coverage.md"])
	}
	if !strings.Contains(string(files["index.md"]), "[Coverage](coverage.md)") {
		t.Error("index.md")
	}
	in := input(t)
	in.IncludeDocText = true
	withText, _ := Docs(in)
	if !strings.Contains(string(withText["logical-nodes.md"]), "Test measurement class") {
		t.Error("IncludeDocText ignored")
	}
}

func TestJSONContent(t *testing.T) {
	files, err := JSON(input(t))
	if err != nil {
		t.Fatal(err)
	}
	var cat struct {
		Generator  string             `json:"generator"`
		Namespaces []*model.Namespace `json:"namespaces"`
	}
	if err := json.Unmarshal(files["catalog.json"], &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Generator != "nsdctl v0.0.0-test" || len(cat.Namespaces) != 4 {
		t.Fatalf("catalog %s, %d namespaces", cat.Generator, len(cat.Namespaces))
	}
	for _, ns := range cat.Namespaces {
		if strings.Contains(ns.Provenance.Source, "/") || ns.Doc.Title != "" {
			t.Errorf("namespace %s keeps a path or text: %+v", ns.ID, ns.Provenance)
		}
	}
	if bytes.Contains(files["ontology.json"], []byte("Test measurement class")) || !bytes.Contains(files["ontology.json"], []byte(`"Voltage, phase A, line-neutral"`)) {
		t.Error("ontology labels")
	}
	if !bytes.Contains(files["coverage.json"], []byte(`"dataObjectsResolved": 17`)) {
		t.Errorf("coverage.json:\n%s", files["coverage.json"])
	}
	in := input(t)
	in.IncludeDocText = true
	in.Publish = true
	withText, _ := JSON(in)
	if !bytes.Contains(withText["ontology.json"], []byte("Test measurement class")) || withText["LOCAL-ONLY.md"] != nil {
		t.Error("IncludeDocText/Publish ignored")
	}
}

func TestWriteAndCheck(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	files, _ := Go(input(t), "names")
	if diffs, err := Check(dir, files); err != nil || len(diffs) != 3 || diffs[0].Kind != DiffMissing {
		t.Fatalf("check of missing dir: %v %v", diffs, err)
	}
	if _, err := Write(dir, files, false); err != nil {
		t.Fatal(err)
	}
	if diffs, err := Check(dir, files); err != nil || len(diffs) != 0 {
		t.Fatalf("check after write: %v %v", diffs, err)
	}
	// Change one file, add a stale generated file and a foreign file.
	if err := os.WriteFile(filepath.Join(dir, GoFileName), append([]byte("// "+Marker+"\n"), "x"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old_gen.go"), []byte("// "+Marker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mine.go"), []byte("package names\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diffs, err := Check(dir, files)
	if err != nil || len(diffs) != 2 || diffs[0].Kind != DiffChanged || diffs[1] != (Difference{Path: "old_gen.go", Kind: DiffStale}) {
		t.Fatalf("diffs %v %v", diffs, err)
	}
	if _, err := Write(dir, files, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old_gen.go")); !os.IsNotExist(err) {
		t.Error("stale generated file not removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "mine.go")); err != nil {
		t.Error("foreign file removed")
	}
	// A foreign file at a generated path is not overwritten without force.
	if err := os.WriteFile(filepath.Join(dir, GoFileName), []byte("package names // hand-written\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, files, false); !errors.Is(err, ErrWouldOverwrite) {
		t.Fatalf("overwrite: %v", err)
	}
	if written, err := Write(dir, files, true); err != nil || len(written) != 3 {
		t.Fatalf("forced write: %v %v", written, err)
	}
}

func TestPolicy(t *testing.T) {
	p := fixtures.Profile(t)
	var pe *PolicyError
	if err := (Policy{}).CheckPublish(p); !errors.As(err, &pe) || !errors.Is(err, ErrPolicyRefused) || len(pe.Blocked) != 4 {
		t.Fatalf("no lock: %v", err)
	}
	lock := &sources.Lock{SchemaVersion: 1}
	for _, ns := range p.Namespaces() {
		lock.Sources = append(lock.Sources, sources.Entry{Namespace: ns.ID.String(), Status: sources.StateAdopted,
			SHA256: ns.Provenance.SHA256, Redistribution: sources.RedistributionPermitted})
	}
	if err := (Policy{Lock: lock}).CheckPublish(p); err != nil {
		t.Fatalf("permitted: %v", err)
	}
	lock.Sources[0].Redistribution = sources.RedistributionLocalOnly
	lock.Sources[1].SHA256 = strings.Repeat("0", 64)
	lock.Sources = lock.Sources[:3]
	err := (Policy{Lock: lock}).CheckPublish(p)
	if !errors.As(err, &pe) || len(pe.Blocked) != 3 || !strings.Contains(err.Error(), "not permitted") || !strings.Contains(err.Error(), "does not match") || !strings.Contains(err.Error(), "not in the lock") {
		t.Errorf("blocked: %v", err)
	}
}

func TestEmptyProfileAndBaseSource(t *testing.T) {
	ns := &model.Namespace{ID: model.NamespaceID{Standard: "E", Version: "1"}, Enumerations: []model.Enumeration{{Name: "Empty"}}}
	c, _ := catalog.New(ns)
	p, _ := c.Profile(ns.ID)
	files, err := Go(Input{Profile: p, Publish: true}, "names")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(files[GoFileName], []byte("strconv")) || files["LOCAL-ONLY.md"] != nil || !bytes.Contains(files[GoFileName], []byte("nsdctl dev")) {
		t.Errorf("minimal output:\n%s", files[GoFileName])
	}
	if got := baseSource("/a/b/pkg.zip!x/y.nsd"); got != "pkg.zip!x/y.nsd" {
		t.Errorf("baseSource = %s", got)
	}
	if _, err := Docs(Input{}); err == nil {
		t.Error("Docs without profile")
	}
	if _, err := JSON(Input{}); err == nil {
		t.Error("JSON without profile")
	}
	if _, err := ComputeCoverage(Input{}); err == nil {
		t.Error("coverage without profile")
	}
}

// TestGolden compares the output for the synthetic fixtures with
// testdata/golden. Run `make golden` (UPDATE_GOLDEN=1) after an
// intentional change of the output format, and bump FormatVersion.
func TestGolden(t *testing.T) {
	in := input(t)
	in.ToolVersion = "golden"
	for kind, files := range all(t, in) {
		dir := filepath.Join("..", "..", "testdata", "golden", kind)
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if _, err := Write(dir, files, false); err != nil {
				t.Fatal(err)
			}
			continue
		}
		diffs, err := Check(dir, files)
		if err != nil {
			t.Fatal(err)
		}
		if len(diffs) > 0 {
			t.Errorf("golden %s output differs: %v (run make golden after an intentional change)", kind, diffs)
		}
	}
}

func TestNotice(t *testing.T) {
	ns := &model.Namespace{ID: model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 6},
		Copyright: model.Copyright{Notice: "COPYRIGHT (c) IEC, 2025. Test notice.", LicenseURI: "www.iec.ch/CCv1", LicenseKind: "Standard"}}
	other := &model.Namespace{ID: model.NamespaceID{Standard: "X", Version: "1"}}
	c, _ := catalog.New(ns, other)
	p, _ := c.Profile(ns.ID, other.ID)
	for _, gen := range []func(Input) (Files, error){func(in Input) (Files, error) { return Go(in, "names") }, Docs, JSON} {
		files, err := gen(Input{Profile: p})
		if err != nil {
			t.Fatal(err)
		}
		n := string(files["NOTICE.md"])
		if !strings.Contains(n, "## IEC 61850-7-4:2007B6") || !strings.Contains(n, "License: www.iec.ch/CCv1 (Standard).") || !strings.Contains(n, "> COPYRIGHT (c) IEC, 2025. Test notice.") || strings.Contains(n, "## X:1") {
			t.Errorf("NOTICE.md:\n%s", n)
		}
	}
	files, _ := Go(Input{Profile: p}, "names")
	if !bytes.Contains(files[GoFileName], []byte("Attribution: derived from IEC code components")) {
		t.Error("Go header lacks the attribution")
	}
	if Input.noticeFile(Input{Profile: fixtures.Profile(t)}, "go") != nil {
		t.Error("no notice without copyright")
	}
}

// typeCheck parses and type-checks the generated file together with its
// generated test.
func typeCheck(t *testing.T, files Files) *types.Package {
	t.Helper()
	fset := token.NewFileSet()
	var parsed []*ast.File
	for _, name := range []string{GoFileName, GoTestFileName} {
		f, err := parser.ParseFile(fset, name, files[name], parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v\n%s", name, err, files[name])
		}
		if !ast.IsGenerated(f) {
			t.Errorf("%s is not recognized as generated", name)
		}
		parsed = append(parsed, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("names", fset, parsed, nil)
	if err != nil {
		t.Fatalf("type check: %v", err)
	}
	return pkg
}

func TestOverlappingDefinitions(t *testing.T) {
	a := model.NamespaceID{Standard: "A", Version: "1"}
	b := model.NamespaceID{Standard: "B", Version: "1"}
	nsA := &model.Namespace{ID: a,
		CDCs:              []model.CommonDataClass{{Name: "MV", Source: a}},
		AbstractLNClasses: []model.LogicalNodeClass{{Name: "BaseLN", Abstract: true, Source: a, DataObjects: []model.DataObject{{Name: "Beh", Type: "MV"}}}},
		LNClasses:         []model.LogicalNodeClass{{Name: "DBAT", Base: "BaseLN", Source: a, DataObjects: []model.DataObject{{Name: "One", Type: "MV"}, {Name: "Both", Type: "MV"}}}},
		Enumerations:      []model.Enumeration{{Name: "KindE", Source: a, Literals: []model.Literal{{Name: "x", Value: 1}, {Name: "y", Value: 2}}}},
	}
	nsB := &model.Namespace{ID: b,
		LNClasses:    []model.LogicalNodeClass{{Name: "DBAT", Base: "Missing", Source: b, DataObjects: []model.DataObject{{Name: "Both", Type: "MV"}, {Name: "Two", Type: "MV"}}}},
		Enumerations: []model.Enumeration{{Name: "KindE", Source: b, Literals: []model.Literal{{Name: "y", Value: 3}, {Name: "z", Value: 1}}}},
	}
	c, _ := catalog.New(nsA, nsB)
	p, err := c.Profile(a, b)
	if err != nil {
		t.Fatal(err)
	}
	files, err := Go(Input{Profile: p}, "names")
	if err != nil {
		t.Fatal(err)
	}
	pkg := typeCheck(t, files)
	src := string(files[GoFileName])
	for _, want := range []string{"DBAT_Beh", "DBAT_One", "DBAT_Both", "DBAT_Two", "KindE_x", "KindE_y", "KindE_z", "LN_DBAT"} {
		if pkg.Scope().Lookup(want) == nil {
			t.Errorf("missing %s", want)
		}
	}
	for _, unwanted := range []string{"LN_DBAT_2", "DBAT_Both_2", "KindE_2"} {
		if pkg.Scope().Lookup(unwanted) != nil {
			t.Errorf("duplicate %s", unwanted)
		}
	}
	for _, note := range []string{"DBAT is defined by A:1, B:1; its data objects are the union", "KindE.y is 2 in one definition and 3 in B:1; 2 is kept", "KindE is defined by A:1, B:1"} {
		if !strings.Contains(src, note) {
			t.Errorf("header lacks %q", note)
		}
	}
	test := string(files[GoTestFileName])
	if !strings.Contains(test, `{KindE_x.String(), "x"}`) || strings.Contains(test, `{KindE_z.String()`) || !strings.Contains(test, `{KindE(3).String(), "KindE(3)"}`) {
		t.Errorf("generated test:\n%s", test)
	}
}
