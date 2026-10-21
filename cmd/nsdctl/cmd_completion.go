// SPDX-License-Identifier: MIT

package main

import (
	"strings"

	"github.com/spf13/cobra"

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
		Long: `Output a completion script for the given shell.

  Bash:        source <(nsdctl completion bash)
  Zsh:         source <(nsdctl completion zsh)
  Fish:        nsdctl completion fish | source
  PowerShell:  nsdctl completion powershell | Out-String | Invoke-Expression

Flags with a fixed set of values (--profile, --ns, --kind, --fc, --class,
--tiers) complete to those values; --profile and --ns to the profiles and
namespaces of the embedded official catalog.`,
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
