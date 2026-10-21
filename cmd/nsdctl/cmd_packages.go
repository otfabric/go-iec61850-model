// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// sourceArgs returns the positional sources, or --source when there are
// none.
func (a *app) sourceArgs(args []string) []string {
	if len(args) > 0 {
		return args
	}
	return a.sources
}

type nsSummary struct {
	ID               string   `json:"id"`
	Source           string   `json:"source"`
	SHA256           string   `json:"sha256"`
	PackageSHA256    string   `json:"packageSHA256,omitempty"`
	PublicationStage string   `json:"publicationStage,omitempty"`
	Dependencies     []string `json:"dependencies,omitempty"`
	LNClasses        int      `json:"lnClasses"`
	AbstractClasses  int      `json:"abstractLNClasses"`
	CDCs             int      `json:"cdcs"`
	Constructed      int      `json:"constructedAttributes"`
	Enumerations     int      `json:"enumerations"`
	BasicTypes       int      `json:"basicTypes"`
	Documented       bool     `json:"documented"`
}

type pkgSummary struct {
	Source      string            `json:"source"`
	SHA256      string            `json:"sha256,omitempty"`
	Manifest    *nsd.Manifest     `json:"manifest,omitempty"`
	Namespaces  []nsSummary       `json:"namespaces"`
	Docs        []string          `json:"docs,omitempty"`
	Diagnostics model.Diagnostics `json:"diagnostics,omitempty"`
}

func summarize(p *nsd.Package) pkgSummary {
	s := pkgSummary{Source: p.Source(), SHA256: p.SHA256(), Manifest: p.Manifest(), Diagnostics: p.Diagnostics()}
	for _, ns := range p.Namespaces() {
		n := nsSummary{
			ID: ns.ID.String(), Source: ns.Provenance.Source, SHA256: ns.Provenance.SHA256, PackageSHA256: ns.Provenance.PackageSHA256,
			PublicationStage: ns.PublicationStage, LNClasses: len(ns.LNClasses), AbstractClasses: len(ns.AbstractLNClasses),
			CDCs: len(ns.CDCs), Constructed: len(ns.ConstructedAttributes), Enumerations: len(ns.Enumerations),
			BasicTypes: len(ns.BasicTypes), Documented: ns.Doc.Title != "" || ns.Doc.Description != "",
		}
		for _, d := range ns.Dependencies {
			n.Dependencies = append(n.Dependencies, d.String())
		}
		s.Namespaces = append(s.Namespaces, n)
	}
	for _, d := range p.Docs() {
		s.Docs = append(s.Docs, fmt.Sprintf("%s (%s, %d texts)", d.ID, orDash(d.Lang), d.Len()))
	}
	return s
}

func (a *app) inspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [SOURCE...]",
		Short: "Summarize NSD packages: namespaces, dependencies, digests, documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgs, err := a.loadPackages(a.sourceArgs(args))
			if err != nil {
				return err
			}
			var out []pkgSummary
			for _, p := range pkgs {
				out = append(out, summarize(p))
			}
			w := cmd.OutOrStdout()
			if a.json {
				return writeJSON(w, out)
			}
			for _, s := range out {
				_, _ = fmt.Fprintf(w, "%s\n", s.Source)
				if s.SHA256 != "" {
					_, _ = fmt.Fprintf(w, "  sha256    %s\n", s.SHA256)
				}
				if s.Manifest != nil {
					_, _ = fmt.Fprintf(w, "  manifest  %s (%d elements)\n", s.Manifest.Root, len(s.Manifest.Elements))
				}
				var rows [][]string
				for _, n := range s.Namespaces {
					rows = append(rows, []string{"  " + n.ID, strconv.Itoa(n.LNClasses), strconv.Itoa(n.AbstractClasses), strconv.Itoa(n.CDCs),
						strconv.Itoa(n.Constructed), strconv.Itoa(n.Enumerations), strconv.Itoa(n.BasicTypes), orDash(strings.Join(n.Dependencies, ", "))})
				}
				if err := table(w, "  NAMESPACE\tLN\tABSTRACT\tCDC\tTYPES\tENUMS\tBASIC\tDEPENDS ON", rows); err != nil {
					return err
				}
				for _, d := range s.Docs {
					_, _ = fmt.Fprintf(w, "  doc       %s\n", d)
				}
				printDiagnostics(w, s.Diagnostics, model.SeverityWarning)
			}
			return nil
		},
	}
}

type validation struct {
	Namespaces  []string          `json:"namespaces"`
	Diagnostics model.Diagnostics `json:"diagnostics"`
	Errors      int               `json:"errors"`
	Warnings    int               `json:"warnings"`
}

