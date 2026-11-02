package main

import (
	"slices"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
)

func TestNewRulesBuildsEveryRuleFromDefaults(t *testing.T) {
	cfg := config.Load().Analysis
	var names []string
	for _, r := range newRules(cfg) {
		names = append(names, r.Name())
	}
	want := []string{"velocity", "card_testing", "impossible_travel", "amount_outlier", "new_device_foreign"}
	if !slices.Equal(names, want) {
		t.Fatalf("newRules names = %v, want %v", names, want)
	}
}

func TestDefaultStoreParamsAndThresholdsAreValid(t *testing.T) {
	cfg := config.Load().Analysis
	if err := newStoreParams(cfg).Validate(); err != nil {
		t.Errorf("newStoreParams(defaults).Validate() = %v, want nil", err)
	}
	if err := newThresholds(cfg).Validate(); err != nil {
		t.Errorf("newThresholds(defaults).Validate() = %v, want nil", err)
	}
}
