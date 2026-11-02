package simulator

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

// fieldMutation rewrites the JSON object of a valid transaction, one key at a time.
type fieldMutation func(r *rand.Rand, fields map[string]json.RawMessage)

var requiredFields = []string{
	"transaction_id", "account_id", "card_id", "amount", "currency",
	"merchant", "channel", "device_id", "ip", "occurred_at",
}

func malformedScenarios(pop *Population) []Scenario {
	return []Scenario{
		bodySpec(pop, "malformed/invalid_json", "truncated JSON document; expects 400", truncateBody),
		bodySpec(pop, "malformed/empty_body", "empty request body; expects 400", func([]byte) []byte { return []byte{} }),
		mutationSpec(pop, ModeMalformed, "malformed/missing_field", "one required field removed; expects 400", http.StatusBadRequest, removeRandomField),
		mutationSpec(pop, ModeMalformed, "malformed/amount_as_string", `amount sent as "129.90"; expects 400`, http.StatusBadRequest, setField("amount", `"129.90"`)),
		mutationSpec(pop, ModeMalformed, "malformed/amount_as_float", "amount sent as float 129.9 (money must be integer minor units); expects 400", http.StatusBadRequest, setField("amount", `129.9`)),
		mutationSpec(pop, ModeMalformed, "malformed/non_positive_amount", "amount zero or negative; expects 400", http.StatusBadRequest, nonPositiveAmount),
		mutationSpec(pop, ModeMalformed, "malformed/invalid_currency", "currency not ISO 4217; expects 400", http.StatusBadRequest, setField("currency", `"REAL"`)),
		mutationSpec(pop, ModeMalformed, "malformed/invalid_timestamp", "occurred_at not RFC 3339; expects 400", http.StatusBadRequest, setField("occurred_at", `"02/11/2026 19:15"`)),
		mutationSpec(pop, ModeMalformed, "malformed/invalid_transaction_id", "transaction_id is not a UUID; expects 400", http.StatusBadRequest, setField("transaction_id", `"tx-123"`)),
		mutationSpec(pop, ModeMalformed, "malformed/invalid_channel", "channel outside the allowed enum; expects 400", http.StatusBadRequest, setField("channel", `"telepathy"`)),
		mutationSpec(pop, ModeMalformed, "malformed/null_merchant", "merchant is null; expects 400", http.StatusBadRequest, setField("merchant", `null`)),
	}
}

// bodySpec builds a scenario that rewrites the raw bytes of a valid payload.
func bodySpec(pop *Population, name, description string, rewrite func([]byte) []byte) Scenario {
	return scenarioSpec{
		name:        name,
		mode:        ModeMalformed,
		description: description,
		next: func(r *rand.Rand, now time.Time) ([]Emission, error) {
			tx := pop.legitPurchase(r, pop.randomCustomer(r), now)
			body, err := json.Marshal(tx)
			if err != nil {
				return nil, fmt.Errorf("encode base transaction for %s: %w", name, err)
			}
			return []Emission{rejectedEmission(ModeMalformed, name, "", http.StatusBadRequest, rewrite(body))}, nil
		},
	}
}

// mutationSpec builds a scenario that edits fields of a valid payload.
func mutationSpec(pop *Population, mode Mode, name, description string, status int, mutate fieldMutation) Scenario {
	return scenarioSpec{
		name:        name,
		mode:        mode,
		description: description,
		next: func(r *rand.Rand, now time.Time) ([]Emission, error) {
			tx := pop.legitPurchase(r, pop.randomCustomer(r), now)
			body, err := mutateTransaction(r, tx, mutate)
			if err != nil {
				return nil, fmt.Errorf("mutate transaction for %s: %w", name, err)
			}
			return []Emission{rejectedEmission(mode, name, tx.TransactionID, status, body)}, nil
		},
	}
}

func mutateTransaction(r *rand.Rand, tx Transaction, mutate fieldMutation) ([]byte, error) {
	raw, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	mutate(r, fields)
	return json.Marshal(fields)
}

func truncateBody(body []byte) []byte {
	return body[:len(body)/2]
}

func setField(key, rawJSON string) fieldMutation {
	return func(_ *rand.Rand, fields map[string]json.RawMessage) {
		fields[key] = json.RawMessage(rawJSON)
	}
}

func removeRandomField(r *rand.Rand, fields map[string]json.RawMessage) {
	delete(fields, requiredFields[r.IntN(len(requiredFields))])
}

func nonPositiveAmount(r *rand.Rand, fields map[string]json.RawMessage) {
	fields["amount"] = json.RawMessage(fmt.Sprintf("%d", -r.IntN(10000)))
}
