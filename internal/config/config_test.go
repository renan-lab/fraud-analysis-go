package config

import "testing"

func TestLoadSimulatorTargetURL(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{env: "", want: "http://localhost:8080"},
		{env: "http://app:8080", want: "http://app:8080"},
	}
	for _, tt := range tests {
		t.Setenv("SIMULATOR_TARGET_URL", tt.env)
		if got := Load().SimulatorTargetURL; got != tt.want {
			t.Errorf("Load() with SIMULATOR_TARGET_URL=%q: SimulatorTargetURL = %q, want %q", tt.env, got, tt.want)
		}
	}
}
