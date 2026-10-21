// SPDX-License-Identifier: MIT

// Package sources reads and checks the source registry
// (sources/index.json) and the version lock (sources/nsd.lock.json):
// metadata about official IEC NSD packages that may be committed. It
// never contains or fetches package content.
package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
)

// SchemaVersion is the version of the index and lock formats.
const SchemaVersion = 1

// Monitor states of a lock entry (REQUIREMENTS §10.2).
const (
	StatePublicationAnnounced = "publication-announced"
	StateNSDAvailable         = "nsd-available"
	StateNSDValidated         = "nsd-validated"
	StateAdoptionBlocked      = "adoption-blocked"
	StateAdopted              = "adopted"
)

var states = map[string]bool{
	StatePublicationAnnounced: true, StateNSDAvailable: true, StateNSDValidated: true,
	StateAdoptionBlocked: true, StateAdopted: true,
}

// Redistribution decisions of a lock entry.
const (
	// RedistributionLocalOnly: output derived from the package stays
	// local. The default.
	RedistributionLocalOnly = "local-only"
	// RedistributionPermitted: a documented licensing decision allows
	// publishing generated output derived from the package.
	RedistributionPermitted = "permitted"
)

// Index is the registry of official discovery locations, tracked IEC
// parts and the code component packages the project covers.
type Index struct {
	SchemaVersion int        `json:"schemaVersion"`
	Registries    []Registry `json:"registries"`
	Parts         []Part     `json:"parts"`
	Packages      []Package  `json:"packages"`
}

// Package tiers, in priority order.
const (
	TierRequired    = "required"
	TierRecommended = "recommended"
	TierOptional    = "optional"
)

// Tiers returns the package tiers in priority order.
func Tiers() []string { return []string{TierRequired, TierRecommended, TierOptional} }

// Package kinds.
const (
	// KindNSD is a package of namespace definitions.
	KindNSD = "nsd"
	// KindSchema is a package of the NSD XML schema (IEC 61850-7-7).
	KindSchema = "schema"
)

// Package is an official code component package the project covers.
type Package struct {
	// Tier is required, recommended or optional.
	Tier string `json:"tier"`
	// Part is the IEC 61850 part, e.g. "7-420" or "90-9".
	Part string `json:"part"`
	// Name is the official package name, with or without ".zip", e.g.
	// "IEC_61850-7-420.NSD.2019A4.Light.zip".
	Name string `json:"name"`
	// Release is the namespace release label, e.g. "2019A4": version
	// year, revision letter and optional release number. It is not a
	// publication date.
	Release string `json:"release"`
	// Kind is "nsd" or "schema".
	Kind string `json:"kind"`
	// Content is "light" or "full".
	Content string `json:"content"`
	// Purpose describes what the package adds.
	Purpose string `json:"purpose"`
	// URL is the official HTTPS download location.
	URL string `json:"url,omitempty"`
	// Verified reports whether name and URL were checked against the
	// official host.
	Verified bool `json:"verified"`
}

// ReleaseID returns the namespace ID fields of the package's release
// label: version, revision and release (0 when the label has none).
func (p Package) ReleaseID() (version, revision string, release int, err error) {
	id, err := model.ParseNamespaceID("x:" + p.Release)
	if err != nil || id.Revision == "" {
		return "", "", 0, fmt.Errorf("%w: release label %q", model.ErrUnknownNamespace, p.Release)
	}
	return id.Version, id.Revision, id.Release, nil
}

// MatchesNamespace reports whether a loaded namespace is the one the
// package carries: its standard ends with the part ("IEC 61850-90-9",
// "IEC TR 61850-90-9") and its version and revision match the release
// label, as does its release when the label has one.
func (p Package) MatchesNamespace(id model.NamespaceID) bool {
	v, r, rel, err := p.ReleaseID()
	if err != nil {
		return false
	}
	std := strings.ReplaceAll(id.Standard, " ", "")
	if !strings.HasSuffix(std, "61850-"+p.Part) {
		return false
	}
	return id.Version == v && strings.EqualFold(id.Revision, r) && (rel == 0 || id.Release == rel)
}

