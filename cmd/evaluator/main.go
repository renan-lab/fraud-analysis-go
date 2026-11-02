// Command evaluator scores the analysis pipeline against the simulator's
// ground truth: it reads a truth file and every decision on
// transactions.analyzed, then prints a report. It exits 1 when a gate fails.
//
//	evaluator --truth=truth.ndjson --wait=1m --min-sequence-recall=0.9
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/renan-lab/fraud-analysis-go/internal/evaluation"
	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// errGatesFailed means the report was printed but did not meet a gate.
var errGatesFailed = errors.New("evaluation gates failed")

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], config.Load(), os.Stdout, os.Stderr); err != nil {
		slog.Error("evaluator stopped with error", "error", err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, cfg config.Config, stdout, stderr io.Writer) error {
	opts, err := parseOptions(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	report, err := evaluate(ctx, opts, cfg)
	if err != nil {
		return err
	}
	if err := writeReport(report, opts.format, stdout); err != nil {
		return err
	}
	return checkGates(opts.gates, report, stderr)
}

func evaluate(ctx context.Context, opts options, cfg config.Config) (evaluation.Report, error) {
	truth, err := loadTruth(opts.truth)
	if err != nil {
		return evaluation.Report{}, err
	}
	reader, err := kafka.NewTopicReader(cfg.KafkaBrokers, events.TopicTransactionsAnalyzed, slog.Default())
	if err != nil {
		return evaluation.Report{}, err
	}
	defer reader.Close()
	slog.Info("evaluation starting", "truth", opts.truth, "accepted", len(truth.Expectations),
		"wait", opts.limits.Wait.String(), "settle", opts.limits.Settle.String())
	return evaluation.NewEvaluator(reader, opts.limits, slog.Default()).Evaluate(ctx, truth)
}

func loadTruth(path string) (evaluation.TruthSet, error) {
	f, err := os.Open(path)
	if err != nil {
		return evaluation.TruthSet{}, fmt.Errorf("open truth %q: %w", path, err)
	}
	defer f.Close()
	return evaluation.ReadTruth(f)
}

func writeReport(report evaluation.Report, format string, w io.Writer) error {
	if format == formatJSON {
		return report.WriteJSON(w)
	}
	return report.WriteText(w)
}

// checkGates prints each violation as plain text, for a human reading CI output.
func checkGates(gates evaluation.Gates, report evaluation.Report, stderr io.Writer) error {
	violations := gates.Check(report)
	for _, v := range violations {
		fmt.Fprintf(stderr, "gate failed: %s\n", v)
	}
	if len(violations) > 0 {
		return fmt.Errorf("%w: %d violation(s)", errGatesFailed, len(violations))
	}
	return nil
}
