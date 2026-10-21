// SPDX-License-Identifier: MIT

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/sources"
)

// fetchCache fills a cache directory with stand-in files for the required
// packages and writes a lock that pins them, so that sources fetch finds
// every package cached and contacts no server.
func fetchCache(t *testing.T, pinAll bool) (dest, lockPath string) {
	t.Helper()
	idx, err := sources.LoadIndex("../../sources/index.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dest = filepath.Join(dir, "cache")
	if err := os.Mkdir(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := &sources.Lock{SchemaVersion: 1}
	for i, p := range idx.Packages {
		if p.Tier != sources.TierRequired {
			continue
		}
		content := []byte("stand-in for " + p.Name)
		if err := os.WriteFile(filepath.Join(dest, cacheName(p.Name)), content, 0o600); err != nil {
			t.Fatal(err)
		}
		if !pinAll && i == 0 {
			continue
		}
		sum := sha256.Sum256(content)
		lock.Sources = append(lock.Sources, sources.Entry{Namespace: p.Name, Package: p.Name, SHA256: hex.EncodeToString(sum[:])})
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	lockPath = filepath.Join(dir, "lock.json")
	if err := os.WriteFile(lockPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dest, lockPath
}

// cacheName is the file name sources fetch caches a package under.
func cacheName(name string) string {
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name += ".zip"
	}
	return filepath.Base(name)
}

func TestSourcesFetchCached(t *testing.T) {
	dest, lockPath := fetchCache(t, true)
	args := []string{"sources", "fetch", "--index", "../../sources/index.json", "--lock", lockPath, "--dest", dest, "--tiers", "required"}
	out, e, code := nsdctl(t, args...)
	if code != exitOK || strings.Count(out, "cached") != 5 || !strings.Contains(out, "PACKAGE") {
		t.Fatalf("cached fetch: %d\n%s\n%s", code, out, e)
	}
	out, _, code = nsdctl(t, append([]string{"--json"}, args...)...)
	var res []sources.FetchResult
	if code != exitOK || json.Unmarshal([]byte(out), &res) != nil || len(res) != 5 || res[0].Status != "cached" {
		t.Errorf("cached fetch JSON: %d\n%s", code, out)
	}
	if _, _, code := nsdctl(t, "sources", "fetch", "--index", "../../sources/index.json", "--lock", "missing.json", "--dest", dest); code != exitSource {
		t.Errorf("missing lock: %d", code)
	}
}

func TestSourcesFetchUnpinned(t *testing.T) {
	dest, lockPath := fetchCache(t, false)
	out, e, code := nsdctl(t, "sources", "fetch", "--index", "../../sources/index.json", "--lock", lockPath, "--dest", dest, "--tiers", "required")
	if code != exitValidation || !strings.Contains(out, "unpinned") {
		t.Errorf("unpinned package: %d\n%s\n%s", code, out, e)
	}
}
