package simulator

import (
	"bytes"
	"context"
	"math/rand/v2"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestParseMix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		spec    string
		want    []ModeWeight
		wantErr bool
	}{
		{spec: "legit:90,fraud:10", want: []ModeWeight{{ModeLegit, 90}, {ModeFraud, 10}}},
		{spec: " attack:1 , legit:0", want: []ModeWeight{{ModeAttack, 1}, {ModeLegit, 0}}},
		{spec: DefaultMix, want: []ModeWeight{{ModeLegit, 90}, {ModeFraud, 5}, {ModeMalformed, 2}, {ModeAttack, 1}, {ModeDuplicate, 1}, {ModeTiming, 1}}},
		{spec: "", wantErr: true},
		{spec: "legit", wantErr: true},
		{spec: "chaos:1", wantErr: true},
		{spec: "legit:-1", wantErr: true},
		{spec: "legit:x", wantErr: true},
		{spec: "legit:1,legit:2", wantErr: true},
		{spec: "legit:0,fraud:0", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseMix(tt.spec)
		if (err != nil) != tt.wantErr || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseMix(%q) = %v, %v; want %v, wantErr %v", tt.spec, got, err, tt.want, tt.wantErr)
		}
	}
}

// modeScenario is a fake Scenario that emits one accepted emission of its mode.
type modeScenario struct{ mode Mode }

func (s modeScenario) Name() string        { return string(s.mode) + "/fake" }
func (s modeScenario) Mode() Mode          { return s.mode }
func (s modeScenario) Description() string { return "fake" }
func (s modeScenario) Next(*rand.Rand, time.Time) ([]Emission, error) {
	return []Emission{{Mode: s.mode, ExpectedStatus: http.StatusAccepted}}, nil
}

func TestGeneratorHonorsWeights(t *testing.T) {
	t.Parallel()
	scenarios := []Scenario{modeScenario{ModeLegit}, modeScenario{ModeFraud}}
	weights := []ModeWeight{{ModeLegit, 0}, {ModeFraud, 1}}
	history := newEmissionHistory(4)
	g, err := newGenerator(newTestRand(1), fixedClock{testNow}, weights, scenarios, history)
	if err != nil {
		t.Fatalf("newGenerator() error = %v", err)
	}
	for range 100 {
		got, err := g.Next()
		if err != nil || got[0].Mode != ModeFraud {
			t.Fatalf("Next() = %v, %v; want only fraud emissions", got, err)
		}
	}
	if len(history.items) == 0 {
		t.Error("history is empty, want accepted emissions recorded")
	}
}

func TestNewGeneratorRejectsWeightedModeWithoutScenarios(t *testing.T) {
	t.Parallel()
	weights := []ModeWeight{{ModeAttack, 1}}
	if _, err := newGenerator(newTestRand(1), fixedClock{testNow}, weights, []Scenario{modeScenario{ModeLegit}}, newEmissionHistory(1)); err == nil {
		t.Error("newGenerator(attack:1 without attack scenarios) error = nil, want error")
	}
}

func TestGeneratorIsDeterministicBySeed(t *testing.T) {
	t.Parallel()
	if a, b := generateNDJSON(t, 99), generateNDJSON(t, 99); !bytes.Equal(a, b) {
		t.Error("two runs with seed 99 produced different NDJSON")
	}
	if a, c := generateNDJSON(t, 99), generateNDJSON(t, 100); bytes.Equal(a, c) {
		t.Error("runs with seeds 99 and 100 produced identical NDJSON")
	}
}

// generateNDJSON renders 300 scenario runs of the default mix with a fixed clock.
func generateNDJSON(t *testing.T, seed uint64) []byte {
	t.Helper()
	r := newTestRand(seed)
	pop, err := NewPopulation(r, 100)
	if err != nil {
		t.Fatalf("NewPopulation() error = %v", err)
	}
	weights, _ := ParseMix(DefaultMix)
	history := newEmissionHistory(historySize)
	g, err := newGenerator(r, fixedClock{testNow}, weights, buildScenarios(pop, history, fixedClock{testNow}), history)
	if err != nil {
		t.Fatalf("newGenerator() error = %v", err)
	}
	var buf bytes.Buffer
	sender := NewNDJSONSender(&buf)
	for range 300 {
		emissions, err := g.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		for _, e := range emissions {
			if _, err := sender.Send(context.Background(), e); err != nil {
				t.Fatalf("Send() error = %v", err)
			}
		}
	}
	return buf.Bytes()
}