// baseName normalizes a package or file name for matching: lower case,
// without a trailing ".zip".
func baseName(name string) string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".zip")
}

// FindFile returns the path of the package's file directly inside dir,
// matching the name case-insensitively with or without ".zip".
func (p Package) FindFile(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	want := baseName(p.Name)
	for _, e := range entries {
		if baseName(e.Name()) == want {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}

// Registry is an official location where NSD packages are published.
type Registry struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Kind     string `json:"kind"`
	Verified bool   `json:"verified"`
}

// Part is an IEC 61850 part the project tracks.
type Part struct {
	Part     string `json:"part"`
	Role     string `json:"role"`
	Baseline string `json:"baseline"`
}

// Lock pins the namespace releases the project has looked at.
type Lock struct {
	SchemaVersion int     `json:"schemaVersion"`
	Sources       []Entry `json:"sources"`
}

// Entry is one locked namespace release. It holds permitted metadata
// only.
type Entry struct {
	// Standard and Publication name the published standard, e.g.
	// "IEC 61850-7-420" and "2021".
	Standard    string `json:"standard"`
	Publication string `json:"publication,omitempty"`
	// Namespace is the canonical namespace ID, e.g.
	// "IEC 61850-7-420:2019A4".
	Namespace string `json:"namespace"`
	// Content is "light" or "full".
	Content string `json:"content,omitempty"`
	// Package is the file name of the code component package that
	// carries the namespace; its SHA256 is the package digest.
	Package string `json:"package,omitempty"`
	// Source is the official location the package was obtained from.
	Source string `json:"source,omitempty"`
	// RetrievedAt is the RFC 3339 retrieval time.
	RetrievedAt string `json:"retrievedAt,omitempty"`
	// SHA256 is the hex digest of the package archive or document.
	SHA256 string `json:"sha256,omitempty"`
	// Dependencies are canonical namespace IDs.
	Dependencies []string `json:"dependencies,omitempty"`
	// Status is one of the monitor states.
	Status string `json:"status"`
	// Redistribution is "local-only" (default) or "permitted".
	Redistribution string `json:"redistribution,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// ID parses the namespace ID of the entry.
func (e Entry) ID() (model.NamespaceID, error) { return model.ParseNamespaceID(e.Namespace) }

// PackageDigest returns the locked SHA-256 of a package file name, when
// an entry records one.
func (l *Lock) PackageDigest(name string) (string, bool) {
	for _, e := range l.Sources {
		if e.Package != "" && e.SHA256 != "" && baseName(e.Package) == baseName(name) {
			return e.SHA256, true
		}
	}
	return "", false
}

// Permitted reports whether output derived from the entry may be
// published.
func (e Entry) Permitted() bool { return e.Redistribution == RedistributionPermitted }

// LoadIndex reads an index file.
func LoadIndex(path string) (*Index, error) {
	var idx Index
	if err := loadJSON(path, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// LoadLock reads a lock file.
func LoadLock(path string) (*Lock, error) {
	var l Lock
	if err := loadJSON(path, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func loadJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return &model.SourceError{Source: path, Reason: "cannot open", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	defer func() { _ = f.Close() }()
	dec := json.NewDecoder(io.LimitReader(f, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &model.SourceError{Source: path, Reason: "invalid JSON", Err: errors.Join(model.ErrInvalidNSD, err)}
	}
	return nil
}

// officialHost reports whether a URL points to an official IEC host.
func officialHost(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "iec.ch" || strings.HasSuffix(h, ".iec.ch")
}

// Validate checks the index: schema version, official HTTPS URLs and
// complete entries.
func (idx *Index) Validate() model.Diagnostics {
	var ds model.Diagnostics
	add := func(path, msg string) {
		ds = append(ds, model.Diagnostic{Severity: model.SeverityError, Code: "invalid-index", Source: "index", Path: path, Message: msg})
	}
	if idx.SchemaVersion != SchemaVersion {
		add("schemaVersion", fmt.Sprintf("schema version %d is not supported (want %d)", idx.SchemaVersion, SchemaVersion))
	}
	for i, r := range idx.Registries {
		p := fmt.Sprintf("registries[%d]", i)
		if r.Name == "" {
			add(p, "registry without name")
		}
		if !officialHost(r.URL) {
			add(p, fmt.Sprintf("URL %q is not an official IEC HTTPS location", r.URL))
		}
		if !r.Verified {
			ds = append(ds, model.Diagnostic{Severity: model.SeverityInfo, Code: "unverified-registry", Source: "index", Path: p, Message: fmt.Sprintf("%s has not been verified yet", r.Name)})
		}
	}
	for i, p := range idx.Parts {
		if p.Part == "" {
			add(fmt.Sprintf("parts[%d]", i), "part without name")
		}
	}
	names := map[string]bool{}
	for i, p := range idx.Packages {
		path := fmt.Sprintf("packages[%d]", i)
		switch {
		case p.Name == "" || p.Part == "":
			add(path, "package needs name and part")
		case names[baseName(p.Name)]:
			add(path, fmt.Sprintf("package %s is listed twice", p.Name))
		default:
			names[baseName(p.Name)] = true
		}
		if !slices.Contains(Tiers(), p.Tier) {
			add(path, fmt.Sprintf("unknown tier %q", p.Tier))
		}
		if p.Kind != KindNSD && p.Kind != KindSchema {
			add(path, fmt.Sprintf("unknown kind %q", p.Kind))
		}
		if p.Content != string(model.ContentLight) && p.Content != string(model.ContentFull) {
			add(path, fmt.Sprintf("unknown content %q", p.Content))
		}
		if _, _, _, err := p.ReleaseID(); err != nil {
			add(path, err.Error())
		}
		if p.URL != "" && (!officialHost(p.URL) || baseName(p.URL) != baseName(p.Name)) {
			add(path, fmt.Sprintf("URL %q must be an official IEC HTTPS location ending in the package name", p.URL))
		}
		if p.Name != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(p.Release)) {
			add(path, fmt.Sprintf("name %s does not carry release %s", p.Name, p.Release))
		}
	}
	return ds
}

// Validate checks the lock: schema version, parsable and unique
// namespace IDs, known states and redistribution values, official
// source URLs, digests, and that adopted entries carry a digest.
func (l *Lock) Validate() model.Diagnostics {
	var ds model.Diagnostics
	add := func(path, msg string) {
		ds = append(ds, model.Diagnostic{Severity: model.SeverityError, Code: "invalid-lock", Source: "lock", Path: path, Message: msg})
	}
	if l.SchemaVersion != SchemaVersion {
		add("schemaVersion", fmt.Sprintf("schema version %d is not supported (want %d)", l.SchemaVersion, SchemaVersion))
	}
	seen := map[string]bool{}
	for i, e := range l.Sources {
		p := fmt.Sprintf("sources[%d]", i)
		id, err := e.ID()
		switch {
		case err != nil:
			add(p, fmt.Sprintf("namespace %q: %v", e.Namespace, err))
		case id.Version == "":
			add(p, fmt.Sprintf("namespace %q needs a version", e.Namespace))
		case seen[id.String()]:
			add(p, fmt.Sprintf("namespace %s is locked twice", id))
		default:
			seen[id.String()] = true
		}
		if e.Standard == "" {
			add(p, "entry without standard")
		}
		if !states[e.Status] {
			add(p, fmt.Sprintf("unknown status %q", e.Status))
		}
		if e.Redistribution != "" && e.Redistribution != RedistributionLocalOnly && e.Redistribution != RedistributionPermitted {
			add(p, fmt.Sprintf("unknown redistribution %q", e.Redistribution))
		}
		if e.Source != "" && !officialHost(e.Source) {
			add(p, fmt.Sprintf("source %q is not an official IEC HTTPS location", e.Source))
		}
		if e.SHA256 != "" && !isSHA256(e.SHA256) {
			add(p, "sha256 must be 64 lower-case hex characters")
		}
		if (e.Status == StateNSDValidated || e.Status == StateAdopted) && e.SHA256 == "" {
			add(p, fmt.Sprintf("status %s requires a sha256", e.Status))
		}
		if e.Content != "" && e.Content != string(model.ContentLight) && e.Content != string(model.ContentFull) {
			add(p, fmt.Sprintf("unknown content %q", e.Content))
		}
		for _, d := range e.Dependencies {
			if _, err := model.ParseNamespaceID(d); err != nil {
				add(p, fmt.Sprintf("dependency %q: %v", d, err))
			}
		}
	}
	return ds
}

func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Entry returns the lock entry of a namespace ID.
func (l *Lock) Entry(id model.NamespaceID) (Entry, bool) {
	for _, e := range l.Sources {
		if eid, err := e.ID(); err == nil && eid == id {
			return e, true
		}
	}
	return Entry{}, false
}

// FileDigest is a local file and its SHA-256.
type FileDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Verification is the result of comparing local packages with the lock.
type Verification struct {
	// Matched maps lock namespaces to the local file with their digest.
	Matched []Match `json:"matched,omitempty"`
	// Missing lists locked namespaces with a digest but no local file.
	Missing []string `json:"missing,omitempty"`
	// Unlocked lists local files whose digest no lock entry records.
	Unlocked []FileDigest `json:"unlocked,omitempty"`
}

// Match pairs a lock entry with a local file.
type Match struct {
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
}

// Clean reports whether every locked digest has a local file and every
// local file is locked.
func (v *Verification) Clean() bool { return len(v.Missing) == 0 && len(v.Unlocked) == 0 }

// Verify hashes the package files directly inside dir (.zip, .nsd,
// .snsd, .nsdoc, and ZIP archives without extension) and compares their
// digests with the lock.
func (l *Lock) Verify(dir string) (*Verification, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &model.SourceError{Source: dir, Reason: "cannot read directory", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	byDigest := map[string]FileDigest{}
	var files []FileDigest
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".zip", ".nsd", ".snsd", ".nsdoc":
		default:
			// Official package names may lack ".zip", e.g.
			// "IEC_61850-7-2.NSD.2007B6.light": recognize archives by content.
			if !startsWithZIP(p) {
				continue
			}
		}
		sum, err := HashFile(p)
		if err != nil {
			return nil, err
		}
		fd := FileDigest{Path: p, SHA256: sum}
		files = append(files, fd)
		byDigest[sum] = fd
	}
	v := &Verification{}
	used := map[string]bool{}
	for _, e := range l.Sources {
		if e.SHA256 == "" {
			continue
		}
		if fd, ok := byDigest[e.SHA256]; ok {
			v.Matched = append(v.Matched, Match{Namespace: e.Namespace, Path: fd.Path})
			used[e.SHA256] = true
		} else {
			v.Missing = append(v.Missing, e.Namespace)
		}
	}
	for _, fd := range files {
		if !used[fd.SHA256] {
			v.Unlocked = append(v.Unlocked, fd)
		}
	}
	sort.Slice(v.Matched, func(i, j int) bool { return v.Matched[i].Namespace < v.Matched[j].Namespace })
	sort.Strings(v.Missing)
	return v, nil
}

// HashFile returns the hex SHA-256 of a file.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", &model.SourceError{Source: path, Reason: "cannot open", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", &model.SourceError{Source: path, Reason: "cannot read", Err: errors.Join(model.ErrSourceUnavailable, err)}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func startsWithZIP(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 4)
	n, _ := io.ReadFull(f, head)
	return n == 4 && string(head[:2]) == "PK" && (head[2] == 3 && head[3] == 4 || head[2] == 5 && head[3] == 6)
}
