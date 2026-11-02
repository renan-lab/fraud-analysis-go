package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
)

func TestParseRunOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantErr   error
		anyErr    bool
	}{
		{name: "defaults to 100 emissions", args: nil, wantCount: defaultCount},
		{name: "duration disables default count", args: []string{"--duration=1s"}, wantCount: 0},
		{name: "explicit count", args: []string{"--count=7", "--burst=5@1s"}, wantCount: 7},
		{name: "help", args: []string{"-h"}, wantErr: flag.ErrHelp},
		{name: "bad target", args: []string{"--target=kafka"}, anyErr: true},
		{name: "bad mix", args: []string{"--mix=chaos:1"}, anyErr: true},
		{name: "bad burst", args: []string{"--burst=lots"}, anyErr: true},
	}
	for _, tt := range tests {
		opts, err := parseRunOptions(tt.args, io.Discard)
		switch {
		case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
			t.Errorf("%s: parseRunOptions(%q) error = %v, want %v", tt.name, tt.args, err, tt.wantErr)
		case tt.anyErr && err == nil:
			t.Errorf("%s: parseRunOptions(%q) error = nil, want error", tt.name, tt.args)
		case tt.wantErr == nil && !tt.anyErr && (err != nil || opts.runConfig.Count != tt.wantCount):
			t.Errorf("%s: parseRunOptions(%q) count = %d, err %v; want %d, nil", tt.name, tt.args, opts.runConfig.Count, err, tt.wantCount)
		}
	}
}

func TestRunRejectsMissingOrUnknownSubcommand(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"explode"}} {
		if err := run(context.Background(), args, config.Config{}, io.Discard, io.Discard); err == nil {
			t.Errorf("run(%q) error = nil, want error", args)
		}
	}
}

func TestRunScenariosListsCatalog(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := run(context.Background(), []string{"scenarios"}, config.Config{}, &out, io.Discard); err != nil {
		t.Fatalf("run(scenarios) error = %v", err)
	}
	for _, want := range []string{"SCENARIO", "legit/purchase", "attack/oversized_body", "timing/future"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("run(scenarios) output missing %q", want)
		}
	}
}

func TestRunDryRunWritesNDJSONAndTruth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, truth := filepath.Join(dir, "out.ndjson"), filepath.Join(dir, "truth.ndjson")
	args := []string{"run", "--count=30", "--rate=10000", "--customers=20", "--out=" + out, "--truth=" + truth}
	var stderr bytes.Buffer
	if err := run(context.Background(), args, config.Config{}, io.Discard, &stderr); err != nil {
		t.Fatalf("run(%q) error = %v", args, err)
	}
	outLines, truthLines := countLines(t, out), countLines(t, truth)
	if outLines < 30 || outLines != truthLines {
		t.Errorf("out has %d lines, truth %d; want equal and >= 30", outLines, truthLines)
	}
	if !strings.Contains(stderr.String(), "expectation violations: none") {
		t.Errorf("report = %q, want no violations in dry run", stderr.String())
	}
}

func TestRunHTTPTargetReportsStatuses(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)
	args := []string{"run", "--target=http", "--count=20", "--rate=10000", "--mix=legit:1", "--customers=5"}
	var stderr bytes.Buffer
	if err := run(context.Background(), args, config.Config{SimulatorTargetURL: srv.URL}, io.Discard, &stderr); err != nil {
		t.Fatalf("run(%q) error = %v", args, err)
	}
	if !strings.Contains(stderr.String(), "202=20") {
		t.Errorf("report = %q, want 202=20", stderr.String())
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return bytes.Count(data, []byte("\n"))
}
