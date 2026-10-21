// SPDX-License-Identifier: MIT

package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/ontology"
	"github.com/otfabric/go-iec61850-model/resolver"
	"github.com/otfabric/go-iec61850-model/semantic"
)

// Docs generates Markdown documentation of the profile: an index with
// the source namespaces, the logical node classes grouped by logical
// node group (with inherited data objects and their semantic mapping),
// the CDCs, the constructed types and enumerations, and a coverage
// report. NSDoc texts are included only with Input.IncludeDocText.
func Docs(in Input) (Files, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	g := &docGen{in: in}
	files := Files{}
	var err error
	if files["logical-nodes.md"], err = g.logicalNodes(); err != nil {
		return nil, err
	}
	files["cdcs.md"] = g.cdcs()
	files["types.md"] = g.types()
	if files["coverage.md"], err = g.coverage(); err != nil {
		return nil, err
	}
	files["index.md"] = g.index()
	in.addNotice(files, "docs")
	if !in.Publish {
		files["LOCAL-ONLY.md"] = in.localNotice("docs")
	}
	return files, nil
}

type docGen struct{ in Input }

func (g *docGen) header(b *bytes.Buffer, title string) {
	b.WriteString("<!--\n")
	for _, l := range g.in.headerLines("docs") {
		b.WriteString(strings.TrimRight(l, " ") + "\n")
	}
	b.WriteString("-->\n\n# " + title + "\n\n")
}

// cell escapes a Markdown table cell.
func cell(s string) string {
	if s == "" {
		return "-"
	}
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.ReplaceAll(s, "\n", " ")
}

func (g *docGen) text(d model.Doc) string {
	if !g.in.IncludeDocText {
		return ""
	}
	if d.Title != "" {
		return d.Title
	}
	return d.Description
}

func (g *docGen) index() []byte {
	var b bytes.Buffer
	g.header(&b, "IEC 61850 namespace profile")
	b.WriteString("| Namespace | Stage | SHA-256 | Classes | CDCs | Types | Enumerations |\n|---|---|---|---|---|---|---|\n")
	for _, ns := range g.in.Profile.Namespaces() {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d | %d |\n", cell(ns.ID.String()), cell(ns.PublicationStage), cell(short(ns.Provenance.SHA256)),
			len(ns.LNClasses)+len(ns.AbstractLNClasses), len(ns.CDCs), len(ns.ConstructedAttributes), len(ns.Enumerations))
	}
	b.WriteString("\n- [Logical node classes](logical-nodes.md)\n- [Common data classes](cdcs.md)\n- [Constructed types and enumerations](types.md)\n- [Coverage](coverage.md)\n")
	return b.Bytes()
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func (g *docGen) logicalNodes() ([]byte, error) {
	p := g.in.Profile
	var b bytes.Buffer
	g.header(&b, "Logical node classes")
	b.WriteString("Semantic columns are OTFabric interpretation (rule-based), not normative content. ")
	b.WriteString("Optional data objects may be absent on a device.\n\n")
	lns := p.ListLogicalNodes()
	sort.SliceStable(lns, func(i, j int) bool { return lns[i].Group() < lns[j].Group() })
	var group model.LNGroup = 255
	for _, ln := range lns {
		if ln.Group() != group {
			group = ln.Group()
			label := group.Label()
			if label == "" {
				label = "Other"
			}
			fmt.Fprintf(&b, "## %s — %s\n\n", cell(group.String()), label)
		}
		var names []string
		chain, err := p.InheritanceIn(ln.Source, ln.Name)
		switch {
		case errors.Is(err, model.ErrAmbiguousReference):
			names = []string{ln.Name, "(ambiguous: " + err.Error() + ")"}
		case err != nil:
			return nil, err
		default:
			for _, c := range chain {
				names = append(names, c.Name)
			}
		}
		fmt.Fprintf(&b, "### %s\n\n", ln.Name)
		fmt.Fprintf(&b, "Namespace: %s. Inheritance: %s.", ln.Source, strings.Join(names, " → "))
		if t := g.text(ln.Doc); t != "" {
			b.WriteString(" " + t)
		}
		b.WriteString("\n\n")
		meas := map[string][]string{}
		if ds, err := semantic.FindMeasurementsIn(p, ln.Source, ln.Name); err == nil {
			for _, d := range ds {
				s := string(d.Quantity)
				if d.Phase != "" && d.SubDataObject == "" {
					s += " (" + string(d.Phase) + ")"
				}
				if d.Reference != "" {
					s += ", " + string(d.Reference)
				}
				if d.Direction != "" {
					s += ", " + string(d.Direction)
				}
				if len(meas[d.DataObject]) == 0 || meas[d.DataObject][0] != s {
					meas[d.DataObject] = append(meas[d.DataObject], s)
				}
			}
		}
		dos, err := p.DataObjectsIn(ln.Source, ln.Name)
		if err != nil {
			dos = nil
			for i := range ln.DataObjects {
				dos = append(dos, catalog.InheritedDataObject{DataObject: &ln.DataObjects[i], DeclaredIn: ln})
			}
		}
		b.WriteString("| Data object | CDC | Presence | Declared in | Semantics | Text |\n|---|---|---|---|---|---|\n")
		for _, do := range dos {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", do.Name, cell(do.Type), cell(do.Presence.String()), do.DeclaredIn.Name,
				cell(strings.Join(meas[do.Name], "; ")), cell(g.text(do.Doc)))
		}
		b.WriteString("\n")
	}
	return b.Bytes(), nil
}

