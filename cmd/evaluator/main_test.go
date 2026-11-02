package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/evaluation"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
)

func TestParseOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		wantWait time.Duration
		wantErr  error
		anyErr   bool
	}{
		{name: "defaults", args: []string{"--truth=t.ndjson"}, wantWait: 30 * time.Second},
		{name: "gates and json", args: []string{"--truth=t.ndjson", "--format=json", "--wait=1m", "--min-recall=0.5"}, wantWait: time.Minute},
		{name: "help", args: []string{"-h"}, wantErr: flag.ErrHelp},
		{name: "missing truth", args: nil, anyErr: true},
		{name: "bad format", args: []string{"--truth=t.ndjson", "--format=xml"}, anyErr: true},
		{name: "non-positive wait", args: []string{"--truth=t.ndjson", "--wait=0s"}, anyErr: true},
		{name: "non-positive settle", args: []string{"--truth=t.ndjson", "--settle=0s"}, anyErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts, err := parseOptions(tt.args, io.Discard)
			switch {
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
				t.Errorf("parseOptions(%q) error = %v, want %v", tt.args, err, tt.wantErr)
			case tt.anyErr && err == nil:
				t.Errorf("parseOptions(%q) error = nil, want error", tt.args)
			case tt.wantErr == nil && !tt.anyErr && (err != nil || opts.limits.Wait != tt.wantWait):
				t.Errorf("parseOptions(%q) wait = %v, err %v; want %v, nil", tt.args, opts.limits.Wait, err, tt.wantWait)
			}
		})
	}
}

// writeTruth writes a truth file with no accepted transaction, so run never
// needs Kafka.
func writeTruth(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "truth.ndjson")
	line := `{"mode":"malformed","scenario":"malformed/empty_body","label":"reject","expected_status":400,"status":400}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write truth: %v", err)
	}
	return path
}

func TestRunPrintsReportForEachFormat(t *testing.T) {
	t.Parallel()
	cfg := config.Config{KafkaBrokers: []string{"127.0.0.1:1"}}
	for format, want := range map[string]string{formatText: "== Pipeline", formatJSON: `"pipeline"`} {
		var out bytes.Buffer
		if err := run(context.Background(), []string{"--truth=" + writeTruth(t), "--format=" + format}, cfg, &out, io.Discard); err != nil {
			t.Fatalf("run(--format=%s) error = %v", format, err)
		}
		if !strings.Contains(out.String(), want) {
			t.Errorf("run(--format=%s) output = %q, want it to contain %q", format, out.String(), want)
		}
	}
}

func TestRunFailsWhenTruthIsMissing(t *testing.T) {
	t.Parallel()
	err := run(context.Background(), []string{"--truth=" + filepath.Join(t.TempDir(), "nope.ndjson")}, config.Config{}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "open truth") {
		t.Fatalf("run(missing truth) error = %v, want open truth error", err)
	}
}

func TestCheckGates(t *testing.T) {
	t.Parallel()
	report := evaluation.Report{Transactions: evaluation.TransactionStats{Recall: 0.4}}
	var stderr bytes.Buffer
	err := checkGates(evaluation.Gates{MinRecall: 0.5}, report, &stderr)
	if !errors.Is(err, errGatesFailed) || !strings.Contains(stderr.String(), "gate failed: recall") {
		t.Errorf("checkGates(min 0.5, recall 0.4) = %v, stderr %q; want errGatesFailed and a recall line", err, stderr.String())
	}
	if err := checkGates(evaluation.Gates{}, report, io.Discard); err != nil {
		t.Errorf("checkGates(no gates) = %v, want nil", err)
	}
}
