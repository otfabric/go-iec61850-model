// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/resolver"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func (a *app) modelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Explore the model of a profile (the embedded official catalog without --source)",
		Long: `Explore the model of a profile: classes, data objects, CDCs, types and paths.

Without --source, the commands use the official catalog embedded in nsdctl.
A name that two namespaces of the profile define (DBAT in IEC 61850-7-420
and IEC TR 61850-90-9 in the extended profile) is ambiguous: select the
namespace with --ns. Definitions are never merged.`,
	}
	cmd.PersistentFlags().StringVar(&a.ns, "ns", "", "Namespace to look names up in, e.g. \"IEC 61850-7-420\" or \"7-420\" (default: the whole profile)")
	cmd.AddCommand(a.modelShowCmd(), a.modelListCmd(), a.modelSearchCmd(), a.modelDescribeCmd(), a.modelMeasurementsCmd(), a.modelLeavesCmd())
	return cmd
}

// dataObjectRow is a data object of a class with its semantics.
type dataObjectRow struct {
	Name       string `json:"name"`
	CDC        string `json:"cdc"`
	Presence   string `json:"presence,omitempty"`
	DeclaredIn string `json:"declaredIn"`
	Namespace  string `json:"namespace"`
	Semantics  string `json:"semantics,omitempty"`
	Text       string `json:"text,omitempty"`
}

type classView struct {
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Namespace   string          `json:"namespace"`
	Group       string          `json:"group,omitempty"`
	Abstract    bool            `json:"abstract,omitempty"`
	Inheritance []string        `json:"inheritance"`
	Title       string          `json:"title,omitempty"`
	DataObjects []dataObjectRow `json:"dataObjects"`
}

func semanticsOf(ds []*semantic.Descriptor) map[string]string {
	out := map[string]string{}
	for _, d := range ds {
		if _, done := out[d.DataObject]; done {
			continue
		}
		s := string(d.Quantity)
		if d.SubDataObject == "" && d.Phase != "" {
			s += " (" + string(d.Phase) + ")"
		} else if d.SubDataObject != "" {
			s += " per phase"
		}
		if d.Reference != "" {
			s += ", " + string(d.Reference)
		}
		if d.Direction != "" {
			s += ", " + string(d.Direction)
		}
		out[d.DataObject] = s
	}
	return out
}

func (a *app) modelShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME|PATH",
		Short: "Show a class, CDC, type or enumeration, or resolve a path such as MMXU.Hz.mag.f",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			ns, err := a.namespace(p)
			if err != nil {
				return err
			}
			name, w := args[0], cmd.OutOrStdout()
			if strings.ContainsAny(name, "./$[") {
				return a.showPath(w, p, ns, name)
			}
			l := lookupIn(p, ns)
			if ln, err := l.logicalNode(name); err == nil {
				return a.showClass(w, p, ln)
			} else if !errors.Is(err, model.ErrUnknownLogicalNode) {
				return ambiguous(err)
			}
			if cs, err := l.cdcVariants(name); err == nil {
				return a.showCDC(w, cs)
			} else if !errors.Is(err, model.ErrUnknownCDC) {
				return ambiguous(err)
			}
			if c, err := l.constructedAttribute(name); err == nil {
				return a.output(w, c, func() error { return showMembers(w, "constructed attribute", c.Name, c.Source, c.SubDataAttributes) })
			}
			if e, err := l.enumeration(name); err == nil {
				return a.output(w, e, func() error {
					_, _ = fmt.Fprintf(w, "enumeration %s (%s)\n", e.Name, e.Source)
					if e.InheritedFrom != "" {
						_, _ = fmt.Fprintf(w, "extends %s\n", e.InheritedFrom)
					}
					var rows [][]string
					for _, l := range e.Literals {
						rows = append(rows, []string{strconv.Itoa(l.Value), orDash(l.Name)})
					}
					return table(w, "VALUE\tLITERAL", rows)
				})
			}
			if b, err := l.basicType(name); err == nil {
				return a.output(w, b, func() error { _, err := fmt.Fprintf(w, "basic type %s (%s)\n", b.Name, b.Source); return err })
			}
			if ns != nil {
				return withCode(exitUsage, "no definition named %q in %s", name, ns)
			}
			return withCode(exitUsage, "no definition named %q in profile %s", name, p)
		},
	}
}

