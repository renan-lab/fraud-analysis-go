package simulator

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

// DefaultMix is a mostly-legit traffic profile with every adversarial mode present.
const DefaultMix = "legit:90,fraud:5,malformed:2,attack:1,duplicate:1,timing:1"

// ModeWeight is the relative frequency of a mode in the generated traffic.
type ModeWeight struct {
	Mode   Mode
	Weight int
}

// ParseMix parses "mode:weight" pairs separated by commas.
//
//	weights, err := ParseMix("legit:90,fraud:10")
func ParseMix(spec string) ([]ModeWeight, error) {
	var weights []ModeWeight
	total := 0
	for pair := range strings.SplitSeq(spec, ",") {
		w, err := parseModeWeight(strings.TrimSpace(pair))
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(weights, func(x ModeWeight) bool { return x.Mode == w.Mode }) {
			return nil, fmt.Errorf("mix %q: mode %q repeated, want each mode at most once", spec, w.Mode)
		}
		weights = append(weights, w)
		total += w.Weight
	}
	if total == 0 {
		return nil, fmt.Errorf("mix %q: total weight is 0, want at least one positive weight", spec)
	}
	return weights, nil
}

func parseModeWeight(pair string) (ModeWeight, error) {
	name, rawWeight, ok := strings.Cut(pair, ":")
	if !ok {
		return ModeWeight{}, fmt.Errorf("mix entry %q: want mode:weight", pair)
	}
	mode := Mode(name)
	if !slices.Contains(Modes, mode) {
		return ModeWeight{}, fmt.Errorf("mix entry %q: unknown mode %q, want one of %v", pair, name, Modes)
	}
	weight, err := strconv.Atoi(rawWeight)
	if err != nil || weight < 0 {
		return ModeWeight{}, fmt.Errorf("mix entry %q: weight %q must be an integer >= 0", pair, rawWeight)
	}
	return ModeWeight{Mode: mode, Weight: weight}, nil
}

// Generator picks a mode by weight, then a scenario of that mode uniformly.
// It is not safe for concurrent use: the runner calls it from one goroutine,
// which also keeps the sequence reproducible for a given seed.
type Generator struct {
	rng     *rand.Rand
	clock   Clock
	weights []ModeWeight
	total   int
	byMode  map[Mode][]Scenario
	history *emissionHistory
}

// NewGenerator wires every scenario of the catalog to pop and clock.
//
//	gen, err := NewGenerator(rng, WallClock{}, pop, weights)
func NewGenerator(rng *rand.Rand, clock Clock, pop *Population, weights []ModeWeight) (*Generator, error) {
	history := newEmissionHistory(historySize)
	return newGenerator(rng, clock, weights, buildScenarios(pop, history, WallClock{}), history)
}

func newGenerator(rng *rand.Rand, clock Clock, weights []ModeWeight, scenarios []Scenario, history *emissionHistory) (*Generator, error) {
	g := &Generator{rng: rng, clock: clock, weights: weights, byMode: map[Mode][]Scenario{}, history: history}
	for _, s := range scenarios {
		g.byMode[s.Mode()] = append(g.byMode[s.Mode()], s)
	}
	for _, w := range weights {
		if w.Weight > 0 && len(g.byMode[w.Mode]) == 0 {
			return nil, fmt.Errorf("mode %q has weight %d but no scenarios", w.Mode, w.Weight)
		}
		g.total += w.Weight
	}
	if g.total == 0 {
		return nil, fmt.Errorf("weights %v: total is 0, want at least one positive weight", weights)
	}
	return g, nil
}

// Next returns the emissions of one scenario run and remembers accepted ones
// so duplicate scenarios can replay them.
func (g *Generator) Next() ([]Emission, error) {
	scenarios := g.byMode[g.pickMode()]
	s := scenarios[g.rng.IntN(len(scenarios))]
	emissions, err := s.Next(g.rng, g.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("scenario %s: %w", s.Name(), err)
	}
	for _, e := range emissions {
		g.history.record(e)
	}
	return emissions, nil
}

func (g *Generator) pickMode() Mode {
	n := g.rng.IntN(g.total)
	for _, w := range g.weights {
		if n < w.Weight {
			return w.Mode
		}
		n -= w.Weight
	}
	return g.weights[len(g.weights)-1].Mode
}
