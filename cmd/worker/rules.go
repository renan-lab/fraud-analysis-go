package main

import (
	"github.com/renan-lab/fraud-analysis-go/internal/analysis"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
)

// newRules maps ANALYSIS_* settings to the rule set, in evaluation order.
// Adding a rule means adding it here and to config.AnalysisConfig.
func newRules(a config.AnalysisConfig) []analysis.Rule {
	return []analysis.Rule{
		analysis.NewVelocityRule(analysis.VelocityParams{Window: a.VelocityWindow, Max: a.VelocityMax, Score: a.VelocityScore}),
		analysis.NewCardTestingRule(analysis.CardTestingParams{
			Window: a.CardTestingWindow, SmallAmount: a.CardTestingSmallAmount, MinCount: a.CardTestingMinCount,
			Score: a.CardTestingScore, CashoutScore: a.CardTestingCashoutScore,
		}),
		analysis.NewImpossibleTravelRule(analysis.TravelParams{MaxKMH: a.TravelMaxKMH, MinKM: a.TravelMinKM, Score: a.TravelScore}),
		analysis.NewAmountOutlierRule(analysis.OutlierParams{Ratio: a.OutlierRatio, MinSamples: a.OutlierMinSamples, Score: a.OutlierScore}),
		analysis.NewNewDeviceForeignRule(analysis.DeviceCountryParams{
			RiskyMCCs: a.RiskyMCCs, NewDeviceScore: a.NewDeviceScore,
			NewCountryScore: a.NewCountryScore, RiskyMCCScore: a.RiskyMCCScore,
		}),
	}
}

func newStoreParams(a config.AnalysisConfig) analysis.StoreParams {
	return analysis.StoreParams{HistoryRetention: a.HistoryRetention, BaselineSize: a.BaselineSize, ProfileTTL: a.ProfileTTL}
}

func newThresholds(a config.AnalysisConfig) analysis.Thresholds {
	return analysis.Thresholds{Review: a.ReviewThreshold, Block: a.BlockThreshold}
}
