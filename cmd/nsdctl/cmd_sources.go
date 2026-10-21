// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/sources"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

const discoveryNote = "not performed: official discovery endpoints are not verified yet (milestone M0); the weekly monitor will use them once they are"

// Package coverage states.
const (
	statusOK       = "ok"
	statusMissing  = "missing"
	statusMismatch = "release-mismatch"
	statusInvalid  = "invalid"
	statusNoSchema = "no-schema"
)

// packageStatus is the coverage of one tracked package in a directory.
type packageStatus struct {
	Tier       string   `json:"tier"`
	Part       string   `json:"part"`
	Name       string   `json:"name"`
	Release    string   `json:"release"`
	Kind       string   `json:"kind"`
	File       string   `json:"file,omitempty"`
	Status     string   `json:"status"`
	Namespaces []string `json:"namespaces,omitempty"`
	Found      []string `json:"found,omitempty"`
	Schemas    int      `json:"schemas,omitempty"`
	Error      string   `json:"error,omitempty"`

	pkg *nsd.Package
}

// survey checks every tracked package of the given tiers against dir.
func (a *app) survey(idx *sources.Index, dir string, tiers []string) []packageStatus {
	var out []packageStatus
	for _, p := range idx.Packages {
		if !slices.Contains(tiers, p.Tier) {
			continue
		}
		st := packageStatus{Tier: p.Tier, Part: p.Part, Name: p.Name, Release: p.Release, Kind: p.Kind}
		file, ok := p.FindFile(dir)
		if !ok {
			st.Status = statusMissing
			out = append(out, st)
			continue
		}
		st.File = file
		pkg, err := a.loadPackage(file)
		if err != nil {
			st.Status, st.Error = statusInvalid, err.Error()
			out = append(out, st)
			continue
		}
		st.pkg = pkg
		st.Schemas = len(pkg.Schemas())
		for _, ns := range pkg.Namespaces() {
			st.Found = append(st.Found, ns.ID.String())
			if p.MatchesNamespace(ns.ID) {
				st.Namespaces = append(st.Namespaces, ns.ID.String())
			}
		}
		switch {
		case p.Kind == sources.KindSchema && st.Schemas == 0:
			st.Status = statusNoSchema
		case p.Kind == sources.KindSchema:
			st.Status = statusOK
		case len(st.Namespaces) == 0:
			st.Status = statusMismatch
		default:
			st.Status = statusOK
		}
		out = append(out, st)
	}
	return out
}

type sourcesReport struct {
	Index        string                `json:"index"`
	Lock         string                `json:"lock"`
	Diagnostics  model.Diagnostics     `json:"diagnostics"`
	Entries      []sources.Entry       `json:"entries"`
	Packages     []packageStatus       `json:"packages,omitempty"`
	Verification *sources.Verification `json:"verification,omitempty"`
	Discovery    string                `json:"discovery"`
}

func (a *app) sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Work with the source registry, the tracked packages and the version lock",
	}
	cmd.AddCommand(a.sourcesCheckCmd(), a.sourcesLockCmd())
	return cmd
}

func parseTiers(s string) ([]string, error) {
	var out []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t == "all" {
			return sources.Tiers(), nil
		}
		if !slices.Contains(sources.Tiers(), t) {
			return nil, withCode(exitUsage, "unknown tier %q (required, recommended, optional, all)", t)
		}
		out = append(out, t)
	}
	return out, nil
}

