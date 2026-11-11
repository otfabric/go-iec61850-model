// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/nsd"
)

func decode(t *testing.T, data []byte) map[string]nsd.DocText {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	sets, err := nsd.ReadDocSets(raw)
	if err != nil || len(sets) != 1 || sets[0].Lang() != "en" {
		t.Fatalf("sets %v: %v", sets, err)
	}
	out := map[string]nsd.DocText{}
	for _, k := range sets[0].Keys() {
		out[k], _ = sets[0].Text(k)
	}
	return out
}

func TestBuildFixture(t *testing.T) {
	data, report, err := build("testdata/texts", "testdata/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, data)
	want := map[string]nsd.DocText{
		"LN:MMXU":                     {Title: "Three-phase measurement"},
		"DO:MMXU.TotW":                {Description: "Total active power"},
		"DO:MMXU.Tm2W":                {Description: "Time 2 active power"},
		"DO:MMXU.Odd":                 {Description: "An odd one"},
		"DO:MMXU.Spec":                {Description: "A specific one"},
		"CDC:MV":                      {Title: "Measured value"},
		"DA:MV.mag":                   {Description: "Magnitude"},
		"SDO:MV.sub":                  {Description: "Sub object"},
		"SP:MV.sp":                    {Description: "Parameter"},
		"CA:Vector":                   {Title: "Vector"},
		"SDA:Vector.mag":              {Description: "Magnitude of the vector"},
		"FC:ST":                       {Title: "Status"},
		"BT:BOOLEAN":                  {Title: "Boolean"},
		"PC:M":                        {Description: "Mandatory"},
		"ABBR:Tot":                    {Description: "Total"},
		"ABBR:W":                      {Description: "Active power"},
		"ABBR:Tm":                     {Description: "Time"},
		"ENUM:HealthKind":             {Title: "Health kind"},
		"LIT:HealthKind.Ok":           {Description: "Ok"},
		"LIT:HealthKind.test-blocked": {Description: "Test blocked"},
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%d keys, want %d", len(got), len(want))
	}
	if !strings.Contains(report, "20 keys") || !strings.Contains(report, "composed=2 table=2") {
		t.Errorf("report %q", report)
	}
	if !strings.Contains(report, "omitted=0") {
		t.Errorf("report %q", report)
	}
	again, _, _ := build("testdata/texts", "testdata/catalog.json")
	if !bytes.Equal(again, data) {
		t.Error("output is not deterministic")
	}
}

// The committed public doc set is current and covers the catalog.
func TestCommittedOutputIsCurrent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	var stderr bytes.Buffer
	if code := run([]string{"-check", "-texts", filepath.Join(root, "texts", "en"),
		"-catalog", filepath.Join(root, "official", "catalog_gen.json"),
		"-out", filepath.Join(root, "official", "texts_gen.json.gz")}, io.Discard, &stderr); code != 0 {
		t.Fatalf("textgen -check: %d %s", code, stderr.String())
	}
}

