package simulator

import (
	"math"
	"slices"
	"testing"
)

func TestDistanceKMMatchesKnownRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b   string
		wantKM float64
	}{
		{a: "BR", b: "PT", wantKM: 7930}, // São Paulo - Lisbon
		{a: "GB", b: "DE", wantKM: 930},  // London - Berlin
		{a: "BR", b: "BR", wantKM: 0},
	}
	for _, tt := range tests {
		a, _ := countryByCode(tt.a)
		b, _ := countryByCode(tt.b)
		if got := distanceKM(a, b); math.Abs(got-tt.wantKM) > tt.wantKM*0.05+1 {
			t.Errorf("distanceKM(%s, %s) = %.0f, want ~%.0f", tt.a, tt.b, got, tt.wantKM)
		}
	}
}

func TestCountriesFartherThan(t *testing.T) {
	t.Parallel()
	br, _ := countryByCode("BR")
	far := countriesFartherThan(br, 5000)
	if slices.ContainsFunc(far, func(c country) bool { return c.code == "BR" }) || len(far) == 0 {
		t.Errorf("countriesFartherThan(BR, 5000) = %v, want non-empty without BR", far)
	}
	if got := countriesFartherThan(br, math.MaxFloat64); len(got) != 1 || got[0].code != "JP" {
		t.Errorf("countriesFartherThan(BR, max) = %v, want only the farthest (JP)", got)
	}
}

func TestCatalogLookups(t *testing.T) {
	t.Parallel()
	if _, ok := countryByCode("XX"); ok {
		t.Error("countryByCode(XX) ok = true, want false")
	}
	if _, ok := categoryByMCC("0000"); ok {
		t.Error("categoryByMCC(0000) ok = true, want false")
	}
	for _, code := range fraudOrigins {
		if c, ok := countryByCode(code); !ok || c.homeWeight != 0 {
			t.Errorf("fraud origin %s: found=%v homeWeight=%d, want found with weight 0", code, ok, c.homeWeight)
		}
	}
}
