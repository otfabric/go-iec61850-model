// SPDX-License-Identifier: MIT

// Command textgen builds the public OT Fabric doc set,
// official/texts_gen.json.gz, from the hand-written tables in texts/<lang>
// and the committed catalog (official/catalog_gen.json).
//
// Every doc set key of every embedded namespace gets a text:
//
//   - LN, CDC, CA, FC, BT, PC, SDA, DA/SDO/SP (by member name) and ABBR come
//     from the tables;
//   - DO texts are composed from the expansions of the abbreviation terms
//     the data object name is built from (TotW = Tot + W), unless do.tsv
//     has an entry for the class and name or for the name;
//   - ENUM and LIT texts are derived from the names.
//
// A key without a text fails the run, so the set always covers the whole
// catalog. With -check the output is compared with the committed file.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

const (
	format = "otfabric-docset/1"
	source = "OT Fabric public texts (texts/en, MIT)"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("textgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	texts := fs.String("texts", "texts/en", "directory with the hand-written tables")
	cat := fs.String("catalog", "official/catalog_gen.json", "committed catalog")
	out := fs.String("out", "official/texts_gen.json.gz", "output file")
	check := fs.Bool("check", false, "fail if the output differs from the committed file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	data, report, err := build(*texts, *cat)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "textgen:", err)
		return 1
	}
	if *check {
		old, err := os.ReadFile(*out)
		if err != nil || !sameContent(old, data) {
			_, _ = fmt.Fprintf(stderr, "textgen: %s is stale; run make texts\n", *out)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil { //nolint:gosec // a public, committed file
		_, _ = fmt.Fprintln(stderr, "textgen:", err)
		return 1
	}
	_, _ = fmt.Fprint(stdout, report)
	return 0
}

// sameContent compares two gzip files by their decompressed content:
// the compressed bytes can differ between Go releases.
func sameContent(a, b []byte) bool {
	ua, errA := gunzip(a)
	ub, errB := gunzip(b)
	return errA == nil && errB == nil && bytes.Equal(ua, ub)
}

func gunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(zr)
}

// tables holds the hand-written texts, by table name and entry name.
type tables map[string]map[string]string

var tableNames = []string{"abbreviations", "ln", "cdc", "da", "ca", "sda", "fc", "bt", "pc", "do"}

func readTables(dir string) (tables, error) {
	t := tables{}
	for _, name := range tableNames {
		m := map[string]string{}
		f, err := os.Open(filepath.Join(dir, name+".tsv"))
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		line := 0
		for sc.Scan() {
			line++
			s := sc.Text()
			if strings.TrimSpace(s) == "" || strings.HasPrefix(s, "#") {
				continue
			}
			parts := strings.Split(s, "\t")
			if len(parts) < 2 || parts[0] == "" || strings.TrimSpace(parts[1]) == "" {
				_ = f.Close()
				return nil, fmt.Errorf("%s.tsv:%d: want name<TAB>text", name, line)
			}
			if _, dup := m[parts[0]]; dup {
				_ = f.Close()
				return nil, fmt.Errorf("%s.tsv:%d: duplicate %q", name, line, parts[0])
			}
			m[parts[0]] = strings.TrimSpace(parts[1])
		}
		_ = f.Close()
		if err := sc.Err(); err != nil {
			return nil, err
		}
		t[name] = m
	}
	return t, nil
}

func build(textsDir, catalogPath string) ([]byte, string, error) {
	t, err := readTables(textsDir)
	if err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(catalogPath)
	if err != nil {
		return nil, "", err
	}
	var doc struct {
		Namespaces []*model.Namespace `json:"namespaces"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", err
	}
	g := &gen{t: t, terms: t["abbreviations"]}
	out := map[string]nsd.DocText{}
	var missing []string
	for _, ns := range doc.Namespaces {
		for _, key := range nsd.DocKeys(ns) {
			if _, done := out[key]; done {
				continue
			}
			txt, ok := g.text(key)
			if !ok {
				missing = append(missing, key)
				continue
			}
			out[key] = txt
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		missing = uniq(missing)
		n := len(missing)
		if n > 40 {
			missing = append(missing[:40], "...")
		}
		return nil, "", fmt.Errorf("%d keys without a text: %s", n, strings.Join(missing, " "))
	}
	if unused := g.unused(); len(unused) > 0 {
		return nil, "", fmt.Errorf("table entries that match no element: %s", strings.Join(unused, " "))
	}
	js, err := json.Marshal(struct {
		Format string `json:"format"`
		Sets   []any  `json:"sets"`
	}{format, []any{struct {
		Lang   string                 `json:"lang"`
		Source string                 `json:"source"`
		Texts  map[string]nsd.DocText `json:"texts"`
	}{"en", source, out}}})
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(js); err != nil {
		return nil, "", err
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	kinds := map[string]int{}
	for k := range out {
		kinds[k[:strings.IndexByte(k, ':')]]++
	}
	var rep strings.Builder
	fmt.Fprintf(&rep, "textgen: %d keys (all elements)", len(out))
	for _, k := range sortedKeys(kinds) {
		fmt.Fprintf(&rep, " %s=%d", k, kinds[k])
	}
	fmt.Fprintf(&rep, "; DO composed=%d table=%d\n", g.composed, g.doTable)
	return buf.Bytes(), rep.String(), nil
}

type gen struct {
	t        tables
	terms    map[string]string
	used     map[string]bool
	composed int
	doTable  int
}

func (g *gen) get(table, name string) (string, bool) {
	v, ok := g.t[table][name]
	if ok {
		if g.used == nil {
			g.used = map[string]bool{}
		}
		g.used[table+"\x00"+name] = true
	}
	return v, ok
}

func (g *gen) unused() []string {
	var out []string
	for _, table := range tableNames {
		if table == "abbreviations" { // terms also serve composition
			continue
		}
		for name := range g.t[table] {
			if !g.used[table+"\x00"+name] {
				out = append(out, table+":"+name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (g *gen) text(key string) (nsd.DocText, bool) {
	kind, path, _ := strings.Cut(key, ":")
	_, member, _ := strings.Cut(path, ".")
	title := func(s string, ok bool) (nsd.DocText, bool) { return nsd.DocText{Title: s}, ok }
	desc := func(s string, ok bool) (nsd.DocText, bool) { return nsd.DocText{Description: s}, ok }
	switch kind {
	case nsd.KeyLogicalNode:
		return title(g.get("ln", path))
	case nsd.KeyCDC:
		return title(g.get("cdc", path))
	case nsd.KeyConstructedAttribute:
		return title(g.get("ca", path))
	case nsd.KeyFunctionalConstraint:
		return title(g.get("fc", path))
	case nsd.KeyBasicType:
		return title(g.get("bt", path))
	case nsd.KeyPresenceCondition:
		return desc(g.get("pc", path))
	case nsd.KeySubDataAttribute:
		return desc(g.get("sda", path))
	case nsd.KeyDataAttribute, nsd.KeySubDataObject, nsd.KeyServiceParameter:
		return desc(g.get("da", member))
	case nsd.KeyAbbreviation:
		v, ok := g.terms[path]
		return desc(capitalize(v), ok)
	case nsd.KeyDataObject:
		if v, ok := g.get("do", path); ok {
			g.doTable++
			return desc(v, true)
		}
		if v, ok := g.get("do", member); ok {
			g.doTable++
			return desc(v, true)
		}
		v, ok := g.compose(member)
		if ok {
			g.composed++
		}
		return desc(v, ok)
	case nsd.KeyEnumeration:
		return title(capitalize(words(strings.TrimSuffix(path, "Kind")+" kind")), true)
	case nsd.KeyLiteral:
		if w := words(member); w != "" {
			return desc(capitalize(w), true)
		}
		return desc(emptyLiteral(path), true)
	case nsd.KeyTriggerOption:
		return title(capitalize(words(path)), true)
	}
	return nsd.DocText{}, false
}

// emptyLiteral is the text of a literal with an empty name.
func emptyLiteral(path string) string {
	switch strings.TrimSuffix(path, ".") {
	case "MultiplierKind":
		return "No multiplier (factor 1)"
	case "SIUnitKind":
		return "No unit (dimensionless)"
	}
	return "Empty literal"
}

// compose expands a data object name into the expansions of its terms,
// choosing the split with the fewest parts; digits stand for themselves.
func (g *gen) compose(name string) (string, bool) {
	memo := map[int][]string{}
	var split func(i int) ([]string, bool)
	split = func(i int) ([]string, bool) {
		if i == len(name) {
			return []string{}, true
		}
		if r, ok := memo[i]; ok {
			return r, r != nil
		}
		var best []string
		for j := len(name); j > i; j-- {
			part := name[i:j]
			exp, ok := g.terms[part]
			if !ok && isDigits(part) {
				exp, ok = part, true
			}
			if !ok {
				continue
			}
			if rest, ok := split(j); ok && (best == nil || len(rest)+1 < len(best)) {
				best = append([]string{exp}, rest...)
			}
		}
		memo[i] = best
		return best, best != nil
	}
	parts, ok := split(0)
	if !ok {
		return "", false
	}
	return capitalize(strings.Join(parts, " ")), true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// words splits a camel-case or punctuated name into words: lowercase,
// except acronyms (runs of capitals), which stay as they are.
func words(s string) string {
	var toks []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			toks = append(toks, string(cur))
			cur = cur[:0]
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		if r == '_' || r == '-' || r == '/' || unicode.IsSpace(r) {
			flush()
			continue
		}
		if i > 0 && len(cur) > 0 {
			p := rs[i-1]
			switch {
			case unicode.IsUpper(r) && unicode.IsLower(p),
				unicode.IsUpper(r) && unicode.IsUpper(p) && i+1 < len(rs) && unicode.IsLower(rs[i+1]),
				unicode.IsDigit(r) != unicode.IsDigit(p):
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	for i, t := range toks {
		if len([]rune(t)) <= 1 || strings.ToUpper(t) != t {
			toks[i] = strings.ToLower(t)
		}
	}
	return strings.Join(toks, " ")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func uniq(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
