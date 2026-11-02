package simulator

import (
	"testing"
	"time"
)

// steppingWall is a fake wall clock advanced by the test.
type steppingWall struct{ now time.Time }

func (w *steppingWall) Now() time.Time { return w.now }

func TestAcceleratedClockAdvancesByScaleAndCapsAtWall(t *testing.T) {
	t.Parallel()
	wall := &steppingWall{now: testNow}
	clock, err := NewAcceleratedClock(wall.Now, 60, time.Hour)
	if err != nil {
		t.Fatalf("NewAcceleratedClock() error = %v", err)
	}
	tests := []struct {
		wallElapsed time.Duration
		want        time.Time
	}{
		{wallElapsed: 0, want: testNow.Add(-time.Hour)},
		{wallElapsed: 30 * time.Second, want: testNow.Add(-30 * time.Minute)},
		{wallElapsed: 2 * time.Minute, want: testNow.Add(2 * time.Minute)},
	}
	for _, tt := range tests {
		wall.now = testNow.Add(tt.wallElapsed)
		if got := clock.Now(); !got.Equal(tt.want) {
			t.Errorf("Now() after %v wall = %v, want %v", tt.wallElapsed, got, tt.want)
		}
	}
}

func TestNewClockValidatesArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		scale    float64
		lookback time.Duration
		wantErr  bool
	}{
		{scale: 1, lookback: time.Hour},
		{scale: 60, lookback: time.Hour},
		{scale: 0.5, lookback: time.Hour, wantErr: true},
		{scale: 60, lookback: -time.Hour, wantErr: true},
	}
	for _, tt := range tests {
		_, err := NewClock(tt.scale, tt.lookback)
		if (err != nil) != tt.wantErr {
			t.Errorf("NewClock(%v, %v) error = %v, wantErr %v", tt.scale, tt.lookback, err, tt.wantErr)
		}
	}
}

func TestNewClockScaleOneIsWallClock(t *testing.T) {
	t.Parallel()
	clock, err := NewClock(1, 0)
	if _, ok := clock.(WallClock); err != nil || !ok {
		t.Errorf("NewClock(1, 0) = %T, %v; want WallClock, nil", clock, err)
	}
	if got := (WallClock{}).Now(); got.Location() != time.UTC {
		t.Errorf("WallClock.Now() location = %v, want UTC", got.Location())
	}
}