// namespace resolves --ns against the profile; nil without --ns.
func (a *app) namespace(p *catalog.Profile) (*model.NamespaceID, error) {
	if a.ns == "" {
		return nil, nil
	}
	id, err := resolveNamespace(p, "--ns", a.ns)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// ambiguous turns an ambiguity into a usage error that names the flag
// selecting a namespace.
func ambiguous(err error) error {
	var ae *model.AmbiguityError
	if errors.As(err, &ae) {
		return &exitError{code: exitUsage, err: fmt.Errorf("%w; select one with --ns", err)}
	}
	return err
}

// lookups are the name lookups of model show, in the whole profile or
// in one namespace (--ns).
type lookups struct {
	logicalNode          func(string) (*model.LogicalNodeClass, error)
	cdcVariants          func(string) ([]*model.CommonDataClass, error)
	constructedAttribute func(string) (*model.ConstructedAttribute, error)
	enumeration          func(string) (*model.Enumeration, error)
	basicType            func(string) (*model.BasicType, error)
}

func lookupIn(p *catalog.Profile, ns *model.NamespaceID) lookups {
	if ns == nil {
		return lookups{p.LogicalNode, p.CDCVariants, p.ConstructedAttribute, p.Enumeration, p.BasicType}
	}
	id := *ns
	return lookups{
		logicalNode:          func(n string) (*model.LogicalNodeClass, error) { return p.LogicalNodeIn(id, n) },
		cdcVariants:          func(n string) ([]*model.CommonDataClass, error) { return p.CDCVariantsIn(id, n) },
		constructedAttribute: func(n string) (*model.ConstructedAttribute, error) { return p.ConstructedAttributeIn(id, n) },
		enumeration:          func(n string) (*model.Enumeration, error) { return p.EnumerationIn(id, n) },
		basicType:            func(n string) (*model.BasicType, error) { return p.BasicTypeFrom(id, n) },
	}
}

// usage reports a lookup failure as a usage error, with the --ns hint
// for an ambiguous name.
func usage(err error) error {
	if e := ambiguous(err); e != err {
		return e
	}
	return withCode(exitUsage, "%v", err)
}

// resolverFor returns a resolver for the profile, scoped to --ns.
func resolverFor(p *catalog.Profile, ns *model.NamespaceID) *resolver.Resolver {
	r := resolver.New(p)
	if ns != nil {
		r = r.In(*ns)
	}
	return r
}

// output writes v as JSON or runs text.
func (a *app) output(w io.Writer, v any, text func() error) error {
	if a.json {
		return writeJSON(w, v)
	}
	return text()
}

func (a *app) showClass(w io.Writer, p *catalog.Profile, ln *model.LogicalNodeClass) error {
	chain, err := p.InheritanceIn(ln.Source, ln.Name)
	if err != nil {
		return err
	}
	v := classView{Kind: "logical node class", Name: ln.Name, Namespace: ln.Source.String(), Abstract: ln.Abstract, Title: ln.Doc.Title}
	if g := ln.Group(); g != 0 {
		v.Group = strings.TrimSpace(g.String() + " " + g.Label())
	}
	for _, c := range chain {
		v.Inheritance = append(v.Inheritance, c.Name)
	}
	dos, err := p.DataObjectsIn(ln.Source, ln.Name)
	if err != nil {
		return err
	}
	sem := map[string]string{}
	if ds, err := semantic.FindMeasurementsIn(p, ln.Source, ln.Name); err == nil {
		sem = semanticsOf(ds)
	}
	for _, do := range dos {
		v.DataObjects = append(v.DataObjects, dataObjectRow{Name: do.Name, CDC: do.Type, Presence: do.Presence.String(),
			DeclaredIn: do.DeclaredIn.Name, Namespace: do.DeclaredIn.Source.String(), Semantics: sem[do.Name], Text: do.Doc.Title})
	}
	return a.output(w, v, func() error {
		_, _ = fmt.Fprintf(w, "%s %s (%s)\n", v.Kind, v.Name, v.Namespace)
		if v.Group != "" {
			_, _ = fmt.Fprintf(w, "group        %s\n", v.Group)
		}
		if v.Title != "" {
			_, _ = fmt.Fprintf(w, "title        %s\n", v.Title)
		}
		_, _ = fmt.Fprintf(w, "inheritance  %s\n\n", strings.Join(v.Inheritance, " → "))
		var rows [][]string
		for _, d := range v.DataObjects {
			rows = append(rows, []string{d.Name, d.CDC, orDash(d.Presence), d.DeclaredIn, d.Namespace, orDash(d.Semantics)})
		}
		return table(w, "DATA OBJECT\tCDC\tPRESENCE\tDECLARED IN\tNAMESPACE\tSEMANTICS", rows)
	})
}

func (a *app) showCDC(w io.Writer, variants []*model.CommonDataClass) error {
	var v any = variants[0]
	if len(variants) > 1 {
		v = variants
	}
	return a.output(w, v, func() error {
		for i, c := range variants {
			if i > 0 {
				_, _ = fmt.Fprintln(w)
			}
			_, _ = fmt.Fprintf(w, "CDC %s (%s)\n\n", c.Key(), c.Source)
			type row struct {
				pos  int
				cols []string
			}
			var rows []row
			for _, s := range c.SubDataObjects {
				rows = append(rows, row{s.Position, []string{s.Name, "sub data object", s.Type, "-", orDash(s.Presence.String())}})
			}
			for _, d := range c.DataAttributes {
				typ := d.Type
				if typ == "" {
					typ = "(" + string(d.TypeKind) + " from data object)"
				}
				rows = append(rows, row{d.Position, []string{d.Name, "data attribute", typ, orDash(string(d.FC)), orDash(d.Presence.String())}})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].pos < rows[j].pos })
			ordered := make([][]string, len(rows))
			for i := range rows {
				ordered[i] = rows[i].cols
			}
			if err := table(w, "MEMBER\tKIND\tTYPE\tFC\tPRESENCE", ordered); err != nil {
				return err
			}
		}
		return nil
	})
}

