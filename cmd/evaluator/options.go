package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/evaluation"
)

const (
	formatText = "text"
	formatJSON = "json"
)

type options struct {
	truth  string
	limits evaluation.ReadLimits
	format string
	gates  evaluation.Gates
}

// parseOptions parses the evaluator flags; usage and errors go to stderr.
func parseOptions(args []string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("evaluator", flag.ContinueOnError)
	fs.SetOutput(stderr)
	o := registerFlags(fs)
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if err := o.validate(); err != nil {
		return options{}, err
	}
	return *o, nil
}

func registerFlags(fs *flag.FlagSet) *options {
	o := &options{}
	fs.StringVar(&o.truth, "truth", "", "ground-truth NDJSON written by simulator run --truth (required)")
	fs.DurationVar(&o.limits.Wait, "wait", 30*time.Second, "total time to wait for the worker to publish every decision")
	fs.DurationVar(&o.limits.Settle, "settle", 2*time.Second, "after every decision arrived, keep reading until the topic is quiet this long (catches redeliveries)")
	fs.StringVar(&o.format, "format", formatText, "report format: text or json")
	fs.Float64Var(&o.gates.MinRecall, "min-recall", 0, "fail if per-transaction recall is below this (0 = off)")
	fs.Float64Var(&o.gates.MinSequenceRecall, "min-sequence-recall", 0, "fail if fraud sequence recall is below this (0 = off)")
	fs.Float64Var(&o.gates.MaxFPR, "max-fpr", 0, "fail if the false positive rate is above this (0 = off)")
	return o
}

func (o *options) validate() error {
	if o.truth == "" {
		return errors.New("--truth is required: pass the file written by simulator run --truth")
	}
	if o.format != formatText && o.format != formatJSON {
		return fmt.Errorf("--format %q: want %s or %s", o.format, formatText, formatJSON)
	}
	if o.limits.Wait <= 0 || o.limits.Settle <= 0 {
		return fmt.Errorf("--wait %s --settle %s: want positive durations", o.limits.Wait, o.limits.Settle)
	}
	return nil
}