func flags(a *model.DataAttribute) string {
	var f []string
	for _, x := range []struct {
		on   bool
		name string
	}{{a.Dchg, "dchg"}, {a.Qchg, "qchg"}, {a.Dupd, "dupd"}, {a.IsArray, "array"}} {
		if x.on {
			f = append(f, x.name)
		}
	}
	return strings.Join(f, ", ")
}

func (g *docGen) cdcs() []byte {
	var b bytes.Buffer
	g.header(&b, "Common data classes")
	for _, c := range g.in.Profile.ListCDCs() {
		fmt.Fprintf(&b, "## %s\n\nNamespace: %s.", c.Key(), c.Source)
		if t := g.text(c.Doc); t != "" {
			b.WriteString(" " + t)
		}
		b.WriteString("\n\n| Member | Kind | Type | FC | Presence | Options |\n|---|---|---|---|---|---|\n")
		type row struct {
			pos  int
			line string
		}
		var rows []row
		for _, s := range c.SubDataObjects {
			rows = append(rows, row{s.Position, fmt.Sprintf("| %s | sub data object | %s | - | %s | %s |", s.Name, cell(s.Type), cell(s.Presence.String()), "-")})
		}
		for i := range c.DataAttributes {
			a := &c.DataAttributes[i]
			typ := a.Type
			if typ == "" {
				typ = "(" + string(a.TypeKind) + ", from data object)"
			}
			rows = append(rows, row{a.Position, fmt.Sprintf("| %s | data attribute | %s | %s | %s | %s |", a.Name, cell(typ), cell(string(a.FC)), cell(a.Presence.String()), cell(flags(a)))})
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].pos < rows[j].pos })
		for _, r := range rows {
			b.WriteString(r.line + "\n")
		}
		b.WriteString("\n")
	}
	return b.Bytes()
}

