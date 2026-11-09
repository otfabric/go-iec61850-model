// SPDX-License-Identifier: MIT

// Command nsdctl inspects, validates and compares IEC 61850 NSD
// packages, explores the model they define, generates Go declarations,
// documentation and JSON exports from it, and checks the source lock.
//
// Without --source, the model and catalog commands use the official
// catalog embedded in the binary (package official). nsdctl never
// downloads anything. Output derived from packages you load is
// local-only unless the lock file permits publishing it (see
// docs/licensing.md).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/cmd/nsdctl/internal/buildinfo"
	"github.com/otfabric/go-iec61850-model/internal/generator"
	"github.com/otfabric/go-iec61850-model/internal/sources"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/official"
)

// Build metadata set at build time via -ldflags (Makefile,
// .github/workflows/release.yml); empty when not injected, and then
// resolved by buildinfo.
var (
	version   = ""
	tag       = ""
	commit    = ""
	buildDate = ""
)

// build is the resolved build information of this binary.
func build() buildinfo.Info {
	return buildinfo.Resolve("nsdctl", version, tag, commit, buildDate, "github.com/otfabric/go-iec61850-model")
}

// Exit codes: the organisation table (docs/exitcodes.md).
const (
	exitOK           = 0   // success
	exitFindings     = 1   // the command completed and reports problems: differences, validation errors, gaps
	exitUsage        = 2   // invalid flags or arguments, unknown name, refused overwrite without --force
	exitInvalidInput = 3   // an input (NSD package, registry, lock) cannot be read or is invalid
	exitTransport    = 4   // a download failed
	exitPermission   = 9   // the license policy refuses publishing
	exitInternal     = 10  // writing the output failed, or another unexpected error
	exitInterrupted  = 130 // SIGINT or SIGTERM before a result
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

// outputError marks a failure to write the result.
func outputError(err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: exitInternal, err: fmt.Errorf("write output: %w", err)}
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
	case errors.Is(err, context.Canceled):
		return exitInterrupted
	case errors.Is(err, generator.ErrPolicyRefused):
		return exitPermission
	case errors.Is(err, generator.ErrWouldOverwrite):
		return exitUsage
	case errors.As(err, &ve), errors.As(err, &de),
		errors.Is(err, model.ErrSourceUnavailable), errors.Is(err, model.ErrInvalidNSD),
		errors.Is(err, model.ErrUnsupportedSchema), errors.Is(err, model.ErrLimitExceeded),
		errors.Is(err, model.ErrUnsafeArchive):
		return exitInvalidInput
	case errors.Is(err, model.ErrUnknownNamespace), errors.Is(err, model.ErrUnknownLogicalNode),
		errors.Is(err, model.ErrUnknownDataObject), errors.Is(err, model.ErrUnknownCDC),
		errors.Is(err, model.ErrUnknownAttribute), errors.Is(err, model.ErrUnknownType),
		errors.Is(err, model.ErrInvalidReference), errors.Is(err, model.ErrAmbiguousReference),
		errors.Is(err, model.ErrNoMapping):
		return exitUsage
	default:
		return exitInternal
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run executes nsdctl with args and returns the exit code. A command
// that fails after ctx is cancelled (SIGINT, SIGTERM) exits with
// exitInterrupted.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return exitOK
	}
	_, _ = fmt.Fprintln(stderr, "nsdctl: "+err.Error())
	if ctx.Err() != nil {
		return exitInterrupted
	}
	return exitCode(err)
}

// usageErrors makes the flag and argument errors of cobra usage errors.
func usageErrors(c *cobra.Command) {
	c.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &exitError{code: exitUsage, err: err} })
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if args := c.Args; args != nil {
			c.Args = func(cmd *cobra.Command, a []string) error {
				if err := args(cmd, a); err != nil {
					var ee *exitError
					if errors.As(err, &ee) {
						return err
					}
					return &exitError{code: exitUsage, err: err}
				}
				return nil
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(c)
}

// app holds the global flags. newRootCmd creates a fresh one, so tests
// do not share state.
type app struct {
	json     bool
	sources  []string
	profile  []string
	strict   bool
	language string
	ns       string
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

Without --source, the model and catalog commands use the official IEC
namespaces embedded in nsdctl (the default profile: IEC 61850-7-2, 7-3,
7-4, 7-410 and 7-420; --profile extended adds the technical reports).

nsdctl never downloads IEC packages. Output derived from packages you
load is local-only unless the lock file permits publishing it.

Exit codes: 0 ok, 1 findings (differences, validation errors), 2 usage,
3 invalid input, 4 download failed, 9 refused by the license policy,
10 output or internal error, 130 interrupted.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&a.json, "json", false, "Write machine-readable JSON")
	pf.StringArrayVarP(&a.sources, "source", "s", nil, "NSD source: ZIP archive, directory or document (repeatable)")
	pf.StringArrayVarP(&a.profile, "profile", "p", nil, "Profile name (default, extended) or namespace to select, e.g. \"IEC 61850-7-4:2007B5\" (repeatable; default: the default profile)")
	completeValues(root, "profile", profileValues)
	pf.BoolVar(&a.strict, "strict", false, "Treat importer warnings as errors")
	pf.StringVar(&a.language, "lang", "en", "NSDoc language to apply (empty: none)")

	root.AddCommand(a.inspectCmd(), a.validateCmd(), a.dependenciesCmd(), a.modelCmd(), a.diffCmd(),
		a.catalogCmd(), a.generateCmd(), a.sourcesCmd(), a.schemaCmd(), a.versionCmd(), noticeCmd(), completionCmd())
	helpWithoutSubcommand(root)
	showHelpOfGroups(root)
	usageErrors(root)
	registerCompletions(root)
	return root
}