func (a *app) sourcesCheckCmd() *cobra.Command {
	var indexPath, lockPath, packages, tierList string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate the registry and lock; with --packages, check coverage of the tracked packages",
		Long: `Check validates sources/index.json and sources/nsd.lock.json: schema,
official HTTPS locations only, tracked packages, namespace IDs, monitor
states, digests and redistribution decisions.

With --packages DIR it looks for every tracked package of the selected
tiers in DIR (by official name, with or without .zip, any case), loads
it, and checks that it carries the namespace release its name announces.
It also compares the packages' digests with the lock. Exit code 4 when a
package is missing or carries another release, or when digests differ;
3 when a package cannot be loaded or the files are invalid.

It does not contact any server.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tiers, err := parseTiers(tierList)
			if err != nil {
				return err
			}
			idx, err := sources.LoadIndex(indexPath)
			if err != nil {
				return err
			}
			lock, err := sources.LoadLock(lockPath)
			if err != nil {
				return err
			}
			rep := sourcesReport{Index: indexPath, Lock: lockPath, Entries: lock.Sources, Discovery: discoveryNote}
			rep.Diagnostics = append(idx.Validate(), lock.Validate()...)
			if rep.Entries == nil {
				rep.Entries = []sources.Entry{}
			}
			if packages != "" {
				if _, err := os.Stat(packages); err != nil {
					return &model.SourceError{Source: packages, Reason: "cannot open", Err: errors.Join(model.ErrSourceUnavailable, err)}
				}
				rep.Packages = a.survey(idx, packages, tiers)
				if rep.Verification, err = lock.Verify(packages); err != nil {
					return err
				}
			}
			w := cmd.OutOrStdout()
			if a.json {
				if err := writeJSON(w, rep); err != nil {
					return err
				}
			} else if err := printSourcesReport(w, rep); err != nil {
				return err
			}
			if rep.Diagnostics.HasErrors() {
				return withCode(exitValidation, "registry or lock is invalid")
			}
			var invalid, missing int
			for _, p := range rep.Packages {
				switch p.Status {
				case statusInvalid:
					invalid++
				case statusMissing, statusMismatch, statusNoSchema:
					missing++
				}
			}
			if invalid > 0 {
				return withCode(exitValidation, "%d tracked packages cannot be loaded", invalid)
			}
			if missing > 0 {
				return withCode(exitDifferent, "%d tracked packages are missing or carry another release", missing)
			}
			if v := rep.Verification; v != nil && len(v.Missing) > 0 {
				return withCode(exitDifferent, "local packages differ from the lock")
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&indexPath, "index", "sources/index.json", "Source registry")
	f.StringVar(&lockPath, "lock", "sources/nsd.lock.json", "Version lock")
	f.StringVar(&packages, "packages", "", "Directory holding the downloaded packages")
	f.StringVar(&tierList, "tiers", "all", "Tiers that must be present: required, recommended, optional or all (comma-separated)")
	return cmd
}

func printSourcesReport(w io.Writer, rep sourcesReport) error {
	printDiagnostics(w, rep.Diagnostics, model.SeverityInfo)
	if rep.Packages != nil {
		var rows [][]string
		for _, p := range rep.Packages {
			detail := strings.Join(p.Namespaces, ", ")
			switch p.Status {
			case statusMismatch:
				detail = "found " + orDash(strings.Join(p.Found, ", "))
			case statusInvalid:
				detail = p.Error
			case statusOK:
				if p.Kind == sources.KindSchema {
					detail = strconv.Itoa(p.Schemas) + " schema files"
				}
			}
			rows = append(rows, []string{p.Tier, p.Part, p.Name, p.Status, orDash(detail)})
		}
		if err := table(w, "TIER\tPART\tPACKAGE\tSTATUS\tNAMESPACES", rows); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(w, "%d locked namespaces\n", len(rep.Entries))
	if len(rep.Entries) > 0 {
		var rows [][]string
		for _, e := range rep.Entries {
			rows = append(rows, []string{e.Namespace, e.Status, orDash(e.Redistribution), orDash(short(e.SHA256))})
		}
		if err := table(w, "NAMESPACE\tSTATUS\tREDISTRIBUTION\tSHA256", rows); err != nil {
			return err
		}
	}
	if v := rep.Verification; v != nil {
		for _, m := range v.Missing {
			_, _ = fmt.Fprintf(w, "lock: no local file for %s\n", m)
		}
		if len(v.Unlocked) > 0 {
			_, _ = fmt.Fprintf(w, "lock: %d local files are not locked yet (nsdctl sources lock)\n", len(v.Unlocked))
		}
	}
	_, _ = fmt.Fprintln(w, "discovery", discoveryNote)
	return nil
}

func (a *app) sourcesLockCmd() *cobra.Command {
	var indexPath, lockPath, packages string
	var write bool
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Pin the tracked packages found in a directory into the lock",
		Long: `Lock loads every tracked package found in --packages and produces lock
entries for the namespaces it carries: namespace ID, content, SHA-256 of
the package file, dependencies and a status. A namespace whose profile
(with the other loaded packages) validates without errors gets
"nsd-validated", otherwise "nsd-available" with a note. Existing entries
of other namespaces are kept; the redistribution decision and source URL
of an existing entry are kept too, and new entries are "local-only".

Without --write the merged lock is printed; with --write it replaces the
lock file. Packages that are missing are left out.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if packages == "" {
				return withCode(exitUsage, "--packages is required")
			}
			idx, err := sources.LoadIndex(indexPath)
			if err != nil {
				return err
			}
			lock, err := sources.LoadLock(lockPath)
			if err != nil {
				return err
			}
			survey := a.survey(idx, packages, sources.Tiers())
			merged, err := mergeLock(lock, survey)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(merged, "", "  ")
			if err != nil {
				return err
			}
			data = append(data, '\n')
			w := cmd.OutOrStdout()
			if !write {
				_, err := w.Write(data)
				return err
			}
			if err := os.WriteFile(lockPath, data, 0o644); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(w, "wrote %s: %d entries\n", lockPath, len(merged.Sources))
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&indexPath, "index", "sources/index.json", "Source registry")
	f.StringVar(&lockPath, "lock", "sources/nsd.lock.json", "Version lock")
	f.StringVar(&packages, "packages", "", "Directory holding the downloaded packages (required)")
	f.BoolVar(&write, "write", false, "Write the lock file instead of printing it")
	return cmd
}

