package simulator

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

const (
	oversizedBodyBytes = 128 * 1024
	deepNestingLevels  = 5000
)

func attackScenarios(pop *Population) []Scenario {
	return []Scenario{
		mutationSpec(pop, ModeAttack, "attack/sql_injection", "SQL injection in account_id; expects 400", http.StatusBadRequest,
			setField("account_id", `"acc_1' OR '1'='1'; DROP TABLE alerts;--"`)),
		mutationSpec(pop, ModeAttack, "attack/xss", "script tag in merchant.id; expects 400", http.StatusBadRequest,
			setMerchantField("id", "<script>alert(document.cookie)</script>")),
		mutationSpec(pop, ModeAttack, "attack/control_chars", "NUL, ANSI escape and right-to-left override in merchant.name; expects 400", http.StatusBadRequest,
			setMerchantField("name", "Shop\x00\x1b[31m\u202egnp.exe")),
		mutationSpec(pop, ModeAttack, "attack/command_injection", "shell command in merchant.mcc; expects 400", http.StatusBadRequest,
			setMerchantField("mcc", "5411; rm -rf /")),
		mutationSpec(pop, ModeAttack, "attack/oversized_body", "128 KiB merchant.name, above the 64 KiB body limit; expects 413", http.StatusRequestEntityTooLarge,
			setMerchantField("name", strings.Repeat("A", oversizedBodyBytes))),
		mutationSpec(pop, ModeAttack, "attack/deep_nesting", "merchant nested 5000 levels deep; expects 400", http.StatusBadRequest,
			setField("merchant", deeplyNested(deepNestingLevels))),
		mutationSpec(pop, ModeAttack, "attack/extra_fields", "mass assignment: is_admin and decision fields injected; expects 400", http.StatusBadRequest,
			addExtraFields),
		mutationSpec(pop, ModeAttack, "attack/int_overflow", "amount larger than int64; expects 400", http.StatusBadRequest,
			setField("amount", "99999999999999999999999")),
		wrongContentTypeSpec(pop),
	}
}

// setMerchantField replaces one merchant field, JSON-escaping the hostile value.
func setMerchantField(key, value string) fieldMutation {
	return func(_ *rand.Rand, fields map[string]json.RawMessage) {
		var merchant map[string]string
		if err := json.Unmarshal(fields["merchant"], &merchant); err != nil {
			return
		}
		merchant[key] = value
		if raw, err := json.Marshal(merchant); err == nil {
			fields["merchant"] = raw
		}
	}
}

func addExtraFields(_ *rand.Rand, fields map[string]json.RawMessage) {
	fields["is_admin"] = json.RawMessage(`true`)
	fields["decision"] = json.RawMessage(`"approve"`)
}

func deeplyNested(levels int) string {
	return strings.Repeat(`{"x":`, levels) + "1" + strings.Repeat("}", levels)
}

func wrongContentTypeSpec(pop *Population) Scenario {
	const name = "attack/wrong_content_type"
	return scenarioSpec{
		name:        name,
		mode:        ModeAttack,
		description: "valid body sent as text/plain; expects 415",
		next: func(r *rand.Rand, now time.Time) ([]Emission, error) {
			tx := pop.legitPurchase(r, pop.randomCustomer(r), now)
			body, err := json.Marshal(tx)
			if err != nil {
				return nil, fmt.Errorf("encode base transaction for %s: %w", name, err)
			}
			e := rejectedEmission(ModeAttack, name, tx.TransactionID, http.StatusUnsupportedMediaType, body)
			e.ContentType = "text/plain"
			return []Emission{e}, nil
		},
	}
}
