// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The model and catalog commands use the embedded official catalog
// without --source.
func TestEmbeddedCatalog(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
		want []string
	}{
		{"profiles", []string{"catalog", "profiles"}, exitOK, []string{"default: ", "  IEC 61850-7-420:2019A4", "extended: ", "  (Tr)IEC 61850-90-9:2018A3"}},
		{"default DBAT", []string{"model", "show", "DBAT"}, exitOK, []string{"logical node class DBAT (IEC 61850-7-420:2019A4)"}},
		{"default has no 90-9", []string{"model", "show", "DBAT", "--ns", "90-9"}, exitUsage, nil},
		{"extended DBAT ambiguous", []string{"-p", "extended", "model", "show", "DBAT"}, exitUsage, nil},
		{"extended DBAT qualified", []string{"-p", "extended", "model", "show", "DBAT", "--ns", "IEC_TR_61850-90-9"}, exitOK, []string{"logical node class DBAT ((Tr)IEC 61850-90-9:2018A3)", "BatTestRsl"}},
		{"extended DBAT 7-420", []string{"-p", "extended", "model", "show", "DBAT", "--ns", "IEC 61850-7-420"}, exitOK, []string{"logical node class DBAT (IEC 61850-7-420:2019A4)", "BatVNom"}},
		{"extended path ambiguous", []string{"-p", "extended", "model", "show", "DBAT.BatSt"}, exitUsage, nil},
		{"extended path qualified", []string{"-p", "extended", "model", "leaves", "DBAT.BatSt", "--ns", "7-420"}, exitOK, []string{"DBAT.BatSt.stVal"}},
		{"cdc in namespace", []string{"model", "show", "BCR", "--ns", "7-3"}, exitOK, []string{"CDC BCR (IEC 61850-7-3:2007B6)"}},
		{"ca in namespace", []string{"model", "show", "Vector", "--ns", "7-3"}, exitOK, []string{"constructed attribute Vector (IEC 61850-7-3:2007B6)"}},
		{"enum in namespace", []string{"model", "show", "BehaviourModeKind", "--ns", "7-4"}, exitOK, []string{"enumeration BehaviourModeKind (IEC 61850-7-4:2007B6)"}},
		{"basic type from dependency", []string{"model", "show", "FLOAT32", "--ns", "7-3"}, exitOK, []string{"basic type FLOAT32 (IEC 61850-7-2:2007B6)"}},
		{"not in namespace", []string{"model", "show", "BCR", "--ns", "7-420"}, exitUsage, nil},
		{"unknown namespace", []string{"model", "show", "BCR", "--ns", "7-99"}, exitUsage, nil},
		{"extended enum ambiguous", []string{"-p", "extended", "model", "show", "BatteryTypeKind"}, exitUsage, nil},
		{"extended enum qualified", []string{"-p", "extended", "model", "show", "BatteryTypeKind", "--ns", "90-9"}, exitOK, []string{"enumeration BatteryTypeKind ((Tr)IEC 61850-90-9:2018A3)"}},
		{"measurements in namespace", []string{"model", "measurements", "MMXU", "--ns", "7-4"}, exitOK, []string{"MMXU.TotW"}},
		{"measurements unknown namespace", []string{"model", "measurements", "MMXU", "--ns", "7-99"}, exitUsage, nil},
		{"describe in namespace", []string{"model", "describe", "MMXU.Hz.mag.f", "--ns", "7-4"}, exitOK, []string{"quantity     frequency"}},
		{"describe ambiguous", []string{"-p", "extended", "model", "describe", "DBAT.BatSt"}, exitUsage, nil},
		{"describe unknown namespace", []string{"model", "describe", "MMXU.Hz", "--ns", "7-99"}, exitUsage, nil},
		{"leaves unknown namespace", []string{"model", "leaves", "MMXU", "--ns", "7-99"}, exitUsage, nil},
		{"path in namespace", []string{"model", "show", "MMXU.Hz.mag.f", "--ns", "IEC 61850-7-4:2007B6"}, exitOK, []string{"→ FLOAT32"}},
		{"overlaps json all", []string{"--json", "catalog", "overlaps", "-p", "default", "--class", "identical"}, exitOK, []string{"[]"}},
		{"profiles json", []string{"--json", "catalog", "profiles"}, exitOK, []string{`"name": "extended"`}},
		{"profiles from sources", []string{"-s", "../../testdata/nsd", "catalog", "profiles"}, exitOK, []string{"(the sources do not carry exactly one release of each part"}},
		{"overlaps sources", []string{"-s", "../../testdata/nsd", "catalog", "overlaps"}, exitOK, nil},
		{"overlaps bad profile", []string{"catalog", "overlaps", "-p", "nope"}, exitUsage, nil},
		{"overlaps", []string{"catalog", "overlaps", "--left", "IEC_61850-7-420", "--right", "IEC_TR_61850-90-9"}, exitOK, []string{
			"ln    DBAT ", "conflicting", "StorageUnitLN", "identical", "only in (Tr)IEC 61850-90-9:2018A3: BatTestRsl",
		}},
		{"overlaps conflicting", []string{"catalog", "overlaps", "-p", "default", "--class", "conflicting"}, exitOK, []string{"FSCH", "IEC 61850-7-4:2007B6", "IEC 61850-7-410:2024A4"}},
		{"overlaps one side", []string{"catalog", "overlaps", "--left", "7-420"}, exitUsage, nil},
		{"overlaps bad class", []string{"catalog", "overlaps", "--class", "similar"}, exitUsage, nil},
		{"overlaps unknown", []string{"catalog", "overlaps", "--left", "7-999", "--right", "7-420"}, exitUsage, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, errOut, code := nsdctl(t, tc.args...)
			if code != tc.code {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", code, tc.code, out, errOut)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q:\n%s", w, out)
				}
			}
		})
	}
	if out, _, _ := nsdctl(t, "catalog", "overlaps", "--left", "7-4", "--right", "7-3"); !strings.Contains(out, "no overlapping names") {
		t.Errorf("no overlaps: %s", out)
	}
	_, errOut, _ := nsdctl(t, "-p", "extended", "model", "show", "DBAT")
	if !strings.Contains(errOut, "ambiguous") || !strings.Contains(errOut, "--ns") {
		t.Errorf("ambiguity message: %s", errOut)
	}
}

