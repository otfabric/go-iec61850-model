// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/internal/generator"
	"github.com/otfabric/go-iec61850-model/model"
)

func nsdctl(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

var profileArgs = []string{"-s", fixtures.Dir(), "-p", "IEC 61850-7-4:2007B1", "-p", "OTF TEST-EXT"}

func withProfile(args ...string) []string {
	return append(append([]string(nil), profileArgs...), args...)
}

func TestCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
		want []string
	}{
		{"version", []string{"version"}, exitOK, []string{"nsdctl dev"}},
		{"inspect", []string{"inspect", fixtures.Dir()}, exitOK, []string{"IEC 61850-7-4:2007B2", "doc       IEC 61850-7-4:2007B1 (en, 6 texts)"}},
		{"inspect file", []string{"inspect", fixtures.Path(fixtures.NS73)}, exitOK, []string{"sha256", "IEC 61850-7-3:2007B1"}},
		{"inspect via --source", []string{"-s", fixtures.Path(fixtures.NS72), "inspect"}, exitOK, []string{"IEC 61850-7-2:2007B1"}},
		{"validate ambiguous", []string{"validate", fixtures.Dir()}, exitValidation, []string{"is ambiguous", "1 errors"}},
		{"validate profile", withProfile("validate"), exitOK, []string{"4 namespaces, 0 errors"}},
		{"deps", []string{"deps", fixtures.Dir()}, exitValidation, []string{"ambiguous", "IEC 61850-7-4:2007B1, IEC 61850-7-4:2007B2"}},
		{"deps ok", []string{"dependencies", fixtures.Path(fixtures.NS72)}, exitOK, []string{"NAMESPACE"}},
		{"diff", []string{"diff", fixtures.Path(fixtures.NS74), fixtures.Path(fixtures.NS74R2)}, exitOK, []string{"added    DataObject MMXU.TotPF added", "4 changes"}},
		{"diff exit code", []string{"diff", "--exit-code", fixtures.Path(fixtures.NS74), fixtures.Path(fixtures.NS74R2)}, exitDifferent, nil},
		{"diff same", []string{"diff", "--exit-code", fixtures.Path(fixtures.NS74), fixtures.Path(fixtures.NS74)}, exitOK, []string{"0 changes"}},
		{"show class", withProfile("model", "show", "MMXU"), exitOK, []string{"group        M Metering and measurement", "PNV          WYE  O         MMXU          voltage per phase, line-neutral"}},
		{"show path", withProfile("model", "show", "LD0/MMXU1.PNV.phsA.cVal.mag.f[MX]"), exitOK, []string{"→ FLOAT32 (BASIC) [MX]", "reference    line-neutral", "rule mmxu.pnv"}},
		{"show cdc", withProfile("model", "show", "ENS"), exitOK, []string{"CDC ENS", "(ENUMERATED from data object)"}},
		{"show ca", withProfile("model", "show", "Vector"), exitOK, []string{"constructed attribute Vector", "AnalogueValue"}},
		{"show enum", withProfile("model", "show", "TestStateKind"), exitOK, []string{"enumeration TestStateKind", "running"}},
		{"show basic", withProfile("model", "show", "FLOAT32"), exitOK, []string{"basic type FLOAT32"}},
		{"show unknown", withProfile("model", "show", "NOPE"), exitUsage, nil},
		{"show bad path", withProfile("model", "show", "MMXU.Nope"), exitUsage, nil},
		{"list", withProfile("model", "list"), exitOK, []string{"DTST", "Distributed energy resources (base StatisticsLN)"}},
		{"list abstract", withProfile("model", "list", "--kind", "abstract"), exitOK, []string{"DomainLN"}},
		{"list cdc", withProfile("model", "list", "--kind", "cdc"), exitOK, []string{"WYE", "6 members"}},
		{"list ca", withProfile("model", "list", "--kind", "ca"), exitOK, []string{"Vector"}},
		{"list enum", withProfile("model", "list", "--kind", "enum"), exitOK, []string{"HealthKind", "3 literals"}},
		{"list basic", withProfile("model", "list", "--kind", "basic"), exitOK, []string{"INT64"}},
		{"list bad kind", withProfile("model", "list", "--kind", "x"), exitUsage, nil},
		{"search", withProfile("model", "search", "--cdc", "MV", "--kind", "do"), exitOK, []string{"MMXU.Hz", "DTST.OutW"}},
		{"search fc", withProfile("model", "search", "--fc", "CF"), exitOK, []string{"MV.units"}},
		{"search group", withProfile("model", "search", "--group", "D"), exitOK, []string{"DTST"}},
		{"search bad group", withProfile("model", "search", "--group", "dd"), exitUsage, nil},
		{"search bad kind", withProfile("model", "search", "--kind", "zz"), exitUsage, nil},
		{"describe", withProfile("model", "describe", "MMXU.PPV.phsAB"), exitOK, []string{"quantity     voltage", "phase        AB", "line-line"}},
		{"describe unmapped", withProfile("model", "describe", "MMXU.Beh"), exitUsage, nil},
		{"measurements", withProfile("model", "measurements", "MMTR"), exitOK, []string{"MMTR.SupWh", "supply", "mmtr.dmdwh"}},
		{"measurements unknown", withProfile("model", "measurements", "XXXX"), exitUsage, nil},
		{"leaves", withProfile("model", "leaves", "MMXU.Hz", "--fc", "MX"), exitOK, []string{"MMXU.Hz.mag.f", "FLOAT32"}},
		{"leaves unknown", withProfile("model", "leaves", "MMXU.Nope"), exitUsage, nil},
		{"no source", []string{"model", "list"}, exitUsage, nil},
		{"no profile", []string{"-s", fixtures.Dir(), "model", "list"}, exitUsage, nil},
		{"bad profile id", []string{"-s", fixtures.Dir(), "-p", ":x", "model", "list"}, exitUsage, nil},
		{"unknown profile", []string{"-s", fixtures.Dir(), "-p", "IEC 61850-7-9", "model", "list"}, exitUsage, nil},
		{"ambiguous dependency", []string{"-s", fixtures.Dir(), "-p", "OTF TEST-EXT", "model", "list"}, exitValidation, nil},
		{"missing source", []string{"inspect", "/no/such/file.zip"}, exitSource, nil},
		{"unknown flag", []string{"inspect", "--nope"}, exitUsage, nil},
		{"diff args", []string{"diff", "a"}, exitUsage, nil},
		{"sources check", []string{"sources", "check", "--index", "../../sources/index.json", "--lock", "../../sources/nsd.lock.json"}, exitOK, []string{"0 locked namespaces", "not verified yet"}},
		{"sources check missing", []string{"sources", "check", "--index", "nope.json"}, exitSource, nil},
		{"generate without out", withProfile("generate", "go"), exitUsage, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, errOut, code := nsdctl(t, tc.args...)
			if code != tc.code {
				t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.code, out, errOut)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestJSONOutput(t *testing.T) {
	for _, args := range [][]string{
		{"inspect", fixtures.Dir()},
		withProfile("validate"),
		{"deps", fixtures.Path(fixtures.NS72)},
		{"diff", fixtures.Path(fixtures.NS74), fixtures.Path(fixtures.NS74R2)},
		{"diff", fixtures.Path(fixtures.NS74), fixtures.Path(fixtures.NS74)},
		withProfile("model", "show", "MMXU"),
		withProfile("model", "show", "MMXU.Hz.mag.f"),
		withProfile("model", "show", "WYE"),
		withProfile("model", "show", "Unit"),
		withProfile("model", "show", "HealthKind"),
		withProfile("model", "show", "BOOLEAN"),
		withProfile("model", "list", "--kind", "cdc"),
		withProfile("model", "search", "--name", "zzz"),
		withProfile("model", "describe", "MMXU.Hz"),
		withProfile("model", "measurements", "GGIO"),
		withProfile("model", "leaves", "MMXU.Hz"),
		{"sources", "check", "--index", "../../sources/index.json", "--lock", "../../sources/nsd.lock.json"},
	} {
		out, errOut, code := nsdctl(t, append([]string{"--json"}, args...)...)
		if code != exitOK {
			t.Errorf("%v: exit %d: %s", args, code, errOut)
			continue
		}
		if !json.Valid([]byte(out)) {
			t.Errorf("%v: invalid JSON:\n%s", args, out)
		}
	}
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	for _, kind := range []string{"go", "docs", "json"} {
		out := filepath.Join(dir, kind)
		if o, e, code := nsdctl(t, withProfile("generate", kind, "-o", out)...); code != exitOK || !strings.Contains(o, "local-only") {
			t.Fatalf("generate %s: %d %s %s", kind, code, o, e)
		}
		if o, _, code := nsdctl(t, withProfile("generate", kind, "-o", out, "--check")...); code != exitOK || !strings.Contains(o, "up to date") {
			t.Errorf("check %s: %d %s", kind, code, o)
		}
	}
	goOut := filepath.Join(dir, "go")
	if o, _, code := nsdctl(t, withProfile("--json", "generate", "go", "-o", goOut, "--package", "other", "--check")...); code != exitDifferent || !strings.Contains(o, `"changed"`) {
		t.Errorf("check with another package: %d %s", code, o)
	}
	if o, _, code := nsdctl(t, withProfile("--json", "generate", "go", "-o", goOut, "--check")...); code != exitOK || strings.TrimSpace(o) != "[]" {
		t.Errorf("json check: %d %s", code, o)
	}
	if _, _, code := nsdctl(t, withProfile("--json", "generate", "go", "-o", filepath.Join(dir, "g2"))...); code != exitOK {
		t.Error("json generate")
	}
	if err := os.WriteFile(filepath.Join(goOut, generator.GoFileName), []byte("package mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, e, code := nsdctl(t, withProfile("generate", "go", "-o", goOut)...); code != exitUsage || !strings.Contains(e, "--force") {
		t.Errorf("overwrite refused: %d %s", code, e)
	}
	if _, _, code := nsdctl(t, withProfile("generate", "go", "-o", goOut, "--force")...); code != exitOK {
		t.Error("forced overwrite")
	}
	if _, e, code := nsdctl(t, withProfile("generate", "go", "-o", goOut, "--publish")...); code != exitPolicy || !strings.Contains(e, "license policy") {
		t.Errorf("publish without lock: %d %s", code, e)
	}

	// A lock that permits every namespace allows publishing.
	var entries []string
	pkgOut, _, _ := nsdctl(t, withProfile("--json", "inspect")...)
	var sums []struct {
		Namespaces []struct{ ID, SHA256 string } `json:"namespaces"`
	}
	if err := json.Unmarshal([]byte(pkgOut), &sums); err != nil {
		t.Fatal(err)
	}
	for _, n := range sums[0].Namespaces {
		entries = append(entries, fmt.Sprintf(`{"standard":"x","namespace":%q,"status":"adopted","sha256":%q,"redistribution":"permitted"}`, n.ID, n.SHA256))
	}
	lock := filepath.Join(dir, "lock.json")
	if err := os.WriteFile(lock, []byte(`{"schemaVersion":1,"sources":[`+strings.Join(entries, ",")+`]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	pub := filepath.Join(dir, "pub")
	if o, e, code := nsdctl(t, withProfile("generate", "go", "-o", pub, "--publish", "--lock", lock)...); code != exitOK || strings.Contains(o, "local-only") {
		t.Fatalf("publish: %d %s %s", code, o, e)
	}
	if _, err := os.Stat(filepath.Join(pub, "LOCAL-ONLY.md")); !os.IsNotExist(err) {
		t.Error("published output has a local-only notice")
	}
	if _, _, code := nsdctl(t, withProfile("generate", "go", "-o", pub, "--publish", "--lock", "missing.json")...); code != exitSource {
		t.Error("missing lock")
	}
	// Verify packages against the lock.
	if _, _, code := nsdctl(t, "sources", "check", "--index", "../../sources/index.json", "--lock", lock, "--packages", fixtures.Dir()); code != exitDifferent {
		t.Errorf("verify: %d", code)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"schemaVersion":1,"sources":[{"namespace":"x","status":"?"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := nsdctl(t, "sources", "check", "--index", "../../sources/index.json", "--lock", bad); code != exitValidation {
		t.Errorf("invalid lock: %d", code)
	}
	if _, _, code := nsdctl(t, "sources", "check", "--index", "../../sources/index.json", "--lock", "../../sources/nsd.lock.json", "--packages", "/no/dir"); code != exitSource {
		t.Errorf("missing packages dir: %d", code)
	}
}

func TestExitCode(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
	}{
		{nil, exitOK},
		{&model.ValidationError{}, exitValidation},
		{&model.DependencyError{Err: model.ErrMissingDependency}, exitValidation},
		{model.ErrUnsafeArchive, exitSource},
		{generator.ErrPolicyRefused, exitPolicy},
		{errors.New("other"), exitUsage},
	} {
		if got := exitCode(tc.err); got != tc.code {
			t.Errorf("exitCode(%v) = %d, want %d", tc.err, got, tc.code)
		}
	}
	version, tag, commit, buildDate = "v1.2.3", "v1.2.3", "0123456789abcdef", "2026-10-21"
	defer func() { version, tag, commit, buildDate = "dev", "", "", "" }()
	if got := versionString(); got != "nsdctl v1.2.3 (v1.2.3) commit 01234567 built 2026-10-21" {
		t.Errorf("versionString = %s", got)
	}
}

func TestStrictAndLanguage(t *testing.T) {
	dir := t.TempDir()
	doc := `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1" future="x"/>`
	p := filepath.Join(dir, "t.nsd")
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _, code := nsdctl(t, "inspect", p); code != exitOK || !strings.Contains(out, "unknown-attribute") {
		t.Errorf("warning shown: %d %s", code, out)
	}
	if _, _, code := nsdctl(t, "--strict", "inspect", p); code != exitValidation {
		t.Errorf("strict: %d", code)
	}
	if out, _, _ := nsdctl(t, withProfile("--lang", "", "model", "show", "MMXU")...); strings.Contains(out, "title") {
		t.Error("--lang '' must not apply documentation")
	}
}
