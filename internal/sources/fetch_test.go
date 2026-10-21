// SPDX-License-Identifier: MIT

package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
)

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func fetchServer(t *testing.T) (*httptest.Server, *Fetcher, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/a.zip":
			_, _ = w.Write([]byte("package a"))
		case "/big.zip":
			_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		case "/redirect.zip":
			http.Redirect(w, r, "https://mirror.example.org/a.zip", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	base, _ := url.Parse(srv.URL)
	f := &Fetcher{Client: srv.Client(), Official: func(u *url.URL) bool { return u.Host == base.Host }, UserAgent: "test"}
	return srv, f, &hits
}

func pkg(srv *httptest.Server, name string) Package {
	return Package{Tier: TierRequired, Part: "7-4", Name: name, Release: "2007B6", Kind: KindNSD, Content: "light", URL: srv.URL + "/" + name}
}

func TestFetch(t *testing.T) {
	srv, f, hits := fetchServer(t)
	dir := t.TempDir()
	idx := &Index{Packages: []Package{pkg(srv, "a.zip"), {Tier: TierOptional, Name: "skipped"}}}
	lock := &Lock{Sources: []Entry{{Namespace: "A:1", Package: "a.zip", SHA256: digest("package a")}}}

	res, err := f.Fetch(context.Background(), idx, lock, dir, Tiers())
	if err != nil || len(res) != 1 || res[0].Status != "downloaded" || !res[0].Pinned || res[0].Size != 9 {
		t.Fatalf("download: %+v %v", res, err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.zip")); string(data) != "package a" {
		t.Error("content")
	}
	if res, err := f.Fetch(context.Background(), idx, lock, dir, Tiers()); err != nil || res[0].Status != "cached" || *hits != 1 {
		t.Errorf("cache: %+v %v hits %d", res, err, *hits)
	}
	f.Refresh = true
	if res, _ := f.Fetch(context.Background(), idx, lock, dir, Tiers()); res[0].Status != "downloaded" || *hits != 2 {
		t.Errorf("refresh: %+v", res)
	}
	f.Refresh = false
	if res, _ := f.Fetch(context.Background(), idx, lock, dir, []string{TierOptional}); len(res) != 0 {
		t.Error("tier filter")
	}

	lock.Sources[0].SHA256 = digest("other")
	_, err = f.Fetch(context.Background(), idx, lock, dir, Tiers())
	if !errors.Is(err, ErrDigestMismatch) {
		t.Errorf("mismatch: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.zip")); string(data) != "package a" {
		t.Error("a mismatching download must not replace the cached file")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temporary files left: %v", entries)
	}
}

func TestFetchRefusals(t *testing.T) {
	srv, f, _ := fetchServer(t)
	dir := t.TempDir()
	empty := &Lock{}
	if _, err := f.Fetch(context.Background(), &Index{Packages: []Package{pkg(srv, "a.zip")}}, empty, dir, Tiers()); !errors.Is(err, ErrNotPinned) {
		t.Errorf("unpinned: %v", err)
	}
	f.AllowUnpinned = true
	if res, err := f.Fetch(context.Background(), &Index{Packages: []Package{pkg(srv, "a.zip")}}, empty, dir, Tiers()); err != nil || res[0].Pinned {
		t.Errorf("allow unpinned: %+v %v", res, err)
	}
	for _, tc := range []struct {
		name     string
		sentinel error
	}{
		{"missing.zip", model.ErrSourceUnavailable},
		{"redirect.zip", model.ErrSourceUnavailable},
	} {
		if _, err := f.Fetch(context.Background(), &Index{Packages: []Package{pkg(srv, tc.name)}}, empty, dir, Tiers()); !errors.Is(err, tc.sentinel) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	f.MaxSize = 10
	if _, err := f.Fetch(context.Background(), &Index{Packages: []Package{pkg(srv, "big.zip")}}, empty, dir, Tiers()); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("size: %v", err)
	}
	unofficial := Package{Tier: TierRequired, Name: "x.zip", URL: "https://example.org/x.zip"}
	if _, err := (&Fetcher{AllowUnpinned: true}).Fetch(context.Background(), &Index{Packages: []Package{unofficial}}, empty, dir, Tiers()); !errors.Is(err, model.ErrSourceUnavailable) {
		t.Errorf("unofficial URL: %v", err)
	}
	if !(&Fetcher{}).official(mustURL("https://assets.iec.ch/public/tc57/a.zip")) || (&Fetcher{}).official(mustURL("http://assets.iec.ch/a.zip")) {
		t.Error("default official host policy")
	}
	if _, err := (&Fetcher{}).Fetch(context.Background(), &Index{}, empty, filepath.Join(writeFile(t, dir, "file", "x"), "sub"), Tiers()); err == nil {
		t.Error("destination below a file")
	}
}

func mustURL(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}

func TestCheckDestination(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dir string
		ok  bool
	}{
		{filepath.Join(repo, CacheDirName), true},
		{filepath.Join(repo, CacheDirName, "sub"), true},
		{filepath.Join(repo, "packages"), false},
		{repo, false},
		{filepath.Join(repo, ".nsd-cache-other"), false},
		{t.TempDir(), true},
	} {
		err := CheckDestination(tc.dir)
		if (err == nil) != tc.ok {
			t.Errorf("CheckDestination(%s) = %v, want ok=%v", tc.dir, err, tc.ok)
		}
	}
	if fileName(Package{Name: "a.light"}) != "a.light.zip" || fileName(Package{Name: "b.ZIP"}) != "b.ZIP" {
		t.Error("fileName")
	}
	lock := &Lock{Sources: []Entry{{Package: "A.Light.zip", SHA256: "x"}, {Package: "b.zip"}}}
	if d, ok := lock.PackageDigest("a.light"); !ok || d != "x" {
		t.Error("PackageDigest matches case-insensitively without .zip")
	}
	if _, ok := lock.PackageDigest("b.zip"); ok {
		t.Error("entry without digest")
	}
}
