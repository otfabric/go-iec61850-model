// SPDX-License-Identifier: MIT

package nsd_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// Every limit must be positive, and every parse and load function says
// which option is not, before it reads anything.
func TestOptionsAreValidated(t *testing.T) {
	ns := fixtures.Read(t, fixtures.NS72)
	archive := buildZIP(t, entry{name: "a.nsd", data: ns})
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "pkg.zip")
	if err := os.WriteFile(zipPath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	loaders := map[string]func(opts ...nsd.Option) error{
		"Parse": func(opts ...nsd.Option) error {
			_, err := nsd.Parse(bytes.NewReader(ns), opts...)
			return err
		},
		"ParseNamespace": func(opts ...nsd.Option) error {
			_, _, err := nsd.ParseNamespace(bytes.NewReader(ns), opts...)
			return err
		},
		"ReadManifest": func(opts ...nsd.Option) error {
			_, err := nsd.ReadManifest(strings.NewReader(`<IECManifest/>`), opts...)
			return err
		},
		"ParseSchema": func(opts ...nsd.Option) error {
			_, err := nsd.ParseSchema(strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`), opts...)
			return err
		},
		"ReadZIP": func(opts ...nsd.Option) error {
			_, err := nsd.ReadZIP(bytes.NewReader(archive), opts...)
			return err
		},
		"LoadZIP": func(opts ...nsd.Option) error {
			_, err := nsd.LoadZIP(zipPath, opts...)
			return err
		},
		"LoadFile": func(opts ...nsd.Option) error {
			_, err := nsd.LoadFile(zipPath, opts...)
			return err
		},
		"LoadDir": func(opts ...nsd.Option) error {
			_, err := nsd.LoadDir(dir, opts...)
			return err
		},
	}
	limits := map[string]func(n int64) nsd.Option{
		"WithMaxDocumentSize":     nsd.WithMaxDocumentSize,
		"WithMaxArchiveSize":      nsd.WithMaxArchiveSize,
		"WithMaxArchiveEntries":   func(n int64) nsd.Option { return nsd.WithMaxArchiveEntries(int(n)) },
		"WithMaxDepth":            func(n int64) nsd.Option { return nsd.WithMaxDepth(int(n)) },
		"WithMaxTokens":           func(n int64) nsd.Option { return nsd.WithMaxTokens(int(n)) },
		"WithMaxCompressionRatio": nsd.WithMaxCompressionRatio,
	}
	for loader, load := range loaders {
		if err := load(); err != nil {
			t.Fatalf("%s with the defaults: %v", loader, err)
		}
		for option, with := range limits {
			for _, v := range []int64{0, -1, math.MinInt32, math.MinInt64} {
				err := load(with(v), nsd.WithSource("src"))
				var se *model.SourceError
				if !errors.Is(err, model.ErrLimitExceeded) || !errors.As(err, &se) || !strings.Contains(se.Reason, "option "+option+":") || !strings.Contains(se.Reason, "is not a positive limit") {
					t.Errorf("%s with %s(%d): error = %v, want ErrLimitExceeded naming the option", loader, option, v, err)
				}
			}
			// The largest value is a valid limit.
			max := int64(math.MaxInt64)
			if option == "WithMaxArchiveEntries" || option == "WithMaxDepth" || option == "WithMaxTokens" {
				max = int64(math.MaxInt)
			}
			if err := load(with(max)); err != nil {
				t.Errorf("%s with %s(%d): %v", loader, option, max, err)
			}
			if err := load(with(max - 1)); err != nil {
				t.Errorf("%s with %s(%d): %v", loader, option, max-1, err)
			}
		}
		err := load(nsd.WithSchemaNamespaces())
		if !errors.Is(err, model.ErrUnsupportedSchema) || !strings.Contains(err.Error(), "option WithSchemaNamespaces: no namespace is accepted") {
			t.Errorf("%s without a schema namespace: error = %v, want ErrUnsupportedSchema naming the option", loader, err)
		}
		// A nil option is skipped.
		if err := load(nil); err != nil {
			t.Errorf("%s with a nil option: %v", loader, err)
		}
	}
	// The error names the source the caller gave.
	_, err := nsd.Parse(bytes.NewReader(ns), nsd.WithSource("ns.nsd"), nsd.WithMaxDepth(0))
	var se *model.SourceError
	if !errors.As(err, &se) || se.Source != "ns.nsd" || se.Reason != "option WithMaxDepth: 0 is not a positive limit" {
		t.Errorf("error = %v", err)
	}
	// An empty namespace URI is a namespace: documents without one are accepted.
	if _, err := nsd.Parse(strings.NewReader(`<NS id="T" version="1"/>`), nsd.WithSchemaNamespaces("")); err != nil {
		t.Errorf("WithSchemaNamespaces(\"\"): %v", err)
	}
}

// A limit of 1, the smallest valid value, is applied as a limit.
func TestSmallestLimits(t *testing.T) {
	doc := nsOpen + `</NS>`
	for name, opt := range map[string]nsd.Option{
		"size":   nsd.WithMaxDocumentSize(1),
		"tokens": nsd.WithMaxTokens(1),
	} {
		_, err := parse(t, doc, opt)
		if !errors.Is(err, model.ErrLimitExceeded) || strings.Contains(err.Error(), "option") {
			t.Errorf("%s 1: error = %v, want the limit itself", name, err)
		}
	}
	if _, err := parse(t, doc, nsd.WithMaxDepth(1)); err != nil {
		t.Errorf("depth 1 for a document of one level: %v", err)
	}
	archive := buildZIP(t, entry{name: "a.nsd", data: []byte(doc)})
	if _, err := nsd.ReadZIP(bytes.NewReader(archive), nsd.WithMaxArchiveEntries(1)); err != nil {
		t.Errorf("one entry with a limit of 1: %v", err)
	}
	if _, err := nsd.ReadZIP(bytes.NewReader(archive), nsd.WithMaxCompressionRatio(1)); err != nil {
		t.Errorf("an entry below 1 MiB with a ratio limit of 1: %v", err)
	}
	if _, err := nsd.ReadZIP(bytes.NewReader(archive), nsd.WithMaxArchiveSize(1)); !errors.Is(err, model.ErrLimitExceeded) || strings.Contains(err.Error(), "option") {
		t.Errorf("archive size 1: error = %v, want the limit itself", err)
	}
}

// reader returns its data in pieces.
type reader struct {
	data  []byte
	piece int
	err   error // returned at the end instead of io.EOF, when set
}

func (r *reader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	n := min(len(p), r.piece, len(r.data))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

// The size limit is exact at its boundary, whatever the pieces the
// reader returns, and a read error after the limit is reported.
func TestSizeLimitBoundaryAndReadErrors(t *testing.T) {
	doc := []byte(nsOpen + `</NS>`)
	for _, piece := range []int{1, 7, len(doc), 4096} {
		for delta, ok := range map[int]bool{-1: false, 0: true, 1: true} {
			_, err := nsd.Parse(&reader{data: doc, piece: piece}, nsd.WithMaxDocumentSize(int64(len(doc)+delta)))
			if ok && err != nil || !ok && !errors.Is(err, model.ErrLimitExceeded) {
				t.Errorf("pieces of %d, limit %+d: error = %v", piece, delta, err)
			}
		}
	}
	boom := errors.New("boom")
	// The reader fails after exactly the limit: the failure is reported,
	// not taken for the end of the input.
	_, err := nsd.Parse(&reader{data: doc, piece: 16, err: boom}, nsd.WithMaxDocumentSize(int64(len(doc))))
	if !errors.Is(err, model.ErrSourceUnavailable) || !errors.Is(err, boom) {
		t.Errorf("failure at the limit: error = %v, want ErrSourceUnavailable", err)
	}
	_, err = nsd.Parse(&reader{data: doc, piece: 16, err: boom}, nsd.WithMaxDocumentSize(int64(len(doc))+10))
	if !errors.Is(err, model.ErrSourceUnavailable) || !errors.Is(err, boom) {
		t.Errorf("failure below the limit: error = %v, want ErrSourceUnavailable", err)
	}
}