func (g *docGen) types() []byte {
	var b bytes.Buffer
	g.header(&b, "Constructed types and enumerations")
	b.WriteString("## Constructed attribute types\n\n")
	for _, c := range g.in.Profile.ListConstructedAttributes() {
		fmt.Fprintf(&b, "### %s\n\nNamespace: %s.\n\n| Member | Type | Kind | Presence |\n|---|---|---|---|\n", c.Name, c.Source)
		for _, a := range c.SubDataAttributes {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", a.Name, cell(a.Type), cell(string(a.TypeKind)), cell(a.Presence.String()))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Enumerations\n\n")
	for _, e := range g.in.Profile.ListEnumerations() {
		fmt.Fprintf(&b, "### %s\n\nNamespace: %s.", e.Name, e.Source)
		if e.InheritedFrom != "" {
			fmt.Fprintf(&b, " Extends %s.", e.InheritedFrom)
		}
		b.WriteString("\n\n| Value | Literal |\n|---|---|\n")
		for _, l := range e.Literals {
			fmt.Fprintf(&b, "| %d | %s |\n", l.Value, cell(l.Name))
		}
		b.WriteString("\n")
	}
	return b.Bytes()
}

// Coverage summarizes, per namespace, what was parsed and how much of
// it resolves; it is also the basis of docs/coverage.md.
type Coverage struct {
	Namespace         string `json:"namespace"`
	LNClasses         int    `json:"lnClasses"`
	AbstractLNClasses int    `json:"abstractLNClasses"`
	DataObjects       int    `json:"dataObjects"`
	DataObjectsOK     int    `json:"dataObjectsResolved"`
	CDCs              int    `json:"cdcs"`
	Constructed       int    `json:"constructedAttributes"`
	Enumerations      int    `json:"enumerations"`
	Measurements      int    `json:"measurements"`
	ProfileErrors     int    `json:"profileErrors"`
	ProfileWarnings   int    `json:"profileWarnings"`
}

// ComputeCoverage resolves every data object of every class of the
// profile down to its leaves and counts the results per namespace.
func ComputeCoverage(in Input) ([]Coverage, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	p := in.Profile
	r := resolver.New(p)
	byNS := map[model.NamespaceID]*Coverage{}
	var order []model.NamespaceID
	for _, ns := range p.Namespaces() {
		cv := &Coverage{Namespace: ns.ID.String(), LNClasses: len(ns.LNClasses), AbstractLNClasses: len(ns.AbstractLNClasses),
			CDCs: len(ns.CDCs), Constructed: len(ns.ConstructedAttributes), Enumerations: len(ns.Enumerations)}
		for _, ln := range ns.LNClasses {
			cv.DataObjects += len(ln.DataObjects)
			for _, do := range ln.DataObjects {
				if _, err := r.In(ns.ID).Leaves(ln.Name, []string{do.Name}, ""); err == nil {
					cv.DataObjectsOK++
				}
			}
			if ds, err := semantic.FindMeasurementsIn(p, ln.Source, ln.Name); err == nil {
				cv.Measurements += len(ds)
			}
		}
		byNS[ns.ID] = cv
		order = append(order, ns.ID)
	}
	for _, d := range p.Diagnostics() {
		if cv, ok := byNS[d.Namespace]; ok {
			switch d.Severity {
			case model.SeverityError:
				cv.ProfileErrors++
			case model.SeverityWarning:
				cv.ProfileWarnings++
			case model.SeverityInfo:
			}
		}
	}
	out := make([]Coverage, len(order))
	for i, id := range order {
		out[i] = *byNS[id]
	}
	return out, nil
}

func (g *docGen) coverage() ([]byte, error) {
	cov, err := ComputeCoverage(g.in)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	g.header(&b, "Coverage")
	b.WriteString("Parsed: definitions imported. Resolved: concrete-class data objects that resolve to their leaf types. ")
	b.WriteString("Validated: profile checks without errors. Measurements: semantic descriptors from the default rules.\n\n")
	b.WriteString("| Namespace | Classes (abstract) | Data objects resolved | CDCs | Types | Enumerations | Measurements | Profile errors | Warnings |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range cov {
		fmt.Fprintf(&b, "| %s | %d (%d) | %d / %d | %d | %d | %d | %d | %d | %d |\n", c.Namespace, c.LNClasses, c.AbstractLNClasses,
			c.DataObjectsOK, c.DataObjects, c.CDCs, c.Constructed, c.Enumerations, c.Measurements, c.ProfileErrors, c.ProfileWarnings)
	}
	return b.Bytes(), nil
}

// jsonDoc is the envelope of the JSON exports.
type jsonDoc struct {
	Generated    string             `json:"generated"`
	Generator    string             `json:"generator"`
	Format       int                `json:"format"`
	Distribution string             `json:"distribution"`
	Namespaces   []*model.Namespace `json:"namespaces,omitempty"`
	Graph        *ontology.Graph    `json:"graph,omitempty"`
	Coverage     []Coverage         `json:"coverage,omitempty"`
}

// JSON exports the profile as catalog.json (the normalized namespaces),
// ontology.json (the concept graph with semantic measurements) and
// coverage.json. Local paths are reduced to file names and NSDoc texts
// are removed unless Input.IncludeDocText is set.
func JSON(in Input) (Files, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	env := func() jsonDoc {
		return jsonDoc{Generated: Marker + " generate json; DO NOT EDIT.", Generator: "nsdctl " + orDev(in.ToolVersion),
			Format: FormatVersion, Distribution: in.distribution()}
	}
	nss, err := copyNamespaces(in)
	if err != nil {
		return nil, err
	}
	graph, err := ontology.BuildGraph(in.Profile, ontology.Options{Semantics: semantic.Default})
	if err != nil {
		return nil, err
	}
	cov, err := ComputeCoverage(in)
	if err != nil {
		return nil, err
	}
	cat, covDoc := env(), env()
	cat.Namespaces, covDoc.Coverage = nss, cov
	docs := map[string]any{"catalog.json": cat, "ontology.json": graphDoc(env(), graph, in.IncludeDocText), "coverage.json": covDoc}
	files := Files{}
	for name, v := range docs {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		files[name] = append(data, '\n')
	}
	in.addNotice(files, "json")
	if !in.Publish {
		files["LOCAL-ONLY.md"] = in.localNotice("json")
	}
	return files, nil
}

// copyNamespaces deep-copies the namespaces of the profile, with local paths
// reduced to file names and, unless requested, documentation texts
// removed.
func copyNamespaces(in Input) ([]*model.Namespace, error) {
	var out []*model.Namespace
	for _, ns := range in.Profile.Namespaces() {
		data, err := json.Marshal(ns)
		if err != nil {
			return nil, err
		}
		var cp model.Namespace
		if err := json.Unmarshal(data, &cp); err != nil {
			return nil, err
		}
		cp.Provenance.Source = baseSource(cp.Provenance.Source)
		if !in.IncludeDocText {
			for _, d := range cp.Docs() {
				d.Title, d.Description = "", ""
			}
		}
		out = append(out, &cp)
	}
	return out, nil
}

// baseSource keeps the file part of each "!"-separated source segment.
func baseSource(s string) string {
	parts := strings.Split(s, "!")
	parts[0] = filepath.Base(parts[0])
	return strings.Join(parts, "!")
}

// graphExport is the ontology export: the envelope plus the graph.
type graphExport struct {
	jsonDoc
	Graph struct {
		Concepts  []ontology.Concept  `json:"concepts"`
		Relations []ontology.Relation `json:"relations"`
	} `json:"graph"`
}

// graphDoc builds the ontology export. Without docText, labels that
// come from NSDoc titles are removed; quantity and measurement labels
// are OTFabric vocabulary and stay.
func graphDoc(env jsonDoc, g *ontology.Graph, docText bool) graphExport {
	var out graphExport
	out.jsonDoc = env
	for _, c := range g.Concepts() {
		if !docText && c.Kind != ontology.KindQuantity && c.Kind != ontology.KindMeasurement {
			c.Label = ""
		}
		out.Graph.Concepts = append(out.Graph.Concepts, c)
	}
	out.Graph.Relations = g.RelationsAll()
	return out
}
