package analysis

import "slices"

// median returns the middle value of amounts (mean of the two middle values
// for an even count), or 0 for an empty slice. amounts is not modified.
//
//	median([]int64{100, 300, 200}) // 200
func median(amounts []int64) float64 {
	n := len(amounts)
	if n == 0 {
		return 0
	}
	sorted := slices.Clone(amounts)
	slices.Sort(sorted)
	if n%2 == 1 {
		return float64(sorted[n/2])
	}
	return (float64(sorted[n/2-1]) + float64(sorted[n/2])) / 2
}
