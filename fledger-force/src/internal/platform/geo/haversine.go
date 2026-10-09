// Package geo implements the Haversine distance formula for geofence
// verification (Sprint 2 of the Fledger Force brief).
//
// Reference: https://en.wikipedia.org/wiki/Haversine_formula
// The Earth radius R is taken as 6,371,000 m.
package geo

import "math"

// EarthRadiusMeters is the mean Earth radius in meters.
const EarthRadiusMeters = 6_371_000.0

// DistanceMeters returns the great-circle distance between two (lat, lng)
// points in meters using the Haversine formula.
func DistanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	phi1 := toRad(lat1)
	phi2 := toRad(lat2)
	dPhi := toRad(lat2 - lat1)
	dLambda := toRad(lng2 - lng1)

	sinDPhi := math.Sin(dPhi / 2)
	sinDLambda := math.Sin(dLambda / 2)

	a := sinDPhi*sinDPhi +
		math.Cos(phi1)*math.Cos(phi2)*sinDLambda*sinDLambda
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return EarthRadiusMeters * c
}

// WithinRadius returns true if the distance between the two points is <=
// radiusMeters. It returns the distance too for convenience.
func WithinRadius(lat1, lng1, lat2, lng2 float64, radiusMeters int) (bool, float64) {
	d := DistanceMeters(lat1, lng1, lat2, lng2)
	return d <= float64(radiusMeters), d
}

func toRad(deg float64) float64 { return deg * math.Pi / 180 }