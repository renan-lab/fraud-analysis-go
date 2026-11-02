package simulator

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestScenariosExpectStatusDefinedByContract(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 1, 50)
	history := newEmissionHistory(historySize)
	r := newTestRand(2)
	for _, s := range buildScenarios(pop, history, fixedClock{testNow}) {
		for range 20 {
			emissions, err := s.Next(r, testNow)
			if err != nil {
				t.Fatalf("%s.Next() error = %v", s.Name(), err)
			}
			for _, e := range emissions {
				history.record(e)
				if got := contractStatus(e, testNow); got != e.ExpectedStatus {
					t.Fatalf("%s: contract answers %d for body %.200q, scenario expects %d", e.Scenario, got, e.Body, e.ExpectedStatus)
				}
			}
		}
	}
}

func TestScenariosLabelRejectIffNotAccepted(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 3, 20)
	r := newTestRand(4)
	for _, s := range buildScenarios(pop, newEmissionHistory(historySize), fixedClock{testNow}) {
		emissions, err := s.Next(r, testNow)
		if err != nil {
			t.Fatalf("%s.Next() error = %v", s.Name(), err)
		}
		for _, e := range emissions {
			rejected := e.ExpectedStatus != http.StatusAccepted
			if rejected != (e.Label == LabelReject) {
				t.Errorf("%s: label %q with expected status %d, want reject iff status != 202", e.Scenario, e.Label, e.ExpectedStatus)
			}
		}
	}
}

func TestScenarioNamesArePrefixedByModeAndUnique(t *testing.T) {
	t.Parallel()
	var names []string
	for _, info := range Catalog() {
		if !strings.HasPrefix(info.Name, string(info.Mode)+"/") {
			t.Errorf("scenario %q has mode %q, want name prefixed by mode", info.Name, info.Mode)
		}
		if slices.Contains(names, info.Name) {
			t.Errorf("scenario %q is duplicated", info.Name)
		}
		names = append(names, info.Name)
	}
	for _, m := range Modes {
		if !slices.ContainsFunc(Catalog(), func(i ScenarioInfo) bool { return i.Mode == m }) {
			t.Errorf("mode %q has no scenario", m)
		}
	}
}
