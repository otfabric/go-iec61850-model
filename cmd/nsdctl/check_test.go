// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/presence"
)

const (
	sclFixture  = "testdata/presence/device.scd"
	jsonFixture = "testdata/presence/xcbr.json"
)

func TestModelCheck(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
		want []string
		not  []string
	}{
		{"flags", []string{"model", "check", "CSWI", "--present", "Beh,Pos", "--value", "Pos.ctlModel=sbo-with-enhanced-security"},
			exitFindings, []string{"Pos.stSeld", "MOsbo", "Pos.operTimeout", "violated", "CSWI: "}, nil},
		{"facts and context", []string{"model", "check", "CSWI", "--present", "Beh", "--fact", "MOcond(1)=true", "--fact", "MONamPlt=false",
			"--ld-ns", "IEC 61850-7-4:2007B", "--ln-ns", "IEC 61850-7-4:2007B", "--ctl-model", "direct-with-normal-security", "--root-ld", "false"},
			exitFindings, []string{"Mir ", "fact MOcond(1)", "Pos "}, []string{"NamPlt "}},
		{"json instance", []string{"model", "check", "--instance", jsonFixture},
			exitFindings, []string{"Pos.sboTimeout", "XCBR: 63 elements", "2 violated"}, nil},
		{"json all", []string{"model", "check", "--instance", jsonFixture, "--all"},
			exitFindings, []string{"Pos.stVal", "satisfied"}, nil},
		{"scl ln", []string{"model", "check", "--scl", sclFixture, "--ln", "CTRL/Q0XCBR1"},
			exitOK, []string{"XCBR: 160 elements", "0 violated"}, []string{"Pos.sboTimeout"}},
		{"scl ln with ied", []string{"model", "check", "--scl", sclFixture, "--ln", "TESTCTRL/LLN0", "--root-ld", "true"},
			exitFindings, []string{"Health", "Mod", "MOrootLD"}, []string{"NamPlt.ldNs "}},
		{"scl ln type", []string{"model", "check", "--scl", sclFixture, "--ln-type", "T_XCBR"},
			exitFindings, []string{"Pos.sboTimeout", "ctlModel sbo-with-normal-security"}, nil},
		{"unknown names", []string{"model", "check", "MMXU", "--present", "Beh,VendorX"},
			exitOK, []string{"not in the model: VendorX"}, nil},
		{"cdc", []string{"model", "check", "MV", "--present", "mag,q,t,mag.i"},
			exitFindings, []string{"sVC", "MFscaledAV"}, nil},
		{"no class", []string{"model", "check", "--present", "Beh"}, exitUsage, []string{"name the class"}, nil},
		{"two sources", []string{"model", "check", "X", "--present", "a", "--instance", jsonFixture}, exitUsage, []string{"one of"}, nil},
		{"ln without scl", []string{"model", "check", "X", "--ln", "A/B"}, exitUsage, []string{"need --scl"}, nil},
		{"scl without ln", []string{"model", "check", "--scl", sclFixture}, exitUsage, []string{"one of --ln"}, nil},
		{"bad value", []string{"model", "check", "X", "--value", "novalue"}, exitUsage, []string{"NAME=VALUE"}, nil},
		{"bad fact", []string{"model", "check", "X", "--fact", "MOcond(1)=maybe"}, exitUsage, []string{"COND=true|false"}, nil},
		{"bad root", []string{"model", "check", "X", "--root-ld", "perhaps"}, exitUsage, []string{"--root-ld"}, nil},
		{"unknown class", []string{"model", "check", "NOPE"}, exitUsage, []string{"no logical node class"}, nil},
		{"missing file", []string{"model", "check", "--instance", "testdata/presence/none.json"}, exitInvalidInput, nil, nil},
		{"missing scl", []string{"model", "check", "--scl", "testdata/presence/none.scd", "--ln-type", "X"}, exitInvalidInput, nil, nil},
		{"bad json", []string{"model", "check", "--instance", sclFixture}, exitInvalidInput, []string{"invalid instance"}, nil},
		{"bad scl", []string{"model", "check", "--scl", jsonFixture, "--ln-type", "X"}, exitInvalidInput, []string{"SCL"}, nil},
		{"unknown ln type", []string{"model", "check", "--scl", sclFixture, "--ln-type", "NOPE"}, exitInvalidInput, []string{"no LNodeType"}, nil},
		{"unknown ln", []string{"model", "check", "--scl", sclFixture, "--ln", "CTRL/Q9XCBR1"}, exitInvalidInput, []string{"no logical node"}, nil},
		{"malformed ln", []string{"model", "check", "--scl", sclFixture, "--ln", "Q0XCBR1"}, exitInvalidInput, []string{"not LD/LN"}, nil},
		{"ns", []string{"model", "--ns", "7-4", "check", "MMXU", "--present", "Beh"}, exitOK, []string{"MMXU: "}, nil},
		{"bad ns", []string{"model", "--ns", "nope", "check", "MMXU"}, exitUsage, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, errOut, code := nsdctl(t, tc.args...)
			if code != tc.code {
				t.Fatalf("exit %d, want %d\n%s%s", code, tc.code, out, errOut)
			}
			for _, w := range tc.want {
				if !strings.Contains(out+errOut, w) {
					t.Errorf("output lacks %q:\n%s%s", w, out, errOut)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(out, n) {
					t.Errorf("output has %q:\n%s", n, out)
				}
			}
		})
	}
}

