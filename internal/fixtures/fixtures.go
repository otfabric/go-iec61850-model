// SPDX-License-Identifier: MIT

// Package fixtures loads the synthetic NSD test fixtures of
// testdata/nsd for the tests of the module's packages.
package fixtures

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// Fixture file names.
const (
	NS72     = "synthetic-7-2.nsd"
	NS73     = "synthetic-7-3.nsd"
	NS74     = "synthetic-7-4.nsd"
	NS74R2   = "synthetic-7-4-r2.nsd"
	NSDoc74  = "synthetic-7-4.en.nsdoc"
	NSExt    = "synthetic-ext.nsd"
	Standard = "IEC 61850-7-4"
)

// Namespace IDs of the fixtures.
var (
	ID72   = model.NamespaceID{Standard: "IEC 61850-7-2", Version: "2007", Revision: "B", Release: 1}
	ID73   = model.NamespaceID{Standard: "IEC 61850-7-3", Version: "2007", Revision: "B", Release: 1}
	ID74   = model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 1}
	ID74R2 = model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 2}
	IDExt  = model.NamespaceID{Standard: "OTF TEST-EXT", Version: "2026", Revision: "A", Release: 1}
)

// Dir returns the absolute path of testdata/nsd.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "nsd")
}

// Path returns the absolute path of a fixture file.
func Path(name string) string { return filepath.Join(Dir(), name) }

// Read returns the content of a fixture file.
func Read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(Path(name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// Package loads every fixture as one package.
func Package(t testing.TB) *nsd.Package {
	t.Helper()
	p, err := nsd.LoadDir(Dir())
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	return p
}

// Catalog builds a catalog of every fixture, including both releases
// of the 7-4 fixture.
func Catalog(t testing.TB) *catalog.Catalog {
	t.Helper()
	c, err := catalog.New(Package(t))
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return c
}

// Profile builds the profile of the extension namespace with release 1
// of the 7-4 fixture.
func Profile(t testing.TB) *catalog.Profile {
	t.Helper()
	p, err := Catalog(t).Profile(IDExt, ID74)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	return p
}
