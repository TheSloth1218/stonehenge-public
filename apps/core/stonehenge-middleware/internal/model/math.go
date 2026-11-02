package model

import "math"

func NormalizeAzimuth(v float64) float64 {
	v = math.Mod(v, 360)
	if v < 0 {
		v += 360
	}
	if v >= 359.999999 {
		return 0
	}
	return v
}

func AngularDelta(a, b float64) float64 {
	a = NormalizeAzimuth(a)
	b = NormalizeAzimuth(b)
	diff := math.Abs(a - b)
	if diff > 180 {
		return 360 - diff
	}
	return diff
}

func SectorCovers(start, end, bearing float64) bool {
	start = NormalizeAzimuth(start)
	end = NormalizeAzimuth(end)
	bearing = NormalizeAzimuth(bearing)
	if start <= end {
		return bearing >= start && bearing <= end
	}
	return bearing >= start || bearing <= end
}
