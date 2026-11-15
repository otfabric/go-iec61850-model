// SPDX-License-Identifier: MIT

package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// FuzzGenerateGo generates the Go file of a namespace whose names are
// arbitrary strings. Whatever they hold, generation succeeds and the
// files declare what the model has and nothing else: a name reaches the
// source as an identifier, as a string constant or inside a comment.
func FuzzGenerateGo(f *testing.F) {
	p := plainNames()
	f.Add(p.standard, p.version, p.class, p.dataObject, p.cdcType, p.presence, p.enum, p.literal)
	f.Add("IEC 61850-7-4", "2007", "MMXU", "Hz", "MV", "O", "BehaviourModeKind", "on")
	f.Add("S\nvar Added = 1 //", "1", "C\nvar Added = 1 //", "D\nvar Added = 1 //", "T\nvar Added = 1 //", "P\nvar Added = 1 //", "E\nvar Added = 1 //", "l\nvar Added = 1 //")
	f.Add("S", "1", "C = \"x\"\n\tD", "D", "T", "P", "E", "l")
	f.Add("", "", "", "", "", "", "", "")
	f.Add("\x00", "\ufeff", "*/", "\u2028", "\xff", "`", "func", "type")
	f.Add("S", "1", "LNClass", "CDC", "DataObjectRef", "LNClasses", "String", "String")

	f.Fuzz(func(t *testing.T, standard, version, class, dataObject, cdcType, presence, enum, literal string) {
		if standard == "" {
			// A namespace needs an ID; the others may be empty.
			standard = "S"
		}
		n := plainNames()
		n.standard, n.version, n.class, n.dataObject, n.cdcType, n.presence, n.enum, n.literal = standard, version, class, dataObject, cdcType, presence, enum, literal
		n.base, n.presenceArgs, n.digest, n.license = class, literal, enum, dataObject
		files, err := Go(named(t, n), "names")
		if err != nil {
			t.Fatalf("generation failed: %v", err)
		}
		if got := declarations(t, GoFileName, files[GoFileName]); strings.Join(got, " ") != strings.Join(wantDeclarations, " ") {
			t.Fatalf("declarations %v, want %v\n%s", got, wantDeclarations, files[GoFileName])
		}
		if got := declarations(t, GoTestFileName, files[GoTestFileName]); strings.Join(got, " ") != strings.Join(wantTestDeclarations, " ") {
			t.Fatalf("test declarations %v, want %v", got, wantTestDeclarations)
		}
		// The names are in the file as string constants, unchanged.
		file, err := parser.ParseFile(token.NewFileSet(), GoFileName, files[GoFileName], 0)
		if err != nil {
			t.Fatal(err)
		}
		constants := map[string]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				constants[lit.Value] = true
			}
			return true
		})
		for _, want := range []string{class, class + "." + dataObject, literal} {
			if !constants[quote(want)] {
				t.Fatalf("the file has no string constant %q", want)
			}
		}
	})
}

// quote returns s as the generator writes a string constant, with %q.
func quote(s string) string { return fmt.Sprintf("%q", s) }
