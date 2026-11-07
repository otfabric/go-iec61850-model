// SPDX-License-Identifier: MIT

package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestCompletion(t *testing.T) {
	for shell, want := range map[string]string{
		"bash": "__start_nsdctl", "zsh": "#compdef nsdctl", "fish": "complete -c nsdctl", "powershell": "Register-ArgumentCompleter",
	} {
		out, e, code := nsdctl(t, "completion", shell)
		if code != exitOK || !strings.Contains(out, want) {
			t.Errorf("completion %s: %d %s\n%.200s", shell, code, e, out)
		}
	}
	for _, args := range [][]string{{"completion"}, {"completion", "tcsh"}} {
		if _, _, code := nsdctl(t, args...); code != exitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}

func TestFlagValueCompletion(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"__complete", "-p", ""}, []string{"default", "extended", "IEC 61850-7-420:2019A4"}},
		{[]string{"__complete", "model", "show", "DBAT", "--ns", ""}, []string{"(Tr)IEC 61850-90-9:2018A3", "90-9", "7-420"}},
		{[]string{"__complete", "model", "list", "--kind", ""}, []string{"ln", "abstract", "basic"}},
		{[]string{"__complete", "model", "search", "--kind", ""}, []string{"do", "da"}},
		{[]string{"__complete", "model", "search", "--fc", ""}, []string{"MX", "ST", "CO"}},
		{[]string{"__complete", "model", "leaves", "MMXU", "--fc", ""}, []string{"MX"}},
		{[]string{"__complete", "catalog", "overlaps", "--class", ""}, []string{"identical", "compatible-extension", "conflicting"}},
		{[]string{"__complete", "catalog", "overlaps", "--left", ""}, []string{"7-420"}},
		{[]string{"__complete", "catalog", "overlaps", "--right", ""}, []string{"90-9"}},
		{[]string{"__complete", "sources", "fetch", "--tiers", ""}, []string{"required", "all"}},
		{[]string{"__complete", "sources", "check", "--tiers", ""}, []string{"optional"}},
		{[]string{"__complete", "completion", ""}, []string{"bash", "zsh", "fish", "powershell"}},
	}
	for _, tc := range tests {
		out, e, code := nsdctl(t, tc.args...)
		if code != exitOK {
			t.Errorf("%v: exit %d %s", tc.args, code, e)
		}
		lines := strings.Split(out, "\n")
		for _, w := range tc.want {
			found := false
			for _, l := range lines {
				found = found || l == w
			}
			if !found {
				t.Errorf("%v: no %q in\n%s", tc.args, w, out)
			}
		}
	}
}

func TestCommandGroups(t *testing.T) {
	for _, group := range []string{"model", "catalog", "sources", "generate"} {
		if out, _, code := nsdctl(t, group); code != exitOK || !strings.Contains(out, "Available Commands:") {
			t.Errorf("%s without a subcommand: %d\n%s", group, code, out)
		}
		if _, e, code := nsdctl(t, group, "frob"); code != exitUsage || !strings.Contains(e, "unknown command") {
			t.Errorf("%s frob: %d %s", group, code, e)
		}
	}
	if _, e, code := nsdctl(t, "model", "list", "MMXU"); code != exitUsage || !strings.Contains(e, `nsdctl model show MMXU`) {
		t.Errorf("model list MMXU: %d %s", code, e)
	}
}

// TestFileAndFreeCompletion: flags and arguments that take files complete
// files of their type or directories; free values offer no file names.
func TestFileAndFreeCompletion(t *testing.T) {
	tests := []struct {
		args      []string
		want      []string
		directive string
	}{
		{[]string{"__complete", "--source", ""}, sourceExts, ":8"},
		{[]string{"__complete", "inspect", ""}, sourceExts, ":8"},
		{[]string{"__complete", "diff", ""}, sourceExts, ":8"},
		{[]string{"__complete", "generate", "go", "--lock", ""}, []string{"json"}, ":8"},
		{[]string{"__complete", "sources", "check", "--index", ""}, []string{"json"}, ":8"},
		{[]string{"__complete", "generate", "docs", "-o", ""}, nil, ":16"},
		{[]string{"__complete", "sources", "fetch", "--dest", ""}, nil, ":16"},
		{[]string{"__complete", "model", "search", "--name", ""}, nil, ":4"},
		{[]string{"__complete", "--lang", ""}, nil, ":4"},
		{[]string{"__complete", "version", ""}, nil, ":4"},
	}
	for _, tc := range tests {
		out, e, code := nsdctl(t, tc.args...)
		if code != exitOK {
			t.Fatalf("%v: exit %d %s", tc.args, code, e)
		}
		var got []string
		dir := ""
		for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.HasPrefix(l, ":") {
				dir = l
			} else if l != "" {
				got = append(got, strings.SplitN(l, "\t", 2)[0])
			}
		}
		if !slices.Equal(got, tc.want) || dir != tc.directive {
			t.Errorf("%v: got %v %s, want %v %s", tc.args, got, dir, tc.want, tc.directive)
		}
	}
}

// TestEveryFlagCompletes: every flag that takes a value has a completion
// function (its value set, files of its type, or no file names).
func TestEveryFlagCompletes(t *testing.T) {
	var check func(c *cobra.Command)
	check = func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Value.Type() == "bool" {
				return
			}
			if _, ok := c.GetFlagCompletionFunc(f.Name); !ok {
				t.Errorf("%s --%s (%s) has no completion", c.CommandPath(), f.Name, f.Usage)
			}
		})
		for _, s := range c.Commands() {
			check(s)
		}
	}
	check(newRootCmd())
}
