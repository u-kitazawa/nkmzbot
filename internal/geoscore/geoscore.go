package geoscore

import (
	"math"
)

const (
	// MinFiveKRadius is the minimum threshold for 5000-point radius in meters
	// This prevents extremely small radii on tight maps
	MinFiveKRadius = 25.0
)

// Haversine distance (meters) between two WGS84 lat/lng points (degrees).
func DistanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371008.8 // mean Earth radius (m)
	φ1 := lat1 * math.Pi / 180.0
	φ2 := lat2 * math.Pi / 180.0
	dφ := (lat2 - lat1) * math.Pi / 180.0
	dλ := (lng2 - lng1) * math.Pi / 180.0

	sinDφ := math.Sin(dφ / 2)
	sinDλ := math.Sin(dλ / 2)

	a := sinDφ*sinDφ + math.Cos(φ1)*math.Cos(φ2)*sinDλ*sinDλ
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// FiveKRadiusMeters returns the "5k radius" threshold in meters.
// Derived from: score=5000*exp(-10*d/maxErrorDistance) and rounding to 5000 at >=4999.5,
// and applying the common minimum threshold of 25m.
func FiveKRadiusMeters(maxErrorDistanceMeters float64) float64 {
	if maxErrorDistanceMeters <= 0 {
		return 0
	}
	// r = ln(5000/4999.5) * maxErrorDistance / 10
	r := math.Log(5000.0/4999.5) * maxErrorDistanceMeters / 10.0
	if r < MinFiveKRadius {
		r = MinFiveKRadius
	}
	return r
}

// GeoGuessrScore returns an integer score in [0, 5000].
// - trueLat/trueLng: correct location
// - guessLat/guessLng: player's guess
// - maxErrorDistanceMeters: map scale parameter (per-map)
func GeoGuessrScore(trueLat, trueLng, guessLat, guessLng, maxErrorDistanceMeters float64) int {
	if maxErrorDistanceMeters <= 0 {
		return 0
	}

	d := DistanceMeters(trueLat, trueLng, guessLat, guessLng)

	// If within 5k radius, return 5000.
	if d <= FiveKRadiusMeters(maxErrorDistanceMeters) {
		return 5000
	}

	// Base scoring (community-reverse-engineered).
	raw := 5000.0 * math.Exp(-10.0*d/maxErrorDistanceMeters)

	// GeoGuessr is commonly treated as rounding to nearest integer.
	score := int(math.Round(raw))

	if score < 0 {
		return 0
	}
	if score > 5000 {
		return 5000
	}
	return score
}

// MaxErrorDistanceFromBounds computes a map-scale parameter from a bounding box diagonal (meters).
// You can use this if you define "domestic map" as "within these bounds".
func MaxErrorDistanceFromBounds(swLat, swLng, neLat, neLng float64) float64 {
	return DistanceMeters(swLat, swLng, neLat, neLng)
}
