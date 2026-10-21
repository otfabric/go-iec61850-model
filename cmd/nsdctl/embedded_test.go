// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
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
