package analysis

import (
	"strconv"
	"strings"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/redis"
)

// Every key starts with "analysis:": this module owns that namespace (ADR 0009).
// Changing a member format below needs a migration or a new key prefix.

func cardHistoryKey(cardID string) string {
	return "analysis:card:" + cardID + ":history"
}

func accountAmountsKey(accountID string) string {
	return "analysis:account:" + accountID + ":amounts"
}

func devicesKey(accountID string) string {
	return "analysis:account:" + accountID + ":devices"
}

func countriesKey(accountID string) string {
	return "analysis:account:" + accountID + ":countries"
}

// encodeCardEvent writes "txid|amount|channel|lat|lon"; lat and lon are empty
// when the location is unknown. IDs and channels never contain "|".
func encodeCardEvent(tx Transaction) string {
	lat, lon := "", ""
	if tx.Location != nil {
		lat = strconv.FormatFloat(tx.Location.Lat, 'f', -1, 64)
		lon = strconv.FormatFloat(tx.Location.Lon, 'f', -1, 64)
	}
	return strings.Join([]string{tx.ID, strconv.FormatInt(tx.Amount, 10), tx.Channel, lat, lon}, "|")
}

// decodeCardEvent parses a member written by encodeCardEvent. Malformed
// members are reported as not ok and skipped by the caller: they can only come
// from a format change, and skipping degrades to less history instead of
// blocking the partition with a message that can never succeed.
func decodeCardEvent(m redis.ZMember) (CardEvent, bool) {
	parts := strings.Split(m.Member, "|")
	if len(parts) != 5 || parts[0] == "" {
		return CardEvent{}, false
	}
	amount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return CardEvent{}, false
	}
	loc, ok := decodeLocation(parts[3], parts[4])
	if !ok {
		return CardEvent{}, false
	}
	at := time.UnixMilli(int64(m.Score)).UTC()
	return CardEvent{TransactionID: parts[0], At: at, Amount: amount, Channel: parts[2], Location: loc}, true
}

func decodeLocation(lat, lon string) (*Location, bool) {
	if lat == "" && lon == "" {
		return nil, true
	}
	la, errLat := strconv.ParseFloat(lat, 64)
	lo, errLon := strconv.ParseFloat(lon, 64)
	if errLat != nil || errLon != nil {
		return nil, false
	}
	return &Location{Lat: la, Lon: lo}, true
}

// encodeAmount writes "txid|amount" so equal amounts of different
// transactions stay distinct members.
func encodeAmount(tx Transaction) string {
	return tx.ID + "|" + strconv.FormatInt(tx.Amount, 10)
}

func decodeAmount(member string) (string, int64, bool) {
	id, raw, found := strings.Cut(member, "|")
	if !found || id == "" {
		return "", 0, false
	}
	amount, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return "", 0, false
	}
	return id, amount, true
}
