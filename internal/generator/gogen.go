// SPDX-License-Identifier: MIT

package generator

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/token"
	"strconv"
	"strings"
	"unicode"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
)

// GoFileName is the name of the generated Go file.
const GoFileName = "names_gen.go"

// idents assigns Go identifiers and resolves collisions explicitly:
// a name that sanitizes to an identifier already taken gets a numeric
// suffix, and every such rename is listed in the file header.
type idents struct {
	taken   map[string]bool
	renames []string
}

func newIdents(reserved ...string) *idents {
	id := &idents{taken: map[string]bool{}}
	for _, r := range reserved {
		id.taken[r] = true
	}
	return id
}

// sanitize maps a model name to identifier characters.
func sanitize(s string) string {
	if s == "" {
		return "Empty"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)), r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// assign returns a unique exported identifier for prefix+name.
func (id *idents) assign(prefix, name string) string {
	base := prefix + sanitize(name)
	if r := rune(base[0]); !unicode.IsUpper(r) {
		base = "X" + base
	}
	if token.Lookup(base).IsKeyword() {
		base += "_"
	}
	ident := base
	for n := 2; id.taken[ident]; n++ {
		ident = base + "_" + strconv.Itoa(n)
	}
	if ident != prefix+name {
		id.renames = append(id.renames, fmt.Sprintf("%q -> %s", prefix+name, ident))
	}
	id.taken[ident] = true
	return ident
}

// Go generates one Go file with typed constants for the profile:
// logical node classes, CDCs, data object references (including
// inherited data objects) and enumeration types with their literals.
// The output is gofmt-formatted and deterministic.
func Go(in Input, pkg string) (Files, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	if !token.IsIdentifier(pkg) {
		return nil, fmt.Errorf("generator: %q is not a valid package name", pkg)
	}
	p := in.Profile
	ids := newIdents("LNClass", "CDC", "DataObjectRef", "LNClasses", "CDCs")
	var body bytes.Buffer
	w := func(format string, args ...any) { fmt.Fprintf(&body, format, args...) }

	lns := p.ListLogicalNodes()
	w("// LNClass is the name of a logical node class.\ntype LNClass string\n\n")
	w("// Logical node classes.\nconst (\n")
	lnIdents := make([]string, len(lns))
	for i, ln := range lns {
		lnIdents[i] = ids.assign("LN_", ln.Name)
		w("\t%s LNClass = %q\n", lnIdents[i], ln.Name)
	}
	w(")\n\n// LNClasses lists the logical node classes in name order.\nvar LNClasses = []LNClass{")
	w("%s}\n\n", strings.Join(lnIdents, ", "))

	// One constant per CDC name; variants (SPG/SP, SPG/SG, ...) share it.
	var cdcNames []string
	for _, c := range p.ListCDCs() {
		if len(cdcNames) == 0 || cdcNames[len(cdcNames)-1] != c.Name {
			cdcNames = append(cdcNames, c.Name)
		}
	}
	w("// CDC is the name of a common data class.\ntype CDC string\n\n")
	w("// Common data classes.\nconst (\n")
	cdcIdents := make([]string, len(cdcNames))
	for i, name := range cdcNames {
		cdcIdents[i] = ids.assign("CDC_", name)
		w("\t%s CDC = %q\n", cdcIdents[i], name)
	}
	w(")\n\n// CDCs lists the common data classes in name order.\nvar CDCs = []CDC{%s}\n\n", strings.Join(cdcIdents, ", "))

	w("// DataObjectRef is a class-level data object reference, e.g. \"MMXU.Hz\".\ntype DataObjectRef string\n\n")
	var notes []string
	for _, ln := range lns {
		dos, err := dataObjects(p, ln)
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s (%s): %v; only its own data objects are declared", ln.Name, ln.Source, err))
		}
		if len(dos) == 0 {
			continue
		}
		w("// Data objects of %s (%s), including inherited ones.\nconst (\n", ln.Name, ln.Source)
		for _, do := range dos {
			w("\t%s DataObjectRef = %q // %s, %s\n", ids.assign(ln.Name+"_", do.Name), ln.Name+"."+do.Name, do.Type, presence(do.Presence))
		}
		w(")\n\n")
	}

	enums := p.ListEnumerations()
	for _, e := range enums {
		typ := ids.assign("", e.Name)
		w("// %s is the enumeration %s.\ntype %s int32\n\n", typ, e.Name, typ)
		if len(e.Literals) == 0 {
			continue
		}
		w("// Literals of %s.\nconst (\n", e.Name)
		names := make([]string, len(e.Literals))
		for i, l := range e.Literals {
			names[i] = ids.assign(typ+"_", l.Name)
			w("\t%s %s = %d\n", names[i], typ, l.Value)
		}
		w(")\n\n")
		w("// String returns the literal name, or %s(n) for an unknown value.\n", typ)
		w("func (v %s) String() string {\n\tswitch v {\n", typ)
		for i, l := range e.Literals {
			w("\tcase %s:\n\t\treturn %q\n", names[i], l.Name)
		}
		w("\tdefault:\n\t\treturn %q + strconv.Itoa(int(v)) + \")\"\n\t}\n}\n\n", typ+"(")
	}

	var out bytes.Buffer
	for _, l := range in.headerLines("go") {
		out.WriteString(strings.TrimRight("// "+l, " ") + "\n")
	}
	if len(notes) > 0 {
		out.WriteString("//\n// Ambiguous names (see the profile diagnostics):\n")
		for _, n := range notes {
			out.WriteString("//   " + n + "\n")
		}
	}
	if len(ids.renames) > 0 {
		out.WriteString("//\n// Renamed identifiers:\n")
		for _, r := range ids.renames {
			out.WriteString("//   " + r + "\n")
		}
	}
	fmt.Fprintf(&out, "\n// Package %s declares the names of an IEC 61850 namespace profile.\npackage %s\n\n", pkg, pkg)
	if hasLiterals(enums) {
		out.WriteString("import \"strconv\"\n\n")
	}
	out.Write(body.Bytes())
	src, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("generator: formatting generated Go: %w", err)
	}
	files := Files{GoFileName: src}
	in.addNotice(files, "go")
	if !in.Publish {
		files["LOCAL-ONLY.md"] = in.localNotice("go")
	}
	return files, nil
}

func hasLiterals(enums []*model.Enumeration) bool {
	for _, e := range enums {
		if len(e.Literals) > 0 {
			return true
		}
	}
	return false
}

func presence(p model.PresenceCondition) string {
	if p.IsZero() {
		return "-"
	}
	return p.String()
}

// dataObjects returns the data objects of a class including inherited
// ones. When a name on the inheritance chain is ambiguous in the
// profile, it falls back to the data objects the class declares itself
// and returns the error as a note.
func dataObjects(p *catalog.Profile, ln *model.LogicalNodeClass) ([]catalog.InheritedDataObject, error) {
	dos, err := p.DataObjects(ln.Name)
	if err == nil {
		return dos, nil
	}
	if !errors.Is(err, model.ErrAmbiguousReference) {
		return nil, err
	}
	own := make([]catalog.InheritedDataObject, len(ln.DataObjects))
	for i := range ln.DataObjects {
		own[i] = catalog.InheritedDataObject{DataObject: &ln.DataObjects[i], DeclaredIn: ln}
	}
	return own, err
}