// showHelpOfGroups makes command groups (model, catalog, generate,
// sources) print their help when run without a subcommand, instead of
// nothing.
func showHelpOfGroups(c *cobra.Command) {
	for _, sub := range c.Commands() {
		if sub.HasSubCommands() && sub.Run == nil && sub.RunE == nil {
			helpWithoutSubcommand(sub)
		}
		showHelpOfGroups(sub)
	}
}

// helpWithoutSubcommand makes c print its help when run without a
// subcommand, and an unknown subcommand a usage error.
func helpWithoutSubcommand(c *cobra.Command) {
	c.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return withCode(exitUsage, "unknown command %q for %q; run \"%s --help\"", args[0], cmd.CommandPath(), cmd.CommandPath())
		}
		return nil
	}
	c.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
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
		return nil, nil, &exitError{code: exitInvalidInput, err: err}
	}
	return c, pkgs, nil
}

// loadProfile builds the profile selected by --profile from --source,
// or from the embedded official catalog without --source.
func (a *app) loadProfile() (*catalog.Profile, error) {
	c, err := a.modelCatalog()
	if err != nil {
		return nil, err
	}
	return a.selectProfile(c)
}

// modelCatalog loads --source, or the embedded official catalog when no
// source is given.
func (a *app) modelCatalog() (*catalog.Catalog, error) {
	if len(a.sources) == 0 {
		c, err := official.Catalog()
		if err != nil {
			return nil, &exitError{code: exitInternal, err: err}
		}
		return c, nil
	}
	c, _, err := a.loadCatalog(a.sources)
	return c, err
}

// namespaceSet is a catalog or profile whose namespaces can be named.
type namespaceSet interface {
	IDs() []model.NamespaceID
	ResolveNamespace(id model.NamespaceID) (model.NamespaceID, error)
}

// resolveNamespace resolves a namespace argument: a full or partial
// namespace ID ("IEC 61850-7-420:2019A4", "IEC 61850-7-420"), the
// package spelling ("IEC_61850-7-420", "IEC_TR_61850-90-9") or the part
// alone ("7-420").
func resolveNamespace(set namespaceSet, flag, s string) (model.NamespaceID, error) {
	id, err := model.ParseNamespaceID(strings.ReplaceAll(s, "_", " "))
	if err != nil {
		return model.NamespaceID{}, withCode(exitUsage, "%s %q: %v", flag, s, err)
	}
	got, err := set.ResolveNamespace(id)
	if errors.Is(err, model.ErrUnknownNamespace) {
		// Another spelling of the standard: match on the part.
		part := id.Standard
		if i := strings.Index(part, "61850-"); i >= 0 {
			part = part[i+len("61850-"):]
		}
		var standards []string
		for _, ns := range set.IDs() {
			if sources.MatchesPart(ns.Standard, part) && !slices.Contains(standards, ns.Standard) {
				standards = append(standards, ns.Standard)
			}
		}
		if len(standards) == 1 {
			id.Standard = standards[0]
			got, err = set.ResolveNamespace(id)
		}
	}
	if err != nil {
		return model.NamespaceID{}, withCode(exitUsage, "%s %q: %v", flag, s, err)
	}
	return got, nil
}

func (a *app) selectProfile(c *catalog.Catalog) (*catalog.Profile, error) {
	if len(a.profile) == 0 {
		// The named default profile, when the sources carry its parts;
		// otherwise the only release of each loaded standard.
		if ids, ok := namedProfile(c, "default"); ok {
			return c.Profile(ids...)
		}
		p, err := c.DefaultProfile()
		if err != nil {
			return nil, withCode(exitUsage, "%v; select releases with --profile", err)
		}
		return p, nil
	}
	var ids []model.NamespaceID
	for _, s := range a.profile {
		if named, ok := namedProfile(c, s); ok {
			ids = append(ids, named...)
			continue
		}
		if _, known := profileParts(s); known {
			return nil, withCode(exitUsage, "--profile %s: the sources do not carry a release of each of its parts", s)
		}
		id, err := model.ParseNamespaceID(s)
		if err != nil {
			return nil, withCode(exitUsage, "--profile %q: %v", s, err)
		}
		ids = append(ids, id)
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

// namedProfile resolves a profile name (default, extended) to the loaded
// namespaces of its parts.
func namedProfile(c *catalog.Catalog, name string) ([]model.NamespaceID, bool) {
	parts, ok := profileParts(name)
	if !ok {
		return nil, false
	}
	ids, err := sources.NamespacesForParts(c.Namespaces(), parts)
	if err != nil {
		return nil, false
	}
	return ids, true
}

// writeJSON writes v as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return outputError(enc.Encode(v))
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
	return outputError(tw.Flush())
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

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of nsdctl",
		Long: `Print the version, tag, commit, build date, Go version and platform on one
line, or with --json as one line of JSON.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b := build()
			line := b.String()
			if a.json {
				line = b.JSON()
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), line)
			return outputError(err)
		},
	}
}

func noticeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "notice",
		Short: "Print the IEC attribution and notices of the embedded official catalog",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), official.Notice())
			return outputError(err)
		},
	}
}

// profileParts returns the parts of a named profile: from the source
// registry when nsdctl runs in the repository, otherwise from the
// profiles embedded with the official catalog.
func profileParts(name string) ([]string, bool) {
	if idx, err := sources.LoadIndex(registryPath); err == nil {
		parts, err := idx.ProfileParts(name)
		return parts, err == nil
	}
	if p, ok := official.Lookup(name); ok {
		return p.Parts, true
	}
	return nil, false
}

// registryPath is the source registry nsdctl reads named profiles from.
var registryPath = "sources/index.json"
