// SPDX-License-Identifier: MIT

package main

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/internal/sources"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
)

const (
	completionBash       = "bash"
	completionZsh        = "zsh"
	completionFish       = "fish"
	completionPowerShell = "powershell"
)

var completionShells = []string{completionBash, completionZsh, completionFish, completionPowerShell}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Output a completion script for the given shell. How to install it: the
"Shell completion" section of the go-iec61850-model README.

Flags with a fixed set of values (--profile, --ns, --kind, --fc, --class,
--left, --right, --tiers) complete to those values; --profile, --ns, --left
and --right to the profiles and namespaces of the embedded official catalog.
--source and NSD source arguments complete NSD, XSD and ZIP files, --lock and
--index JSON files, and --out, --dest and --packages directories.`,
		ValidArgs: completionShells,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, out := cmd.Root(), cmd.OutOrStdout()
			switch args[0] {
			case completionBash:
				return root.GenBashCompletionV2(out, true)
			case completionZsh:
				return root.GenZshCompletion(out)
			case completionFish:
				return root.GenFishCompletion(out, true)
			default:
				return root.GenPowerShellCompletion(out)
			}
		},
	}
}

// completeValues makes a flag complete to a fixed set of values.
func completeValues(cmd *cobra.Command, flag string, values func() []string) {
	_ = cmd.RegisterFlagCompletionFunc(flag, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return values(), cobra.ShellCompDirectiveNoFileComp
	})
}

func fixed(values ...string) func() []string { return func() []string { return values } }

// profileValues are the named profiles and the namespaces of the
// embedded catalog.
func profileValues() []string {
	var out []string
	for _, p := range official.Profiles() {
		out = append(out, p.Name)
	}
	for _, id := range official.Namespaces() {
		out = append(out, id.String())
	}
	return out
}

// namespaceValues are the namespaces of the embedded catalog, by ID and
// by part.
func namespaceValues() []string {
	var out []string
	for _, id := range official.Namespaces() {
		out = append(out, id.String())
		if i := strings.LastIndex(id.Standard, "61850-"); i >= 0 {
			out = append(out, id.Standard[i+len("61850-"):])
		}
	}
	return out
}

func fcValues() []string {
	var out []string
	for _, fc := range []model.FunctionalConstraint{
		model.FCST, model.FCMX, model.FCSP, model.FCSV, model.FCCF, model.FCDC, model.FCSG, model.FCSE, model.FCSR,
		model.FCOR, model.FCBL, model.FCEX, model.FCCO, model.FCUS, model.FCMS, model.FCRP, model.FCBR, model.FCLG, model.FCGO,
	} {
		out = append(out, string(fc))
	}
	return out
}

func searchKindValues() []string {
	var out []string
	for _, k := range catalog.Kinds() {
		out = append(out, string(k))
	}
	return out
}

func tierValues() []string { return append(sources.Tiers(), "all") }

func overlapClassValues() []string {
	return []string{string(catalog.OverlapIdentical), string(catalog.OverlapCompatible), string(catalog.OverlapConflicting)}
}

// sourceExts are the file types an NSD source can be (directories complete
// too).
var sourceExts = []string{"zip", "nsd", "snsd", "nsdoc", "appns", "xsd"}

// fileFlags are the flags that take a file or a directory, by name.
var fileFlags = map[string]cobra.CompletionFunc{
	"source":   fileExt(sourceExts...),
	"lock":     fileExt("json"),
	"index":    fileExt("json"),
	"out":      dirs,
	"dest":     dirs,
	"packages": dirs,
}

func fileExt(exts ...string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return exts, cobra.ShellCompDirectiveFilterFileExt
	}
}

func dirs(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

// registerCompletions completes what the commands do not complete
// themselves: flags that take files or directories, NSD sources as
// arguments, and no file names for every other flag and argument.
func registerCompletions(root *cobra.Command) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		each := func(f *pflag.Flag) {
			if _, ok := c.GetFlagCompletionFunc(f.Name); ok || f.Value.Type() == "bool" {
				return
			}
			fn, ok := fileFlags[f.Name]
			if !ok {
				fn = cobra.NoFileCompletions
			}
			_ = c.RegisterFlagCompletionFunc(f.Name, fn)
		}
		c.LocalFlags().VisitAll(each)
		c.PersistentFlags().VisitAll(each)
		if !c.HasSubCommands() && c.ValidArgsFunction == nil && len(c.ValidArgs) == 0 {
			if strings.Contains(c.Use, "SOURCE") || strings.Contains(c.Use, "OLD NEW") {
				c.ValidArgsFunction = fileExt(sourceExts...)
			} else {
				c.ValidArgsFunction = cobra.NoFileCompletions
			}
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(root)
}