func TestOverlapsJSON(t *testing.T) {
	out, errOut, code := nsdctl(t, "--json", "catalog", "overlaps", "--left", "7-420", "--right", "90-9")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var got []struct {
		Kind, Name, Class string
		OnlyRight         []string
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	classes := map[string]string{}
	for _, o := range got {
		if o.Kind == "ln" {
			classes[o.Name] = o.Class
		}
	}
	for name, want := range map[string]string{"DBAT": "conflicting", "DSTO": "conflicting", "SBAT": "compatible-extension", "StorageUnitLN": "identical"} {
		if classes[name] != want {
			t.Errorf("%s: %q, want %q", name, classes[name], want)
		}
	}
}

func TestGenerateCatalog(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "index.json")
	if err := os.WriteFile(index, []byte(`{"schemaVersion":1,"profiles":[{"name":"core","description":"Core","parts":["7-3","7-4"]},{"name":"wide","inherits":"core","parts":["7-2"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "embedded")
	o, e, code := nsdctl(t, withProfile("generate", "catalog", "-o", out, "--package", "embedded", "--index", index)...)
	if code != exitOK || !strings.Contains(o, "catalog_gen.go") {
		t.Fatalf("generate catalog: %d %s %s", code, o, e)
	}
	src, err := os.ReadFile(filepath.Join(out, "catalog_gen.go"))
	if err != nil || !strings.Contains(string(src), `Name:        "wide"`) || !strings.Contains(string(src), `"7-3", "7-4", "7-2"`) {
		t.Errorf("catalog_gen.go: %v\n%s", err, src)
	}
	if o, _, code := nsdctl(t, withProfile("generate", "catalog", "-o", out, "--package", "embedded", "--index", index, "--check")...); code != exitOK || !strings.Contains(o, "up to date") {
		t.Errorf("check: %d %s", code, o)
	}
	if _, _, code := nsdctl(t, withProfile("generate", "catalog", "-o", filepath.Join(dir, "bare"), "--package", "embedded", "--index", "")...); code != exitOK {
		t.Errorf("without profiles: %d", code)
	}
	if _, _, code := nsdctl(t, withProfile("generate", "catalog", "-o", filepath.Join(dir, "x"), "--index", filepath.Join(dir, "missing.json"))...); code != exitSource {
		t.Errorf("missing index: %d", code)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"profiles":[{"name":"a","inherits":"nope","parts":["7-4"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := nsdctl(t, withProfile("generate", "catalog", "-o", filepath.Join(dir, "y"), "--index", bad)...); code == exitOK {
		t.Error("profile inheriting an unknown profile accepted")
	}
	if _, _, code := nsdctl(t, withProfile("generate", "catalog", "-o", filepath.Join(dir, "z"), "--index", index, "--package", "not valid")...); code == exitOK {
		t.Error("invalid package name accepted")
	}
}

func TestProfilePartsFromRegistry(t *testing.T) {
	saved := registryPath
	t.Cleanup(func() { registryPath = saved })

	registryPath = "../../sources/index.json"
	if parts, ok := profileParts("extended"); !ok || len(parts) != 12 || parts[0] != "7-2" {
		t.Errorf("registry profile: %v %v", parts, ok)
	}
	if _, ok := profileParts("nope"); ok {
		t.Error("unknown registry profile")
	}
	registryPath = filepath.Join(t.TempDir(), "missing.json")
	if parts, ok := profileParts("default"); !ok || len(parts) != 5 {
		t.Errorf("embedded profile: %v %v", parts, ok)
	}
	if _, ok := profileParts("nope"); ok {
		t.Error("unknown embedded profile")
	}
}