func (a *app) validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [SOURCE...]",
		Short: "Validate NSD packages and the profiles they form",
		Long: `Validate parses every source (use --strict to make importer warnings
fatal) and checks the profile of each loaded namespace with its
dependencies: missing or ambiguous dependencies, conflicting definitions
and unresolved type references. With --profile only that profile is
checked. Exit code 3 when errors are found.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pkgs, err := a.loadCatalog(a.sourceArgs(args))
			if err != nil {
				return err
			}
			res := validation{}
			for _, p := range pkgs {
				res.Diagnostics = append(res.Diagnostics, p.Diagnostics().AtLeast(model.SeverityWarning)...)
			}
			seen := map[string]bool{}
			addAll := func(ds model.Diagnostics) {
				for _, d := range ds {
					k := d.String() + d.Namespace.String()
					if !seen[k] {
						seen[k] = true
						res.Diagnostics = append(res.Diagnostics, d)
					}
				}
			}
			if len(a.profile) > 0 {
				p, err := a.selectProfile(c)
				if err != nil {
					return err
				}
				for _, id := range p.IDs() {
					res.Namespaces = append(res.Namespaces, id.String())
				}
				addAll(p.Diagnostics())
			} else {
				for _, ns := range c.Namespaces() {
					res.Namespaces = append(res.Namespaces, ns.ID.String())
					p, err := c.Profile(ns.ID)
					if err != nil {
						addAll(model.Diagnostics{{Severity: model.SeverityError, Code: "profile", Namespace: ns.ID, Message: err.Error()}})
						continue
					}
					addAll(p.Diagnostics())
				}
			}
			res.Errors = len(res.Diagnostics.Errors())
			res.Warnings = len(res.Diagnostics.AtLeast(model.SeverityWarning)) - res.Errors
			w := cmd.OutOrStdout()
			if a.json {
				if err := writeJSON(w, res); err != nil {
					return err
				}
			} else {
				printDiagnostics(w, res.Diagnostics, model.SeverityWarning)
				_, _ = fmt.Fprintf(w, "%d namespaces, %d errors, %d warnings\n", len(res.Namespaces), res.Errors, res.Warnings)
			}
			if res.Errors > 0 {
				return withCode(exitValidation, "validation failed with %d errors", res.Errors)
			}
			return nil
		},
	}
}

type dependency struct {
	Namespace  string   `json:"namespace"`
	Requires   string   `json:"requires"`
	Status     string   `json:"status"`
	Candidates []string `json:"candidates,omitempty"`
}

func (a *app) dependenciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "dependencies [SOURCE...]",
		Aliases: []string{"deps"},
		Short:   "List namespace dependencies and whether the loaded sources satisfy them",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := a.loadCatalog(a.sourceArgs(args))
			if err != nil {
				return err
			}
			deps := resolveDependencies(c)
			if deps == nil {
				deps = []dependency{}
			}
			w := cmd.OutOrStdout()
			if a.json {
				if err := writeJSON(w, deps); err != nil {
					return err
				}
			} else {
				var rows [][]string
				for _, d := range deps {
					rows = append(rows, []string{d.Namespace, d.Requires, d.Status, orDash(strings.Join(d.Candidates, ", "))})
				}
				if err := table(w, "NAMESPACE\tREQUIRES\tSTATUS\tSATISFIED BY", rows); err != nil {
					return err
				}
			}
			for _, d := range deps {
				if d.Status != "ok" {
					return withCode(exitValidation, "unsatisfied dependencies")
				}
			}
			return nil
		},
	}
}

func resolveDependencies(c *catalog.Catalog) []dependency {
	var out []dependency
	for _, ns := range c.Namespaces() {
		for _, dep := range ns.Dependencies {
			d := dependency{Namespace: ns.ID.String(), Requires: dep.String()}
			for _, cand := range c.Namespaces() {
				if cand.ID.Satisfies(dep) {
					d.Candidates = append(d.Candidates, cand.ID.String())
				}
			}
			switch len(d.Candidates) {
			case 0:
				d.Status = "missing"
			case 1:
				d.Status = "ok"
			default:
				d.Status = "ambiguous"
			}
			out = append(out, d)
		}
	}
	return out
}

func (a *app) diffCmd() *cobra.Command {
	var exitOnChange bool
	cmd := &cobra.Command{
		Use:   "diff OLD NEW",
		Short: "Compare two sources (packages, directories or documents) definition by definition",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgs, err := a.loadPackages(args)
			if err != nil {
				return err
			}
			changes := catalog.Diff(pkgs[0], pkgs[1])
			w := cmd.OutOrStdout()
			if a.json {
				if changes == nil {
					changes = []catalog.Change{}
				}
				if err := writeJSON(w, changes); err != nil {
					return err
				}
			} else {
				for _, ch := range changes {
					_, _ = fmt.Fprintf(w, "%-8s %s\n", ch.Kind, ch.Description)
				}
				_, _ = fmt.Fprintf(w, "%d changes\n", len(changes))
			}
			if exitOnChange && len(changes) > 0 {
				return &exitError{code: exitDifferent, err: errors.New("sources differ")}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&exitOnChange, "exit-code", false, "Exit with code 4 when there are changes")
	return cmd
}