// mergeLock builds lock entries for the loaded packages and merges them
// into lock.
func mergeLock(lock *sources.Lock, survey []packageStatus) (*sources.Lock, error) {
	var srcs []catalog.Source
	for _, st := range survey {
		if st.pkg != nil {
			srcs = append(srcs, st.pkg)
		}
	}
	c, err := catalog.New(srcs...)
	if err != nil {
		return nil, &exitError{code: exitValidation, err: err}
	}
	byNS := map[string]sources.Entry{}
	for _, e := range lock.Sources {
		byNS[e.Namespace] = e
	}
	for _, st := range survey {
		if st.pkg == nil {
			continue
		}
		digest, err := sources.HashFile(st.File)
		if err != nil {
			return nil, err
		}
		if st.Kind == sources.KindSchema {
			ns := "IEC 61850-" + st.Part + ":" + st.Release
			e := byNS[ns]
			e.Standard, e.Namespace, e.Content, e.SHA256 = "IEC 61850-"+st.Part, ns, string(st.pkg.Content()), digest
			e.Status, e.Notes = sources.StateNSDAvailable, fmt.Sprintf("schema package, %d XSD files", st.Schemas)
			if e.Redistribution == "" {
				e.Redistribution = sources.RedistributionLocalOnly
			}
			byNS[ns] = e
			continue
		}
		for _, ns := range st.pkg.Namespaces() {
			e := byNS[ns.ID.String()]
			e.Standard, e.Namespace, e.SHA256 = ns.ID.Standard, ns.ID.String(), digest
			e.Content = string(ns.Provenance.Content)
			e.Dependencies = nil
			for _, d := range ns.Dependencies {
				e.Dependencies = append(e.Dependencies, d.String())
			}
			if e.Redistribution == "" {
				e.Redistribution = sources.RedistributionLocalOnly
			}
			e.Status, e.Notes = sources.StateNSDValidated, "validated by nsdctl sources lock"
			p, err := c.Profile(ns.ID)
			switch {
			case err != nil:
				e.Status, e.Notes = sources.StateNSDAvailable, "profile: "+err.Error()
			case p.Diagnostics().HasErrors():
				e.Status, e.Notes = sources.StateNSDAvailable, fmt.Sprintf("profile has %d errors (nsdctl validate)", len(p.Diagnostics().Errors()))
			}
			byNS[ns.ID.String()] = e
		}
	}
	out := &sources.Lock{SchemaVersion: sources.SchemaVersion}
	for _, e := range byNS {
		out.Sources = append(out.Sources, e)
	}
	sort.Slice(out.Sources, func(i, j int) bool { return out.Sources[i].Namespace < out.Sources[j].Namespace })
	if out.Sources == nil {
		out.Sources = []sources.Entry{}
	}
	return out, nil
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

type schemaReport struct {
	Source            string   `json:"source"`
	SHA256            string   `json:"sha256"`
	TargetNamespace   string   `json:"targetNamespace,omitempty"`
	NamespaceAccepted bool     `json:"namespaceAccepted"`
	Elements          int      `json:"elements"`
	Attributes        int      `json:"attributes"`
	MissingElements   []string `json:"missingElements,omitempty"`
	MissingAttributes []string `json:"missingAttributes,omitempty"`
}

func (a *app) schemaCmd() *cobra.Command {
	var exitOnGap bool
	cmd := &cobra.Command{
		Use:   "schema [SOURCE...]",
		Short: "Compare the NSD XML schemas (IEC 61850-7-7) of a package with what the importer models",
		Long: `Schema reads the XSD files of the given sources, for instance the
IEC 61850-7-7 package or the grammar files shipped in light packages, and
lists the elements and attributes they declare that the importer does not
model, and whether their target namespace is accepted. Content of such
constructs is reported as importer warnings, never dropped silently; this
command shows the gaps up front. --exit-code exits with 4 when there are
gaps.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgs, err := a.loadPackages(a.sourceArgs(args))
			if err != nil {
				return err
			}
			var reps []schemaReport
			for _, p := range pkgs {
				for _, s := range p.Schemas() {
					els, attrs := s.Gaps()
					reps = append(reps, schemaReport{
						Source: s.Source, SHA256: s.SHA256, TargetNamespace: s.TargetNamespace,
						NamespaceAccepted: s.TargetNamespace == nsd.SchemaNamespace2016,
						Elements:          len(s.Elements), Attributes: len(s.Attributes),
						MissingElements: els, MissingAttributes: attrs,
					})
				}
			}
			if len(reps) == 0 {
				return withCode(exitUsage, "no XSD files in the given sources")
			}
			w := cmd.OutOrStdout()
			gaps := 0
			for _, r := range reps {
				gaps += len(r.MissingElements) + len(r.MissingAttributes)
				if !r.NamespaceAccepted {
					gaps++
				}
			}
			if a.json {
				if err := writeJSON(w, reps); err != nil {
					return err
				}
			} else {
				for _, r := range reps {
					_, _ = fmt.Fprintf(w, "%s\n  target namespace  %s (accepted: %t)\n  declares          %d elements, %d attributes\n",
						r.Source, orDash(r.TargetNamespace), r.NamespaceAccepted, r.Elements, r.Attributes)
					_, _ = fmt.Fprintf(w, "  not modeled       elements: %s\n", orDash(strings.Join(r.MissingElements, ", ")))
					_, _ = fmt.Fprintf(w, "                    attributes: %s\n", orDash(strings.Join(r.MissingAttributes, ", ")))
				}
			}
			if exitOnGap && gaps > 0 {
				return withCode(exitDifferent, "%d schema constructs are not modeled", gaps)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&exitOnGap, "exit-code", false, "Exit with code 4 when the schemas declare constructs the importer does not model")
	return cmd
}
