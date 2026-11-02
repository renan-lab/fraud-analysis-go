package analysis

import (
	"math"
	"testing"
)

func TestHaversineKM(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		a, b   Location
		wantKM float64
		tolKM  float64
	}{
		{name: "same point", a: saoPaulo, b: saoPaulo, wantKM: 0, tolKM: 0.001},
		{name: "sao paulo to campinas", a: saoPaulo, b: campinas, wantKM: 84, tolKM: 3},
		{name: "sao paulo to lisbon", a: saoPaulo, b: lisbon, wantKM: 7930, tolKM: 30},
		{name: "antipodes", a: Location{Lat: 0, Lon: 0}, b: Location{Lat: 0, Lon: 180}, wantKM: math.Pi * earthRadiusKM, tolKM: 0.001},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := haversineKM(tt.a, tt.b)
			if math.Abs(got-tt.wantKM) > tt.tolKM {
				t.Errorf("haversineKM(%+v, %+v) = %.1f km, want %.1f +/- %.1f", tt.a, tt.b, got, tt.wantKM, tt.tolKM)
			}
			if back := haversineKM(tt.b, tt.a); math.Abs(back-got) > 1e-9 {
				t.Errorf("haversineKM not symmetric: %v vs %v", got, back)
			}
		})
	}
}
