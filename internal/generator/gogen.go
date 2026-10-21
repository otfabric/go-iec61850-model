// SPDX-License-Identifier: MIT

package generator

import (
	"bytes"
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

	classes := groupClasses(p.ListLogicalNodes())
	w("// LNClass is the name of a logical node class.\ntype LNClass string\n\n")
	w("// Logical node classes.\nconst (\n")
	lnIdents := make([]string, len(classes))
	for i, c := range classes {
		lnIdents[i] = ids.assign("LN_", c.name)
		w("\t%s LNClass = %q\n", lnIdents[i], c.name)
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
	for _, c := range classes {
		refs, note := classDataObjects(p, c)
		if note != "" {
			notes = append(notes, note)
		}
		if len(refs) == 0 {
			continue
		}
		w("// Data objects of %s (%s), including inherited ones.\nconst (\n", c.name, c.sources())
		for _, ref := range refs {
			w("\t%s DataObjectRef = %q // %s\n", ids.assign(c.name+"_", ref.name), c.name+"."+ref.name, strings.Join(ref.notes, "; "))
		}
		w(")\n\n")
	}

	enums, enumNotes := splitEnumerations(p.ListEnumerations())
	notes = append(notes, enumNotes...)
	var tests []enumTest
	for _, e := range enums {
		typ := ids.assign("", e.typeName)
		w("// %s is the enumeration %s (%s).\ntype %s int32\n\n", typ, e.name, e.source, typ)
		if len(e.literals) == 0 {
			continue
		}
		w("// Literals of %s.\nconst (\n", e.name)
		names := make([]string, len(e.literals))
		for i, l := range e.literals {
			names[i] = ids.assign(typ+"_", l.Name)
			w("\t%s %s = %d\n", names[i], typ, l.Value)
		}
		w(")\n\n")
		w("// String returns the literal name, or %s(n) for an unknown value.\n", typ)
		w("func (v %s) String() string {\n\tswitch v {\n", typ)
		seen := map[int]bool{}
		maxV := e.literals[0].Value
		for i, l := range e.literals {
			maxV = max(maxV, l.Value)
			if seen[l.Value] {
				// Another literal has the same value; String reports the first.
				continue
			}
			seen[l.Value] = true
			w("\tcase %s:\n\t\treturn %q\n", names[i], l.Name)
			tests = append(tests, enumTest{expr: names[i] + ".String()", want: l.Name})
		}
		w("\tdefault:\n\t\treturn %q + strconv.Itoa(int(v)) + \")\"\n\t}\n}\n\n", typ+"(")
		unknown := maxV + 1
		tests = append(tests, enumTest{expr: fmt.Sprintf("%s(%d).String()", typ, unknown), want: fmt.Sprintf("%s(%d)", typ, unknown)})
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
	test, err := goTest(pkg, len(classes), len(cdcNames), tests)
	if err != nil {
		return nil, err
	}
	files := Files{GoFileName: src, GoTestFileName: test}
	in.addNotice(files, "go")
	if !in.Publish {
		files["LOCAL-ONLY.md"] = in.localNotice("go")
	}
	return files, nil
}

func hasLiterals(enums []enumType) bool {
	for _, e := range enums {
		if len(e.literals) > 0 {
			return true
		}
	}
	return false
}

// GoTestFileName is the name of the generated test of the Go file.
const GoTestFileName = "names_gen_test.go"

type enumTest struct{ expr, want string }

// goTest generates a test that checks the generated String methods and
// the listings, so a committed package is fully covered by its tests.
func goTest(pkg string, classes, cdcs int, tests []enumTest) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("// " + Marker + " generate go; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\nimport \"testing\"\n\n", pkg)
	b.WriteString("func TestGeneratedNames(t *testing.T) {\n")
	fmt.Fprintf(&b, "\tif len(LNClasses) != %d || len(CDCs) != %d {\n\t\tt.Errorf(\"listings: %%d classes, %%d CDCs\", len(LNClasses), len(CDCs))\n\t}\n", classes, cdcs)
	if len(tests) > 0 {
		b.WriteString("\tfor _, c := range []struct{ got, want string }{\n")
		for _, tc := range tests {
			fmt.Fprintf(&b, "\t\t{%s, %q},\n", tc.expr, tc.want)
		}
		b.WriteString("\t} {\n\t\tif c.got != c.want {\n\t\t\tt.Errorf(\"String() = %q, want %q\", c.got, c.want)\n\t\t}\n\t}\n")
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}

// class is a logical node class name with its definitions in the
// profile: one, or several when namespaces overlap.
type class struct {
	name string
	defs []*model.LogicalNodeClass
}

func (c class) sources() string {
	ids := make([]string, len(c.defs))
	for i, d := range c.defs {
		ids[i] = d.Source.String()
	}
	return strings.Join(ids, ", ")
}

// groupClasses groups the listed classes by name; the listing is sorted
// by name.
func groupClasses(lns []*model.LogicalNodeClass) []class {
	var out []class
	for _, ln := range lns {
		if n := len(out); n > 0 && out[n-1].name == ln.Name {
			out[n-1].defs = append(out[n-1].defs, ln)
			continue
		}
		out = append(out, class{name: ln.Name, defs: []*model.LogicalNodeClass{ln}})
	}
	return out
}

func presence(p model.PresenceCondition) string {
	if p.IsZero() {
		return "-"
	}
	return p.String()
}

// doRef is a data object name of a class with, per definition that
// declares it, its CDC, presence and namespace.
type doRef struct {
	name  string
	notes []string
}

// classDataObjects returns the data object names of a class. For a class
// that several namespaces define, the names of every definition are
// listed with the namespace of each in the note; the definitions are
// not merged.
func classDataObjects(p *catalog.Profile, c class) ([]doRef, string) {
	var refs []doRef
	index := map[string]int{}
	var note string
	for _, def := range c.defs {
		dos, err := p.DataObjectsIn(def.Source, c.name)
		if err != nil {
			note = fmt.Sprintf("%s (%s): %v; only its own data objects are declared", c.name, def.Source, err)
			dos = nil
			for i := range def.DataObjects {
				dos = append(dos, catalog.InheritedDataObject{DataObject: &def.DataObjects[i], DeclaredIn: def})
			}
		}
		for _, do := range dos {
			text := do.Type + ", " + presence(do.Presence)
			if len(c.defs) > 1 {
				text += " (" + def.Source.String() + ")"
			}
			i, ok := index[do.Name]
			if !ok {
				index[do.Name] = len(refs)
				refs = append(refs, doRef{name: do.Name})
				i = len(refs) - 1
			}
			refs[i].notes = append(refs[i].notes, text)
		}
	}
	if len(c.defs) > 1 {
		note = fmt.Sprintf("%s is defined by %s; the constants name the data objects of each definition, with its namespace; the definitions are not merged (nsdctl catalog overlaps)", c.name, c.sources())
	}
	return refs, note
}

// enumType is one enumeration definition with its Go type name.
type enumType struct {
	name, typeName string
	source         model.NamespaceID
	literals       []model.Literal
}

// splitEnumerations keeps every definition separate. An enumeration that
// several namespaces define gets one type per definition, named after
// the namespace (BatteryTypeKind_7_420, BatteryTypeKind_90_9), so no
// definition takes precedence.
func splitEnumerations(enums []*model.Enumeration) ([]enumType, []string) {
	count := map[string]int{}
	for _, e := range enums {
		count[e.Name]++
	}
	var out []enumType
	var notes []string
	for _, e := range enums {
		t := enumType{name: e.Name, typeName: e.Name, source: e.Source, literals: e.Literals}
		if count[e.Name] > 1 {
			t.typeName = e.Name + "_" + namespaceToken(e.Source)
			notes = append(notes, fmt.Sprintf("%s is defined by several namespaces; %s is the definition of %s", e.Name, t.typeName, e.Source))
		}
		out = append(out, t)
	}
	return out, notes
}

// namespaceToken derives an identifier part from a namespace, e.g.
// "7_420" for IEC 61850-7-420 and "90_9" for (Tr)IEC 61850-90-9.
func namespaceToken(id model.NamespaceID) string {
	std := strings.ReplaceAll(id.Standard, " ", "")
	if i := strings.LastIndex(std, "61850-"); i >= 0 {
		std = std[i+len("61850-"):]
	}
	return sanitize(strings.ReplaceAll(std, "-", "_"))
}
