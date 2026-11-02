// Command simulator generates realistic and adversarial card transactions
// against the ingestion API (or a dry-run NDJSON file).
//
//	simulator run --target=http --rate=50 --duration=1m --truth=truth.ndjson
//	simulator scenarios
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/config"
	"github.com/renan-lab/fraud-analysis-go/internal/simulator"
)

// seedStream decorrelates the two PCG state words derived from one --seed.
const seedStream = 0x9e3779b97f4a7c15

const usage = "usage: simulator <run|scenarios> [flags]; see simulator run -h"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], config.Load(), os.Stdout, os.Stderr); err != nil {
		slog.Error("simulator stopped with error", "error", err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, cfg config.Config, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "run":
		opts, err := parseRunOptions(args[1:], stderr)
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		if err != nil {
			return err
		}
		return simulate(ctx, opts, cfg, stdout, stderr)
	case "scenarios":
		return listScenarios(stdout)
	default:
		return fmt.Errorf("unknown subcommand %q; %s", args[0], usage)
	}
}

func listScenarios(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SCENARIO\tMODE\tDESCRIPTION")
	for _, s := range simulator.Catalog() {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, s.Mode, s.Description)
	}
	return tw.Flush()
}

func simulate(ctx context.Context, opts runOptions, cfg config.Config, stdout, stderr io.Writer) error {
	gen, err := newGenerator(opts)
	if err != nil {
		return err
	}
	sender, closeSender, err := newSender(opts, cfg, stdout)
	if err != nil {
		return err
	}
	defer closeSender()
	report := simulator.NewReport(time.Now())
	truth, closeTruth, err := newTruthWriter(opts.truth)
	if err != nil {
		return err
	}
	defer closeTruth()
	if err := execute(ctx, opts, gen, sender, report, truth); err != nil {
		return err
	}
	return report.Summary(time.Now()).WriteText(stderr)
}

func execute(ctx context.Context, opts runOptions, gen *simulator.Generator, sender simulator.Sender, report *simulator.Report, truth *simulator.TruthWriter) error {
	observers := []simulator.Observer{report}
	if truth != nil {
		observers = append(observers, truth)
	}
	runner, err := simulator.NewRunner(opts.runConfig, gen, sender, observers...)
	if err != nil {
		return err
	}
	slog.Info("simulation starting", "target", opts.target, "seed", opts.seed, "rate", opts.runConfig.Rate)
	if err := runner.Run(ctx); err != nil {
		return err
	}
	if truth != nil {
		return truth.Err()
	}
	return nil
}

func newGenerator(opts runOptions) (*simulator.Generator, error) {
	rng := rand.New(rand.NewPCG(opts.seed, opts.seed^seedStream))
	pop, err := simulator.NewPopulation(rng, opts.customers)
	if err != nil {
		return nil, err
	}
	clock, err := simulator.NewClock(opts.clockScale, opts.lookback)
	if err != nil {
		return nil, err
	}
	return simulator.NewGenerator(rng, clock, pop, opts.mix)
}

func newSender(opts runOptions, cfg config.Config, stdout io.Writer) (simulator.Sender, func(), error) {
	if opts.target == targetHTTP {
		s, err := simulator.NewHTTPSender(cfg.SimulatorTargetURL, opts.timeout, opts.runConfig.Concurrency)
		return s, func() {}, err
	}
	w, closeOut, err := openOutput(opts.out, stdout)
	if err != nil {
		return nil, nil, err
	}
	return simulator.NewNDJSONSender(w), closeOut, nil
}

func newTruthWriter(path string) (*simulator.TruthWriter, func(), error) {
	if path == "" {
		return nil, func() {}, nil
	}
	w, closeFile, err := openOutput(path, nil)
	if err != nil {
		return nil, nil, err
	}
	return simulator.NewTruthWriter(w), closeFile, nil
}

// openOutput returns stdout for "-" and a created file otherwise.
func openOutput(path string, stdout io.Writer) (io.Writer, func(), error) {
	if path == "-" && stdout != nil {
		return stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output %q: %w", path, err)
	}
	return f, func() {
		if err := f.Close(); err != nil {
			slog.Error("close output", "path", path, "error", err)
		}
	}, nil
}