func TestModelCheckStdinAndJSON(t *testing.T) {
	data, err := os.ReadFile(jsonFixture)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--json", "model", "check", "--instance", "-"})
	cmd.SetIn(bytes.NewReader(data))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err = cmd.ExecuteContext(context.Background())
	if exitCode(err) != exitFindings {
		t.Fatalf("exit %d: %v", exitCode(err), err)
	}
	var rep struct {
		Class    string `json:"class"`
		Results  []any  `json:"results"`
		Children []any  `json:"children"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil || rep.Class != "XCBR" || len(rep.Children) != 1 {
		t.Fatalf("JSON report: %v %+v\n%s", err, rep, out.String())
	}

	// SCL from stdin.
	scl, err := os.ReadFile(sclFixture)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	cmd = newRootCmd()
	cmd.SetArgs([]string{"model", "check", "--scl", "-", "--ln", "CTRL/Q0XCBR1"})
	cmd.SetIn(bytes.NewReader(scl))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("SCL stdin: %v\n%s", err, out.String())
	}
}

func TestModelCheckWriteFailure(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"model", "check", "MMXU", "--present", "VendorX"})
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	if code := exitCode(cmd.ExecuteContext(context.Background())); code != exitInternal {
		t.Errorf("exit %d, want %d", code, exitInternal)
	}
}

// Two IEDs with the same LD and LN are ambiguous without the IED name.
func TestSCLAmbiguousLN(t *testing.T) {
	data, err := os.ReadFile(sclFixture)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	ied := s[strings.Index(s, "  <IED"):strings.Index(s, "  <DataTypeTemplates>")]
	two := strings.Replace(s, ied, ied+strings.Replace(ied, `name="TEST"`, `name="OTHER"`, 1), 1)
	path := filepath.Join(t.TempDir(), "two.scd")
	if err := os.WriteFile(path, []byte(two), 0o600); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := nsdctl(t, "model", "check", "--scl", path, "--ln", "CTRL/Q0XCBR1")
	if code != exitInvalidInput || !strings.Contains(errOut, "prefix the LD with the IED name") {
		t.Errorf("exit %d: %s", code, errOut)
	}
	if _, errOut, code := nsdctl(t, "model", "check", "--scl", path, "--ln", "OTHERCTRL/Q0XCBR1"); code != exitOK {
		t.Errorf("OTHERCTRL: exit %d: %s", code, errOut)
	}
}

func TestSCLLimits(t *testing.T) {
	// A DOType that contains itself.
	loop := `<SCL><DataTypeTemplates><LNodeType id="L" lnClass="MMXU"><DO name="A" type="D"/></LNodeType>` +
		`<DOType id="D" cdc="WYE"><SDO name="phsA" type="D"/></DOType></DataTypeTemplates></SCL>`
	doc, err := readSCL(strings.NewReader(loop))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSCLIndex(doc).lnType("L"); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("DOType loop: %v", err)
	}
	daLoop := `<SCL><DataTypeTemplates><LNodeType id="L" lnClass="MMXU"><DO name="A" type="D"/></LNodeType>` +
		`<DOType id="D" cdc="MV"><DA name="mag" bType="Struct" type="S"/></DOType>` +
		`<DAType id="S"><BDA name="x" bType="Struct" type="S"/></DAType></DataTypeTemplates></SCL>`
	doc, err = readSCL(strings.NewReader(daLoop))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSCLIndex(doc).lnType("L"); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("DAType loop: %v", err)
	}
	for name, in := range map[string]string{
		"missing DOType": `<SCL><DataTypeTemplates><LNodeType id="L"><DO name="A" type="X"/></LNodeType></DataTypeTemplates></SCL>`,
		"missing SDO":    `<SCL><DataTypeTemplates><LNodeType id="L"><DO name="A" type="D"/></LNodeType><DOType id="D"><SDO name="s" type="X"/></DOType></DataTypeTemplates></SCL>`,
		"missing DAType": `<SCL><DataTypeTemplates><LNodeType id="L"><DO name="A" type="D"/></LNodeType><DOType id="D"><DA name="s" bType="Struct" type="X"/></DOType></DataTypeTemplates></SCL>`,
	} {
		doc, err := readSCL(strings.NewReader(in))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := newSCLIndex(doc).lnType("L"); !errors.Is(err, model.ErrInvalidInstance) {
			t.Errorf("%s: %v", name, err)
		}
	}
	big := strings.NewReader("<SCL>" + strings.Repeat(" ", maxSCLBytes) + "</SCL>")
	if _, err := readSCL(big); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("big SCL: %v", err)
	}
	if _, err := readSCL(failingReader{}); !errors.Is(err, model.ErrInvalidInstance) {
		t.Errorf("failing reader: %v", err)
	}
	// An LN whose type is missing.
	doc, err = readSCL(strings.NewReader(`<SCL><IED name="I"><AccessPoint><Server><LDevice inst="LD"><LN lnClass="MMXU" inst="1" lnType="NONE"/></LDevice></Server></AccessPoint></IED></SCL>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSCLIndex(doc).ln("LD/MMXU1"); !errors.Is(err, model.ErrInvalidInstance) {
		t.Errorf("missing lnType: %v", err)
	}
}

// nestedSCL returns data type templates with LNodeType L, whose data
// object has a DOType of two SDOs, each of the next DOType, depth times;
// the last DOType has one attribute.
func nestedSCL(depth int) string {
	var b strings.Builder
	b.WriteString(`<SCL><DataTypeTemplates><LNodeType id="L" lnClass="MMXU"><DO name="A" type="D1"/></LNodeType>`)
	for i := 1; i < depth; i++ {
		fmt.Fprintf(&b, `<DOType id="D%d" cdc="WYE"><SDO name="x" type="D%d"/><SDO name="y" type="D%d"/></DOType>`, i, i+1, i+1)
	}
	fmt.Fprintf(&b, `<DOType id="D%d" cdc="MV"><DA name="q" bType="Quality"/></DOType></DataTypeTemplates></SCL>`, depth)
	return b.String()
}

// names counts the names and values of an instance, as the limit of
// the JSON form does.
func names(inst *presence.Instance) int {
	n := len(inst.Present) + len(inst.Values)
	for _, c := range inst.Children {
		n += names(c)
	}
	return n
}

// An instance built from SCL templates has at most as many names as the
// JSON form of an instance may have: every reference to a type builds
// its members again, so a few types nested a few levels deep multiply.
func TestSCLInstanceNameLimit(t *testing.T) {
	build := func(doc string) (*presence.Instance, error) {
		t.Helper()
		d, err := readSCL(strings.NewReader(doc))
		if err != nil {
			t.Fatal(err)
		}
		return newSCLIndex(d).lnType("L")
	}
	// Depth d has 3*2^(d-1)-1 names: 49 151 at 15, 98 303 at 16, 196 607 at 17.
	for depth, want := range map[int]int{1: 2, 2: 5, 3: 11, 15: 49151, 16: 98303} {
		inst, err := build(nestedSCL(depth))
		if err != nil || names(inst) != want {
			t.Errorf("depth %d: %d names, error %v, want %d", depth, names(inst), err, want)
		}
	}
	for _, depth := range []int{17, 20, 30} {
		doc := nestedSCL(depth)
		d, err := readSCL(strings.NewReader(doc))
		if err != nil {
			t.Fatal(err)
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		inst, err := newSCLIndex(d).lnType("L")
		runtime.ReadMemStats(&after)
		if inst != nil || !errors.Is(err, model.ErrLimitExceeded) || !errors.Is(err, model.ErrInvalidInstance) || !strings.Contains(err.Error(), "more than 100000 names") {
			t.Errorf("depth %d: error = %v, want ErrLimitExceeded", depth, err)
		}
		// What is built before the limit is reached is the same at every
		// depth: about 30 MiB here for 100 000 names.
		if got := after.TotalAlloc - before.TotalAlloc; got > 64<<20 {
			t.Errorf("depth %d, a file of %d bytes: %d MiB allocated before the limit, budget 64 MiB", depth, len(doc), got>>20)
		}
	}

	// At the boundary: a data object with n attributes has n+1 names; an
	// attribute with a value the conditions read counts twice.
	flat := func(n int, extra string) string {
		return `<SCL><DataTypeTemplates><LNodeType id="L" lnClass="MMXU"><DO name="A" type="D"/></LNodeType><DOType id="D" cdc="MV">` +
			strings.Repeat(`<DA name="a" bType="INT32"/>`, n) + extra + `</DOType></DataTypeTemplates></SCL>`
	}
	const value = `<DA name="ctlModel" bType="Enum"><Val>status-only</Val></DA>`
	for _, tc := range []struct {
		n     int
		extra string
		ok    bool
	}{
		{presence.MaxInstanceNames - 2, "", true},
		{presence.MaxInstanceNames - 1, "", true},
		{presence.MaxInstanceNames, "", false},
		{presence.MaxInstanceNames - 3, value, true},
		{presence.MaxInstanceNames - 2, value, false},
	} {
		inst, err := build(flat(tc.n, tc.extra))
		switch {
		case tc.ok && (err != nil || names(inst) > presence.MaxInstanceNames):
			t.Errorf("%d attributes%s: error = %v", tc.n, tc.extra, err)
		case !tc.ok && !errors.Is(err, model.ErrLimitExceeded):
			t.Errorf("%d attributes%s: error = %v, want ErrLimitExceeded", tc.n, tc.extra, err)
		}
	}

	// The limit is per instance: a second one starts from zero.
	d, err := readSCL(strings.NewReader(nestedSCL(16)))
	if err != nil {
		t.Fatal(err)
	}
	ix := newSCLIndex(d)
	for range 2 {
		if _, err := ix.lnType("L"); err != nil {
			t.Errorf("a second instance of 98 303 names: %v", err)
		}
	}

	// Through the command: exit code 3, as for the other limits of the input.
	path := filepath.Join(t.TempDir(), "nested.scd")
	if err := os.WriteFile(path, []byte(nestedSCL(24)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := nsdctl(t, "model", "check", "--scl", path, "--ln-type", "L"); code != exitInvalidInput || !strings.Contains(errOut, "more than 100000 names") {
		t.Errorf("exit %d: %s", code, errOut)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestValidatePresence(t *testing.T) {
	out, errOut, code := nsdctl(t, "validate", "--presence", fixtures.Dir())
	if code != exitOK {
		t.Fatalf("exit %d: %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "namespaces") {
		t.Errorf("output: %s", out)
	}
}

func FuzzReadSCL(f *testing.F) {
	data, err := os.ReadFile(sclFixture)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data, "CTRL/Q0XCBR1", "T_XCBR")
	f.Add([]byte(`<SCL><DataTypeTemplates><LNodeType id="L"><DO name="A" type="D"/></LNodeType><DOType id="D"><SDO name="s" type="D"/></DOType></DataTypeTemplates></SCL>`), "a/b", "L")
	f.Add([]byte(nestedSCL(4)), "a/b", "L")
	f.Add([]byte(nestedSCL(18)), "a/b", "L")
	f.Fuzz(func(t *testing.T, data []byte, ln, lnType string) {
		doc, err := readSCL(bytes.NewReader(data))
		if err != nil {
			if !errors.Is(err, model.ErrInvalidInstance) {
				t.Fatalf("error does not wrap ErrInvalidInstance: %v", err)
			}
			return
		}
		ix := newSCLIndex(doc)
		for _, fn := range []func() (*presence.Instance, error){
			func() (*presence.Instance, error) { return ix.ln(ln) },
			func() (*presence.Instance, error) { return ix.lnType(lnType) },
		} {
			inst, err := fn()
			switch {
			case err != nil && inst != nil:
				t.Fatalf("an instance with an error: %v", err)
			case err != nil && !errors.Is(err, model.ErrInvalidInstance):
				t.Fatalf("error does not wrap ErrInvalidInstance: %v", err)
			case err == nil && inst == nil:
				t.Fatal("neither an instance nor an error")
			case err == nil && names(inst) > presence.MaxInstanceNames+len(data):
				// The templates give at most MaxInstanceNames names; the
				// values of the DOIs come from the file itself.
				t.Fatalf("an instance of %d names from a file of %d bytes", names(inst), len(data))
			}
		}
	})
}
