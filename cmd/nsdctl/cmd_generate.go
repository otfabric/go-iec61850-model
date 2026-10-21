// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/internal/generator"
	"github.com/otfabric/go-iec61850-model/internal/sources"
)

type genFlags struct {
	out     string
	check   bool
	force   bool
	publish bool
	lock    string
	docText bool
	goPkg   string
	index   string
}

func (a *app) generateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Go declarations, documentation or JSON exports from a profile",
		Long: `Generate writes deterministic output for the profile selected by
--source and --profile into --out. Files the generator did not write are
never overwritten without --force; generated files that are no longer
produced are removed. --check compares instead of writing and exits with
code 4 when the output is out of date.

Output is local-only by default and carries a LOCAL-ONLY.md notice.
--publish drops the notice, and is refused (exit code 5) unless the lock
file marks every source namespace as redistributable and its digest
matches.`,
	}
	cmd.AddCommand(
		a.generateKindCmd("go", "Go constants for classes, CDCs, data objects and enumeration types", func(f *genFlags) func(generator.Input) (generator.Files, error) {
			return func(in generator.Input) (generator.Files, error) { return generator.Go(in, f.goPkg) }
		}),
		a.generateKindCmd("docs", "Markdown documentation of classes, CDCs, types and coverage", func(*genFlags) func(generator.Input) (generator.Files, error) {
			return generator.Docs
		}),
		a.generateKindCmd("json", "JSON exports: catalog, ontology graph and coverage", func(*genFlags) func(generator.Input) (generator.Files, error) {
			return generator.JSON
		}),
		a.generateKindCmd("catalog", "Embedded catalog of a Go package: the namespaces as JSON and the named profiles", func(f *genFlags) func(generator.Input) (generator.Files, error) {
			return func(in generator.Input) (generator.Files, error) {
				var specs []generator.ProfileSpec
				if f.index != "" {
					idx, err := sources.LoadIndex(f.index)
					if err != nil {
						return nil, err
					}
					for _, p := range idx.Profiles {
						parts, err := idx.ProfileParts(p.Name)
						if err != nil {
							return nil, err
						}
						specs = append(specs, generator.ProfileSpec{Name: p.Name, Description: p.Description, Parts: parts})
					}
				}
				return generator.Catalog(in, f.goPkg, specs)
			}
		}),
	)
	return cmd
}

func (a *app) generateKindCmd(kind, short string, gen func(*genFlags) func(generator.Input) (generator.Files, error)) *cobra.Command {
	f := &genFlags{}
	cmd := &cobra.Command{
		Use:   kind,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.out == "" {
				return withCode(exitUsage, "--out is required")
			}
			if len(a.sources) == 0 {
				return withCode(exitUsage, "no source given (use --source with the official packages)")
			}
			p, err := a.loadProfile()
			if err != nil {
				return err
			}
			in := generator.Input{Profile: p, ToolVersion: version, IncludeDocText: f.docText, Publish: f.publish}
			if f.publish {
				var lock *sources.Lock
				if f.lock != "" {
					if lock, err = sources.LoadLock(f.lock); err != nil {
						return err
					}
				}
				if err := (generator.Policy{Lock: lock}).CheckPublish(p); err != nil {
					return err
				}
			}
			files, err := gen(f)(in)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if f.check {
				diffs, err := generator.Check(f.out, files)
				if err != nil {
					return err
				}
				if a.json {
					if diffs == nil {
						diffs = []generator.Difference{}
					}
					if err := writeJSON(w, diffs); err != nil {
						return err
					}
				} else {
					for _, d := range diffs {
						_, _ = fmt.Fprintf(w, "%-8s %s\n", d.Kind, d.Path)
					}
				}
				if len(diffs) > 0 {
					return &exitError{code: exitDifferent, err: fmt.Errorf("%s is out of date: %d files differ", f.out, len(diffs))}
				}
				if !a.json {
					_, _ = fmt.Fprintf(w, "%s is up to date (%d files)\n", f.out, len(files))
				}
				return nil
			}
			written, err := generator.Write(f.out, files, f.force)
			if err != nil {
				if errors.Is(err, generator.ErrWouldOverwrite) {
					return withCode(exitUsage, "%v", err)
				}
				return err
			}
			if a.json {
				return writeJSON(w, written)
			}
			for _, p := range written {
				_, _ = fmt.Fprintln(w, "wrote", p)
			}
			if !f.publish {
				_, _ = fmt.Fprintln(w, "output is local-only: do not commit or publish it (see LOCAL-ONLY.md)")
			}
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVarP(&f.out, "out", "o", "", "Output directory (required)")
	fl.BoolVar(&f.check, "check", false, "Compare with the output directory instead of writing; exit 4 when out of date")
	fl.BoolVar(&f.force, "force", false, "Overwrite files that were not written by nsdctl")
	fl.BoolVar(&f.publish, "publish", false, "Produce publishable output; requires --lock permitting every namespace")
	fl.StringVar(&f.lock, "lock", "", "Lock file for --publish, e.g. sources/nsd.lock.json")
	fl.BoolVar(&f.docText, "include-doc-text", false, "Include NSDoc titles and descriptions (IEC text)")
	switch kind {
	case "go":
		fl.StringVar(&f.goPkg, "package", "iec61850names", "Go package name")
	case "catalog":
		fl.StringVar(&f.goPkg, "package", "official", "Go package name")
		fl.StringVar(&f.index, "index", "sources/index.json", "Source registry with the named profiles (empty: none)")
	}
	return cmd
}
