package simulator

import (
	"math"
	"math/rand/v2"
	"slices"
)

// country holds the geographic and economic traits used to keep traffic plausible.
type country struct {
	code      string
	currency  string
	lat, lon  float64
	utcOffset int
	ipPrefix  string
	// typicalSpend is the median purchase in minor units of currency.
	typicalSpend float64
	// homeWeight is the share of customers living here; 0 means fraud origin only.
	homeWeight int
}

var countries = []country{
	{code: "BR", currency: "BRL", lat: -23.55, lon: -46.63, utcOffset: -3, ipPrefix: "177.", typicalSpend: 8000, homeWeight: 80},
	{code: "US", currency: "USD", lat: 40.71, lon: -74.01, utcOffset: -5, ipPrefix: "23.", typicalSpend: 4000, homeWeight: 6},
	{code: "PT", currency: "EUR", lat: 38.72, lon: -9.14, utcOffset: 0, ipPrefix: "85.", typicalSpend: 3500, homeWeight: 5},
	{code: "GB", currency: "GBP", lat: 51.51, lon: -0.13, utcOffset: 0, ipPrefix: "81.", typicalSpend: 3000, homeWeight: 3},
	{code: "DE", currency: "EUR", lat: 52.52, lon: 13.40, utcOffset: 1, ipPrefix: "91.", typicalSpend: 3500, homeWeight: 3},
	{code: "JP", currency: "JPY", lat: 35.68, lon: 139.69, utcOffset: 9, ipPrefix: "126.", typicalSpend: 4000, homeWeight: 3},
	{code: "NG", currency: "NGN", lat: 6.52, lon: 3.38, utcOffset: 1, ipPrefix: "105.", typicalSpend: 2000000},
	{code: "RU", currency: "RUB", lat: 55.75, lon: 37.62, utcOffset: 3, ipPrefix: "95.", typicalSpend: 300000},
}

// fraudOrigins are countries that only appear as the origin of fraudulent traffic.
var fraudOrigins = []string{"NG", "RU"}

// category is a merchant category code with its spending and channel profile.
type category struct {
	mcc          string
	label        string
	amountFactor float64
	channels     []Channel
	risky        bool
}

const (
	mccGrocery      = "5411"
	mccElectronics  = "5732"
	mccOnlineRetail = "5999"
)

var categories = []category{
	{mcc: mccGrocery, label: "Grocery", amountFactor: 1.0, channels: []Channel{ChannelCardPresent, ChannelContactless}},
	{mcc: "5812", label: "Restaurant", amountFactor: 0.6, channels: []Channel{ChannelCardPresent, ChannelContactless}},
	{mcc: "5814", label: "Fast Food", amountFactor: 0.3, channels: []Channel{ChannelContactless, ChannelCardPresent}},
	{mcc: "5541", label: "Gas Station", amountFactor: 0.8, channels: []Channel{ChannelCardPresent}},
	{mcc: "4121", label: "Ride Hailing", amountFactor: 0.3, channels: []Channel{ChannelEcommerce}},
	{mcc: "5311", label: "Department Store", amountFactor: 1.5, channels: []Channel{ChannelCardPresent, ChannelEcommerce}},
	{mcc: mccElectronics, label: "Electronics", amountFactor: 4.0, channels: []Channel{ChannelEcommerce, ChannelCardPresent}},
	{mcc: mccOnlineRetail, label: "Online Retail", amountFactor: 0.9, channels: []Channel{ChannelEcommerce}},
	{mcc: "7995", label: "Betting", amountFactor: 2.0, channels: []Channel{ChannelEcommerce}, risky: true},
	{mcc: "6051", label: "Crypto Exchange", amountFactor: 5.0, channels: []Channel{ChannelEcommerce}, risky: true},
}

func countryByCode(code string) (country, bool) {
	i := slices.IndexFunc(countries, func(c country) bool { return c.code == code })
	if i < 0 {
		return country{}, false
	}
	return countries[i], true
}

func categoryByMCC(mcc string) (category, bool) {
	i := slices.IndexFunc(categories, func(c category) bool { return c.mcc == mcc })
	if i < 0 {
		return category{}, false
	}
	return categories[i], true
}

// distanceKM is the great-circle distance between two countries' reference cities.
func distanceKM(a, b country) float64 {
	return haversineKM(a.lat, a.lon, b.lat, b.lon)
}

// haversineKM is the great-circle distance between two points in decimal degrees.
func haversineKM(aLat, aLon, bLat, bLon float64) float64 {
	const earthRadiusKM = 6371.0
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat := rad(bLat - aLat)
	dLon := rad(bLon - aLon)
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(rad(aLat))*math.Cos(rad(bLat))*math.Pow(math.Sin(dLon/2), 2)
	return 2 * earthRadiusKM * math.Asin(math.Sqrt(h))
}

// maxJitterDegrees spreads locations around a country's reference city (about 30 km).
const maxJitterDegrees = 0.25

// nearbyLocation returns a point near the country's reference city, rounded
// to 4 decimals (about 11 m) like a typical device or terminal fix.
func nearbyLocation(r *rand.Rand, c country) *Location {
	jitter := func() float64 { return (2*r.Float64() - 1) * maxJitterDegrees }
	round := func(deg float64) float64 { return math.Round(deg*1e4) / 1e4 }
	return &Location{Lat: round(c.lat + jitter()), Lon: round(c.lon + jitter())}
}

// countriesFartherThan returns countries at least minKM away from home, or the farthest one.
func countriesFartherThan(home country, minKM float64) []country {
	var far []country
	farthest := home
	for _, c := range countries {
		if distanceKM(home, c) >= minKM {
			far = append(far, c)
		}
		if distanceKM(home, c) > distanceKM(home, farthest) {
			farthest = c
		}
	}
	if len(far) == 0 {
		return []country{farthest}
	}
	return far
}
