package simulator

import (
	"math/rand/v2"
	"time"
)

// Scenario produces one or more related emissions per call. Sequences (e.g. a
// velocity burst) are returned together so their timestamps stay coherent.
type Scenario interface {
	Name() string
	Mode() Mode
	Description() string
	Next(r *rand.Rand, now time.Time) ([]Emission, error)
}

// scenarioSpec implements Scenario with a closure, keeping each scenario a small function.
type scenarioSpec struct {
	name        string
	mode        Mode
	description string
	next        func(r *rand.Rand, now time.Time) ([]Emission, error)
}

func (s scenarioSpec) Name() string        { return s.name }
func (s scenarioSpec) Mode() Mode          { return s.mode }
func (s scenarioSpec) Description() string { return s.description }

func (s scenarioSpec) Next(r *rand.Rand, now time.Time) ([]Emission, error) {
	return s.next(r, now)
}

// ScenarioInfo describes a scenario for the `simulator scenarios` command.
type ScenarioInfo struct {
	Name        string
	Mode        Mode
	Description string
}

// Catalog lists every scenario the generator can pick.
//
//	for _, s := range Catalog() { fmt.Println(s.Name) }
func Catalog() []ScenarioInfo {
	scenarios := buildScenarios(nil, nil, WallClock{})
	infos := make([]ScenarioInfo, len(scenarios))
	for i, s := range scenarios {
		infos[i] = ScenarioInfo{Name: s.Name(), Mode: s.Mode(), Description: s.Description()}
	}
	return infos
}

func buildScenarios(pop *Population, history *emissionHistory, wall Clock) []Scenario {
	var all []Scenario
	all = append(all, legitScenarios(pop)...)
	all = append(all, fraudScenarios(pop)...)
	all = append(all, malformedScenarios(pop)...)
	all = append(all, attackScenarios(pop)...)
	all = append(all, duplicateScenarios(pop, history)...)
	all = append(all, timingScenarios(pop, wall)...)
	return all
}

// single wraps one emission and its error into the Scenario.Next return shape.
func single(e Emission, err error) ([]Emission, error) {
	if err != nil {
		return nil, err
	}
	return []Emission{e}, nil
}
