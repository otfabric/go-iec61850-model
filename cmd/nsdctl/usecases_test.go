// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"strings"
	"testing"
)

// TestUseCaseOutput re-runs the commands of docs/use-cases.md that have no
// pipe and checks that the documented output is still what nsdctl prints.
// A block cut with "…" at its end (or start) must be a prefix (or suffix).
func TestUseCaseOutput(t *testing.T) {
	data, err := os.ReadFile("../../docs/use-cases.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := strings.Split(string(data), "```console\n")[1:]
	checked := 0
	for _, b := range blocks {
		lines := strings.Split(strings.SplitN(b, "```", 2)[0], "\n")
		cmd := strings.TrimPrefix(lines[0], "$ ")
		if !strings.HasPrefix(cmd, "nsdctl ") || strings.Contains(cmd, "|") {
			continue
		}
		want := lines[1 : len(lines)-1]
		out, errOut, _ := nsdctl(t, shellWords(t, strings.TrimPrefix(cmd, "nsdctl "))...)
		got := strings.Split(strings.TrimRight(out+errOut, "\n"), "\n")
		switch {
		case len(want) > 0 && want[len(want)-1] == "…":
			want = want[:len(want)-1]
			if len(got) < len(want) {
				t.Errorf("%s: %d lines, want at least %d", cmd, len(got), len(want))
				continue
			}
			got = got[:len(want)]
		case len(want) > 0 && want[0] == "…":
			want = want[1:]
			if len(got) < len(want) {
				t.Errorf("%s: %d lines, want at least %d", cmd, len(got), len(want))
				continue
			}
			got = got[len(got)-len(want):]
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("docs/use-cases.md is out of date for %q:\n got:\n%s\nwant:\n%s", cmd, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		checked++
	}
	if checked < 10 {
		t.Errorf("only %d use-case commands checked", checked)
	}
}

// shellWords splits a command line on spaces, honouring single quotes.
func shellWords(t *testing.T, s string) []string {
	t.Helper()
	var out []string
	var cur strings.Builder
	quoted, have := false, false
	for _, r := range s {
		switch {
		case r == '\'':
			quoted, have = !quoted, true
		case r == ' ' && !quoted:
			if have {
				out = append(out, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteRune(r)
			have = true
		}
	}
	if quoted {
		t.Fatalf("unbalanced quote in %q", s)
	}
	if have {
		out = append(out, cur.String())
	}
	return out
}
