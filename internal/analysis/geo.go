package analysis

import "math"

const earthRadiusKM = 6371.0

// haversineKM is the great-circle distance between a and b in kilometers.
//
//	haversineKM(Location{Lat: -23.55, Lon: -46.63}, Location{Lat: 40.71, Lon: -74.01}) // ~7680
func haversineKM(a, b Location) float64 {
	lat1, lat2 := radians(a.Lat), radians(b.Lat)
	dLat := lat2 - lat1
	dLon := radians(b.Lon - a.Lon)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKM * math.Asin(math.Min(1, math.Sqrt(h)))
}

func radians(deg float64) float64 {
	return deg * math.Pi / 180
}
