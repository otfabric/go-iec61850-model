// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/internal/fixtures"
	"github.com/otfabric/go-iec61850-model/internal/sources"
	"github.com/otfabric/go-iec61850-model/model"
)

// roundTrip is an http.RoundTripper backed by a function.
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// withFetchClient makes sources fetch use rt instead of the network.
func withFetchClient(t *testing.T, rt roundTrip) {
	t.Helper()
	old := fetchClient
	fetchClient = &http.Client{Transport: rt}
	t.Cleanup(func() { fetchClient = old })
}

func failingTransport(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

// failingWriter fails every write, like a closed pipe.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestFetchExitCode(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
	}{
		{fmt.Errorf("a: %w", sources.ErrDigestMismatch), exitInvalidInput},
		{errors.Join(fmt.Errorf("a: %w", model.ErrSourceUnavailable), fmt.Errorf("b: %w", sources.ErrNotPinned)), exitInvalidInput},
		{fmt.Errorf("a: %w", model.ErrLimitExceeded), exitInvalidInput},
		{fmt.Errorf("a: %w: HTTP 503", model.ErrSourceUnavailable), exitTransport},
		{fmt.Errorf("a: %w", context.Canceled), exitInterrupted},
		{errors.New("mkdir: not a directory"), exitInternal},
	} {
		if got := fetchExitCode(tc.err); got != tc.code {
			t.Errorf("fetchExitCode(%v) = %d, want %d", tc.err, got, tc.code)
		}
	}
}

// TestExitCodesByCommand drives the root command once for every exit
// code it can produce.
func TestExitCodesByCommand(t *testing.T) {
	dest, lockPath := fetchCache(t, true)
	fetch := []string{"sources", "fetch", "--index", "../../sources/index.json", "--lock", lockPath, "--dest", dest, "--tiers", "required", "--refresh"}
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		rt   roundTrip
		args []string
		code int
		want string
	}{
		{"ok", nil, []string{"version"}, exitOK, ""},
		{"findings: diff", nil, []string{"diff", "--exit-code", fixtures.Path(fixtures.NS74), fixtures.Path(fixtures.NS74R2)}, exitFindings, "sources differ"},
		{"findings: validate", nil, []string{"validate", fixtures.Path(fixtures.NS74)}, exitFindings, "validation failed"},
		{"usage: unknown flag", nil, []string{"inspect", "--nope"}, exitUsage, "unknown flag"},
		{"usage: flag value", nil, []string{"--strict=maybe", "inspect"}, exitUsage, "invalid argument"},
		{"usage: argument count", nil, []string{"diff", "a"}, exitUsage, "accepts 2 arg(s)"},
		{"usage: unknown command", nil, []string{"frob"}, exitUsage, `unknown command "frob"`},
		{"usage: completion shell", nil, []string{"completion", "tcsh"}, exitUsage, "invalid argument"},
		{"invalid input: missing source", nil, []string{"inspect", "/no/such/file.zip"}, exitInvalidInput, "cannot open"},
		{"invalid input: digest", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("other content"))}, nil
		}, fetch, exitInvalidInput, "digest differs"},
		{"transport: download", failingTransport, fetch, exitTransport, "connection refused"},
		{"permission: publish", nil, withProfile("generate", "go", "-o", filepath.Join(t.TempDir(), "g"), "--publish"), exitPermission, "license policy"},
		{"internal: cache not writable", failingTransport, []string{"sources", "fetch", "--index", "../../sources/index.json", "--lock", lockPath, "--dest", notDir}, exitInternal, "nsdctl: "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.rt != nil {
				withFetchClient(t, tc.rt)
			}
			out, e, code := nsdctl(t, tc.args...)
			if code != tc.code || !strings.Contains(e, tc.want) {
				t.Errorf("exit %d, want %d; stderr %q does not contain %q\n%s", code, tc.code, e, tc.want, out)
			}
		})
	}
}

func TestExitOutputFailure(t *testing.T) {
	var errOut bytes.Buffer
	if code := run(context.Background(), []string{"--json", "version"}, failingWriter{}, &errOut); code != exitInternal || !strings.Contains(errOut.String(), "write output: broken pipe") {
		t.Errorf("exit %d, stderr %q", code, errOut.String())
	}
}

func TestExitInterrupted(t *testing.T) {
	withFetchClient(t, func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	dest, lockPath := fetchCache(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	code := run(ctx, []string{"sources", "fetch", "--index", "../../sources/index.json", "--lock", lockPath, "--dest", dest, "--tiers", "required", "--refresh"}, &out, &errOut)
	if code != exitInterrupted || !strings.Contains(errOut.String(), "nsdctl: ") {
		t.Errorf("exit %d, stderr %q", code, errOut.String())
	}
}
