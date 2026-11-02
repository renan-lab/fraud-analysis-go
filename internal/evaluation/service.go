package evaluation

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ReadLimits bound how long the evaluator reads transactions.analyzed.
type ReadLimits struct {
	// Wait is the total budget: the worker may still be catching up when the
	// simulator finishes.
	Wait time.Duration
	// Settle is how long the topic must stay quiet after every expected
	// decision arrived, to also catch late redeliveries.
	Settle time.Duration
}

// Evaluator reads decisions until every expected transaction has one and the
// topic settles, or the wait expires, then builds the Report.
type Evaluator struct {
	reader recordReader
	limits ReadLimits
	logger *slog.Logger
}

// NewEvaluator reads through reader within limits.
//
//	ev := evaluation.NewEvaluator(topicReader, evaluation.ReadLimits{Wait: 30 * time.Second, Settle: 2 * time.Second}, logger)
func NewEvaluator(reader recordReader, limits ReadLimits, logger *slog.Logger) *Evaluator {
	return &Evaluator{reader: reader, limits: limits, logger: logger}
}

// Evaluate returns a report even when the wait expires or ctx is cancelled:
// transactions without a decision are reported as missing. Only a reader
// failure returns an error.
func (e *Evaluator) Evaluate(ctx context.Context, truth TruthSet) (Report, error) {
	collector := NewCollector(truth)
	if !collector.Complete() {
		if err := e.read(ctx, collector); err != nil {
			return Report{}, err
		}
	}
	return BuildReport(collector.Results()), nil
}

func (e *Evaluator) read(ctx context.Context, collector *Collector) error {
	ctx, cancel := context.WithTimeout(ctx, e.limits.Wait)
	defer cancel()
	watch := &quietWatch{}
	go watch.run(ctx, e.limits.Settle, cancel)
	if err := e.reader.Read(ctx, newAnalyzedHandler(collector, watch, e.logger)); err != nil {
		return fmt.Errorf("read decisions: %w", err)
	}
	if !collector.Complete() {
		e.logger.Warn("stopped before every transaction had a decision", "wait", e.limits.Wait.String(), "cause", context.Cause(ctx))
	}
	return nil
}
