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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/generator"
	"github.com/otfabric/go-iec61850-model/internal/sources"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/official"
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
		a.catalogCmd(), a.generateCmd(), a.sourcesCmd(), a.schemaCmd(), a.versionCmd(), noticeCmd())
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
			return nil, &exitError{code: exitSource, err: err}
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
			return nil, withCode(exitUsage, "--profile %s: the sources do not carry exactly one release of each of its parts", s)
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

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of nsdctl",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, _ := debug.ReadBuildInfo()
			v := currentVersion(info)
			out := cmd.OutOrStdout()
			if a.json {
				return writeJSON(out, v)
			}
			_, _ = fmt.Fprint(out, v.String())
			return nil
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
			return err
		},
	}
}

// devVersion is the version of a binary built without the ldflags of
// the Makefile and the release.
const devVersion = "dev"

// versionInfo is what nsdctl version reports.
type versionInfo struct {
	Version   string `json:"version"`
	Tag       string `json:"tag,omitempty"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"buildDate,omitempty"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// String formats the version in the layout of the other otfabric CLIs.
func (v versionInfo) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "nsdctl version: %s\n", v.Version)
	if v.Tag != "" {
		fmt.Fprintf(&b, "tag:         %s\n", v.Tag)
	}
	if v.Commit != "" {
		fmt.Fprintf(&b, "commit:      %s\n", v.Commit)
	}
	if v.BuildDate != "" {
		fmt.Fprintf(&b, "build date:  %s\n", v.BuildDate)
	}
	fmt.Fprintf(&b, "go version:  %s\n", v.GoVersion)
	fmt.Fprintf(&b, "platform:    %s\n", v.Platform)
	return b.String()
}

// currentVersion returns the version to report. The ldflags of the
// Makefile and of the release set it; a binary built without them
// (go install, go build) reports what the go toolchain records: the
// module version (v0.3.0 for go install …@v0.3.0, or one derived from
// the git tags for go build in a clone) and the git revision.
func currentVersion(info *debug.BuildInfo) versionInfo {
	v := versionInfo{
		Version: strings.TrimSpace(version), Tag: strings.TrimSpace(tag), Commit: strings.TrimSpace(commit),
		BuildDate: strings.TrimSpace(buildDate), GoVersion: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	if info != nil {
		if v.Version == devVersion || v.Version == "" {
			if mv := info.Main.Version; mv != "" && mv != "(devel)" {
				v.Version = mv
			}
		}
		if v.Commit == "" {
			for _, st := range info.Settings {
				if st.Key == "vcs.revision" {
					v.Commit = st.Value
				}
			}
		}
	}
	if len(v.Commit) > 8 {
		v.Commit = v.Commit[:8]
	}
	if v.Version == "" {
		v.Version = "(unknown)"
	}
	return v
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