func showMembers(w io.Writer, kind, name string, src model.NamespaceID, attrs []model.DataAttribute) error {
	_, _ = fmt.Fprintf(w, "%s %s (%s)\n\n", kind, name, src)
	var rows [][]string
	for _, d := range attrs {
		rows = append(rows, []string{d.Name, orDash(d.Type), orDash(string(d.TypeKind)), orDash(d.Presence.String())})
	}
	return table(w, "MEMBER\tTYPE\tKIND\tPRESENCE", rows)
}

type pathView struct {
	Path      string               `json:"path"`
	Type      string               `json:"type"`
	TypeKind  string               `json:"typeKind,omitempty"`
	FC        string               `json:"fc,omitempty"`
	Steps     []resolver.Step      `json:"steps"`
	Semantics *semantic.Descriptor `json:"semantics,omitempty"`
}

func (a *app) showPath(w io.Writer, p *catalog.Profile, ns *model.NamespaceID, ref string) error {
	res, err := resolverFor(p, ns).ResolveString(ref)
	if err != nil {
		return usage(err)
	}
	v := pathView{Path: res.Path(), Type: res.Type(), TypeKind: string(res.TypeKind()), FC: string(res.FC()), Steps: res.Steps}
	if d, err := semantic.Default.DescribeResolution(p, res); err == nil {
		v.Semantics = d
	}
	return a.output(w, v, func() error {
		_, _ = fmt.Fprintf(w, "%s → %s", v.Path, v.Type)
		if v.TypeKind != "" {
			_, _ = fmt.Fprintf(w, " (%s)", v.TypeKind)
		}
		if v.FC != "" {
			_, _ = fmt.Fprintf(w, " [%s]", v.FC)
		}
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w)
		var rows [][]string
		for _, s := range v.Steps {
			rows = append(rows, []string{string(s.Kind), s.Name, s.Type, orDash(string(s.FC)), orDash(s.Presence.String()), s.DeclaredIn, s.Source.String()})
		}
		if err := table(w, "STEP\tNAME\tTYPE\tFC\tPRESENCE\tDECLARED IN\tNAMESPACE", rows); err != nil {
			return err
		}
		for _, st := range v.Steps {
			if len(st.Variants) > 0 {
				_, _ = fmt.Fprintf(w, "\nThe FC of %s depends on the realization: %s.\nAppend [FC] to the reference to select one.\n", st.Name, strings.Join(st.Variants, ", "))
			}
		}
		if v.Semantics != nil {
			_, _ = fmt.Fprintln(w)
			printDescriptor(w, v.Semantics)
		}
		return nil
	})
}

