package simulator

import (
	"fmt"
	"time"
)

// Clock is the source of occurred_at timestamps.
type Clock interface {
	Now() time.Time
}

// WallClock returns the real current time in UTC.
type WallClock struct{}

// Now returns time.Now in UTC.
func (WallClock) Now() time.Time {
	return time.Now().UTC()
}

// AcceleratedClock replays simulated time faster than wall time, starting
// lookback in the past. It never runs ahead of wall time, because the ingestion
// API rejects events more than 5 minutes in the future.
type AcceleratedClock struct {
	wall      func() time.Time
	wallStart time.Time
	origin    time.Time
	scale     float64
}

// NewAcceleratedClock makes one wall second worth scale simulated seconds.
//
//	clock, err := NewAcceleratedClock(time.Now, 1440, 24*time.Hour) // one day per wall minute
func NewAcceleratedClock(wall func() time.Time, scale float64, lookback time.Duration) (*AcceleratedClock, error) {
	if scale < 1 {
		return nil, fmt.Errorf("time scale must be >= 1, got %v", scale)
	}
	if lookback < 0 {
		return nil, fmt.Errorf("lookback must be >= 0, got %v", lookback)
	}
	start := wall()
	return &AcceleratedClock{wall: wall, wallStart: start, origin: start.Add(-lookback), scale: scale}, nil
}

// Now returns the simulated time, capped at the wall time.
func (c *AcceleratedClock) Now() time.Time {
	w := c.wall().UTC()
	elapsed := time.Duration(float64(w.Sub(c.wallStart)) * c.scale)
	sim := c.origin.Add(elapsed).UTC()
	if sim.After(w) {
		return w
	}
	return sim
}

// NewClock returns WallClock for scale 1 and an AcceleratedClock otherwise.
//
//	clock, err := NewClock(60, time.Hour)
func NewClock(scale float64, lookback time.Duration) (Clock, error) {
	if scale == 1 {
		return WallClock{}, nil
	}
	return NewAcceleratedClock(time.Now, scale, lookback)
}
