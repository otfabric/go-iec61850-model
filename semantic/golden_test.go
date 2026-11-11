// SPDX-License-Identifier: MIT

package semantic_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/semantic"
)

// goldenClasses are the classes whose descriptors and coverage are
// pinned in testdata/golden, with the profile that defines them.
var goldenClasses = []struct{ profile, class string }{
	{"default", "MMXU"}, {"default", "MMXN"}, {"default", "MMTR"}, {"default", "MMTN"},
	{"default", "MSQI"}, {"default", "MHAI"}, {"default", "MHAN"}, {"default", "MMDC"},
	{"default", "MMET"}, {"default", "MFLW"}, {"default", "MENV"}, {"default", "MHYD"},
	{"default", "SIML"}, {"default", "SARC"}, {"default", "STMP"}, {"default", "SPRS"}, {"default", "SVBR"},
	{"default", "ZBAT"}, {"default", "ZAXN"}, {"extended", "ZBTC"},
	{"default", "XCBR"}, {"default", "XSWI"}, {"default", "YLTC"}, {"default", "ATCC"},
	{"default", "KPMP"}, {"default", "KFAN"},
	{"default", "DGEN"}, {"default", "DLOD"}, {"default", "DSTO"}, {"default", "DBAT"}, {"default", "DINV"},
	{"default", "DPVC"}, {"default", "SBAT"}, {"default", "SINV"},
}

// TestGoldenMeasurements pins, per class, the descriptors of
// FindMeasurements and the coverage of every data object. Run `make
// golden` (UPDATE_GOLDEN=1) after an intentional rule change and review
// the diff.
func TestGoldenMeasurements(t *testing.T) {
	for _, g := range goldenClasses {
		t.Run(g.class, func(t *testing.T) {
			p := officialProfile(t, g.profile)
			var ns *model.NamespaceID
			for _, ln := range p.ListLogicalNodes() {
				// The default profile has no technical reports, so
				// every class has one definition there.
				if ln.Name == g.class && ns == nil {
					id := ln.Source
					ns = &id
				}
			}
			if ns == nil {
				t.Fatalf("%s is not in the %s profile", g.class, g.profile)
			}
			ds, err := semantic.FindMeasurementsIn(p, *ns, g.class)
			if err != nil {
				t.Fatal(err)
			}
			cov, err := semantic.Default.CoverageOf(p, *ns, g.class)
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			fmt.Fprintf(&b, "# %s (%s), profile %s\n\n", g.class, *ns, g.profile)
			fmt.Fprintf(&b, "coverage: %d measured or mapped, %d mapped, %d excluded, %d unmapped, %d not measured\n\n",
				cov.MeasuredValues(), cov.Mapped, cov.Excluded, cov.Unmapped, cov.NotMeasured)
			b.WriteString("## Measurements\n\n")
			for _, d := range ds {
				fmt.Fprintf(&b, "%s | %s", d.Path, d.Quantity)
				for _, f := range [][2]string{{"phase", string(d.Phase)}, {"reference", string(d.Reference)}, {"direction", string(d.Direction)},
					{"of", string(d.Of)}, {"sequence", string(d.Sequence)}, {"unit", d.UnitHint.String()}} {
					if f[1] != "" {
						fmt.Fprintf(&b, " | %s %s", f[0], f[1])
					}
				}
				fmt.Fprintf(&b, " | %s\n", d.Rule)
			}
			b.WriteString("\n## Data objects\n\n")
			for _, d := range cov.DataObjects {
				if d.Status == semantic.StatusNotMeasured {
					continue
				}
				fmt.Fprintf(&b, "%s %s (%s): %s", d.Name, d.CDC, d.DeclaredIn, d.Status)
				if d.Rule != "" {
					fmt.Fprintf(&b, " %s", d.Rule)
				}
				if d.Reason != "" {
					fmt.Fprintf(&b, " — %s", d.Reason)
				}
				b.WriteString("\n")
			}
			path := filepath.Join("testdata", "golden", strings.ToLower(g.class)+".txt")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run make golden)", err)
			}
			if !bytes.Equal(want, b.Bytes()) {
				t.Errorf("%s differs from the descriptors; run make golden and review the diff", path)
			}
		})
	}
}