func printDescriptor(w io.Writer, d *semantic.Descriptor) {
	_, _ = fmt.Fprintf(w, "quantity     %s\n", d.Quantity)
	for _, kv := range [][2]string{
		{"phase", string(d.Phase)}, {"reference", string(d.Reference)}, {"direction", string(d.Direction)},
		{"component", string(d.Component)}, {"unit hint", d.UnitHint.String()},
	} {
		if kv[1] != "" {
			_, _ = fmt.Fprintf(w, "%-12s %s\n", kv[0], kv[1])
		}
	}
	_, _ = fmt.Fprintf(w, "mapping      rule %s, %s, %s confidence (%s)\n", d.Rule, d.Basis, d.Confidence, d.Source)
	for _, n := range d.Notes {
		_, _ = fmt.Fprintf(w, "note         %s\n", n)
	}
}

func (a *app) modelListCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List definitions of one kind: ln, abstract, cdc, ca, enum or basic",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			type item struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Info      string `json:"info,omitempty"`
			}
			var items []item
			switch kind {
			case "ln", "abstract":
				lns := p.ListLogicalNodes()
				if kind == "abstract" {
					lns = p.ListAbstractLogicalNodes()
				}
				for _, ln := range lns {
					info := ln.Group().Label()
					if ln.Base != "" {
						info = strings.TrimSpace(info + " (base " + ln.Base + ")")
					}
					items = append(items, item{ln.Name, ln.Source.String(), info})
				}
			case "cdc":
				for _, c := range p.ListCDCs() {
					items = append(items, item{c.Name, c.Source.String(), fmt.Sprintf("%d members", len(c.SubDataObjects)+len(c.DataAttributes))})
				}
			case "ca":
				for _, c := range p.ListConstructedAttributes() {
					items = append(items, item{c.Name, c.Source.String(), fmt.Sprintf("%d members", len(c.SubDataAttributes))})
				}
			case "enum":
				for _, e := range p.ListEnumerations() {
					items = append(items, item{e.Name, e.Source.String(), fmt.Sprintf("%d literals", len(e.Literals))})
				}
			case "basic":
				for _, b := range p.ListBasicTypes() {
					items = append(items, item{b.Name, b.Source.String(), ""})
				}
			default:
				return withCode(exitUsage, "unknown kind %q (ln, abstract, cdc, ca, enum, basic)", kind)
			}
			w := cmd.OutOrStdout()
			return a.output(w, items, func() error {
				rows := make([][]string, len(items))
				for i, it := range items {
					rows[i] = []string{it.Name, it.Namespace, it.Info}
				}
				return table(w, "NAME\tNAMESPACE\tINFO", rows)
			})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "ln", "Kind: ln, abstract, cdc, ca, enum, basic")
	return cmd
}

