// SPDX-License-Identifier: MIT

package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
)

func repoFile(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "sources", name)
}

func TestRepositoryFilesAreValid(t *testing.T) {
	idx, err := LoadIndex(repoFile("index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if ds := idx.Validate(); ds.HasErrors() {
		t.Errorf("index: %v", ds)
	}
	lock, err := LoadLock(repoFile("nsd.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if ds := lock.Validate(); ds.HasErrors() {
		t.Errorf("lock: %v", ds)
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadLock(filepath.Join(dir, "missing.json")); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("missing: %v", err)
	}
	p := writeFile(t, dir, "bad.json", `{"schemaVersion": 1, "unknown": true}`)
	if _, err := LoadLock(p); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("unknown field: %v", err)
	}
	if _, err := LoadIndex(p); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("unknown field: %v", err)
	}
}

func TestIndexValidate(t *testing.T) {
	idx := &Index{SchemaVersion: 2, Registries: []Registry{
		{Name: "", URL: "http://iec.ch/x", Verified: true},
		{Name: "mirror", URL: "https://github.com/someone/nsd", Verified: true},
		{Name: "ok", URL: "https://webstore.iec.ch/", Verified: true},
	}, Parts: []Part{{}}}
	ds := idx.Validate()
	if len(ds.Errors()) != 5 {
		t.Errorf("errors: %v", ds)
	}
}

func TestLockValidate(t *testing.T) {
	sha := strings.Repeat("a", 64)
	lock := &Lock{SchemaVersion: 1, Sources: []Entry{
		{Standard: "IEC 61850-7-4", Namespace: "IEC 61850-7-4:2007B5", Status: StateAdopted, SHA256: sha, Source: "https://iec61850.dvl.iec.ch/x", Redistribution: RedistributionPermitted, Content: "light", Dependencies: []string{"IEC 61850-7-3:2007B"}},
		{Standard: "IEC 61850-7-4", Namespace: "IEC 61850-7-4:2007B5", Status: StateNSDAvailable},
		{Namespace: "IEC 61850-7-3", Status: "weird", Redistribution: "maybe", Source: "https://example.com", SHA256: "XYZ", Content: "medium", Dependencies: []string{""}},
		{Standard: "S", Namespace: ":", Status: StateNSDValidated},
	}}
	ds := lock.Validate()
	for _, want := range []string{"locked twice", "needs a version", "without standard", "unknown status", "unknown redistribution",
		"not an official", "64 lower-case", "unknown content", "dependency \"\"", "requires a sha256", "namespace \":\""} {
		found := false
		for _, d := range ds {
			if strings.Contains(d.Message, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %q in %v", want, ds)
		}
	}
	if (&Lock{SchemaVersion: 9}).Validate().HasErrors() == false {
		t.Error("schema version")
	}
	e, ok := lock.Entry(model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 5})
	if !ok || !e.Permitted() {
		t.Error("Entry/Permitted")
	}
	if _, ok := lock.Entry(model.NamespaceID{Standard: "nope"}); ok {
		t.Error("unknown entry found")
	}
}

func TestVerify(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.zip", "package a")
	writeFile(t, dir, "b.nsd", "package b")
	writeFile(t, dir, "notes.txt", "ignored")
	if err := os.Mkdir(filepath.Join(dir, "sub.zip"), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("package a"))
	lock := &Lock{SchemaVersion: 1, Sources: []Entry{
		{Namespace: "A:1", SHA256: hex.EncodeToString(sum[:])},
		{Namespace: "C:1", SHA256: strings.Repeat("0", 64)},
		{Namespace: "D:1"},
	}}
	v, err := lock.Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Matched) != 1 || v.Matched[0].Path != a || len(v.Missing) != 1 || v.Missing[0] != "C:1" || len(v.Unlocked) != 1 || v.Clean() {
		t.Errorf("verification %+v", v)
	}
	if _, err := lock.Verify(filepath.Join(dir, "missing")); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("missing dir: %v", err)
	}
	if (&Verification{}).Clean() != true {
		t.Error("empty verification is clean")
	}
}

func TestPackages(t *testing.T) {
	p := Package{Tier: TierRecommended, Part: "90-9", Name: "IEC_TR_61850-90-9.NSD.2018A3.Light.zip", Release: "2018A3", Kind: KindNSD, Content: "light"}
	if v, r, rel, err := p.ReleaseID(); err != nil || v != "2018" || r != "A" || rel != 3 {
		t.Fatalf("ReleaseID = %s %s %d %v", v, r, rel, err)
	}
	for _, tc := range []struct {
		id   model.NamespaceID
		want bool
	}{
		{model.NamespaceID{Standard: "IEC 61850-90-9", Version: "2018", Revision: "A", Release: 3}, true},
		{model.NamespaceID{Standard: "IEC TR 61850-90-9", Version: "2018", Revision: "A", Release: 3}, true},
		{model.NamespaceID{Standard: "IEC 61850-90-9", Version: "2018", Revision: "A", Release: 2}, false},
		{model.NamespaceID{Standard: "IEC 61850-90-19", Version: "2018", Revision: "A", Release: 3}, false},
		{model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2018", Revision: "A", Release: 3}, false},
	} {
		if got := p.MatchesNamespace(tc.id); got != tc.want {
			t.Errorf("MatchesNamespace(%s) = %v", tc.id, got)
		}
	}
	noRel := Package{Part: "90-6", Release: "2018B"}
	if !noRel.MatchesNamespace(model.NamespaceID{Standard: "IEC 61850-90-6", Version: "2018", Revision: "B", Release: 7}) {
		t.Error("a label without release matches any release")
	}
	if (Package{Release: "nope"}).MatchesNamespace(model.NamespaceID{}) {
		t.Error("invalid label")
	}

	dir := t.TempDir()
	writeFile(t, dir, "iec_tr_61850-90-9.nsd.2018a3.light.ZIP", "x")
	if f, ok := p.FindFile(dir); !ok || !strings.HasSuffix(f, ".ZIP") {
		t.Errorf("FindFile = %s %v", f, ok)
	}
	if _, ok := (Package{Name: "IEC_61850-7-2.NSD.2007B6.light"}).FindFile(dir); ok {
		t.Error("unexpected match")
	}
	if _, ok := p.FindFile(filepath.Join(dir, "missing")); ok {
		t.Error("missing dir")
	}

	idx := &Index{SchemaVersion: 1, Packages: []Package{
		p, p,
		{Tier: "someday", Part: "x", Name: "n.2020A", Release: "2020A", Kind: "pdf", Content: "medium"},
		{Tier: TierOptional, Part: "y", Name: "other", Release: "2020", Kind: KindNSD, Content: "light"},
		{Tier: TierOptional, Name: "", Release: "2020A", Kind: KindNSD, Content: "light"},
	}}
	ds := idx.Validate()
	for _, want := range []string{"listed twice", "unknown tier", "unknown kind", "unknown content", "release label", "does not carry release", "needs name and part"} {
		found := false
		for _, d := range ds {
			if strings.Contains(d.Message, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %q in %v", want, ds)
		}
	}
	if len(Tiers()) != 3 {
		t.Error("Tiers")
	}
	z := writeFile(t, dir, "IEC_61850-7-2.NSD.2007B6.light", "PK\x03\x04rest")
	lock := &Lock{SchemaVersion: 1}
	v, err := lock.Verify(dir)
	if err != nil || len(v.Unlocked) != 2 {
		t.Errorf("extensionless archives are hashed: %+v %v (%s)", v, err, z)
	}
	if _, err := HashFile(filepath.Join(dir, "nope")); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Error("HashFile missing")
	}
}
