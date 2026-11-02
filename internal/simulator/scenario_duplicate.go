package simulator

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

const historySize = 256

// emissionHistory is a ring buffer of recently accepted emissions that
// duplicate scenarios replay. It is used only by the generator goroutine.
type emissionHistory struct {
	items []Emission
	next  int
}

func newEmissionHistory(size int) *emissionHistory {
	return &emissionHistory{items: make([]Emission, 0, size)}
}

// record keeps only emissions the API is expected to accept.
func (h *emissionHistory) record(e Emission) {
	if e.ExpectedStatus != http.StatusAccepted {
		return
	}
	if len(h.items) < cap(h.items) {
		h.items = append(h.items, e)
		return
	}
	h.items[h.next] = e
	h.next = (h.next + 1) % len(h.items)
}

func (h *emissionHistory) pick(r *rand.Rand) (Emission, bool) {
	if len(h.items) == 0 {
		return Emission{}, false
	}
	return h.items[r.IntN(len(h.items))], true
}

// replay turns an earlier emission into a duplicate.
type replay func(r *rand.Rand, original Emission) (Emission, error)

func duplicateScenarios(pop *Population, history *emissionHistory) []Scenario {
	return []Scenario{
		duplicateSpec(pop, history, "duplicate/exact_replay", "byte-identical resend of an accepted transaction (same transaction_id); expects 202", exactReplay),
		duplicateSpec(pop, history, "duplicate/same_content_new_id", "same purchase resent with a new transaction_id (double charge); expects 202", newIDReplay),
		duplicateSpec(pop, history, "duplicate/concurrent_retry", "3-5 identical copies sent concurrently (client retry storm); expects 202", concurrentReplay),
	}
}

// duplicateSpec replays a past emission; with an empty history it emits a fresh
// legit purchase first so the duplicate always has an original.
func duplicateSpec(pop *Population, history *emissionHistory, name, description string, dup replay) Scenario {
	return scenarioSpec{
		name:        name,
		mode:        ModeDuplicate,
		description: description,
		next: func(r *rand.Rand, now time.Time) ([]Emission, error) {
			if original, ok := history.pick(r); ok {
				return single(asDuplicate(r, name, original, dup))
			}
			original, err := acceptedEmission(ModeLegit, "legit/purchase", LabelLegit, pop.legitPurchase(r, pop.randomCustomer(r), now))
			if err != nil {
				return nil, err
			}
			e, err := asDuplicate(r, name, original, dup)
			return []Emission{original, e}, err
		},
	}
}

func asDuplicate(r *rand.Rand, name string, original Emission, dup replay) (Emission, error) {
	e, err := dup(r, original)
	if err != nil {
		return Emission{}, fmt.Errorf("%s: %w", name, err)
	}
	e.Mode = ModeDuplicate
	e.Scenario = name
	return e, nil
}

func exactReplay(_ *rand.Rand, original Emission) (Emission, error) {
	original.Copies = 0
	return original, nil
}

func concurrentReplay(r *rand.Rand, original Emission) (Emission, error) {
	original.Copies = 3 + r.IntN(3)
	return original, nil
}

func newIDReplay(r *rand.Rand, original Emission) (Emission, error) {
	var tx Transaction
	if err := json.Unmarshal(original.Body, &tx); err != nil {
		return Emission{}, fmt.Errorf("decode original %s: %w", original.TransactionID, err)
	}
	tx.TransactionID = newUUID(r)
	return acceptedEmission(ModeDuplicate, "", original.Label, tx)
}
