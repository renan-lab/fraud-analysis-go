package ingestion

import (
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"time"
	"unicode"
	"unicode/utf8"
)

// maxClockSkew is how far in the future occurred_at may be, relative to the API clock.
const maxClockSkew = 5 * time.Minute

// maxEchoedRunes bounds how much of an offending value is echoed back in errors.
const maxEchoedRunes = 64

var (
	uuidV4Pattern  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	idPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	currencyRegexp = regexp.MustCompile(`^[A-Z]{3}$`)
	mccPattern     = regexp.MustCompile(`^[0-9]{4}$`)
	countryRegexp  = regexp.MustCompile(`^[A-Z]{2}$`)

	idRule = matches(idPattern, "must match ^[A-Za-z0-9_-]{1,64}$")
)

// FieldError is one violated contract rule. Field is the JSON path, e.g. "merchant.mcc".
type FieldError struct {
	Field  string
	Detail string
}

// stringRule returns "" when v is valid, or a detail that names the expected shape.
type stringRule func(v string) string

// fieldValidator collects every violation instead of stopping at the first,
// so a client sees all problems of a payload in one response.
type fieldValidator struct {
	errs []FieldError
}

func (fv *fieldValidator) fail(field, format string, args ...any) {
	fv.errs = append(fv.errs, FieldError{Field: field, Detail: fmt.Sprintf(format, args...)})
}

func (fv *fieldValidator) str(field string, v *string, rule stringRule) string {
	if v == nil {
		fv.fail(field, "is required")
		return ""
	}
	if detail := rule(*v); detail != "" {
		fv.fail(field, "%q %s", echo(*v), detail)
	}
	return *v
}

func (fv *fieldValidator) amount(v *int64) int64 {
	if v == nil {
		fv.fail("amount", "is required")
		return 0
	}
	if *v <= 0 {
		fv.fail("amount", "%d must be an integer > 0 in minor units", *v)
	}
	return *v
}

func (fv *fieldValidator) occurredAt(v *string, now time.Time) time.Time {
	if v == nil {
		fv.fail("occurred_at", "is required")
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, *v)
	if err != nil {
		fv.fail("occurred_at", "%q must be an RFC 3339 timestamp like 2026-11-02T19:15:00Z", echo(*v))
		return time.Time{}
	}
	if t.After(now.Add(maxClockSkew)) {
		fv.fail("occurred_at", "%s is more than %s ahead of server time %s", *v, maxClockSkew, now.Format(time.RFC3339))
	}
	return t.UTC()
}

func (fv *fieldValidator) merchant(m *merchantRequest) Merchant {
	if m == nil {
		fv.fail("merchant", "is required")
		return Merchant{}
	}
	return Merchant{
		ID:      fv.str("merchant.id", m.ID, idRule),
		Name:    fv.str("merchant.name", m.Name, merchantName),
		MCC:     fv.str("merchant.mcc", m.MCC, matches(mccPattern, "must be a 4-digit MCC")),
		Country: fv.str("merchant.country", m.Country, matches(countryRegexp, "must be an ISO 3166-1 alpha-2 code like BR")),
	}
}

// toTransaction applies every rule of the contract in ARCHITECTURE.md.
func (req transactionRequest) toTransaction(now time.Time) (Transaction, []FieldError) {
	var fv fieldValidator
	tx := Transaction{
		TransactionID: fv.str("transaction_id", req.TransactionID, matches(uuidV4Pattern, "must be a lowercase UUID v4")),
		AccountID:     fv.str("account_id", req.AccountID, idRule),
		CardID:        fv.str("card_id", req.CardID, idRule),
		Amount:        fv.amount(req.Amount),
		Currency:      fv.str("currency", req.Currency, matches(currencyRegexp, "must be an ISO 4217 code like BRL")),
		Merchant:      fv.merchant(req.Merchant),
		Channel:       Channel(fv.str("channel", req.Channel, channel)),
		DeviceID:      fv.str("device_id", req.DeviceID, idRule),
		IP:            fv.str("ip", req.IP, ipLiteral),
		OccurredAt:    fv.occurredAt(req.OccurredAt, now),
	}
	return tx, fv.errs
}

func matches(re *regexp.Regexp, detail string) stringRule {
	return func(v string) string {
		if re.MatchString(v) {
			return ""
		}
		return detail
	}
}

func channel(v string) string {
	if slices.Contains(channels, Channel(v)) {
		return ""
	}
	return fmt.Sprintf("must be one of %v", channels)
}

func ipLiteral(v string) string {
	if _, err := netip.ParseAddr(v); err != nil {
		return "must be an IPv4 or IPv6 literal"
	}
	return ""
}

// merchantName rejects control and format characters (NUL, ANSI escapes,
// right-to-left override), which unicode.IsPrint excludes.
func merchantName(v string) string {
	const detail = "must be 1-128 printable characters"
	if n := utf8.RuneCountInString(v); n < 1 || n > 128 || !utf8.ValidString(v) {
		return detail
	}
	for _, r := range v {
		if !unicode.IsPrint(r) {
			return detail
		}
	}
	return ""
}

// echo truncates v so error bodies stay small even for hostile input.
func echo(v string) string {
	if utf8.RuneCountInString(v) <= maxEchoedRunes {
		return v
	}
	return string([]rune(v)[:maxEchoedRunes]) + "..."
}
