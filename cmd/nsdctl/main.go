// SPDX-License-Identifier: MIT

// Command nsdctl inspects, validates and compares IEC 61850 NSD
// packages, explores the model they define, generates Go declarations,
// documentation and JSON exports from it, and checks the source lock.
//
// It reads official packages you obtained yourself and never downloads
// anything. Output derived from them is local-only unless the lock file
// permits publishing it (see docs/licensing.md).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/generator"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// Build metadata set at build time via -ldflags.
var (
	version   = "dev"
	tag       = ""
	commit    = ""
	buildDate = ""
)

// Exit codes.
const (
	exitOK         = 0 // success
	exitUsage      = 1 // usage error, unknown name, other failure
	exitSource     = 2 // a source cannot be read or parsed
	exitValidation = 3 // validation, dependency or lock errors
	exitDifferent  = 4 // diff --exit-code found changes; generate --check is out of date; packages differ from the lock
	exitPolicy     = 5 // the license policy refuses publishing
)

// exitError carries an explicit exit code.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func withCode(code int, format string, args ...any) error {
	return &exitError{code: code, err: fmt.Errorf(format, args...)}
}

// exitCode maps an error to the exit code of the command.
func exitCode(err error) int {
	var ee *exitError
	var ve *model.ValidationError
	var de *model.DependencyError
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &ee):
		return ee.code
	case errors.Is(err, generator.ErrPolicyRefused):
		return exitPolicy
	case errors.As(err, &ve), errors.As(err, &de):
		return exitValidation
	case errors.Is(err, model.ErrSourceUnavailable), errors.Is(err, model.ErrInvalidNSD),
		errors.Is(err, model.ErrUnsupportedSchema), errors.Is(err, model.ErrLimitExceeded),
		errors.Is(err, model.ErrUnsafeArchive):
		return exitSource
	default:
		return exitUsage
	}
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run executes nsdctl with args and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.Execute()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "nsdctl: "+err.Error())
	}
	return exitCode(err)
}

// app holds the global flags. newRootCmd creates a fresh one, so tests
// do not share state.
type app struct {
	json     bool
	sources  []string
	profile  []string
	strict   bool
	language string
}

func newRootCmd() *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:   "nsdctl",
		Short: "Inspect, validate and generate from IEC 61850 NSD namespace packages",
		Long: `nsdctl works with IEC 61850 namespace definitions (NSD, IEC 61850-7-7).

It inspects and validates NSD packages, resolves their dependencies,
explores the model (logical node classes, data objects, CDCs, types),
compares releases, generates Go declarations, documentation and JSON
exports, and checks the source lock.

nsdctl never downloads IEC packages. Output derived from them is
local-only unless the lock file permits publishing it.

Exit codes: 0 ok, 1 usage or unknown name, 2 source error, 3 validation
error, 4 differences found, 5 refused by the license policy.`,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&a.json, "json", false, "Write machine-readable JSON")
	pf.StringArrayVarP(&a.sources, "source", "s", nil, "NSD source: ZIP archive, directory or document (repeatable)")
	pf.StringArrayVarP(&a.profile, "profile", "p", nil, "Namespace to select, e.g. \"IEC 61850-7-4:2007B5\" (repeatable; default: the only release of each standard)")
	pf.BoolVar(&a.strict, "strict", false, "Treat importer warnings as errors")
	pf.StringVar(&a.language, "lang", "en", "NSDoc language to apply (empty: none)")

	root.AddCommand(a.inspectCmd(), a.validateCmd(), a.dependenciesCmd(), a.modelCmd(), a.diffCmd(),
		a.generateCmd(), a.sourcesCmd(), a.schemaCmd(), versionCmd())
	return root
}

func (a *app) options(source string) []nsd.Option {
	opts := []nsd.Option{nsd.WithLanguage(a.language)}
	if source != "" {
		opts = append(opts, nsd.WithSource(source))
	}
	if a.strict {
		opts = append(opts, nsd.WithStrict())
	}
	return opts
}

// loadPackage loads one source: a directory, a ZIP archive or a
// document.
func (a *app) loadPackage(path string) (*nsd.Package, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, &model.SourceError{Source: path, Reason: "cannot open", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	if st.IsDir() {
		return nsd.LoadDir(path, a.options("")...)
	}
	return nsd.LoadFile(path, a.options(path)...)
}

func (a *app) loadPackages(paths []string) ([]*nsd.Package, error) {
	if len(paths) == 0 {
		return nil, withCode(exitUsage, "no source given (use --source or arguments)")
	}
	var out []*nsd.Package
	for _, p := range paths {
		pkg, err := a.loadPackage(p)
		if err != nil {
			return nil, err
		}
		out = append(out, pkg)
	}
	return out, nil
}

func (a *app) loadCatalog(paths []string) (*catalog.Catalog, []*nsd.Package, error) {
	pkgs, err := a.loadPackages(paths)
	if err != nil {
		return nil, nil, err
	}
	srcs := make([]catalog.Source, len(pkgs))
	for i, p := range pkgs {
		srcs[i] = p
	}
	c, err := catalog.New(srcs...)
	if err != nil {
		return nil, nil, &exitError{code: exitValidation, err: err}
	}
	return c, pkgs, nil
}

// loadProfile builds the profile selected by --profile from --source.
func (a *app) loadProfile() (*catalog.Profile, error) {
	c, _, err := a.loadCatalog(a.sources)
	if err != nil {
		return nil, err
	}
	return a.selectProfile(c)
}

func (a *app) selectProfile(c *catalog.Catalog) (*catalog.Profile, error) {
	if len(a.profile) == 0 {
		p, err := c.DefaultProfile()
		if err != nil {
			return nil, withCode(exitUsage, "%v; select releases with --profile", err)
		}
		return p, nil
	}
	ids := make([]model.NamespaceID, len(a.profile))
	for i, s := range a.profile {
		id, err := model.ParseNamespaceID(s)
		if err != nil {
			return nil, withCode(exitUsage, "--profile %q: %v", s, err)
		}
		ids[i] = id
	}
	p, err := c.Profile(ids...)
	if err != nil {
		var de *model.DependencyError
		if errors.As(err, &de) {
			return nil, err
		}
		return nil, withCode(exitUsage, "%v", err)
	}
	return p, nil
}

// writeJSON writes v as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// table writes rows separated by tabs, aligned.
func table(w io.Writer, header string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if header != "" {
		_, _ = fmt.Fprintln(tw, header)
	}
	for _, r := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// printDiagnostics writes diagnostics of at least severity min.
func printDiagnostics(w io.Writer, ds model.Diagnostics, min model.Severity) {
	for _, d := range ds.AtLeast(min) {
		_, _ = fmt.Fprintln(w, "  "+d.String())
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), versionString())
		},
	}
}

func versionString() string {
	s := "nsdctl " + version
	if tag != "" {
		s += " (" + tag + ")"
	}
	if commit != "" {
		c := commit
		if len(c) > 8 {
			c = c[:8]
		}
		s += " commit " + c
	}
	if buildDate != "" {
		s += " built " + buildDate
	}
	return s
}
