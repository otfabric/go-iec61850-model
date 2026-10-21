// SPDX-License-Identifier: MIT

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/internal/sources"
)

// The full-set test builds a directory with every package tracked in
// sources/index.json under its official name, filled with synthetic,
// independently authored content in the shape of each package. It
// proves the tooling covers the whole set without any IEC file.

const nsHeader = `<?xml version="1.0" encoding="UTF-8"?>
<!-- Synthetic test content, not IEC material. -->
`

// extension returns a synthetic extension namespace with one class.
func extension(id, version, revision, release, depends, class, do string) []byte {
	rel := ""
	if release != "" {
		rel = fmt.Sprintf(` release="%s"`, release)
	}
	return []byte(nsHeader + fmt.Sprintf(`<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="%s" version="%s" revision="%s"%s>
  %s
  <LNClasses>
    <LNClass name="%s" base="DomainLN">
      <DataObject name="%s" type="MV" presCond="O"/>
    </LNClass>
  </LNClasses>
</NS>
`, id, version, revision, rel, depends, class, do))
}

const dependsOn74 = `<DependsOn id="IEC 61850-7-4" version="2007" revision="B"/>`
const dependsOn420 = `<DependsOn id="IEC 61850-7-4" version="2007" revision="B"/><DependsOn id="IEC 61850-7-420" version="2019" revision="A"/>`

const syntheticXSD = `<?xml version="1.0" encoding="UTF-8"?>
<!-- Synthetic test schema, not IEC material. -->
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.iec.ch/61850/2016/NSD">
  <xs:element name="NS"/>
  <xs:element name="LNClass"/>
  <xs:element name="FutureElement"/>
  <xs:attribute name="name"/>
  <xs:attribute name="futureAttr"/>
</xs:schema>
`

func zipOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range sortedNames(files) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sortedNames(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func asRelease6(t *testing.T, name string) []byte {
	return bytes.Replace(fixtures.Read(t, name), []byte(`release="1"`), []byte(`release="6"`), 1)
}

// fullSet writes the synthetic package directory and returns its path.
func fullSet(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	manifest := []byte(`<IECManifest><Copyright>synthetic</Copyright></IECManifest>`)
	pkgs := map[string]map[string][]byte{
		"IEC_61850-7-2.NSD.2007B6.light": {"IEC_61850-7-2_2007B6.nsd": asRelease6(t, fixtures.NS72), "NSD.xsd": []byte(syntheticXSD), "IECManifest.xml": manifest},
		"IEC_61850-7-3.NSD.2007B6.Light": {"IEC_61850-7-3_2007B6.nsd": asRelease6(t, fixtures.NS73)},
		"IEC_61850-7-4.NSD.2007B6.Light": {"IEC_61850-7-4_2007B6.nsd": asRelease6(t, fixtures.NS74),
			"IEC_61850-7-4_2007B6.en.nsdoc": fixtures.Read(t, fixtures.NSDoc74)},
		"IEC_61850-7-410.NSD.2024A4.Light.zip":    {"7-410.nsd": extension("IEC 61850-7-410", "2024", "A", "4", dependsOn74, "HXTS", "TurSpd")},
		"IEC_61850-7-420.NSD.2019A4.Light.zip":    {"7-420.nsd": extension("IEC 61850-7-420", "2019", "A", "4", dependsOn74, "DXTS", "OutW")},
		"IEC_TR_61850-90-9.NSD.2018A3.Light.zip":  {"90-9.nsd": extension("IEC TR 61850-90-9", "2018", "A", "3", dependsOn420, "ZXBA", "SocW")},
		"IEC_TR_61850-90-14.NSD.2020A3.Light.zip": {"90-14.nsd": extension("IEC 61850-90-14", "2020", "A", "3", dependsOn74, "ZXCV", "DcW")},
		"IEC_61850-90-6.NSD.2018B.light.zip":      {"90-6.nsd": extension("IEC 61850-90-6", "2018", "B", "", dependsOn74, "AXDA", "FltA")},
		"IEC_TR_61850-90-3.NSD.2015B.Light.zip":   {"90-3.nsd": extension("IEC 61850-90-3", "2015", "B", "2", dependsOn74, "SXCM", "Temp")},
		"IEC_TR_61850-90-4.NSD.2018A.Light.zip":   {"90-4.nsd": extension("IEC 61850-90-4", "2018", "A", "", dependsOn74, "LXBR", "Load")},
		"IEC_TR_61850-90-11.NSD.2019A2.light.zip": {"90-11.nsd": extension("IEC 61850-90-11", "2019", "A", "2", dependsOn74, "FXLG", "Out")},
		"IEC_TR_61850-90-21.NSD.2022A2.Light":     {"90-21.nsd": extension("IEC 61850-90-21", "2022", "A", "2", dependsOn74, "RXTW", "Dist")},
		"IEC_61850-8-1.NSD.2003A4.Light": {"8-1.nsd": []byte(nsHeader + `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="IEC 61850-8-1" version="2003" revision="A" release="4">
  <DependsOn id="IEC 61850-7-2" version="2007" revision="B"/>
  <Enumerations><Enumeration name="SyntheticMappingKind"><Literal name="a" literalVal="1"/></Enumeration></Enumerations>
</NS>`)},
		"IEC_61850-7-7.2017.NSD.2017A.full.zip": {"NSD.xsd": []byte(syntheticXSD), "NSDoc.xsd": []byte(syntheticXSD), "IECManifest.xml": manifest},
	}
	for name, files := range pkgs {
		if err := os.WriteFile(filepath.Join(dir, name), zipOf(t, files), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestFullSetCoverage(t *testing.T) {
	dir := fullSet(t)
	index, lock := "../../sources/index.json", "../../sources/nsd.lock.json"

	idx, err := sources.LoadIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Packages) != 14 {
		t.Fatalf("the registry tracks %d packages, want 14", len(idx.Packages))
	}

	out, errOut, code := nsdctl(t, "--json", "sources", "check", "--index", index, "--lock", lock, "--packages", dir)
	if code != exitOK {
		t.Fatalf("sources check: exit %d\n%s\n%s", code, out, errOut)
	}
	var rep struct {
		Packages []packageStatus `json:"packages"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Packages) != 14 {
		t.Fatalf("%d packages checked", len(rep.Packages))
	}
	for _, p := range rep.Packages {
		if p.Status != statusOK {
			t.Errorf("%s: %s %v %s", p.Name, p.Status, p.Found, p.Error)
		}
		if p.Kind == sources.KindNSD && len(p.Namespaces) != 1 {
			t.Errorf("%s carries %v", p.Name, p.Namespaces)
		}
	}

	// A missing recommended package fails the check unless only the required tier is asked for.
	if err := os.Remove(filepath.Join(dir, "IEC_TR_61850-90-14.NSD.2020A3.Light.zip")); err != nil {
		t.Fatal(err)
	}
	if out, _, code := nsdctl(t, "sources", "check", "--index", index, "--lock", lock, "--packages", dir); code != exitDifferent || !strings.Contains(out, "missing") {
		t.Errorf("missing package: exit %d\n%s", code, out)
	}
	if _, _, code := nsdctl(t, "sources", "check", "--index", index, "--lock", lock, "--packages", dir, "--tiers", "required"); code != exitOK {
		t.Errorf("required tier only: exit %d", code)
	}
	if _, _, code := nsdctl(t, "sources", "check", "--index", index, "--lock", lock, "--packages", dir, "--tiers", "bogus"); code != exitUsage {
		t.Errorf("bad tier: exit %d", code)
	}
}

func TestFullSetLockAndModel(t *testing.T) {
	dir := fullSet(t)
	index := "../../sources/index.json"
	lockPath := filepath.Join(t.TempDir(), "nsd.lock.json")
	if err := os.WriteFile(lockPath, []byte(`{"schemaVersion":1,"sources":[{"standard":"IEC 61850-7-420","namespace":"IEC 61850-7-420:2019A4","status":"nsd-available","redistribution":"permitted","source":"https://webstore.iec.ch/x"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := nsdctl(t, "sources", "lock", "--index", index, "--lock", lockPath, "--packages", dir)
	if code != exitOK {
		t.Fatalf("sources lock: %d %s", code, errOut)
	}
	var lock sources.Lock
	if err := json.Unmarshal([]byte(out), &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.Sources) != 14 {
		t.Fatalf("%d lock entries, want 13 namespaces and the schema package", len(lock.Sources))
	}
	if ds := lock.Validate(); ds.HasErrors() {
		t.Errorf("generated lock is invalid: %v", ds)
	}
	for _, e := range lock.Sources {
		switch e.Namespace {
		case "IEC 61850-7-420:2019A4":
			if e.Redistribution != sources.RedistributionPermitted || e.Source == "" || e.Status != sources.StateNSDValidated || e.Content != "light" {
				t.Errorf("existing decisions must be kept: %+v", e)
			}
		case "IEC 61850-7-7:2017A":
			if e.Status != sources.StateNSDAvailable || e.Content != "full" || !strings.Contains(e.Notes, "2 XSD files") {
				t.Errorf("schema entry %+v", e)
			}
		default:
			if e.Status != sources.StateNSDValidated || e.Redistribution != sources.RedistributionLocalOnly || len(e.SHA256) != 64 {
				t.Errorf("entry %+v", e)
			}
		}
	}
	if _, _, code := nsdctl(t, "sources", "lock", "--index", index, "--lock", lockPath, "--packages", dir, "--write"); code != exitOK {
		t.Fatal("lock --write")
	}
	if _, _, code := nsdctl(t, "sources", "check", "--index", index, "--lock", lockPath, "--packages", dir); code != exitOK {
		t.Errorf("check against the written lock: exit %d", code)
	}
	if _, _, code := nsdctl(t, "sources", "lock", "--index", index, "--lock", lockPath); code != exitUsage {
		t.Error("lock without --packages")
	}

	// The whole set forms one profile: exactly one release per standard.
	if out, e, code := nsdctl(t, "validate", dir); code != exitOK || !strings.Contains(out, "13 namespaces, 0 errors") {
		t.Errorf("validate: %d\n%s\n%s", code, out, e)
	}
	if out, _, code := nsdctl(t, "-s", dir, "model", "list"); code != exitOK || !containsAll(out, "AXDA", "DXTS", "FXLG", "HXTS", "LXBR", "MMXU", "RXTW", "SXCM", "ZXBA", "ZXCV") {
		t.Errorf("model list: %d\n%s", code, out)
	}
	if out, _, code := nsdctl(t, "-s", dir, "model", "show", "ZXBA"); code != exitOK || !strings.Contains(out, "IEC TR 61850-90-9:2018A3") {
		t.Errorf("90-9 class: %d\n%s", code, out)
	}
	if out, _, code := nsdctl(t, "-s", dir, "model", "show", "LD0/MMXU1.Hz.mag.f[MX]"); code != exitOK || !strings.Contains(out, "IEC 61850-7-3:2007B6") {
		t.Errorf("resolution across the set: %d\n%s", code, out)
	}
	if _, _, code := nsdctl(t, "-s", dir, "generate", "docs", "-o", filepath.Join(t.TempDir(), "docs")); code != exitOK {
		t.Error("generate docs over the full set")
	}

	schemaPkg := filepath.Join(dir, "IEC_61850-7-7.2017.NSD.2017A.full.zip")
	out, _, code = nsdctl(t, "schema", schemaPkg)
	if code != exitOK || !containsAll(out, "FutureElement", "futureAttr", "accepted: true") {
		t.Errorf("schema: %d\n%s", code, out)
	}
	if _, _, code := nsdctl(t, "schema", "--exit-code", schemaPkg); code != exitDifferent {
		t.Errorf("schema --exit-code: %d", code)
	}
	if out, _, code := nsdctl(t, "--json", "schema", schemaPkg); code != exitOK || !json.Valid([]byte(out)) {
		t.Errorf("schema json: %d", code)
	}
	if _, _, code := nsdctl(t, "schema", fixtures.Path(fixtures.NS72)); code != exitUsage {
		t.Error("schema without XSD files")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}

// TestOfficialPackages runs against official packages when NSD_DIR
// points to a directory holding them (see docs/nsd-sources.md). The
// packages are read in place; nothing is copied or written next to
// them, and test output contains no IEC text.
func TestOfficialPackages(t *testing.T) {
	dir := os.Getenv("NSD_DIR")
	if dir == "" {
		t.Skip("NSD_DIR not set; official packages are optional local inputs")
	}
	index, lock := "../../sources/index.json", "../../sources/nsd.lock.json"
	out, errOut, code := nsdctl(t, "sources", "check", "--index", index, "--lock", lock, "--packages", dir, "--tiers", "required")
	t.Logf("sources check (required):\n%s%s", out, errOut)
	if code != exitOK {
		t.Fatalf("required packages: exit %d", code)
	}
	out, _, _ = nsdctl(t, "sources", "check", "--index", index, "--lock", lock, "--packages", dir)
	t.Logf("sources check (all tiers):\n%s", out)

	out, errOut, code = nsdctl(t, "validate", dir)
	t.Logf("validate: exit %d\n%s%s", code, out, errOut)

	for _, ref := range []string{"MMXU.Hz.mag.f", "MMXU.TotW.mag.f", "LD0/MMXU1.A.phsA.cVal.mag.f[MX]"} {
		out, errOut, code := nsdctl(t, "-s", dir, "model", "show", ref)
		if code != exitOK {
			t.Errorf("%s: exit %d\n%s", ref, code, errOut)
			continue
		}
		if !strings.Contains(out, "FLOAT32") {
			t.Errorf("%s does not resolve to FLOAT32:\n%s", ref, out)
		}
	}
	if out, _, code := nsdctl(t, "-s", dir, "model", "describe", "MMXU.Hz"); code != exitOK || !strings.Contains(out, "frequency") {
		t.Errorf("describe MMXU.Hz: %d\n%s", code, out)
	}
	out, _, _ = nsdctl(t, "schema", dir)
	t.Logf("schema coverage:\n%s", out)
	out, errOut, code = nsdctl(t, "-s", dir, "generate", "docs", "-o", t.TempDir())
	if code != exitOK {
		t.Errorf("generate docs: %d %s", code, errOut)
	}
	_ = out
}