func copyTexts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range tableNames {
		b, err := os.ReadFile(filepath.Join("testdata", "texts", n+".tsv"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, n+".tsv"), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBuildErrors(t *testing.T) {
	cases := []struct {
		name, table, content, want string
	}{
		{"missing text", "ln.tsv", "", "keys without a text: LN:MMXU"},
		{"unused entry", "cdc.tsv", "MV\tMeasured value\nXYZ\tNothing\n", "match no element: cdc:XYZ"},
		{"duplicate", "fc.tsv", "ST\tA\nST\tB\n", `duplicate "ST"`},
		{"malformed", "bt.tsv", "BOOLEAN\n", "want name<TAB>text"},
		{"no split", "abbreviations.tsv", "Tot\ttotal\nTm\ttime\n", "DO:MMXU.TotW"},
		{"omitted term in use", "abbreviations.tsv", "Tot\ttotal\nW\t-\nTm\ttime\n", "DO:MMXU.TotW"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyTexts(t)
			if err := os.WriteFile(filepath.Join(dir, tc.table), []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := build(dir, "testdata/catalog.json"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
	dir := copyTexts(t)
	_ = os.Remove(filepath.Join(dir, "pc.tsv"))
	if _, _, err := build(dir, "testdata/catalog.json"); err == nil {
		t.Error("missing table accepted")
	}
	if _, _, err := build("testdata/texts", filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Error("missing catalog accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte("{"), 0o600)
	if _, _, err := build("testdata/texts", bad); err == nil {
		t.Error("bad catalog accepted")
	}
}

// A term with the text "-" gets no ABBR text and counts as omitted.
func TestBuildOmittedTerm(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	cat := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(cat, bytes.Replace(raw, []byte(`{"name":"Tm"}`), []byte(`{"name":"Tm"},{"name":"Unused"}`), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := copyTexts(t)
	if err := os.WriteFile(filepath.Join(dir, "abbreviations.tsv"), []byte("Tot\ttotal\nW\tactive power\nTm\ttime\nUnused\t-\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, report, err := build(dir, cat)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := decode(t, data)["ABBR:Unused"]; ok {
		t.Error("omitted term has a text")
	}
	if !strings.Contains(report, "20 keys") || !strings.Contains(report, "omitted=1") {
		t.Errorf("report %q", report)
	}
}

func TestBuildManyMissing(t *testing.T) {
	var lns []map[string]any
	for i := range 45 {
		lns = append(lns, map[string]any{"name": "L" + strings.Repeat("x", i+1)})
	}
	cat, _ := json.Marshal(map[string]any{"namespaces": []any{map[string]any{"id": map[string]string{"standard": "T"}, "lnClasses": lns}}})
	path := filepath.Join(t.TempDir(), "cat.json")
	_ = os.WriteFile(path, cat, 0o600)
	dir := copyTexts(t)
	for _, n := range tableNames {
		_ = os.WriteFile(filepath.Join(dir, n+".tsv"), nil, 0o600)
	}
	_, _, err := build(dir, path)
	if err == nil || !strings.Contains(err.Error(), "45 keys without a text") || !strings.HasSuffix(err.Error(), "...") {
		t.Errorf("err = %v", err)
	}
}

func TestRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "texts.json.gz")
	args := []string{"-texts", "testdata/texts", "-catalog", "testdata/catalog.json", "-out", out}
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "textgen: 20 keys") {
		t.Fatalf("run: %d %q %q", code, stdout.String(), stderr.String())
	}
	if code := run(append(args, "-check"), io.Discard, &stderr); code != 0 {
		t.Errorf("check after write: %d %s", code, stderr.String())
	}
	_ = os.WriteFile(out, []byte("stale"), 0o600)
	stderr.Reset()
	if code := run(append(args, "-check"), io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "stale") {
		t.Errorf("stale check: %d %q", code, stderr.String())
	}
	if code := run([]string{"-texts", t.TempDir()}, io.Discard, io.Discard); code != 1 {
		t.Errorf("missing tables: %d", code)
	}
	if code := run([]string{"-nope"}, io.Discard, io.Discard); code != 2 {
		t.Errorf("bad flag: %d", code)
	}
	if code := run([]string{"-texts", "testdata/texts", "-catalog", "testdata/catalog.json", "-out", filepath.Join(t.TempDir(), "no", "dir", "x")}, io.Discard, io.Discard); code != 1 {
		t.Errorf("unwritable output: %d", code)
	}
}

func TestWords(t *testing.T) {
	for in, want := range map[string]string{
		"HealthKind":   "health kind",
		"test-blocked": "test blocked",
		"SIUnitKind":   "SI unit kind",
		"on_off/x":     "on off x",
		"Type2Kind":    "type 2 kind",
		"kWh":          "k wh",
	} {
		if got := words(in); got != want {
			t.Errorf("words(%q) = %q, want %q", in, got, want)
		}
	}
	if capitalize("") != "" || capitalize("é") != "É" {
		t.Error("capitalize")
	}
	if emptyLiteral("MultiplierKind.") != "No multiplier (factor 1)" || emptyLiteral("SIUnitKind.") != "No unit (dimensionless)" || emptyLiteral("X.") != "Empty literal" {
		t.Error("emptyLiteral")
	}
	gz, _, _ := build("testdata/texts", "testdata/catalog.json")
	var other bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&other, gzip.NoCompression)
	raw, _ := gunzip(gz)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	if !sameContent(gz, other.Bytes()) || sameContent(gz, []byte("x")) || sameContent([]byte("x"), gz) {
		t.Error("sameContent")
	}
	if isDigits("") || !isDigits("12") || isDigits("1a") {
		t.Error("isDigits")
	}
}
