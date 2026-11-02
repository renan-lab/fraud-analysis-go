package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/simulator"
)

const (
	targetHTTP   = "http"
	targetNDJSON = "ndjson"
	defaultCount = 100
)

type runOptions struct {
	target     string
	out        string
	truth      string
	seed       uint64
	customers  int
	mix        []simulator.ModeWeight
	clockScale float64
	lookback   time.Duration
	timeout    time.Duration
	runConfig  simulator.RunConfig
}

type rawRunFlags struct {
	mix, burst string
	opts       runOptions
}

// parseRunOptions parses the flags of `simulator run`; output goes to stderr.
func parseRunOptions(args []string, stderr io.Writer) (runOptions, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	raw := registerRunFlags(fs)
	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}
	return raw.resolve()
}

func registerRunFlags(fs *flag.FlagSet) *rawRunFlags {
	raw := &rawRunFlags{}
	o, rc := &raw.opts, &raw.opts.runConfig
	fs.StringVar(&o.target, "target", targetNDJSON, "where to send: http (SIMULATOR_TARGET_URL) or ndjson (dry run)")
	fs.StringVar(&o.out, "out", "-", "ndjson output file, - for stdout")
	fs.StringVar(&o.truth, "truth", "", "ground-truth NDJSON file (disabled when empty)")
	fs.Uint64Var(&o.seed, "seed", 42, "random seed; same seed and flags generate the same traffic")
	fs.IntVar(&o.customers, "customers", 1000, "number of simulated customers")
	fs.StringVar(&raw.mix, "mix", simulator.DefaultMix, "mode weights as mode:weight pairs")
	fs.Float64Var(&o.clockScale, "time-scale", 1, "simulated seconds per wall second (1 = real time)")
	fs.DurationVar(&o.lookback, "lookback", 24*time.Hour, "how far in the past simulated time starts when time-scale > 1")
	fs.DurationVar(&o.timeout, "timeout", 5*time.Second, "HTTP request timeout")
	fs.Float64Var(&rc.Rate, "rate", 50, "emissions per second")
	fs.DurationVar(&rc.Duration, "duration", 0, "stop after this long (0 = no limit)")
	fs.IntVar(&rc.Count, "count", 0, fmt.Sprintf("stop after this many emissions (0 = no limit; %d if duration is also 0)", defaultCount))
	fs.IntVar(&rc.Concurrency, "concurrency", 8, "concurrent senders")
	fs.DurationVar(&rc.RampUp, "ramp-up", 0, "linearly ramp the rate up over this period")
	fs.StringVar(&raw.burst, "burst", "", "extra emissions fired at once, as size@offset (e.g. 500@30s)")
	return raw
}

func (raw *rawRunFlags) resolve() (runOptions, error) {
	o := raw.opts
	if o.target != targetHTTP && o.target != targetNDJSON {
		return runOptions{}, fmt.Errorf("--target %q: want %s or %s", o.target, targetHTTP, targetNDJSON)
	}
	mix, err := simulator.ParseMix(raw.mix)
	if err != nil {
		return runOptions{}, err
	}
	burst, err := simulator.ParseBurst(raw.burst)
	if err != nil {
		return runOptions{}, err
	}
	o.mix, o.runConfig.Burst = mix, burst
	if o.runConfig.Count == 0 && o.runConfig.Duration == 0 {
		o.runConfig.Count = defaultCount
	}
	return o, nil
}
