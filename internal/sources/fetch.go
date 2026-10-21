// SPDX-License-Identifier: MIT

package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otfabric/go-iec61850-model/model"
)

// ErrNotPinned indicates a package without a locked digest, fetched
// without permission to accept unpinned packages.
var ErrNotPinned = errors.New("sources: package digest is not pinned in the lock")

// ErrDigestMismatch indicates a downloaded package whose digest differs
// from the lock.
var ErrDigestMismatch = errors.New("sources: package digest differs from the lock")

// MaxPackageSize bounds one download.
const MaxPackageSize = 256 << 20

// CacheDirName is the git-ignored directory inside the repository where
// downloaded packages may be kept.
const CacheDirName = ".nsd-cache"

// Fetcher downloads official packages into a cache directory and
// verifies them against the lock.
type Fetcher struct {
	// Client performs the requests; nil uses a client with a timeout.
	Client *http.Client
	// Official decides whether a URL is an allowed download location;
	// nil allows official IEC HTTPS hosts only. Redirects are checked
	// too.
	Official func(*url.URL) bool
	// AllowUnpinned accepts packages without a locked digest.
	AllowUnpinned bool
	// Refresh downloads packages that are already cached.
	Refresh bool
	// UserAgent is sent with every request.
	UserAgent string
	// MaxSize bounds one download; 0 means MaxPackageSize.
	MaxSize int64
}

// FetchResult is the outcome for one package.
type FetchResult struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	// Status is "downloaded", "cached" or "failed".
	Status string `json:"status"`
	// Pinned reports whether the digest was verified against the lock.
	Pinned bool   `json:"pinned"`
	Error  string `json:"error,omitempty"`
}

func (f *Fetcher) official(u *url.URL) bool {
	if f.Official != nil {
		return f.Official(u)
	}
	return officialHost(u.String())
}

func (f *Fetcher) client() *http.Client {
	base := f.Client
	if base == nil {
		base = &http.Client{Timeout: 5 * time.Minute}
	}
	c := *base
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if !f.official(req.URL) {
			return fmt.Errorf("redirect to unofficial location %s refused", req.URL.Host)
		}
		return nil
	}
	return &c
}

// fileName is the cache file name of a package: its name with ".zip".
func fileName(p Package) string {
	n := filepath.Base(p.Name)
	if !strings.HasSuffix(strings.ToLower(n), ".zip") {
		n += ".zip"
	}
	return n
}

// CheckDestination refuses a cache directory inside a Git work tree,
// except the git-ignored .nsd-cache directory at its root, so that
// downloaded packages cannot be committed by accident.
func CheckDestination(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for cur := abs; ; cur = filepath.Dir(cur) {
		if st, err := os.Stat(filepath.Join(cur, ".git")); err == nil && (st.IsDir() || st.Mode().IsRegular()) {
			allowed := filepath.Join(cur, CacheDirName)
			if abs == allowed || strings.HasPrefix(abs, allowed+string(filepath.Separator)) {
				return nil
			}
			return fmt.Errorf("%w: %s is inside the Git work tree %s; use %s or a directory outside the repository",
				model.ErrSourceUnavailable, dir, cur, allowed)
		}
		if parent := filepath.Dir(cur); parent == cur {
			return nil
		}
	}
}

// Fetch downloads every package of the given tiers that has a URL into
// dir and verifies it against the lock. Cached files whose digest
// matches the lock are not downloaded again. A digest mismatch, an
// unpinned package (unless AllowUnpinned) or a download failure is
// reported per package and as the returned error; the other packages
// are still processed. Files are written atomically: a failed download
// leaves no partial file.
func (f *Fetcher) Fetch(ctx context.Context, idx *Index, lock *Lock, dir string, tiers []string) ([]FetchResult, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var results []FetchResult
	var errs []error
	for _, p := range idx.Packages {
		if p.URL == "" || !containsString(tiers, p.Tier) {
			continue
		}
		r, err := f.fetchOne(ctx, p, lock, dir)
		if err != nil {
			r.Status, r.Error = "failed", err.Error()
			errs = append(errs, fmt.Errorf("%s: %w", p.Name, err))
		}
		results = append(results, r)
	}
	return results, errors.Join(errs...)
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (f *Fetcher) fetchOne(ctx context.Context, p Package, lock *Lock, dir string) (FetchResult, error) {
	path := filepath.Join(dir, fileName(p))
	r := FetchResult{Name: p.Name, Path: path}
	pin, pinned := lock.PackageDigest(p.Name)
	if !pinned && !f.AllowUnpinned {
		return r, ErrNotPinned
	}
	r.Pinned = pinned
	if !f.Refresh && pinned {
		if sum, err := HashFile(path); err == nil && sum == pin {
			st, _ := os.Stat(path)
			r.SHA256, r.Size, r.Status = sum, st.Size(), "cached"
			return r, nil
		}
	}
	u, err := url.Parse(p.URL)
	if err != nil || !f.official(u) {
		return r, fmt.Errorf("%w: %q is not an official download location", model.ErrSourceUnavailable, p.URL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return r, err
	}
	if f.UserAgent != "" {
		req.Header.Set("User-Agent", f.UserAgent)
	}
	resp, err := f.client().Do(req)
	if err != nil {
		return r, fmt.Errorf("%w: %v", model.ErrSourceUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return r, fmt.Errorf("%w: HTTP %s", model.ErrSourceUnavailable, resp.Status)
	}
	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return r, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	h := sha256.New()
	limit := f.MaxSize
	if limit <= 0 {
		limit = MaxPackageSize
	}
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, limit+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return r, fmt.Errorf("%w: %v", model.ErrSourceUnavailable, err)
	}
	if n > limit {
		return r, fmt.Errorf("%w: package exceeds %d bytes", model.ErrLimitExceeded, limit)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	r.SHA256, r.Size = sum, n
	if pinned && sum != pin {
		return r, fmt.Errorf("%w: got %s, locked %s", ErrDigestMismatch, sum, pin)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return r, err
	}
	r.Status = "downloaded"
	return r, nil
}
