package analysis

import (
	"strings"
	"testing"
)

func TestNewDeviceForeignRule(t *testing.T) {
	t.Parallel()
	rule := NewNewDeviceForeignRule(DeviceCountryParams{RiskyMCCs: []string{"7995", "6051"}, NewDeviceScore: 20, NewCountryScore: 25, RiskyMCCScore: 25})
	tests := []struct {
		name       string
		features   Features
		mcc        string
		wantScore  int
		wantReason string
	}{
		{name: "known device and country", features: Features{DeviceKnown: true, CountryKnown: true}, mcc: "5411", wantScore: 0},
		{name: "new device only", features: Features{CountryKnown: true}, mcc: "5411", wantScore: 20, wantReason: "new device dev_test"},
		{name: "first purchase of account", features: Features{}, mcc: "5411", wantScore: 45},
		{name: "risky mcc on known profile", features: Features{DeviceKnown: true, CountryKnown: true}, mcc: "7995", wantScore: 25, wantReason: "risky mcc 7995"},
		{name: "all signals", features: Features{}, mcc: "6051", wantScore: 70, wantReason: "new device dev_test, first purchase in BR, risky mcc 6051"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tx := newTestTransaction()
			tx.MerchantMCC = tt.mcc
			got := rule.Evaluate(tx, tt.features)
			if got.Score != tt.wantScore {
				t.Errorf("Evaluate(%+v, mcc %s) = %+v, want score %d", tt.features, tt.mcc, got, tt.wantScore)
			}
			if tt.wantReason != "" && got.Reason != tt.wantReason {
				t.Errorf("Evaluate(%+v, mcc %s) reason = %q, want %q", tt.features, tt.mcc, got.Reason, tt.wantReason)
			}
			if !got.Fired() && strings.TrimSpace(got.Reason) != "" {
				t.Errorf("Evaluate = %+v, want empty reason when not fired", got)
			}
		})
	}
}
