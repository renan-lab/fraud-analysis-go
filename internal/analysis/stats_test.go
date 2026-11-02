package analysis

import (
	"slices"
	"testing"
)

func TestMedian(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   []int64
		want float64
	}{
		{in: nil, want: 0},
		{in: []int64{700}, want: 700},
		{in: []int64{300, 100, 200}, want: 200},
		{in: []int64{400, 100, 300, 200}, want: 250},
		{in: []int64{100, 100, 100, 90000}, want: 100},
	}
	for _, tt := range tests {
		in := slices.Clone(tt.in)
		if got := median(in); got != tt.want {
			t.Errorf("median(%v) = %v, want %v", tt.in, got, tt.want)
		}
		if !slices.Equal(in, tt.in) {
			t.Errorf("median(%v) modified its input to %v", tt.in, in)
		}
	}
}
