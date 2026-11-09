// SPDX-License-Identifier: MIT

package official

import (
	"bytes"
	"compress/gzip"
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
)

func TestLoad(t *testing.T) {
	if _, err := load([]byte("{")); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("invalid JSON: %v", err)
	}
	if c, err := load([]byte(`{"namespaces":[]}`)); err != nil || len(c.IDs()) != 0 {
		t.Errorf("empty catalog: %v", err)
	}
	// The generated catalog is gzip-compressed JSON.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(`{"namespaces":[{"id":{"standard":"OTF X","version":"2026","revision":"A"}}]}`))
	_ = zw.Close()
	if c, err := load(buf.Bytes()); err != nil || len(c.IDs()) != 1 {
		t.Errorf("gzip catalog: %v", err)
	}
	if _, err := load(buf.Bytes()[:buf.Len()-6]); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("truncated gzip: %v", err)
	}
	if _, err := load([]byte{0x1f, 0x8b, 0}); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("bad gzip header: %v", err)
	}
}

func TestNotice(t *testing.T) {
	n := Notice()
	for _, id := range Namespaces() {
		if !strings.Contains(n, "## "+id.String()) {
			t.Errorf("NOTICE.md lacks %s", id)
		}
	}
	if !strings.Contains(n, "www.iec.ch/CCv1") {
		t.Error("NOTICE.md lacks the license")
	}
}
