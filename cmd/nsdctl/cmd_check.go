// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/presence"
)

type checkFlags struct {
	instance string
	present  []string
	values   []string
	facts    []string
	ldNs     string
	lnNs     string
	ctlModel string
	rootLD   string
	scl      string
	lnType   string
	ln       string
	all      bool
}

func (a *app) modelCheckCmd() *cobra.Command {
	f := &checkFlags{}
	cmd := &cobra.Command{
		Use:   "check [CLASS]",
		Short: "Evaluate the presence conditions of an instance (JSON, SCL or flags)",
		Long: `Check evaluates the presence conditions (M, O, MOcond, AtLeastOne, MOsbo, ...)
of one instance of a logical node class, CDC or constructed type: which
data objects or attributes it requires, allows or forbids, and whether the
instance has them.

The instance comes from one of:
  --instance FILE   JSON (see docs/presence.md); "-" reads stdin
  --scl FILE --ln LD/LN        a logical node of an SCL file, with the
                               ldNs, lnNs, ctlModel and hvRef values it sets
  --scl FILE --ln-type ID      an LNodeType of the data type templates
  --present a,b,c   the present elements of CLASS

CLASS names the class when the instance does not. --value, --fact, --ld-ns,
--ln-ns, --ctl-model and --root-ld add to the instance. A condition that
needs a fact the instance does not give is undetermined, never guessed.

The output lists the violated and undetermined elements (--all: every
element). Exit code 1 when a condition is violated.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inst, err := f.load(cmd.InOrStdin())
			if err != nil {
				return err
			}
			if len(args) == 1 {
				inst.Class = args[0]
			}
			if inst.Class == "" {
				return withCode(exitUsage, "name the class: CLASS, \"class\" in the instance, or --scl with --ln or --ln-type")
			}
			if err := f.apply(inst); err != nil {
				return err
			}
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			ns, err := a.namespace(p)
			if err != nil {
				return err
			}
			ev := presence.New(p)
			if ns != nil {
				ev = ev.In(*ns)
			}
			rep, err := ev.Check(inst)
			if err != nil {
				return usage(err)
			}
			w := cmd.OutOrStdout()
			if err := a.output(w, rep, func() error { return printPresence(w, rep, f.all) }); err != nil {
				return err
			}
			if n := len(rep.Violations()); n > 0 {
				return withCode(exitFindings, "%s: %d presence conditions violated", rep.Class, n)
			}
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.instance, "instance", "", "JSON instance file (\"-\": stdin)")
	fl.StringSliceVar(&f.present, "present", nil, "Present elements, e.g. Beh,Pos,Ind1")
	fl.StringArrayVar(&f.values, "value", nil, "Attribute value NAME=VALUE (ctlModel, hvRef); PATH.NAME=VALUE for a child, e.g. Pos.ctlModel=sbo-with-normal-security")
	fl.StringArrayVar(&f.facts, "fact", nil, "Fact COND=true|false, e.g. MOcond(1)=true or Mir:MOcond(1)=false")
	fl.StringVar(&f.ldNs, "ld-ns", "", "Namespace of the logical device (LLN0.NamPlt.ldNs)")
	fl.StringVar(&f.lnNs, "ln-ns", "", "Namespace of the logical node (NamPlt.lnNs)")
	fl.StringVar(&f.ctlModel, "ctl-model", "", "Control model for data objects without a ctlModel value")
	fl.StringVar(&f.rootLD, "root-ld", "", "Whether the logical device is the root logical device (true|false)")
	fl.StringVar(&f.scl, "scl", "", "SCL file (ICD, CID, SCD, ...) to read the instance from")
	fl.StringVar(&f.lnType, "ln-type", "", "With --scl: LNodeType id")
	fl.StringVar(&f.ln, "ln", "", "With --scl: logical node LD/LN, e.g. CTRL/Q0CSWI1 (IED name + LD inst when several IEDs have it)")
	fl.BoolVar(&f.all, "all", false, "List every element, not only violated and undetermined ones")
	completeValues(cmd, "ctl-model", func() []string {
		return []string{"status-only", "direct-with-normal-security", "sbo-with-normal-security",
			"direct-with-enhanced-security", "sbo-with-enhanced-security"}
	})
	return cmd
}

// load reads the instance from --instance, --scl or --present.
func (f *checkFlags) load(stdin io.Reader) (*presence.Instance, error) {
	sources := 0
	for _, set := range []bool{f.instance != "", f.scl != "", len(f.present) > 0} {
		if set {
			sources++
		}
	}
	if sources > 1 {
		return nil, withCode(exitUsage, "use one of --instance, --scl and --present")
	}
	if (f.ln != "" || f.lnType != "") && f.scl == "" {
		return nil, withCode(exitUsage, "--ln and --ln-type need --scl")
	}
	switch {
	case f.instance != "":
		r, closeFn, err := open(f.instance, stdin)
		if err != nil {
			return nil, err
		}
		defer closeFn()
		inst, err := presence.ReadInstance(r)
		if err != nil {
			return nil, invalid(err)
		}
		return inst, nil
	case f.scl != "":
		if (f.ln == "") == (f.lnType == "") {
			return nil, withCode(exitUsage, "--scl needs one of --ln and --ln-type")
		}
		r, closeFn, err := open(f.scl, stdin)
		if err != nil {
			return nil, err
		}
		defer closeFn()
		doc, err := readSCL(r)
		if err != nil {
			return nil, invalid(err)
		}
		ix := newSCLIndex(doc)
		var inst *presence.Instance
		if f.ln != "" {
			inst, err = ix.ln(f.ln)
		} else {
			inst, err = ix.lnType(f.lnType)
		}
		if err != nil {
			return nil, invalid(err)
		}
		return inst, nil
	}
	return &presence.Instance{Present: append([]string{}, f.present...)}, nil
}

func open(name string, stdin io.Reader) (io.Reader, func(), error) {
	if name == "-" {
		return stdin, func() {}, nil
	}
	fh, err := os.Open(name) //nolint:gosec // the user names the file to check
	if err != nil {
		return nil, nil, withCode(exitInvalidInput, "%v", err)
	}
	return fh, func() { _ = fh.Close() }, nil
}

func invalid(err error) error {
	return &exitError{code: exitInvalidInput, err: err}
}

// apply adds the flag values to the instance.
func (f *checkFlags) apply(inst *presence.Instance) error {
	for _, kv := range f.values {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return withCode(exitUsage, "--value %q is not NAME=VALUE", kv)
		}
		target := inst
		parts := strings.Split(k, ".")
		for _, p := range parts[:len(parts)-1] {
			if target.Children == nil {
				target.Children = map[string]*presence.Instance{}
			}
			if target.Children[p] == nil {
				target.Children[p] = &presence.Instance{}
			}
			target = target.Children[p]
		}
		setValue(target, parts[len(parts)-1], v)
	}
	for _, kv := range f.facts {
		k, v, ok := strings.Cut(kv, "=")
		b, err := strconv.ParseBool(v)
		if !ok || k == "" || err != nil {
			return withCode(exitUsage, "--fact %q is not COND=true|false", kv)
		}
		if inst.Context.Facts == nil {
			inst.Context.Facts = map[string]bool{}
		}
		inst.Context.Facts[k] = b
	}
	if f.ldNs != "" {
		inst.Context.LDNamespace = f.ldNs
	}
	if f.lnNs != "" {
		inst.Context.LNNamespace = f.lnNs
	}
	if f.ctlModel != "" {
		inst.Context.CtlModel = f.ctlModel
	}
	if f.rootLD != "" {
		b, err := strconv.ParseBool(f.rootLD)
		if err != nil {
			return withCode(exitUsage, "--root-ld %q is not true or false", f.rootLD)
		}
		inst.Context.RootLD = &b
	}
	return nil
}

func printPresence(w io.Writer, rep *presence.Report, all bool) error {
	var rows [][]string
	results := rep.All()
	for _, r := range results {
		if !all && r.Verdict == presence.Satisfied {
			continue
		}
		missing := ""
		if r.Missing != "" {
			missing = " [needs " + r.Missing + "]"
		}
		rows = append(rows, []string{r.Path, r.Condition.String(), string(r.Requirement), string(r.Verdict), r.Reason + missing})
	}
	if len(rows) > 0 {
		if err := table(w, "PATH\tCONDITION\tREQUIREMENT\tVERDICT\tREASON", rows); err != nil {
			return err
		}
	}
	unknown := unknownNames(rep)
	if len(unknown) > 0 {
		if _, err := fmt.Fprintf(w, "not in the model: %s\n", strings.Join(unknown, ", ")); err != nil {
			return outputError(err)
		}
	}
	counts := map[presence.Verdict]int{}
	for _, r := range results {
		counts[r.Verdict]++
	}
	_, err := fmt.Fprintf(w, "%s: %d elements, %d satisfied, %d violated, %d undetermined\n", rep.Class, len(results),
		counts[presence.Satisfied], counts[presence.Violated], counts[presence.Unresolved])
	return outputError(err)
}

func unknownNames(rep *presence.Report) []string {
	var out []string
	for _, u := range rep.Unknown {
		out = append(out, joinName(rep.Path, u))
	}
	for _, c := range rep.Children {
		out = append(out, unknownNames(c)...)
	}
	return out
}

func joinName(base, name string) string {
	if base == "" {
		return name
	}
	return base + "." + name
}

// presenceDiagnostics returns the presence findings of every namespace
// of the catalog, for validate --presence.
func presenceDiagnostics(nss []*model.Namespace) model.Diagnostics {
	var ds model.Diagnostics
	for _, ns := range nss {
		ds = append(ds, presence.ValidateNamespace(ns)...)
	}
	return ds
}
