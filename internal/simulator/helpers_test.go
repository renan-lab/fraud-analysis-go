package simulator

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/netip"
	"regexp"
	"slices"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

var testNow = time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func newTestRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed))
}

func newTestPopulation(t *testing.T, seed uint64, size int) *Population {
	t.Helper()
	pop, err := NewPopulation(newTestRand(seed), size)
	if err != nil {
		t.Fatalf("NewPopulation(%d) error = %v", size, err)
	}
	return pop
}

func (p *Population) customerByAccount(t *testing.T, accountID string) customer {
	t.Helper()
	i := slices.IndexFunc(p.customers, func(c customer) bool { return c.accountID == accountID })
	if i < 0 {
		t.Fatalf("no customer with account_id %q", accountID)
	}
	return p.customers[i]
}

func decodeTransaction(t *testing.T, e Emission) Transaction {
	t.Helper()
	var tx Transaction
	if err := json.Unmarshal(e.Body, &tx); err != nil {
		t.Fatalf("decode %s body %q: %v", e.Scenario, e.Body, err)
	}
	return tx
}

var (
	uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	currencyRe    = regexp.MustCompile(`^[A-Z]{3}$`)
	mccPattern    = regexp.MustCompile(`^[0-9]{4}$`)
	countryRe     = regexp.MustCompile(`^[A-Z]{2}$`)
	channels      = []Channel{ChannelCardPresent, ChannelEcommerce, ChannelContactless}
)

// contractStatus is a reference implementation of the ingestion contract in
// ARCHITECTURE.md, used to check that every scenario expects the right status.
func contractStatus(e Emission, now time.Time) int {
	if e.ContentType != contentTypeJSON {
		return http.StatusUnsupportedMediaType
	}
	if len(e.Body) > 64*1024 {
		return http.StatusRequestEntityTooLarge
	}
	var tx Transaction
	dec := json.NewDecoder(bytes.NewReader(e.Body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tx); err != nil || !satisfiesRules(tx, now) {
		return http.StatusBadRequest
	}
	return http.StatusAccepted
}

func satisfiesRules(tx Transaction, now time.Time) bool {
	_, ipErr := netip.ParseAddr(tx.IP)
	checks := []bool{
		uuidV4Pattern.MatchString(tx.TransactionID),
		idPattern.MatchString(tx.AccountID),
		idPattern.MatchString(tx.CardID),
		idPattern.MatchString(tx.DeviceID),
		idPattern.MatchString(tx.Merchant.ID),
		tx.Amount > 0,
		currencyRe.MatchString(tx.Currency),
		validMerchantName(tx.Merchant.Name),
		mccPattern.MatchString(tx.Merchant.MCC),
		countryRe.MatchString(tx.Merchant.Country),
		slices.Contains(channels, tx.Channel),
		ipErr == nil,
		!tx.OccurredAt.IsZero() && !tx.OccurredAt.After(now.Add(5*time.Minute)),
	}
	return !slices.Contains(checks, false)
}

func validMerchantName(name string) bool {
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 128 {
		return false
	}
	for _, r := range name {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
