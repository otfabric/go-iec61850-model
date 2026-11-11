// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/nsd"
)

// writeDocSet writes a doc set with OTFabric-authored texts for the
// fixture and the official MMXU class and returns its path.
func writeDocSet(t *testing.T, lang string) string {
	t.Helper()
	set := nsd.NewDocSet(lang)
	for key, text := range map[string]string{
		"LN:MMXU#title":        "otf measuring unit",
		"LN:MMXU#desc":         "otf measuring unit, long form",
		"DO:MMXU.Hz#title":     "otf frequency",
		"DA:MV.mag#desc":       "otf magnitude of a measured value",
		"DO:MMXU.TotVAr#title": "otf total reactive power",
	} {
		if err := set.Set(key, text); err != nil {
			t.Fatal(err)
		}
	}
	data, err := nsd.EncodeDocSets(set)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "docs.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDocsFlag(t *testing.T) {
	docs := writeDocSet(t, "en")
	tests := []struct {
		name string
		args []string
		code int
		want []string
		not  []string
	}{
		{"official show", []string{"--docs", docs, "model", "show", "MMXU"}, exitOK,
			[]string{"title        otf measuring unit", "description  otf measuring unit, long form", "TEXT", "otf frequency"}, nil},
		{"official path", []string{"--docs", docs, "model", "show", "MMXU.Hz.mag"}, exitOK,
			[]string{"otf magnitude of a measured value"}, nil},
		{"official search text", []string{"--docs", docs, "model", "search", "--text", "OTF Frequency"}, exitOK,
			[]string{"MMXU.Hz", "otf frequency"}, []string{"ln    MMXU "}},
		{"json description", []string{"--json", "--docs", docs, "model", "show", "MMXU.Hz.mag"}, exitOK,
			[]string{`"description": "otf magnitude of a measured value"`}, nil},
		{"lang off", []string{"--lang", "", "--docs", docs, "model", "show", "MMXU"}, exitOK,
			nil, []string{"otf measuring unit"}},
		{"source fallback", withProfile("--docs", docs, "model", "show", "MMXU.Hz.mag"), exitOK,
			[]string{"otf magnitude of a measured value"}, nil},
		{"wrong language", []string{"--lang", "de", "--docs", docs, "model", "show", "MMXU"}, exitUsage,
			nil, nil},
		{"wrong language source", withProfile("--lang", "de", "--docs", docs, "model", "show", "MMXU"), exitUsage,
			nil, nil},
		{"missing file", []string{"--docs", filepath.Join(t.TempDir(), "none.json"), "model", "show", "MMXU"}, exitInvalidInput,
			nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errOut, code := nsdctl(t, tt.args...)
			if code != tt.code {
				t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.code, out, errOut)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q:\n%s", w, out)
				}
			}
			for _, n := range tt.not {
				if strings.Contains(out, n) {
					t.Errorf("output has %q:\n%s", n, out)
				}
			}
		})
	}
}

func TestDocsFlagGenerateDocs(t *testing.T) {
	docs := writeDocSet(t, "en")
	out := t.TempDir()
	if o, e, code := nsdctl(t, withProfile("--docs", docs, "generate", "docs", "-o", out, "--include-doc-text")...); code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, o, e)
	}
	found := false
	err := filepath.WalkDir(out, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		found = found || strings.Contains(string(data), "| otf total reactive power |")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("generated docs lack the --docs text")
	}
}
