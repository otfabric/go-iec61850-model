// SPDX-License-Identifier: MIT

package official

import (
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
