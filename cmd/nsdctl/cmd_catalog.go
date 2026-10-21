// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
)

func (a *app) catalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Inspect the namespaces of a catalog and the names they share",
		Long: `Inspect the namespaces of a catalog: the named profiles and the names that
two namespaces both define.

Without --source, the commands use the official catalog embedded in nsdctl.
Definitions of the same name in two namespaces are never merged; the
overlap report shows how they relate.`,
	}
	cmd.AddCommand(a.catalogProfilesCmd(), a.catalogOverlapsCmd())
	return cmd
}

// profileView is a named profile and the namespaces it selects.
type profileView struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Parts       []string `json:"parts"`
	Namespaces  []string `json:"namespaces"`
}

func (a *app) catalogProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List the named profiles and the namespaces they select",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.modelCatalog()
			if err != nil {
				return err
			}
			var out []profileView
			for _, p := range official.Profiles() {
				v := profileView{Name: p.Name, Description: p.Description, Parts: p.Parts, Namespaces: []string{}}
				if ids, ok := namedProfile(c, p.Name); ok {
					for _, id := range ids {
						v.Namespaces = append(v.Namespaces, id.String())
					}
				}
				out = append(out, v)
			}
			w := cmd.OutOrStdout()
			return a.output(w, out, func() error {
				for i, p := range out {
					if i > 0 {
						_, _ = fmt.Fprintln(w)
					}
					_, _ = fmt.Fprintf(w, "%s: %s\n", p.Name, p.Description)
					if len(p.Namespaces) == 0 {
						_, _ = fmt.Fprintf(w, "  (the sources do not carry exactly one release of each part: %s)\n", strings.Join(p.Parts, ", "))
					}
					for _, ns := range p.Namespaces {
						_, _ = fmt.Fprintf(w, "  %s\n", ns)
					}
				}
				return nil
			})
		},
	}
}

func (a *app) catalogOverlapsCmd() *cobra.Command {
	var left, right string
	var classes []string
	cmd := &cobra.Command{
		Use:   "overlaps",
		Short: "Compare the definitions that two namespaces both define",
		Long: `Compare every name that two namespaces both define: logical node classes
(resolved inheritance chain and every data object with its CDC and
presence), CDCs, constructed attributes and enumerations.

Each overlap is classified:
  identical             the resolved structures are equal
  compatible-extension  one definition extends the other without changing
                        anything the other defines
  conflicting           the definitions differ in what both define, or each
                        defines something the other does not

Without --left and --right, every pair of namespaces of the catalog (or of
--profile) is compared. Namespaces may be partial: "IEC 61850-7-420",
"IEC_TR_61850-90-9" or "90-9".`,
		Example: `  nsdctl catalog overlaps --left IEC_61850-7-420 --right IEC_TR_61850-90-9
  nsdctl catalog overlaps --profile extended --class conflicting`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (left == "") != (right == "") {
				return withCode(exitUsage, "give both --left and --right, or neither")
			}
			for _, c := range classes {
				switch catalog.OverlapClass(c) {
				case catalog.OverlapIdentical, catalog.OverlapCompatible, catalog.OverlapConflicting:
				default:
					return withCode(exitUsage, "unknown --class %q (identical, compatible-extension, conflicting)", c)
				}
			}
			set, err := a.overlapSet()
			if err != nil {
				return err
			}
			var l, r model.NamespaceID
			if left != "" {
				if l, err = resolveNamespace(set, "--left", left); err != nil {
					return err
				}
				if r, err = resolveNamespace(set, "--right", right); err != nil {
					return err
				}
			}
			all, err := set.Overlaps(l, r)
			if err != nil {
				return withCode(exitUsage, "%v", err)
			}
			out := []catalog.Overlap{}
			for _, o := range all {
				if len(classes) == 0 || containsClass(classes, o.Class) {
					out = append(out, o)
				}
			}
			w := cmd.OutOrStdout()
			return a.output(w, out, func() error {
				if len(out) == 0 {
					_, _ = fmt.Fprintln(w, "no overlapping names")
					return nil
				}
				rows := make([][]string, len(out))
				for i, o := range out {
					rows[i] = []string{string(o.Kind), o.Name, o.Left.String(), o.Right.String(), string(o.Class), overlapSummary(o)}
				}
				if err := table(w, "KIND\tNAME\tLEFT\tRIGHT\tCLASS\tDETAILS", rows); err != nil {
					return err
				}
				for _, o := range out {
					if o.Class == catalog.OverlapIdentical {
						continue
					}
					_, _ = fmt.Fprintf(w, "\n%s %s (%s / %s): %s\n", o.Kind, o.Name, o.Left, o.Right, o.Class)
					if len(o.OnlyLeft) > 0 {
						_, _ = fmt.Fprintf(w, "  only in %s: %s\n", o.Left, strings.Join(o.OnlyLeft, ", "))
					}
					if len(o.OnlyRight) > 0 {
						_, _ = fmt.Fprintf(w, "  only in %s: %s\n", o.Right, strings.Join(o.OnlyRight, ", "))
					}
					for _, d := range o.Differences {
						_, _ = fmt.Fprintf(w, "  differs: %s\n", d)
					}
				}
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&left, "left", "", "First namespace, e.g. \"IEC_61850-7-420\"")
	f.StringVar(&right, "right", "", "Second namespace, e.g. \"IEC_TR_61850-90-9\"")
	f.StringArrayVar(&classes, "class", nil, "Only overlaps of this class: identical, compatible-extension, conflicting (repeatable)")
	completeValues(cmd, "class", overlapClassValues)
	completeValues(cmd, "left", namespaceValues)
	completeValues(cmd, "right", namespaceValues)
	return cmd
}

// overlapSet is the set the overlap report compares: the profile
// selected with --profile, or the whole catalog.
type overlapSet interface {
	namespaceSet
	Overlaps(left, right model.NamespaceID) ([]catalog.Overlap, error)
}

func (a *app) overlapSet() (overlapSet, error) {
	c, err := a.modelCatalog()
	if err != nil {
		return nil, err
	}
	if len(a.profile) == 0 {
		return c, nil
	}
	return a.selectProfile(c)
}

func containsClass(classes []string, c catalog.OverlapClass) bool {
	for _, x := range classes {
		if catalog.OverlapClass(x) == c {
			return true
		}
	}
	return false
}

// overlapSummary counts the member differences of an overlap.
func overlapSummary(o catalog.Overlap) string {
	var parts []string
	if n := len(o.OnlyLeft); n > 0 {
		parts = append(parts, fmt.Sprintf("%d only left", n))
	}
	if n := len(o.OnlyRight); n > 0 {
		parts = append(parts, fmt.Sprintf("%d only right", n))
	}
	if n := len(o.Differences); n > 0 {
		parts = append(parts, fmt.Sprintf("%d differ", n))
	}
	return orDash(strings.Join(parts, ", "))
}