func (a *app) modelSearchCmd() *cobra.Command {
	var q catalog.Query
	var kinds []string
	var fc, group string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search classes, data objects, CDCs, attributes and types",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, k := range kinds {
				kk, ok := catalog.ParseKind(k)
				if !ok {
					return withCode(exitUsage, "unknown kind %q", k)
				}
				q.Kinds = append(q.Kinds, kk)
			}
			q.FC = model.FunctionalConstraint(fc)
			if group != "" {
				if len(group) != 1 || group[0] < 'A' || group[0] > 'Z' {
					return withCode(exitUsage, "--group takes one upper-case letter")
				}
				q.Group = model.LNGroup(group[0])
			}
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			res := p.Search(q)
			w := cmd.OutOrStdout()
			if res == nil {
				res = []catalog.Result{}
			}
			return a.output(w, res, func() error {
				rows := make([][]string, len(res))
				for i, r := range res {
					rows[i] = []string{string(r.Kind), r.Path, orDash(r.Type), orDash(string(r.FC)), orDash(r.Presence.String()), r.Namespace.String()}
				}
				return table(w, "KIND\tPATH\tTYPE\tFC\tPRESENCE\tNAMESPACE", rows)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&q.Name, "name", "", "Name substring (case-insensitive)")
	f.BoolVar(&q.Exact, "exact", false, "Match --name exactly")
	f.StringVar(&q.CDC, "cdc", "", "Only data objects of this CDC and classes declaring one")
	f.StringVar(&q.Type, "type", "", "Only attributes of this type")
	f.StringVar(&fc, "fc", "", "Only data attributes with this functional constraint")
	f.StringVar(&q.Standard, "standard", "", "Only definitions of this standard, e.g. \"IEC 61850-7-4\"")
	f.StringVar(&group, "group", "", "Only logical node classes of this group letter")
	f.BoolVar(&q.IncludeAbstract, "abstract", false, "Include abstract classes")
	f.StringArrayVar(&kinds, "kind", nil, "Only these kinds: ln, do, cdc, da, ca, enum, basic (repeatable)")
	return cmd
}

func (a *app) modelDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe REF",
		Short: "Describe what a reference means: quantity, phase, reference, unit hint (rule-based)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			ns, err := a.namespace(p)
			if err != nil {
				return err
			}
			res, err := resolverFor(p, ns).ResolveString(args[0])
			if err != nil {
				return usage(err)
			}
			d, err := semantic.Default.DescribeResolution(p, res)
			if err != nil {
				return withCode(exitUsage, "%v", err)
			}
			w := cmd.OutOrStdout()
			return a.output(w, d, func() error {
				_, _ = fmt.Fprintln(w, d.Path)
				printDescriptor(w, d)
				return nil
			})
		},
	}
}

func (a *app) modelMeasurementsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "measurements LNCLASS",
		Short: "List the mapped measurements of a logical node class, per phase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			ns, err := a.namespace(p)
			if err != nil {
				return err
			}
			var ds []*semantic.Descriptor
			if ns != nil {
				ds, err = semantic.FindMeasurementsIn(p, *ns, args[0])
			} else {
				ds, err = semantic.FindMeasurements(p, args[0])
			}
			if err != nil {
				return usage(err)
			}
			w := cmd.OutOrStdout()
			if ds == nil {
				ds = []*semantic.Descriptor{}
			}
			return a.output(w, ds, func() error {
				rows := make([][]string, len(ds))
				for i, d := range ds {
					rows[i] = []string{d.Path, string(d.Quantity), orDash(string(d.Phase)), orDash(string(d.Reference)),
						orDash(string(d.Direction)), orDash(d.UnitHint.String()), d.Rule}
				}
				return table(w, "PATH\tQUANTITY\tPHASE\tREFERENCE\tDIRECTION\tUNIT\tRULE", rows)
			})
		},
	}
}

func (a *app) modelLeavesCmd() *cobra.Command {
	var fc string
	cmd := &cobra.Command{
		Use:   "leaves LNCLASS[.DO...]",
		Short: "Expand a class or path into its leaf attributes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			ns, err := a.namespace(p)
			if err != nil {
				return err
			}
			parts := strings.Split(args[0], ".")
			leaves, err := resolverFor(p, ns).Leaves(parts[0], parts[1:], model.FunctionalConstraint(fc))
			if err != nil {
				return usage(err)
			}
			type leaf struct {
				Path     string `json:"path"`
				Type     string `json:"type"`
				TypeKind string `json:"typeKind"`
				FC       string `json:"fc"`
			}
			out := make([]leaf, len(leaves))
			for i, l := range leaves {
				out[i] = leaf{l.Path(), l.Type(), string(l.TypeKind()), string(l.FC())}
			}
			w := cmd.OutOrStdout()
			return a.output(w, out, func() error {
				rows := make([][]string, len(out))
				for i, l := range out {
					rows[i] = []string{l.Path, l.Type, l.TypeKind, l.FC}
				}
				return table(w, "PATH\tTYPE\tKIND\tFC", rows)
			})
		},
	}
	cmd.Flags().StringVar(&fc, "fc", "", "Only data attributes with this functional constraint")
	return cmd
}
